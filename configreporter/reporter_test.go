package configreporter_test

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	configloader "github.com/jmcampanini/go-config-loader/configloader"
	"github.com/jmcampanini/go-config-loader/configreporter"
)

type reportServer struct {
	Host string `toml:"host"`
	Port int    `toml:"port"`
}

type reportConfig struct {
	Name    string        `toml:"name"`
	Debug   bool          `toml:"debug"`
	Timeout time.Duration `toml:"timeout"`
	Server  reportServer  `toml:"server"`
	Omit    string        `toml:"-"`
}

type provenanceConfig struct {
	Name     string
	Debug    bool
	Timeout  time.Duration
	Profiles []string
	Pair     [2]int
	Labels   map[string]string
	Servers  map[string]reportServer
}

func TestReporterTOML(t *testing.T) {
	r := configreporter.New(reportConfig{
		Name:    "app",
		Debug:   true,
		Timeout: 5 * time.Second,
		Server:  reportServer{Host: "localhost", Port: 8080},
		Omit:    "secret",
	}, configloader.LoadReport{})

	got, err := r.TOML()
	if err != nil {
		t.Fatalf("TOML() error = %v", err)
	}
	text := string(got)
	for _, want := range []string{
		`name = "app"`,
		`debug = true`,
		`timeout = "5s"`,
		`[server]`,
		`host = "localhost"`,
		`port = 8080`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("TOML() = %q, missing %q", text, want)
		}
	}
	if strings.Contains(text, "secret") || strings.Contains(text, "Omit") {
		t.Fatalf("TOML() = %q, want toml:- field omitted", text)
	}

	var buf bytes.Buffer
	if err := r.WriteTOML(&buf); err != nil {
		t.Fatalf("WriteTOML() error = %v", err)
	}
	if !bytes.Equal(buf.Bytes(), got) {
		t.Fatalf("WriteTOML() = %q, TOML() = %q", buf.String(), text)
	}
}

func TestReporterWriteTOMLRejectsNilWriterAndInvalidConfig(t *testing.T) {
	r := configreporter.New(reportConfig{}, configloader.LoadReport{})
	if err := r.WriteTOML(nil); err == nil || !strings.Contains(err.Error(), "writer is nil") {
		t.Fatalf("WriteTOML(nil) error = %v, want nil-writer error", err)
	}

	type invalidConfig struct {
		When time.Time
	}
	bad := configreporter.New(invalidConfig{}, configloader.LoadReport{})
	if _, err := bad.TOML(); err == nil {
		t.Fatalf("TOML() error = nil for invalid config type")
	}
}

func TestReporterProvenanceHeadersAndRows(t *testing.T) {
	updates := configloader.Updates{
		"debug":                configloader.SourceEnv,
		`labels["prod"]`:       configloader.SourceDefault,
		`labels["prod.env"]`:   configloader.SourceDefault,
		"name":                 "<pflag>",
		"not.a.real.path":      "custom",
		"pair":                 "/tmp/config.toml",
		"profiles":             "/tmp/config.toml",
		`servers["prod"].port`: "/tmp/config.toml",
		"timeout":              configloader.SourceDefault,
	}
	r := configreporter.New(provenanceConfig{
		Name:     "my app",
		Debug:    true,
		Timeout:  5 * time.Second,
		Profiles: []string{"prod", "canary"},
		Pair:     [2]int{1, 2},
		Labels:   map[string]string{"prod": "blue", "prod.env": "green"},
		Servers:  map[string]reportServer{"prod": {Host: "localhost", Port: 9090}},
	}, configloader.LoadReport{Updates: updates})
	updates["debug"] = "mutated"
	updates["new"] = "mutated"

	headers := r.ProvenanceHeaders()
	wantHeaders := []string{"Path", "Value", "Source"}
	if !reflect.DeepEqual(headers, wantHeaders) {
		t.Fatalf("ProvenanceHeaders() = %#v, want %#v", headers, wantHeaders)
	}
	headers[0] = "mutated"
	if got := r.ProvenanceHeaders(); !reflect.DeepEqual(got, wantHeaders) {
		t.Fatalf("ProvenanceHeaders() after caller mutation = %#v, want %#v", got, wantHeaders)
	}

	wantRows := [][]string{
		{"debug", "true", configloader.SourceEnv},
		{`labels["prod"]`, `"blue"`, configloader.SourceDefault},
		{`labels["prod.env"]`, `"green"`, configloader.SourceDefault},
		{"name", `"my app"`, "<pflag>"},
		{"not.a.real.path", "<unavailable>", "custom"},
		{"pair", "[1, 2]", "/tmp/config.toml"},
		{"profiles", `["prod", "canary"]`, "/tmp/config.toml"},
		{`servers["prod"].port`, "9090", "/tmp/config.toml"},
		{"timeout", `"5s"`, configloader.SourceDefault},
	}
	rows := r.ProvenanceRows()
	if !reflect.DeepEqual(rows, wantRows) {
		t.Fatalf("ProvenanceRows() = %#v, want %#v", rows, wantRows)
	}
	rows[0][1] = "mutated"
	rows[0][2] = "mutated"
	if got := r.ProvenanceRows(); !reflect.DeepEqual(got, wantRows) {
		t.Fatalf("ProvenanceRows() after caller mutation = %#v, want %#v", got, wantRows)
	}
}
