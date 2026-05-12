package configloader_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	configloader "github.com/jmcampanini/go-config-loader"
)

type tomlServerConfig struct {
	Host string
	Port int
}

type tomlFileConfig struct {
	Name      string `config:"name"`
	Debug     bool
	Timeout   time.Duration
	Server    tomlServerConfig `toml:"srv"`
	Labels    map[string]string
	Servers   map[string]tomlServerConfig
	Tags      []string
	Pair      [2]int
	FileOnly  string `toml:"file_only" config:"env-only"`
	Untouched string
}

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	return path
}

func TestTomlFileLoaderEmptyListsAreNoOp(t *testing.T) {
	for name, constructor := range map[string]func([]string) (configloader.ConfigLoader[tomlFileConfig], error){
		"merge-all": func(files []string) (configloader.ConfigLoader[tomlFileConfig], error) {
			return configloader.NewMergeAllFilesLoader[tomlFileConfig](files)
		},
		"pick-last": func(files []string) (configloader.ConfigLoader[tomlFileConfig], error) {
			return configloader.NewPickLastFileLoader[tomlFileConfig](files)
		},
	} {
		t.Run(name, func(t *testing.T) {
			loader, err := constructor(nil)
			if err != nil {
				t.Fatalf("constructor() error = %v", err)
			}
			defaults := tomlFileConfig{Name: "default"}
			got, report, err := loader(defaults)
			if err != nil {
				t.Fatalf("loader() error = %v", err)
			}
			if !reflect.DeepEqual(got, defaults) {
				t.Fatalf("loader() config = %#v, want %#v", got, defaults)
			}
			if len(report.Updates) != 0 {
				t.Fatalf("loader() report.Updates = %#v, want empty", report.Updates)
			}
		})
	}
}

func TestTomlFileLoaderConstructorRejectsEmptyPathsAndCopiesSlice(t *testing.T) {
	if _, err := configloader.NewMergeAllFilesLoader[tomlFileConfig]([]string{"ok", ""}); err == nil {
		t.Fatalf("NewMergeAllFilesLoader() error = nil for empty path")
	}
	if _, err := configloader.NewPickLastFileLoader[tomlFileConfig]([]string{""}); err == nil {
		t.Fatalf("NewPickLastFileLoader() error = nil for empty path")
	}

	dir := t.TempDir()
	first := writeTestFile(t, dir, "first.conf", "name = 'first'\n")
	second := writeTestFile(t, dir, "second.conf", "name = 'second'\n")
	files := []string{first}
	loader, err := configloader.NewMergeAllFilesLoader[tomlFileConfig](files)
	if err != nil {
		t.Fatalf("NewMergeAllFilesLoader() error = %v", err)
	}
	files[0] = second

	got, report, err := loader(tomlFileConfig{})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if got.Name != "first" {
		t.Fatalf("loader() Name = %q, want first", got.Name)
	}
	if report.Updates["name"] != first {
		t.Fatalf("report.Updates[name] = %q, want %q", report.Updates["name"], first)
	}
}

func TestTomlFileLoaderMissingMalformedUnknownAndExtensionHandling(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing")
	validNoExt := writeTestFile(t, dir, "config", "name = 'loaded'\n")
	malformed := writeTestFile(t, dir, "bad", "name = [\n")
	unknown := writeTestFile(t, dir, "unknown", "does_not_exist = true\n")

	loader, err := configloader.NewMergeAllFilesLoader[tomlFileConfig]([]string{missing, validNoExt})
	if err != nil {
		t.Fatalf("NewMergeAllFilesLoader() error = %v", err)
	}
	got, report, err := loader(tomlFileConfig{Name: "default"})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if got.Name != "loaded" || report.Updates["name"] != validNoExt {
		t.Fatalf("loader() got Name=%q report.Updates=%#v, want loaded from %q", got.Name, report.Updates, validNoExt)
	}

	badLoader, err := configloader.NewMergeAllFilesLoader[tomlFileConfig]([]string{malformed})
	if err != nil {
		t.Fatalf("NewMergeAllFilesLoader(malformed) error = %v", err)
	}
	if _, _, err := badLoader(tomlFileConfig{}); err == nil {
		t.Fatalf("malformed loader error = nil")
	}

	unknownLoader, err := configloader.NewMergeAllFilesLoader[tomlFileConfig]([]string{unknown})
	if err != nil {
		t.Fatalf("NewMergeAllFilesLoader(unknown) error = %v", err)
	}
	if _, _, err := unknownLoader(tomlFileConfig{}); err == nil || !strings.Contains(err.Error(), "config file") || !strings.Contains(err.Error(), "unknown keys") || !strings.Contains(err.Error(), "does_not_exist") {
		t.Fatalf("unknown loader error = %v, want format-neutral unknown-key error", err)
	}
}

