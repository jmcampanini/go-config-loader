package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCobraExampleLoadsStringSlicesFromDefaultsFileEnvAndPflags(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configFile, []byte("file_profiles = ['file-a', 'file-b']\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	t.Setenv("COBRA_SLICES_DEMO_ENV_PROFILES", "env-a,env-b")

	var out bytes.Buffer
	cmd, err := newRootCommand(&out)
	if err != nil {
		t.Fatalf("newRootCommand() error = %v", err)
	}
	cmd.SetArgs([]string{"--config", configFile, "--flag-profiles", "flag-a", "--flag-profiles", "flag-b"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"default_only_profiles=[abc] source=<default>",
		"file_profiles=[file-a file-b] source=" + configFile,
		"env_profiles=[env-a env-b] source=<env>",
		"flag_profiles=[flag-a flag-b] source=<pflag>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output = %q, want line containing %q", got, want)
		}
	}
}
