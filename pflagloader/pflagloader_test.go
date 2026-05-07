package pflagloader_test

import (
	"io"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	configloader "github.com/jmcampanini/go-config-loader"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/spf13/pflag"
)

type pflagNestedConfig struct {
	Host string `config:"host-name" help:"server host"`
	Port int
}

type pflagBasicConfig struct {
	Name    string `config:"name" help:"display name"`
	Debug   bool   `config:"debug" help:"enable debug"`
	Ignored string
	Nested  pflagNestedConfig
}

func newFlagSet(t *testing.T) *pflag.FlagSet {
	t.Helper()
	flags := pflag.NewFlagSet(t.Name(), pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	return flags
}

func TestAPISignaturesAndSourceConstant(t *testing.T) {
	requireRegisterFunc(pflagloader.Register[pflagBasicConfig])
	requireNewLoaderFunc(pflagloader.NewLoader[pflagBasicConfig])
	if pflagloader.SourcePFlag != "<pflag>" {
		t.Fatalf("SourcePFlag = %q, want <pflag>", pflagloader.SourcePFlag)
	}
}

func requireRegisterFunc(func(*pflag.FlagSet) error) {}
func requireNewLoaderFunc(func(*pflag.FlagSet) (configloader.ConfigLoader[pflagBasicConfig], error)) {
}

func TestNilFlagSetErrors(t *testing.T) {
	if err := pflagloader.Register[pflagBasicConfig](nil); err == nil {
		t.Fatalf("Register(nil) error = nil")
	}
	if _, err := pflagloader.NewLoader[pflagBasicConfig](nil); err == nil {
		t.Fatalf("NewLoader(nil) error = nil")
	}
}

func TestRegisterRequiresHelpTags(t *testing.T) {
	type missingHelp struct {
		Name string `config:"name"`
	}
	if err := pflagloader.Register[missingHelp](newFlagSet(t)); err == nil {
		t.Fatalf("Register() error = nil for missing help tag")
	}

	type emptyHelp struct {
		Name string `config:"name" help:""`
	}
	if err := pflagloader.Register[emptyHelp](newFlagSet(t)); err == nil {
		t.Fatalf("Register() error = nil for empty help tag")
	}
}

func TestRegisterFlagsWithExactNamesAndHelp(t *testing.T) {
	flags := newFlagSet(t)
	if err := pflagloader.Register[pflagBasicConfig](flags); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	name := flags.Lookup("name")
	if name == nil {
		t.Fatalf("flag name was not registered")
	}
	if name.Usage != "display name" || name.Shorthand != "" || name.Hidden || name.Deprecated != "" || len(name.Annotations) != 0 {
		t.Fatalf("name flag = %#v, want exact long flag with help and no extras", name)
	}
	if name.DefValue != "" {
		t.Fatalf("name DefValue = %q, want empty non-application default", name.DefValue)
	}

	host := flags.Lookup("host-name")
	if host == nil || host.Usage != "server host" {
		t.Fatalf("host-name flag = %#v, want registered with exact help", host)
	}
	if flags.Lookup("ignored") != nil || flags.Lookup("port") != nil {
		t.Fatalf("untagged fields were registered")
	}
}

func TestRegisterDuplicateFlagsReturnErrors(t *testing.T) {
	flags := newFlagSet(t)
	flags.String("name", "existing", "existing flag")
	if err := pflagloader.Register[pflagBasicConfig](flags); err == nil {
		t.Fatalf("Register() error = nil for duplicate flag")
	}
}

func TestBoolNoValueFormParsesAsTrue(t *testing.T) {
	flags := newFlagSet(t)
	if err := pflagloader.Register[pflagBasicConfig](flags); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := flags.Parse([]string{"--debug"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	loader, err := pflagloader.NewLoader[pflagBasicConfig](flags)
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}
	got, updates, err := loader(pflagBasicConfig{})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if !got.Debug {
		t.Fatalf("Debug = false, want true")
	}
	if updates["debug"] != pflagloader.SourcePFlag {
		t.Fatalf("updates[debug] = %q, want SourcePFlag", updates["debug"])
	}
}

func TestUnchangedFlagsDoNotUpdateConfigOrProvenance(t *testing.T) {
	flags := newFlagSet(t)
	if err := pflagloader.Register[pflagBasicConfig](flags); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := flags.Parse(nil); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	loader, err := pflagloader.NewLoader[pflagBasicConfig](flags)
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}
	base := pflagBasicConfig{Name: "default", Debug: true, Ignored: "keep", Nested: pflagNestedConfig{Host: "localhost", Port: 80}}
	got, updates, err := loader(base)
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if !reflect.DeepEqual(got, base) {
		t.Fatalf("loader() config = %#v, want unchanged %#v", got, base)
	}
	if len(updates) != 0 {
		t.Fatalf("loader() updates = %#v, want empty", updates)
	}
}

func TestChangedFlagsUpdateConfigAndCanonicalProvenance(t *testing.T) {
	flags := newFlagSet(t)
	if err := pflagloader.Register[pflagBasicConfig](flags); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := flags.Parse([]string{"--name=from-flag", "--host-name=db.example.com"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	loader, err := pflagloader.NewLoader[pflagBasicConfig](flags)
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}
	got, updates, err := loader(pflagBasicConfig{Name: "default", Debug: true, Ignored: "keep", Nested: pflagNestedConfig{Host: "localhost", Port: 80}})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}

	want := pflagBasicConfig{Name: "from-flag", Debug: true, Ignored: "keep", Nested: pflagNestedConfig{Host: "db.example.com", Port: 80}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loader() config = %#v, want %#v", got, want)
	}
	wantUpdates := configloader.Updates{
		"name":        pflagloader.SourcePFlag,
		"nested.host": pflagloader.SourcePFlag,
	}
	if !reflect.DeepEqual(updates, wantUpdates) {
		t.Fatalf("loader() updates = %#v, want %#v", updates, wantUpdates)
	}
	if _, ok := updates["host-name"]; ok {
		t.Fatalf("updates used config tag instead of canonical Go path")
	}
}

func TestStringSliceFlagsAppendValuesAndUpdateCanonicalProvenance(t *testing.T) {
	type config struct {
		Profiles []string `config:"profiles" help:"profile names"`
	}

	flags := newFlagSet(t)
	if err := pflagloader.Register[config](flags); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := flags.Parse([]string{"--profiles=flag-a, flag-b", "--profiles", "flag-c,flag-a"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	loader, err := pflagloader.NewLoader[config](flags)
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}
	got, updates, err := loader(config{Profiles: []string{"abc"}})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}

	want := config{Profiles: []string{"flag-a", "flag-b", "flag-c"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loader() config = %#v, want %#v", got, want)
	}
	if updates["profiles"] != pflagloader.SourcePFlag {
		t.Fatalf("updates[profiles] = %q, want SourcePFlag", updates["profiles"])
	}
}

type pflagAllScalarsConfig struct {
	String   string        `config:"string" help:"string"`
	Bool     bool          `config:"bool" help:"bool"`
	Int      int           `config:"int" help:"int"`
	Int8     int8          `config:"int8" help:"int8"`
	Int16    int16         `config:"int16" help:"int16"`
	Int32    int32         `config:"int32" help:"int32"`
	Int64    int64         `config:"int64" help:"int64"`
	Uint     uint          `config:"uint" help:"uint"`
	Uint8    uint8         `config:"uint8" help:"uint8"`
	Uint16   uint16        `config:"uint16" help:"uint16"`
	Uint32   uint32        `config:"uint32" help:"uint32"`
	Uint64   uint64        `config:"uint64" help:"uint64"`
	Uintptr  uintptr       `config:"uintptr" help:"uintptr"`
	Float32  float32       `config:"float32" help:"float32"`
	Float64  float64       `config:"float64" help:"float64"`
	Duration time.Duration `config:"duration" help:"duration"`
}

func TestAllSupportedScalarTypesParseFromFlags(t *testing.T) {
	flags := newFlagSet(t)
	if err := pflagloader.Register[pflagAllScalarsConfig](flags); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	args := []string{
		"--string=hello",
		"--bool=true",
		"--int=-1",
		"--int8=-8",
		"--int16=-16",
		"--int32=-32",
		"--int64=-64",
		"--uint=1",
		"--uint8=8",
		"--uint16=16",
		"--uint32=32",
		"--uint64=64",
		"--uintptr=99",
		"--float32=1.5",
		"--float64=2.25",
		"--duration=1h2m3s",
	}
	if err := flags.Parse(args); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	loader, err := pflagloader.NewLoader[pflagAllScalarsConfig](flags)
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}
	got, updates, err := loader(pflagAllScalarsConfig{})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}

	want := pflagAllScalarsConfig{
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
}

func TestParseFailureReturnsErrorWithoutPartialUpdate(t *testing.T) {
	flags := newFlagSet(t)
	if err := pflagloader.Register[pflagBasicConfig](flags); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := flags.Parse([]string{"--name=changed", "--debug=not-bool"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	loader, err := pflagloader.NewLoader[pflagBasicConfig](flags)
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}
	base := pflagBasicConfig{Name: "default"}
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

func TestMissingExpectedFlagsCauseLoaderErrors(t *testing.T) {
	flags := newFlagSet(t)
	flags.String("name", "", "registered manually")

	loader, err := pflagloader.NewLoader[pflagBasicConfig](flags)
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}
	if _, _, err := loader(pflagBasicConfig{}); err == nil {
		t.Fatalf("loader() error = nil for missing expected flags")
	}
}

func TestRootPackageDoesNotDependOnPflag(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", ".")
	cmd.Dir = ".."
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("go list -deps . error = %v", err)
	}
	for _, dep := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if dep == "github.com/spf13/pflag" {
			t.Fatalf("root package unexpectedly depends on github.com/spf13/pflag")
		}
	}
}