func TestFileLoaderUnknownKeyOptionsValidation(t *testing.T) {
	if _, err := configloader.NewMergeAllFilesLoader[tomlFileConfig](nil, configloader.WarnUnknownKeys()); err != nil {
		t.Fatalf("NewMergeAllFilesLoader(WarnUnknownKeys) error = %v, want nil", err)
	}
	if _, err := configloader.NewRequiredFileLoader[tomlFileConfig]("config.toml", nil); err == nil || !strings.Contains(err.Error(), "option at index 0 is nil") {
		t.Fatalf("NewRequiredFileLoader(nil option) error = %v, want nil-option error", err)
	}
	if _, err := configloader.NewMergeAllFilesLoader[tomlFileConfig](nil,
		configloader.WarnUnknownKeys(),
		configloader.IgnoreUnknownKeys(),
	); err != nil {
		t.Fatalf("NewMergeAllFilesLoader(last policy wins) error = %v, want nil", err)
	}
}

func TestTomlFileLoaderReportsLoadedFilesEvenWithoutUpdates(t *testing.T) {
	dir := t.TempDir()
	empty := writeTestFile(t, dir, "empty.toml", "")
	unknownOnly := writeTestFile(t, dir, "unknown.toml", "extra = true\n")
	missing := filepath.Join(dir, "missing.toml")

	loader, err := configloader.NewMergeAllFilesLoader[tomlFileConfig]([]string{missing, empty, unknownOnly, empty},
		configloader.IgnoreUnknownKeys(),
	)
	if err != nil {
		t.Fatalf("NewMergeAllFilesLoader() error = %v", err)
	}
	got, report, err := loader(tomlFileConfig{Name: "default"})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if got.Name != "default" {
		t.Fatalf("loader() Name = %q, want default", got.Name)
	}
	if len(report.Updates) != 0 {
		t.Fatalf("updates = %#v, want none", report.Updates)
	}
	wantLoadedFiles := []string{empty, unknownOnly}
	if !reflect.DeepEqual(report.LoadedFiles, wantLoadedFiles) {
		t.Fatalf("LoadedFiles = %#v, want %#v", report.LoadedFiles, wantLoadedFiles)
	}
}

func TestFileLoaderUnknownKeyIgnoreAppliesKnownKeys(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "config.toml", "name = 'loaded'\nextra = true\n")
	loader, err := configloader.NewMergeAllFilesLoader[tomlFileConfig]([]string{path},
		configloader.IgnoreUnknownKeys(),
	)
	if err != nil {
		t.Fatalf("NewMergeAllFilesLoader() error = %v", err)
	}
	got, report, err := loader(tomlFileConfig{Name: "default"})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if got.Name != "loaded" || report.Updates["name"] != path {
		t.Fatalf("loader() config=%#v report.Updates=%#v, want known key applied", got, report.Updates)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("warnings = %#v, want none", report.Warnings)
	}
}

func TestFileLoaderUnknownKeyWarnAppliesKnownKeysAndReportsWarning(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "config.toml", "extra_b = true\nname = 'loaded'\nextra_a = false\n")
	loader, err := configloader.NewMergeAllFilesLoader[tomlFileConfig]([]string{path},
		configloader.WarnUnknownKeys(),
	)
	if err != nil {
		t.Fatalf("NewMergeAllFilesLoader() error = %v", err)
	}
	got, report, err := loader(tomlFileConfig{})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if got.Name != "loaded" || report.Updates["name"] != path {
		t.Fatalf("loader() config=%#v report.Updates=%#v, want known key applied", got, report.Updates)
	}
	wantWarnings := []configloader.Warning{{Source: path, Message: "contains unknown keys: extra_a, extra_b"}}
	if !reflect.DeepEqual(report.Warnings, wantWarnings) {
		t.Fatalf("warnings = %#v, want %#v", report.Warnings, wantWarnings)
	}

	_, secondReport, err := loader(tomlFileConfig{})
	if err != nil {
		t.Fatalf("second loader() error = %v", err)
	}
	if !reflect.DeepEqual(secondReport.Warnings, wantWarnings) {
		t.Fatalf("second warnings = %#v, want %#v", secondReport.Warnings, wantWarnings)
	}
}

