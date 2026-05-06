package configloader_test

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	configloader "github.com/jmcampanini/go-config-loader"
)

func TestNewFileHelperValidation(t *testing.T) {
	helper, err := configloader.NewFileHelper("my-cli", "config.toml")
	if err != nil {
		t.Fatalf("NewFileHelper() error = %v", err)
	}
	if helper.AppName != "my-cli" || helper.ConfigFileName != "config.toml" {
		t.Fatalf("NewFileHelper() = %#v", helper)
	}

	for _, tt := range []struct {
		name           string
		appName        string
		configFileName string
	}{
		{name: "empty app", appName: "", configFileName: "config.toml"},
		{name: "uppercase app", appName: "My_CLI", configFileName: "config.toml"},
		{name: "empty file", appName: "my-cli", configFileName: ""},
		{name: "slash in file", appName: "my-cli", configFileName: "dir/config.toml"},
		{name: "backslash in file", appName: "my-cli", configFileName: `dir\config.toml`},
		{name: "current directory", appName: "my-cli", configFileName: "."},
		{name: "parent directory", appName: "my-cli", configFileName: ".."},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := configloader.NewFileHelper(tt.appName, tt.configFileName); err == nil {
				t.Fatalf("NewFileHelper(%q, %q) error = nil", tt.appName, tt.configFileName)
			}
		})
	}
}

func TestFileHelperCwdHomeHiddenAndBothVariants(t *testing.T) {
	home := t.TempDir()
	cwd := filepath.Join(t.TempDir(), "work")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	chdir(t, cwd)
	cwd = mustGetwd(t)
	t.Setenv("HOME", home)

	helper := mustFileHelper(t)

	assertStrings(t, "Cwd", helper.Cwd(), []string{filepath.Join(cwd, "config.toml")})
	assertStrings(t, "CwdHidden", helper.CwdHidden(), []string{filepath.Join(cwd, ".config.toml")})
	assertStrings(t, "CwdBoth", helper.CwdBoth(), []string{
		filepath.Join(cwd, "config.toml"),
		filepath.Join(cwd, ".config.toml"),
	})
	assertStrings(t, "Home", helper.Home(), []string{filepath.Join(home, "config.toml")})
	assertStrings(t, "HomeHidden", helper.HomeHidden(), []string{filepath.Join(home, ".config.toml")})
	assertStrings(t, "HomeBoth", helper.HomeBoth(), []string{
		filepath.Join(home, "config.toml"),
		filepath.Join(home, ".config.toml"),
	})

	for name, paths := range map[string][]string{
		"Cwd":        helper.Cwd(),
		"CwdHidden":  helper.CwdHidden(),
		"CwdBoth":    helper.CwdBoth(),
		"Home":       helper.Home(),
		"HomeHidden": helper.HomeHidden(),
		"HomeBoth":   helper.HomeBoth(),
	} {
		for _, path := range paths {
			if !filepath.IsAbs(path) {
				t.Fatalf("%s returned non-absolute path %q", name, path)
			}
		}
	}
}

func TestFileHelperXDGConfigFile(t *testing.T) {
	home := t.TempDir()
	xdg := filepath.Join(t.TempDir(), "xdg-config")
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)

	helper := mustFileHelper(t)
	assertStrings(t, "XDGConfigFile with env", helper.XDGConfigFile(), []string{
		filepath.Join(xdg, "my-cli", "config.toml"),
	})

	t.Setenv("XDG_CONFIG_HOME", "")
	assertStrings(t, "XDGConfigFile default", helper.XDGConfigFile(), []string{
		filepath.Join(home, ".config", "my-cli", "config.toml"),
	})
}

