package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCobraExample(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(configFile, []byte("name = 'from-file'\nport = 9090\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	var out bytes.Buffer
	cmd, err := newRootCommand(&out)
	if err != nil {
		t.Fatalf("newRootCommand() error = %v", err)
	}
	cmd.SetArgs([]string{"--config", configFile, "--name", "from-flag", "--debug"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	got := out.String()
	for _, want := range []string{
		"name=from-flag source=<pflag>",
		"debug=true source=<pflag>",
		"port=9090 source=" + configFile,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("output = %q, want line containing %q", got, want)
		}
	}
}