func TestTomlTagsCustomizeFileKeysAndConfigTagsDoNot(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "config.toml", `
name = "from-file"
file_only = "uses toml tag"
srv = { port = 9090 }
`)
	loader, err := configloader.NewMergeAllFilesLoader[tomlFileConfig]([]string{path})
	if err != nil {
		t.Fatalf("NewMergeAllFilesLoader() error = %v", err)
	}
	got, report, err := loader(tomlFileConfig{Server: tomlServerConfig{Host: "default-host"}})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if got.Name != "from-file" || got.FileOnly != "uses toml tag" || got.Server.Host != "default-host" || got.Server.Port != 9090 {
		t.Fatalf("loader() config = %#v", got)
	}
	wantUpdates := configloader.Updates{"name": path, "fileonly": path, "server.port": path}
	if !reflect.DeepEqual(report.Updates, wantUpdates) {
		t.Fatalf("report.Updates = %#v, want %#v", report.Updates, wantUpdates)
	}

	bad := writeTestFile(t, dir, "bad-config-tag.toml", "env-only = 'not a toml key'\n")
	badLoader, err := configloader.NewMergeAllFilesLoader[tomlFileConfig]([]string{bad})
	if err != nil {
		t.Fatalf("NewMergeAllFilesLoader() error = %v", err)
	}
	if _, _, err := badLoader(tomlFileConfig{}); err == nil {
		t.Fatalf("config-tag TOML key unexpectedly loaded")
	}
}

func TestTomlMergeSemanticsAndProvenance(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "merge", `
debug = false
tags = ["file"]
pair = [3, 4]

[srv]
port = 9090

[labels]
env = "file"
new = "value"

[servers.prod]
port = 443

[servers.staging]
host = "staging.example.com"
`)
	loader, err := configloader.NewMergeAllFilesLoader[tomlFileConfig]([]string{path})
	if err != nil {
		t.Fatalf("NewMergeAllFilesLoader() error = %v", err)
	}
	defaults := tomlFileConfig{
		Debug:  false,
		Server: tomlServerConfig{Host: "localhost", Port: 80},
		Labels: map[string]string{"env": "default", "keep": "yes"},
		Servers: map[string]tomlServerConfig{
			"prod": {Host: "prod.example.com", Port: 80},
		},
		Tags: []string{"default", "tags"},
		Pair: [2]int{1, 2},
	}
	got, report, err := loader(defaults)
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}

	want := defaults
	want.Debug = false
	want.Server.Port = 9090
	want.Labels = map[string]string{"env": "file", "keep": "yes", "new": "value"}
	want.Servers = map[string]tomlServerConfig{
		// TOML map keys merge, but a present map entry is decoded as a new value
		// rather than overlaid onto the previous entry value.
		"prod":    {Port: 443},
		"staging": {Host: "staging.example.com"},
	}
	want.Tags = []string{"file"}
	want.Pair = [2]int{3, 4}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loader() config = %#v, want %#v", got, want)
	}
	wantUpdates := configloader.Updates{
		"debug":                   path,
		"tags":                    path,
		"pair":                    path,
		"server.port":             path,
		`labels["env"]`:           path,
		`labels["new"]`:           path,
		`servers["prod"].host`:    path,
		`servers["prod"].port`:    path,
		`servers["staging"].host`: path,
		`servers["staging"].port`: path,
	}
	if !reflect.DeepEqual(report.Updates, wantUpdates) {
		t.Fatalf("report.Updates = %#v, want %#v", report.Updates, wantUpdates)
	}
}

func TestTomlMergeAllPrecedence(t *testing.T) {
	dir := t.TempDir()
	low := writeTestFile(t, dir, "low", "name = 'low'\n[srv]\nhost = 'low-host'\nport = 80\n")
	high := writeTestFile(t, dir, "high", "name = 'high'\n[srv]\nport = 443\n")
	loader, err := configloader.NewMergeAllFilesLoader[tomlFileConfig]([]string{low, high})
	if err != nil {
		t.Fatalf("NewMergeAllFilesLoader() error = %v", err)
	}
	got, report, err := loader(tomlFileConfig{})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if got.Name != "high" || got.Server.Host != "low-host" || got.Server.Port != 443 {
		t.Fatalf("loader() config = %#v", got)
	}
	if report.Updates["name"] != high || report.Updates["server.host"] != low || report.Updates["server.port"] != high {
		t.Fatalf("report.Updates = %#v", report.Updates)
	}
}