func TestFileHelperWalkCwdToHomeInsideHome(t *testing.T) {
	homeRoot := t.TempDir()
	cwd := filepath.Join(homeRoot, "projects", "app")
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	chdir(t, cwd)
	actualCWD := mustGetwd(t)
	home := filepath.Dir(filepath.Dir(actualCWD))
	t.Setenv("HOME", home)

	helper := mustFileHelper(t)
	assertStrings(t, "WalkCwdToHome", helper.WalkCwdToHome(), []string{
		filepath.Join(home, "config.toml"),
		filepath.Join(home, "projects", "config.toml"),
		filepath.Join(home, "projects", "app", "config.toml"),
	})
	assertStrings(t, "WalkCwdToHomeHidden", helper.WalkCwdToHomeHidden(), []string{
		filepath.Join(home, ".config.toml"),
		filepath.Join(home, "projects", ".config.toml"),
		filepath.Join(home, "projects", "app", ".config.toml"),
	})
	assertStrings(t, "WalkCwdToHomeBoth", helper.WalkCwdToHomeBoth(), []string{
		filepath.Join(home, "config.toml"),
		filepath.Join(home, ".config.toml"),
		filepath.Join(home, "projects", "config.toml"),
		filepath.Join(home, "projects", ".config.toml"),
		filepath.Join(home, "projects", "app", "config.toml"),
		filepath.Join(home, "projects", "app", ".config.toml"),
	})
}

func TestFileHelperWalkCwdToHomeOutsideHome(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	cwd := filepath.Join(t.TempDir(), "outside", "project")
	if err := os.MkdirAll(home, 0o755); err != nil {
		t.Fatalf("MkdirAll(home) error = %v", err)
	}
	if err := os.MkdirAll(cwd, 0o755); err != nil {
		t.Fatalf("MkdirAll(cwd) error = %v", err)
	}
	chdir(t, cwd)
	actualCWD := mustGetwd(t)
	t.Setenv("HOME", home)

	helper := mustFileHelper(t)
	want := appendFileName(dirsFromRoot(actualCWD), "config.toml")
	got := helper.WalkCwdToHome()
	assertStrings(t, "WalkCwdToHome outside home", got, want)
	if len(got) == 0 || got[len(got)-1] != filepath.Join(actualCWD, "config.toml") {
		t.Fatalf("WalkCwdToHome() last path = %q, want cwd path", got)
	}
}

func TestFilesAndFileHelpers(t *testing.T) {
	got := configloader.Files(
		[]string{"a", "", "b"},
		nil,
		[]string{"b", "c", ""},
	)
	assertStrings(t, "Files", got, []string{"a", "b", "b", "c"})

	if got := configloader.File(""); got != nil {
		t.Fatalf("File(empty) = %#v, want nil", got)
	}
	assertStrings(t, "File", configloader.File("/custom/config"), []string{"/custom/config"})
}

func mustFileHelper(t *testing.T) configloader.FileHelper {
	t.Helper()
	helper, err := configloader.NewFileHelper("my-cli", "config.toml")
	if err != nil {
		t.Fatalf("NewFileHelper() error = %v", err)
	}
	return helper
}

func chdir(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q) error = %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(old); err != nil {
			t.Fatalf("restore cwd: %v", err)
		}
	})
}

func mustGetwd(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	return filepath.Clean(cwd)
}

func dirsFromRoot(path string) []string {
	path = filepath.Clean(path)
	root := filepath.VolumeName(path) + string(filepath.Separator)
	var reversed []string
	for dir := path; ; dir = filepath.Dir(dir) {
		reversed = append(reversed, dir)
		if dir == root {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
	}
	for i, j := 0, len(reversed)-1; i < j; i, j = i+1, j-1 {
		reversed[i], reversed[j] = reversed[j], reversed[i]
	}
	return reversed
}

func appendFileName(dirs []string, name string) []string {
	out := make([]string, len(dirs))
	for i, dir := range dirs {
		out[i] = filepath.Join(dir, name)
	}
	return out
}

func assertStrings(t *testing.T, name string, got, want []string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%s = %#v, want %#v", name, got, want)
	}
}
