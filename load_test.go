package configloader_test

import (
	"reflect"
	"testing"
	"time"

	configloader "github.com/jmcampanini/go-config-loader"
)

type phase2NestedConfig struct {
	Host string
	Port int
}

type phase2Config struct {
	Name     string
	Debug    bool
	Count    int
	Timeout  time.Duration
	Nested   phase2NestedConfig
	Tags     []string
	Pair     [2]int
	Labels   map[string]string
	Servers  map[string]phase2NestedConfig
	NilMap   map[string]string
	EmptyMap map[string]string
}

func TestLoadDefaultsOnlyReturnsDefaultsAndDefaultProvenance(t *testing.T) {
	defaults := phase2Config{
		Timeout: time.Second,
		Pair:    [2]int{1, 0},
		Labels: map[string]string{
			"env":   "prod",
			`a"b\c`: "quoted",
		},
		Servers: map[string]phase2NestedConfig{
			"prod": {Host: "example.com", Port: 443},
		},
		EmptyMap: map[string]string{},
	}

	got, updates, err := configloader.Load(defaults)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !reflect.DeepEqual(got, defaults) {
		t.Fatalf("Load() config = %#v, want %#v", got, defaults)
	}

	wantUpdates := configloader.Updates{
		"name":                 configloader.SourceDefault,
		"debug":                configloader.SourceDefault,
		"count":                configloader.SourceDefault,
		"timeout":              configloader.SourceDefault,
		"nested.host":          configloader.SourceDefault,
		"nested.port":          configloader.SourceDefault,
		"tags":                 configloader.SourceDefault,
		"pair":                 configloader.SourceDefault,
		`labels["a\"b\\c"]`:    configloader.SourceDefault,
		`labels["env"]`:        configloader.SourceDefault,
		`servers["prod"].host`: configloader.SourceDefault,
		`servers["prod"].port`: configloader.SourceDefault,
	}
	if !reflect.DeepEqual(updates, wantUpdates) {
		t.Fatalf("Load() updates = %#v, want %#v", updates, wantUpdates)
	}
	if _, ok := updates["nilmap"]; ok {
		t.Fatalf("Load() recorded nil map parent provenance")
	}
	if _, ok := updates["emptymap"]; ok {
		t.Fatalf("Load() recorded empty map parent provenance")
	}
}

func TestLoadRunsLoadersInOrderWithCurrentConfig(t *testing.T) {
	defaults := phase2Config{Count: 1}
	var calls []string

	first := func(base phase2Config) (phase2Config, configloader.Updates, error) {
		calls = append(calls, "first")
		if base.Count != 1 {
			t.Fatalf("first loader base.Count = %d, want 1", base.Count)
		}
		base.Count = 2
		return base, configloader.Updates{"count": "first"}, nil
	}
	second := func(base phase2Config) (phase2Config, configloader.Updates, error) {
		calls = append(calls, "second")
		if base.Count != 2 {
			t.Fatalf("second loader base.Count = %d, want 2", base.Count)
		}
		base.Count = 3
		return base, configloader.Updates{"count": "second"}, nil
	}

	got, updates, err := configloader.Load(defaults, first, second)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Count != 3 {
		t.Fatalf("Load() Count = %d, want 3", got.Count)
	}
	if !reflect.DeepEqual(calls, []string{"first", "second"}) {
		t.Fatalf("loader calls = %#v", calls)
	}
	if updates["count"] != "second" {
		t.Fatalf("updates[count] = %q, want second", updates["count"])
	}
}

func TestLoadMergesUpdatesLastWriteWinsAndTrustsKeys(t *testing.T) {
	first := func(base phase2Config) (phase2Config, configloader.Updates, error) {
		base.Name = "first"
		return base, configloader.Updates{
			"name":            "first",
			"not.a.real.path": "custom",
		}, nil
	}
	second := func(base phase2Config) (phase2Config, configloader.Updates, error) {
		base.Name = "second"
		return base, configloader.Updates{"name": "second"}, nil
	}

	got, updates, err := configloader.Load(phase2Config{}, first, second)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.Name != "second" {
		t.Fatalf("Load() Name = %q, want second", got.Name)
	}
	if updates["name"] != "second" {
		t.Fatalf("updates[name] = %q, want second", updates["name"])
	}
	if updates["not.a.real.path"] != "custom" {
		t.Fatalf("Load() did not trust loader-returned arbitrary update key")
	}
}

func TestLoadDefaultProvenanceRejectsEmptyMapKeys(t *testing.T) {
	_, _, err := configloader.Load(phase2Config{
		Labels: map[string]string{"": "bad"},
	})
	if err == nil {
		t.Fatalf("Load() error = nil")
	}
}