func TestFileLoaderMergeAllWarningsAreBufferedAndOrdered(t *testing.T) {
	dir := t.TempDir()
	low := writeTestFile(t, dir, "low", "name = 'low'\nextra_b = true\n")
	high := writeTestFile(t, dir, "high", "debug = true\nextra_a = true\n")
	loader, err := configloader.NewMergeAllFilesLoader[tomlFileConfig]([]string{low, high},
		configloader.WarnUnknownKeys(),
	)
	if err != nil {
		t.Fatalf("NewMergeAllFilesLoader() error = %v", err)
	}
	got, report, err := loader(tomlFileConfig{})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if got.Name != "low" || !got.Debug {
		t.Fatalf("loader() config = %#v, want both files applied", got)
	}
	wantWarnings := []configloader.Warning{
		{Source: low, Message: "contains unknown keys: extra_b"},
		{Source: high, Message: "contains unknown keys: extra_a"},
	}
	if !reflect.DeepEqual(report.Warnings, wantWarnings) {
		t.Fatalf("warnings = %#v, want %#v", report.Warnings, wantWarnings)
	}
	if !reflect.DeepEqual(report.LoadedFiles, []string{low, high}) {
		t.Fatalf("loaded files = %#v, want %#v", report.LoadedFiles, []string{low, high})
	}

	badHigh := writeTestFile(t, dir, "bad-high", "name = [\n")
	badLoader, err := configloader.NewMergeAllFilesLoader[tomlFileConfig]([]string{low, badHigh},
		configloader.WarnUnknownKeys(),
	)
	if err != nil {
		t.Fatalf("NewMergeAllFilesLoader(badHigh) error = %v", err)
	}
	got, report, err = badLoader(tomlFileConfig{Name: "base"})
	if err == nil {
		t.Fatalf("badLoader() error = nil, want malformed-file error")
	}
	if got.Name != "base" {
		t.Fatalf("badLoader() got.Name = %q, want original base", got.Name)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("warnings after failed merge = %#v, want none", report.Warnings)
	}
	if len(report.LoadedFiles) != 0 {
		t.Fatalf("loaded files after failed merge = %#v, want none", report.LoadedFiles)
	}
}

func TestTomlPickLastLoadsOnlyHighestPriorityExistingFile(t *testing.T) {
	dir := t.TempDir()
	low := writeTestFile(t, dir, "low", "name = 'low'\n")
	high := writeTestFile(t, dir, "high", "name = 'high'\n")
	missing := filepath.Join(dir, "missing")
	loader, err := configloader.NewPickLastFileLoader[tomlFileConfig]([]string{low, missing, high})
	if err != nil {
		t.Fatalf("NewPickLastFileLoader() error = %v", err)
	}
	got, report, err := loader(tomlFileConfig{})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if got.Name != "high" || !reflect.DeepEqual(report.Updates, configloader.Updates{"name": high}) {
		t.Fatalf("loader() config=%#v report.Updates=%#v, want high only", got, report.Updates)
	}
}

func TestFileLoaderPickLastUnknownKeysApplyOnlyToSelectedFile(t *testing.T) {
	dir := t.TempDir()
	lowUnknown := writeTestFile(t, dir, "low", "name = 'low'\nextra = true\n")
	high := writeTestFile(t, dir, "high", "name = 'high'\n")
	loader, err := configloader.NewPickLastFileLoader[tomlFileConfig]([]string{lowUnknown, high},
		configloader.WarnUnknownKeys(),
	)
	if err != nil {
		t.Fatalf("NewPickLastFileLoader() error = %v", err)
	}
	got, report, err := loader(tomlFileConfig{})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if got.Name != "high" || !reflect.DeepEqual(report.Updates, configloader.Updates{"name": high}) {
		t.Fatalf("loader() config=%#v report.Updates=%#v, want high only", got, report.Updates)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("warnings = %#v, want none for lower-priority file", report.Warnings)
	}
	if !reflect.DeepEqual(report.LoadedFiles, []string{high}) {
		t.Fatalf("loaded files = %#v, want %#v", report.LoadedFiles, []string{high})
	}

	selectedLoader, err := configloader.NewPickLastFileLoader[tomlFileConfig]([]string{lowUnknown, filepath.Join(dir, "missing")},
		configloader.WarnUnknownKeys(),
	)
	if err != nil {
		t.Fatalf("NewPickLastFileLoader(selected low) error = %v", err)
	}
	got, report, err = selectedLoader(tomlFileConfig{})
	if err != nil {
		t.Fatalf("selectedLoader() error = %v", err)
	}
	if got.Name != "low" {
		t.Fatalf("selectedLoader() Name = %q, want low", got.Name)
	}
	wantWarnings := []configloader.Warning{{Source: lowUnknown, Message: "contains unknown keys: extra"}}
	if !reflect.DeepEqual(report.Warnings, wantWarnings) {
		t.Fatalf("warnings = %#v, want %#v", report.Warnings, wantWarnings)
	}
}

