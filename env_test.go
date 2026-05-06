package configloader_test

import (
	"math"
	"reflect"
	"testing"
	"time"

	configloader "github.com/jmcampanini/go-config-loader"
)

type envNestedConfig struct {
	Host string `config:"host-name"`
	Port int
}

type envBasicConfig struct {
	APIURL  string `config:"api-url"`
	Debug   bool   `config:"debug"`
	Ignored string
	Nested  envNestedConfig
}

func TestEnvironmentLoaderGeneratesNamesLoadsTaggedLeavesAndCanonicalUpdates(t *testing.T) {
	loader, err := configloader.NewEnvironmentLoader[envBasicConfig]("my-cli", map[string]string{
		"MY_CLI_API_URL":   "https://example.com",
		"MY_CLI_DEBUG":     "true",
		"MY_CLI_IGNORED":   "do-not-use",
		"MY_CLI_HOST_NAME": "db.example.com",
		"MY_CLI_PORT":      "1234",
		"my_cli_debug":     "false",
	})
	if err != nil {
		t.Fatalf("NewEnvironmentLoader() error = %v", err)
	}

	got, updates, err := loader(envBasicConfig{
		APIURL:  "default",
		Ignored: "keep",
		Nested:  envNestedConfig{Port: 80},
	})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}

	want := envBasicConfig{
		APIURL:  "https://example.com",
		Debug:   true,
		Ignored: "keep",
		Nested:  envNestedConfig{Host: "db.example.com", Port: 80},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loader() config = %#v, want %#v", got, want)
	}

	wantUpdates := configloader.Updates{
		"apiurl":      configloader.SourceEnv,
		"debug":       configloader.SourceEnv,
		"nested.host": configloader.SourceEnv,
	}
	if !reflect.DeepEqual(updates, wantUpdates) {
		t.Fatalf("loader() updates = %#v, want %#v", updates, wantUpdates)
	}
	if _, ok := updates["api-url"]; ok {
		t.Fatalf("updates used config tag instead of canonical Go path")
	}
}

type envAllScalarsConfig struct {
	String   string        `config:"string"`
	Bool     bool          `config:"bool"`
	Int      int           `config:"int"`
	Int8     int8          `config:"int8"`
	Int16    int16         `config:"int16"`
	Int32    int32         `config:"int32"`
	Int64    int64         `config:"int64"`
	Uint     uint          `config:"uint"`
	Uint8    uint8         `config:"uint8"`
	Uint16   uint16        `config:"uint16"`
	Uint32   uint32        `config:"uint32"`
	Uint64   uint64        `config:"uint64"`
	Uintptr  uintptr       `config:"uintptr"`
	Float32  float32       `config:"float32"`
	Float64  float64       `config:"float64"`
	Duration time.Duration `config:"duration"`
}

func TestLoadWithEnvironmentLoaderKeepsDefaultsForUnsetEnvVars(t *testing.T) {
	type config struct {
		Message string `config:"message"`
		Debug   bool   `config:"debug"`
	}

	loader, err := configloader.NewEnvironmentLoader[config]("my-app", map[string]string{
		"MY_APP_MESSAGE": "hello from env",
	})
	if err != nil {
		t.Fatalf("NewEnvironmentLoader() error = %v", err)
	}

	got, updates, err := configloader.Load(config{Message: "default message", Debug: false}, loader)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	want := config{Message: "hello from env", Debug: false}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Load() config = %#v, want %#v", got, want)
	}
	if updates["message"] != configloader.SourceEnv {
		t.Fatalf("updates[message] = %q, want SourceEnv", updates["message"])
	}
	if updates["debug"] != configloader.SourceDefault {
		t.Fatalf("updates[debug] = %q, want SourceDefault", updates["debug"])
	}
}

