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
	got, report, err := loader(pflagBasicConfig{})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if !got.Debug {
		t.Fatalf("Debug = false, want true")
	}
	if report.Updates["debug"] != pflagloader.SourcePFlag {
		t.Fatalf("report.Updates[debug] = %q, want SourcePFlag", report.Updates["debug"])
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
	got, report, err := loader(base)
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if !reflect.DeepEqual(got, base) {
		t.Fatalf("loader() config = %#v, want unchanged %#v", got, base)
	}
	if len(report.Updates) != 0 {
		t.Fatalf("loader() report.Updates = %#v, want empty", report.Updates)
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
	got, report, err := loader(pflagBasicConfig{Name: "default", Debug: true, Ignored: "keep", Nested: pflagNestedConfig{Host: "localhost", Port: 80}})
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
	if !reflect.DeepEqual(report.Updates, wantUpdates) {
		t.Fatalf("loader() report.Updates = %#v, want %#v", report.Updates, wantUpdates)
	}
	if _, ok := report.Updates["host-name"]; ok {
		t.Fatalf("report.Updates used config tag instead of canonical Go path")
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
	got, report, err := loader(config{Profiles: []string{"abc"}})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}

	want := config{Profiles: []string{"flag-a", "flag-b", "flag-c"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loader() config = %#v, want %#v", got, want)
	}
	if report.Updates["profiles"] != pflagloader.SourcePFlag {
		t.Fatalf("report.Updates[profiles] = %q, want SourcePFlag", report.Updates["profiles"])
	}
}

func TestExistingStringSliceVarLoadsParsedSliceValue(t *testing.T) {
	type config struct {
		Profiles []string `config:"profiles" help:"profile names"`
	}

	flags := newFlagSet(t)
	var profiles []string
	flags.StringSliceVar(&profiles, "profiles", nil, "profile names")
	if err := flags.Parse([]string{"--profiles=work,vpn", "--profiles=personal,work"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	loader, err := pflagloader.NewLoader[config](flags)
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}
	got, report, err := loader(config{Profiles: []string{"default"}})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}

	want := config{Profiles: []string{"work", "vpn", "personal"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loader() config = %#v, want %#v", got, want)
	}
	if report.Updates["profiles"] != pflagloader.SourcePFlag {
		t.Fatalf("report.Updates[profiles] = %q, want SourcePFlag", report.Updates["profiles"])
	}
	if len(report.Updates) != 1 {
		t.Fatalf("report.Updates = %#v, want only profiles update", report.Updates)
	}
}

func TestExistingStringSliceVarUnchangedDoesNotUpdate(t *testing.T) {
	type config struct {
		Profiles []string `config:"profiles" help:"profile names"`
	}

	flags := newFlagSet(t)
	var profiles []string
	flags.StringSliceVar(&profiles, "profiles", nil, "profile names")
	if err := flags.Parse(nil); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	loader, err := pflagloader.NewLoader[config](flags)
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}
	base := config{Profiles: []string{"default"}}
	got, report, err := loader(base)
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	if !reflect.DeepEqual(got, base) {
		t.Fatalf("loader() config = %#v, want unchanged %#v", got, base)
	}
	if len(report.Updates) != 0 {
		t.Fatalf("loader() report.Updates = %#v, want empty", report.Updates)
	}
}

func TestExistingStringSliceVarHonorsPflagCSVQuoting(t *testing.T) {
	type config struct {
		Profiles []string `config:"profiles" help:"profile names"`
	}

	flags := newFlagSet(t)
	var profiles []string
	flags.StringSliceVar(&profiles, "profiles", nil, "profile names")
	if err := flags.Parse([]string{`--profiles=work,"vpn,home"`}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	loader, err := pflagloader.NewLoader[config](flags)
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}
	got, _, err := loader(config{})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}

	want := config{Profiles: []string{"work", "vpn,home"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loader() config = %#v, want %#v", got, want)
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
	got, report, err := loader(pflagAllScalarsConfig{})
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
	if len(report.Updates) != 16 {
		t.Fatalf("len(report.Updates) = %d, want 16", len(report.Updates))
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
	got, report, err := loader(base)
	if err == nil {
		t.Fatalf("loader() error = nil")
	}
	if !reflect.DeepEqual(got, base) {
		t.Fatalf("loader() config = %#v, want unchanged %#v", got, base)
	}
	if report.Updates != nil {
		t.Fatalf("loader() report.Updates = %#v, want nil", report.Updates)
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

type pflagSingularProfilesConfig struct {
	Profiles []string `config:"profiles" pflag_singular:"profile" help:"profile names"`
}

func TestSingularStringFlagAppendsTrimmedRawValues(t *testing.T) {
	flags := newFlagSet(t)
	if err := pflagloader.Register[pflagSingularProfilesConfig](flags); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := flags.Parse([]string{"--profile", " alpha ", "--profile=a,b"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	loader, err := pflagloader.NewLoader[pflagSingularProfilesConfig](flags)
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}
	got, report, err := loader(pflagSingularProfilesConfig{})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}

	want := pflagSingularProfilesConfig{Profiles: []string{"alpha", "a,b"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loader() config = %#v, want %#v", got, want)
	}
	if report.Updates["profiles"] != pflagloader.SourcePFlag {
		t.Fatalf("report.Updates[profiles] = %q, want SourcePFlag", report.Updates["profiles"])
	}
}

func TestSingularStringFlagRejectsEmptyValues(t *testing.T) {
	tests := [][]string{
		{"--profile="},
		{"--profile=   "},
		{"--profile", ""},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			flags := newFlagSet(t)
			if err := pflagloader.Register[pflagSingularProfilesConfig](flags); err != nil {
				t.Fatalf("Register() error = %v", err)
			}
			if err := flags.Parse(args); err == nil {
				t.Fatalf("Parse(%#v) error = nil", args)
			}
		})
	}
}

func TestSingularIntFlagParsesOneScalarAndRejectsCSV(t *testing.T) {
	type config struct {
		IDs []int `config:"ids" pflag_singular:"id" help:"ids"`
	}

	flags := newFlagSet(t)
	if err := pflagloader.Register[config](flags); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := flags.Parse([]string{"--id=1", "--id", "2"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	loader, err := pflagloader.NewLoader[config](flags)
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}
	got, _, err := loader(config{})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	want := config{IDs: []int{1, 2}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loader() config = %#v, want %#v", got, want)
	}

	badFlags := newFlagSet(t)
	if err := pflagloader.Register[config](badFlags); err != nil {
		t.Fatalf("Register() badFlags error = %v", err)
	}
	if err := badFlags.Parse([]string{"--id=1,2"}); err != nil {
		t.Fatalf("Parse() badFlags error = %v", err)
	}
	badLoader, err := pflagloader.NewLoader[config](badFlags)
	if err != nil {
		t.Fatalf("NewLoader() badFlags error = %v", err)
	}
	if _, _, err := badLoader(config{}); err == nil {
		t.Fatalf("loader() error = nil for singular int CSV")
	}
}

func TestSingularDurationAndBoolFlagsParseOneScalar(t *testing.T) {
	type config struct {
		Timeouts []time.Duration `config:"timeouts" pflag_singular:"timeout" help:"timeouts"`
		Toggles  []bool          `config:"toggles" pflag_singular:"toggle" help:"toggles"`
	}

	flags := newFlagSet(t)
	if err := pflagloader.Register[config](flags); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := flags.Parse([]string{"--timeout=1s", "--toggle", "--toggle=false"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	loader, err := pflagloader.NewLoader[config](flags)
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}
	got, _, err := loader(config{})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	want := config{Timeouts: []time.Duration{time.Second}, Toggles: []bool{true, false}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loader() config = %#v, want %#v", got, want)
	}

	badFlags := newFlagSet(t)
	if err := pflagloader.Register[config](badFlags); err != nil {
		t.Fatalf("Register() badFlags error = %v", err)
	}
	if err := badFlags.Parse([]string{"--toggle="}); err == nil {
		t.Fatalf("Parse() error = nil for empty singular bool")
	}
}

func TestCanonicalAndSingularSliceFlagsCombineInWavesAndDedupe(t *testing.T) {
	flags := newFlagSet(t)
	if err := pflagloader.Register[pflagSingularProfilesConfig](flags); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := flags.Parse([]string{"--profile=c", "--profiles=a,b", "--profile=b", "--profiles=d,a"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	loader, err := pflagloader.NewLoader[pflagSingularProfilesConfig](flags)
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}
	got, _, err := loader(pflagSingularProfilesConfig{Profiles: []string{"default"}})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}

	want := pflagSingularProfilesConfig{Profiles: []string{"a", "b", "d", "c"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loader() config = %#v, want %#v", got, want)
	}
}

func TestSingularOnlyReplacesLowerPrioritySliceValues(t *testing.T) {
	flags := newFlagSet(t)
	if err := pflagloader.Register[pflagSingularProfilesConfig](flags); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := flags.Parse([]string{"--profile=flag"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	loader, err := pflagloader.NewLoader[pflagSingularProfilesConfig](flags)
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}
	got, _, err := loader(pflagSingularProfilesConfig{Profiles: []string{"default"}})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	want := pflagSingularProfilesConfig{Profiles: []string{"flag"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loader() config = %#v, want %#v", got, want)
	}
}

func TestCanonicalEmptySliceFlagCombinesWithSingularValues(t *testing.T) {
	flags := newFlagSet(t)
	if err := pflagloader.Register[pflagSingularProfilesConfig](flags); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := flags.Parse([]string{"--profiles=", "--profile=flag"}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	loader, err := pflagloader.NewLoader[pflagSingularProfilesConfig](flags)
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}
	got, _, err := loader(pflagSingularProfilesConfig{Profiles: []string{"default"}})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	want := pflagSingularProfilesConfig{Profiles: []string{"flag"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loader() config = %#v, want %#v", got, want)
	}
}

func TestCanonicalEmptySliceFlagAloneYieldsEmptySlice(t *testing.T) {
	flags := newFlagSet(t)
	if err := pflagloader.Register[pflagSingularProfilesConfig](flags); err != nil {
		t.Fatalf("Register() error = %v", err)
	}
	if err := flags.Parse([]string{"--profiles="}); err != nil {
		t.Fatalf("Parse() error = %v", err)
	}

	loader, err := pflagloader.NewLoader[pflagSingularProfilesConfig](flags)
	if err != nil {
		t.Fatalf("NewLoader() error = %v", err)
	}
	got, _, err := loader(pflagSingularProfilesConfig{Profiles: []string{"default"}})
	if err != nil {
		t.Fatalf("loader() error = %v", err)
	}
	want := pflagSingularProfilesConfig{Profiles: []string{}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("loader() config = %#v, want %#v", got, want)
	}
}

func TestSingularTagValidation(t *testing.T) {
	tests := []struct {
		name string
		err  func(*testing.T) error
	}{
		{
			name: "without config tag",
			err: func(t *testing.T) error {
				type config struct {
					Profiles []string `pflag_singular:"profile"`
				}
				return pflagloader.Register[config](newFlagSet(t))
			},
		},
		{
			name: "non slice field",
			err: func(t *testing.T) error {
				type config struct {
					Profile string `config:"profiles" pflag_singular:"profile" help:"profile"`
				}
				return pflagloader.Register[config](newFlagSet(t))
			},
		},
		{
			name: "unsupported slice element",
			err: func(t *testing.T) error {
				type config struct {
					Values [][]string `config:"values" pflag_singular:"value" help:"values"`
				}
				return pflagloader.Register[config](newFlagSet(t))
			},
		},
		{
			name: "nested in slice element",
			err: func(t *testing.T) error {
				type item struct {
					Values []string `config:"values" pflag_singular:"value" help:"values"`
				}
				type config struct {
					Items []item
				}
				return pflagloader.Register[config](newFlagSet(t))
			},
		},
		{
			name: "empty name",
			err: func(t *testing.T) error {
				type config struct {
					Profiles []string `config:"profiles" pflag_singular:"" help:"profiles"`
				}
				return pflagloader.Register[config](newFlagSet(t))
			},
		},
		{
			name: "dash name",
			err: func(t *testing.T) error {
				type config struct {
					Profiles []string `config:"profiles" pflag_singular:"-" help:"profiles"`
				}
				return pflagloader.Register[config](newFlagSet(t))
			},
		},
		{
			name: "non kebab name",
			err: func(t *testing.T) error {
				type config struct {
					Profiles []string `config:"profiles" pflag_singular:"Profile" help:"profiles"`
				}
				return pflagloader.Register[config](newFlagSet(t))
			},
		},
		{
			name: "same as canonical",
			err: func(t *testing.T) error {
				type config struct {
					Profiles []string `config:"profiles" pflag_singular:"profiles" help:"profiles"`
				}
				return pflagloader.Register[config](newFlagSet(t))
			},
		},
		{
			name: "collides with canonical",
			err: func(t *testing.T) error {
				type config struct {
					Profiles []string `config:"profiles" pflag_singular:"name" help:"profiles"`
					Name     string   `config:"name" help:"name"`
				}
				return pflagloader.Register[config](newFlagSet(t))
			},
		},
		{
			name: "collides with singular",
			err: func(t *testing.T) error {
				type config struct {
					Profiles []string `config:"profiles" pflag_singular:"item" help:"profiles"`
					IDs      []int    `config:"ids" pflag_singular:"item" help:"ids"`
				}
				return pflagloader.Register[config](newFlagSet(t))
			},
		},
		{
			name: "unexported field",
			err: func(t *testing.T) error {
				type config struct {
					_ []string `pflag_singular:"profile"`
				}
				return pflagloader.Register[config](newFlagSet(t))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.err(t); err == nil {
				t.Fatalf("Register() error = nil")
			}
		})
	}
}

func TestNewLoaderValidatesSingularTags(t *testing.T) {
	type config struct {
		Profiles []string `pflag_singular:"profile"`
	}
	if _, err := pflagloader.NewLoader[config](newFlagSet(t)); err == nil {
		t.Fatalf("NewLoader() error = nil")
	}
}

func TestValidateConfigIgnoresPFlagSingularTags(t *testing.T) {
	type config struct {
		Profiles []string `pflag_singular:"profile"`
	}
	if err := configloader.ValidateConfig[config](); err != nil {
		t.Fatalf("ValidateConfig() error = %v", err)
	}
	if err := pflagloader.Register[config](newFlagSet(t)); err == nil {
		t.Fatalf("Register() error = nil")
	}
}

func TestRegisterRejectsExistingSingularFlagWithoutPartialRegistration(t *testing.T) {
	flags := newFlagSet(t)
	flags.String("profile", "", "existing singular")
	if err := pflagloader.Register[pflagSingularProfilesConfig](flags); err == nil {
		t.Fatalf("Register() error = nil")
	}
	if flags.Lookup("profiles") != nil {
		t.Fatalf("Register() registered canonical flag after singular collision")
	}
}

func TestSingularHelpTextIncludesNote(t *testing.T) {
	flags := newFlagSet(t)
	if err := pflagloader.Register[pflagSingularProfilesConfig](flags); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	canonical := flags.Lookup("profiles")
	if canonical == nil || canonical.Usage != "profile names" || canonical.Value.Type() != "stringSlice" {
		t.Fatalf("canonical flag = %#v, want help and stringSlice type", canonical)
	}
	singular := flags.Lookup("profile")
	if singular == nil {
		t.Fatalf("singular flag was not registered")
	}
	if singular.Value.Type() != "string" {
		t.Fatalf("singular type = %q, want string", singular.Value.Type())
	}
	if !strings.Contains(singular.Usage, "profile names") || !strings.Contains(singular.Usage, "Adds a single value to the array; empty values are not allowed.") {
		t.Fatalf("singular Usage = %q, want help plus note", singular.Usage)
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