func TestTomlPickLastDoesNotFallBackAfterInvalidFile(t *testing.T) {
	dir := t.TempDir()
	low := writeTestFile(t, dir, "low", "name = 'low'\n")
	highInvalid := writeTestFile(t, dir, "high", "name = [\n")
	loader, err := configloader.NewPickLastFileLoader[tomlFileConfig]([]string{low, highInvalid})
	if err != nil {
		t.Fatalf("NewPickLastFileLoader() error = %v", err)
	}
	if got, _, err := loader(tomlFileConfig{}); err == nil || got.Name == "low" {
		t.Fatalf("loader() got=%#v err=%v, want invalid high file error without fallback", got, err)
	}
}

func TestTomlFileLoadersNormalizePathsToAbsolute(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "config.toml", "name = 'relative'\n")
	oldWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	defer func() {
		if err := os.Chdir(oldWD); err != nil {
			t.Fatalf("Chdir(%q) error = %v", oldWD, err)
		}
	}()

	expectedPath, err := filepath.Abs("config.toml")
	if err != nil {
		t.Fatalf("Abs() error = %v", err)
	}
	loader, err := configloader.NewMergeAllFilesLoader[tomlFileConfig]([]string{"config.toml"})
	if err != nil {
		t.Fatalf("NewMergeAllFilesLoader() error = %v", err)
	}
	got, report, err := loader(tomlFileConfig{})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if got.Name != "relative" || report.Updates["name"] != expectedPath {
		t.Fatalf("loader() config=%#v report.Updates=%#v, want source %q", got, report.Updates, expectedPath)
	}
}

func TestTomlRequiredFileLoader(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "required.toml", "name = 'required'\n")

	loader, err := configloader.NewRequiredFileLoader[tomlFileConfig](path)
	if err != nil {
		t.Fatalf("NewRequiredFileLoader() error = %v", err)
	}
	got, report, err := loader(tomlFileConfig{Name: "default"})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if got.Name != "required" || !reflect.DeepEqual(report.Updates, configloader.Updates{"name": path}) {
		t.Fatalf("loader() config=%#v report.Updates=%#v, want required file source", got, report.Updates)
	}
}

func TestTomlRequiredFileLoaderUnknownKeyOptions(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "required.toml", "name = 'required'\nextra = true\n")

	ignoreLoader, err := configloader.NewRequiredFileLoader[tomlFileConfig](path, configloader.IgnoreUnknownKeys())
	if err != nil {
		t.Fatalf("NewRequiredFileLoader(ignore) error = %v", err)
	}
	got, report, err := ignoreLoader(tomlFileConfig{})
	if err != nil {
		t.Fatalf("ignoreLoader() error = %v", err)
	}
	if got.Name != "required" || !reflect.DeepEqual(report.Updates, configloader.Updates{"name": path}) {
		t.Fatalf("ignoreLoader() config=%#v report.Updates=%#v, want known key applied", got, report.Updates)
	}

	warnLoader, err := configloader.NewRequiredFileLoader[tomlFileConfig](path,
		configloader.WarnUnknownKeys(),
	)
	if err != nil {
		t.Fatalf("NewRequiredFileLoader(warn) error = %v", err)
	}
	got, report, err = warnLoader(tomlFileConfig{})
	if err != nil {
		t.Fatalf("warnLoader() error = %v", err)
	}
	if got.Name != "required" || !reflect.DeepEqual(report.Updates, configloader.Updates{"name": path}) {
		t.Fatalf("warnLoader() config=%#v report.Updates=%#v, want known key applied", got, report.Updates)
	}
	wantWarnings := []configloader.Warning{{Source: path, Message: "contains unknown keys: extra"}}
	if !reflect.DeepEqual(report.Warnings, wantWarnings) {
		t.Fatalf("warnings = %#v, want %#v", report.Warnings, wantWarnings)
	}
}

func TestTomlRequiredFileLoaderMissingAndDirectory(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.toml")
	loader, err := configloader.NewRequiredFileLoader[tomlFileConfig](missing)
	if err != nil {
		t.Fatalf("NewRequiredFileLoader() error = %v", err)
	}
	if _, _, err := loader(tomlFileConfig{}); err == nil || !strings.Contains(err.Error(), "required config file") || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("loader() missing error = %v, want required missing error", err)
	}

	dirLoader, err := configloader.NewRequiredFileLoader[tomlFileConfig](dir)
	if err != nil {
		t.Fatalf("NewRequiredFileLoader(dir) error = %v", err)
	}
	if _, _, err := dirLoader(tomlFileConfig{}); err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("loader() directory error = %v, want directory error", err)
	}
}