func TestEnvironmentLoaderParsesEverySupportedScalarType(t *testing.T) {
	loader, err := configloader.NewEnvironmentLoader[envAllScalarsConfig]("app", map[string]string{
		"APP_STRING":   "hello",
		"APP_BOOL":     "true",
		"APP_INT":      "-1",
		"APP_INT8":     "-8",
		"APP_INT16":    "-16",
		"APP_INT32":    "-32",
		"APP_INT64":    "-64",
		"APP_UINT":     "1",
		"APP_UINT8":    "8",
		"APP_UINT16":   "16",
		"APP_UINT32":   "32",
		"APP_UINT64":   "64",
		"APP_UINTPTR":  "99",
		"APP_FLOAT32":  "1.5",
		"APP_FLOAT64":  "2.25",
		"APP_DURATION": "1h2m3s",
	})
	if err != nil {
		t.Fatalf("NewEnvironmentLoader() error = %v", err)
	}

	got, updates, err := loader(envAllScalarsConfig{})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}

	want := envAllScalarsConfig{
		String:   "hello",
		Bool:     true,
		Int:      -1,
		Int8:     -8,
		Int16:    -16,
		Int32:    -32,
		Int64:    -64,
		Uint:     1,
		Uint8:    8,
		Uint16:   16,
		Uint32:   32,
		Uint64:   64,
		Uintptr:  99,
		Float32:  1.5,
		Float64:  2.25,
		Duration: time.Hour + 2*time.Minute + 3*time.Second,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loader() config = %#v, want %#v", got, want)
	}
	if len(updates) != 16 {
		t.Fatalf("len(updates) = %d, want 16", len(updates))
	}
	if math.Abs(float64(got.Float32-1.5)) > 0 || math.Abs(got.Float64-2.25) > 0 {
		t.Fatalf("floats did not parse correctly: %#v", got)
	}
}

func TestEnvironmentLoaderParseFailureReturnsErrorWithoutApplyingPartialUpdate(t *testing.T) {
	loader, err := configloader.NewEnvironmentLoader[envBasicConfig]("my-cli", map[string]string{
		"MY_CLI_API_URL": "changed",
		"MY_CLI_DEBUG":   "not-bool",
	})
	if err != nil {
		t.Fatalf("NewEnvironmentLoader() error = %v", err)
	}

	base := envBasicConfig{APIURL: "default"}
	got, updates, err := loader(base)
	if err == nil {
		t.Fatalf("loader() error = nil")
	}
	if !reflect.DeepEqual(got, base) {
		t.Fatalf("loader() config = %#v, want unchanged %#v", got, base)
	}
	if updates != nil {
		t.Fatalf("loader() updates = %#v, want nil", updates)
	}
}

func TestEnvironmentLoaderCopiesEnvMap(t *testing.T) {
	env := map[string]string{"APP_STRING": "before"}
	loader, err := configloader.NewEnvironmentLoader[struct {
		String string `config:"string"`
	}]("app", env)
	if err != nil {
		t.Fatalf("NewEnvironmentLoader() error = %v", err)
	}
	env["APP_STRING"] = "after"

	got, _, err := loader(struct {
		String string `config:"string"`
	}{})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if got.String != "before" {
		t.Fatalf("String = %q, want copied env value before", got.String)
	}
}

func TestEnvironmentLoaderEmptyStringCountsAsSet(t *testing.T) {
	loader, err := configloader.NewEnvironmentLoader[struct {
		Name string `config:"name"`
	}]("app", map[string]string{"APP_NAME": ""})
	if err != nil {
		t.Fatalf("NewEnvironmentLoader() error = %v", err)
	}

	got, updates, err := loader(struct {
		Name string `config:"name"`
	}{Name: "default"})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if got.Name != "" {
		t.Fatalf("Name = %q, want empty string", got.Name)
	}
	if updates["name"] != configloader.SourceEnv {
		t.Fatalf("updates[name] = %q, want SourceEnv", updates["name"])
	}
}

func TestNewEnvironmentLoaderRejectsInvalidPrefixAndConfig(t *testing.T) {
	if _, err := configloader.NewEnvironmentLoader[envBasicConfig]("MY_CLI", nil); err == nil {
		t.Fatalf("NewEnvironmentLoader() invalid prefix error = nil")
	}
	if _, err := configloader.NewEnvironmentLoader[*envBasicConfig]("my-cli", nil); err == nil {
		t.Fatalf("NewEnvironmentLoader() invalid config error = nil")
	}
}
