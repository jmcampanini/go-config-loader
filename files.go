package configloader

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jmcampanini/go-config-loader/internal/configmeta"
)

// FileHelper generates candidate config file paths for an application.
type FileHelper struct {
	AppName        string
	ConfigFileName string
}

// NewFileHelper validates names and returns a FileHelper.
func NewFileHelper(appName, configFileName string) (FileHelper, error) {
	if !configmeta.IsKebabCase(appName) {
		return FileHelper{}, fmt.Errorf("configloader: invalid app name %q", appName)
	}
	if configFileName == "" {
		return FileHelper{}, fmt.Errorf("configloader: config file name must be non-empty")
	}
	if configFileName == "." || configFileName == ".." || strings.ContainsAny(configFileName, `/\\`) || filepath.Base(configFileName) != configFileName {
		return FileHelper{}, fmt.Errorf("configloader: config file name %q must be a base filename", configFileName)
	}

	return FileHelper{AppName: appName, ConfigFileName: configFileName}, nil
}

// Cwd returns the config file path in the current working directory.
func (f FileHelper) Cwd() []string {
	dir, ok := currentWorkingDirectory()
	if !ok {
		return nil
	}
	return []string{filepath.Join(dir, f.ConfigFileName)}
}

// CwdHidden returns the hidden config file path in the current working directory.
func (f FileHelper) CwdHidden() []string {
	dir, ok := currentWorkingDirectory()
	if !ok {
		return nil
	}
	return []string{filepath.Join(dir, hiddenFileName(f.ConfigFileName))}
}

// CwdBoth returns normal then hidden config file paths in the current working directory.
func (f FileHelper) CwdBoth() []string { return Files(f.Cwd(), f.CwdHidden()) }

// XDGConfigFile returns the XDG config file path.
func (f FileHelper) XDGConfigFile() []string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, ok := homeDirectory()
		if !ok {
			return nil
		}
		base = filepath.Join(home, ".config")
	} else {
		abs, err := filepath.Abs(base)
		if err != nil {
			return nil
		}
		base = abs
	}
	return []string{filepath.Join(base, f.AppName, f.ConfigFileName)}
}

// Home returns the config file path in the user's home directory.
func (f FileHelper) Home() []string {
	dir, ok := homeDirectory()
	if !ok {
		return nil
	}
	return []string{filepath.Join(dir, f.ConfigFileName)}
}

// HomeHidden returns the hidden config file path in the user's home directory.
func (f FileHelper) HomeHidden() []string {
	dir, ok := homeDirectory()
	if !ok {
		return nil
	}
	return []string{filepath.Join(dir, hiddenFileName(f.ConfigFileName))}
}

// HomeBoth returns normal then hidden config file paths in the user's home directory.
func (f FileHelper) HomeBoth() []string { return Files(f.Home(), f.HomeHidden()) }

// WalkCwdToHome returns config file paths walking from home/root to cwd.
func (f FileHelper) WalkCwdToHome() []string {
	return f.walkCwdToHome(func(dir string) []string {
		return []string{filepath.Join(dir, f.ConfigFileName)}
	})
}

// WalkCwdToHomeHidden returns hidden config file paths walking from home/root to cwd.
func (f FileHelper) WalkCwdToHomeHidden() []string {
	return f.walkCwdToHome(func(dir string) []string {
		return []string{filepath.Join(dir, hiddenFileName(f.ConfigFileName))}
	})
}

// WalkCwdToHomeBoth returns normal then hidden paths for each walked directory.
func (f FileHelper) WalkCwdToHomeBoth() []string {
	return f.walkCwdToHome(func(dir string) []string {
		return []string{
			filepath.Join(dir, f.ConfigFileName),
			filepath.Join(dir, hiddenFileName(f.ConfigFileName)),
		}
	})
}

// Files composes file groups in order, filtering empty strings.
func Files(groups ...[]string) []string {
	var out []string
	for _, group := range groups {
		for _, path := range group {
			if path != "" {
				out = append(out, path)
			}
		}
	}
	return out
}

// File returns nil for an empty path or a one-element file list otherwise.
func File(path string) []string {
	if path == "" {
		return nil
	}
	return []string{path}
}

func (f FileHelper) walkCwdToHome(pathsForDir func(string) []string) []string {
	cwd, ok := currentWorkingDirectory()
	if !ok {
		return nil
	}

	start := filepath.VolumeName(cwd) + string(filepath.Separator)
	if home, ok := homeDirectory(); ok && isWithinDir(home, cwd) {
		start = home
	}

	dirs := directoriesBetween(start, cwd)
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		out = append(out, pathsForDir(dir)...)
	}
	return out
}

func currentWorkingDirectory() (string, bool) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", false
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return "", false
	}
	return filepath.Clean(abs), true
}

func homeDirectory() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", false
	}
	abs, err := filepath.Abs(home)
	if err != nil {
		return "", false
	}
	return filepath.Clean(abs), true
}

func hiddenFileName(name string) string {
	return "." + name
}

func isWithinDir(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

func directoriesBetween(start, end string) []string {
	start = filepath.Clean(start)
	end = filepath.Clean(end)

	if start == end {
		return []string{start}
	}

	var reversed []string
	for dir := end; ; dir = filepath.Dir(dir) {
		reversed = append(reversed, dir)
		if dir == start {
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

// NewMergeAllFilesLoader constructs a low-to-high priority merge-all TOML loader.
func NewMergeAllFilesLoader[C any](files []string) (ConfigLoader[C], error) {
	return newFilesLoader[C](files, false)
}

// NewPickLastFileLoader constructs a loader that uses the highest-priority existing TOML file.
func NewPickLastFileLoader[C any](files []string) (ConfigLoader[C], error) {
	return newFilesLoader[C](files, true)
}
