package configloader_test

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	configloader "github.com/jmcampanini/go-config-loader"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/spf13/pflag"
)

type integrationServerConfig struct {
	Host string `config:"host-name" help:"server host"`
	Port int
}

type integrationEndpointConfig struct {
	Host string
	Port int
}

type integrationConfig struct {
	Name        string                  `config:"name" help:"display name"`
	Debug       bool                    `config:"debug" help:"enable debug"`
	Timeout     time.Duration           `config:"timeout" help:"request timeout"`
	Server      integrationServerConfig `toml:"srv"`
	Labels      map[string]string
	Endpoints   map[string]integrationEndpointConfig `toml:"servers"`
	Tags        []string
	Pair        [2]int
	FileOnly    string `toml:"file_only"`
	DefaultOnly string
	EnvOnly     string `config:"env-only" help:"env-only value"`
	FlagOnly    string `config:"flag-only" help:"flag-only value"`
}

func TestEndToEndPrecedenceAndProvenance(t *testing.T) {
	dir := t.TempDir()
	lowFile := writeIntegrationFile(t, dir, "low.toml", `
name = "from-low-file"
debug = true
tags = ["low"]
pair = [1, 2]
file_only = "from-low-file"

[srv]
host = "low-file-host"
port = 80

[labels]
keep = "from-low-file"
"a\"b\\c" = "from-low-file"

[servers.prod]
host = "prod-low"
port = 8080
`)
	highFile := writeIntegrationFile(t, dir, "high.toml", `
name = "from-high-file"
tags = ["high"]
pair = [3, 4]

[srv]
port = 443

[labels]
"a\"b\\c" = "from-high-file"
new = "from-high-file"

[servers.canary]
host = "canary-high"
port = 9000
`)

	fileLoader, err := configloader.NewMergeAllFilesLoader[integrationConfig]([]string{lowFile, highFile})
	if err != nil {
		t.Fatalf("NewMergeAllFilesLoader() error = %v", err)
	}
	envLoader, err := configloader.NewEnvironmentLoader[integrationConfig]("my-app", map[string]string{
		"MY_APP_NAME":      "from-env",
		"MY_APP_HOST_NAME": "env-host",
		"MY_APP_TIMEOUT":   "5s",
		"MY_APP_ENV_ONLY":  "from-env",
	})
	if err != nil {
		t.Fatalf("NewEnvironmentLoader() error = %v", err)
	}

	flags := pflag.NewFlagSet(t.Name(), pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	if err := pflagloader.Register[integrationConfig](flags); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := flags.Parse([]string{
		"--name=from-flag",
		"--debug=false",
		"--host-name=flag-host",
		"--timeout=10s",
		"--flag-only=from-flag",
	}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	flagLoader, err := pflagloader.NewLoader[integrationConfig](flags)
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}

	defaults := integrationConfig{
		Name:        "default-name",
		Debug:       false,
		Timeout:     time.Second,
		Server:      integrationServerConfig{Host: "default-host", Port: 8080},
		Labels:      map[string]string{"default-only-key": "default", `a"b\c`: "default"},
		Endpoints:   map[string]integrationEndpointConfig{"default": {Host: "default-endpoint", Port: 7000}},
		Tags:        []string{"default"},
		Pair:        [2]int{0, 0},
		FileOnly:    "default-file-only",
		DefaultOnly: "default-only",
		EnvOnly:     "default-env-only",
		FlagOnly:    "default-flag-only",
	}

	got, updates, err := configloader.Load(defaults, fileLoader, envLoader, flagLoader)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := integrationConfig{
		Name:    "from-flag",
		Debug:   false,
		Timeout: 10 * time.Second,
		Server:  integrationServerConfig{Host: "flag-host", Port: 443},
		Labels: map[string]string{
			"default-only-key": "default",
			`a"b\c`:            "from-high-file",
			"keep":             "from-low-file",
			"new":              "from-high-file",
		},
		Endpoints: map[string]integrationEndpointConfig{
			"default": {Host: "default-endpoint", Port: 7000},
			"prod":    {Host: "prod-low", Port: 8080},
			"canary":  {Host: "canary-high", Port: 9000},
		},
		Tags:        []string{"high"},
		Pair:        [2]int{3, 4},
		FileOnly:    "from-low-file",
		DefaultOnly: "default-only",
		EnvOnly:     "from-env",
		FlagOnly:    "from-flag",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() config = %#v, want %#v", got, want)
	}

	wantSources := map[string]string{
		"name":                       pflagloader.SourcePFlag,
		"debug":                      pflagloader.SourcePFlag,
		"timeout":                    pflagloader.SourcePFlag,
		"server.host":                pflagloader.SourcePFlag,
		"server.port":                highFile,
		"tags":                       highFile,
		"pair":                       highFile,
		"fileonly":                   lowFile,
		"envonly":                    configloader.SourceEnv,
		"flagonly":                   pflagloader.SourcePFlag,
		"defaultonly":                configloader.SourceDefault,
		`labels["a\"b\\c"]`:          highFile,
		`labels["keep"]`:             lowFile,
		`labels["new"]`:              highFile,
		`labels["default-only-key"]`: configloader.SourceDefault,
		`endpoints["default"].host`:  configloader.SourceDefault,
		`endpoints["default"].port`:  configloader.SourceDefault,
		`endpoints["prod"].host`:     lowFile,
		`endpoints["prod"].port`:     lowFile,
		`endpoints["canary"].host`:   highFile,
		`endpoints["canary"].port`:   highFile,
	}
	if len(updates) != len(wantSources) {
		t.Fatalf("len(updates) = %d, want %d (updates: %#v)", len(updates), len(wantSources), updates)
	}
	for path, wantSource := range wantSources {
		if gotSource := updates[path]; gotSource != wantSource {
			t.Fatalf("updates[%q] = %q, want %q (all updates: %#v)", path, gotSource, wantSource, updates)
		}
	}
}

func writeIntegrationFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
	return path
}
