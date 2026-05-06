package configloader_test

import (
	"encoding"
	"testing"
	"time"
	"unsafe"

	configloader "github.com/jmcampanini/go-config-loader"
)

var _ encoding.TextUnmarshaler = (*customText)(nil)

type customText string

func (*customText) UnmarshalText([]byte) error { return nil }

type validateNested struct {
	Host string `config:"host"`
}

type validateMapValue struct {
	Host string
}

type validateValidConfig struct {
	Name     string        `config:"name"`
	Debug    bool          `config:"debug"`
	Port     int           `config:"port"`
	Timeout  time.Duration `config:"timeout"`
	Nested   validateNested
	Values   []int `config:"values"`
	Pair     [2]string
	Labels   map[string]string
	Backends map[string]validateMapValue
	_        string
}

type validatePointerField struct{ P *int }
type validateInterfaceField struct{ I any }
type validateFuncField struct{ F func() }
type validateChanField struct{ C chan int }
type validateUnsafePointerField struct{ P unsafe.Pointer }
type validateTimeField struct{ T time.Time }
type validateEmbeddedField struct{ validateNested }
type validateNonStringMapKey struct{ M map[int]string }
type validateTaggedStruct struct {
	Nested validateNested `config:"nested"`
}
type validateTaggedSlice struct {
	Values []int `config:"values"`
}
type validateTaggedArray struct {
	Values [2]int `config:"values"`
}
type validateTaggedMap struct {
	Labels map[string]string `config:"labels"`
}
type validateTaggedPointer struct {
	P *int `config:"ptr"`
}
type validateTaggedInterface struct {
	I any `config:"iface"`
}
type validateDuplicateTags struct {
	A string `config:"dup"`
	B int    `config:"dup"`
}
type validateInvalidTagUpper struct {
	A string `config:"API_URL"`
}
type validateInvalidTagDash struct {
	A string `config:"-"`
}
type validateInvalidTagEmpty struct {
	A string `config:""`
}
type validateInvalidTagDoubleDash struct {
	A string `config:"api--url"`
}
type validateUnexportedTagged struct {
	_ string `config:"hidden"`
}
type validateTextUnmarshaler struct{ C customText }

func TestValidateConfigValidShape(t *testing.T) {
	if err := configloader.ValidateConfig[validateValidConfig](); err != nil {
		t.Fatalf("ValidateConfig(valid) error = %v", err)
	}
}

func TestValidateConfigRejectsInvalidShapes(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "pointer root", err: configloader.ValidateConfig[*validateValidConfig]()},
		{name: "exported pointer field", err: configloader.ValidateConfig[validatePointerField]()},
		{name: "interface field", err: configloader.ValidateConfig[validateInterfaceField]()},
		{name: "function field", err: configloader.ValidateConfig[validateFuncField]()},
		{name: "channel field", err: configloader.ValidateConfig[validateChanField]()},
		{name: "unsafe pointer field", err: configloader.ValidateConfig[validateUnsafePointerField]()},
		{name: "time.Time field", err: configloader.ValidateConfig[validateTimeField]()},
		{name: "anonymous embedded field", err: configloader.ValidateConfig[validateEmbeddedField]()},
		{name: "non-string map key", err: configloader.ValidateConfig[validateNonStringMapKey]()},
		{name: "tagged struct", err: configloader.ValidateConfig[validateTaggedStruct]()},
		{name: "tagged array", err: configloader.ValidateConfig[validateTaggedArray]()},
		{name: "tagged map", err: configloader.ValidateConfig[validateTaggedMap]()},
		{name: "tagged pointer", err: configloader.ValidateConfig[validateTaggedPointer]()},
		{name: "tagged interface", err: configloader.ValidateConfig[validateTaggedInterface]()},
		{name: "duplicate config tags", err: configloader.ValidateConfig[validateDuplicateTags]()},
		{name: "invalid upper tag", err: configloader.ValidateConfig[validateInvalidTagUpper]()},
		{name: "invalid dash tag", err: configloader.ValidateConfig[validateInvalidTagDash]()},
		{name: "invalid empty tag", err: configloader.ValidateConfig[validateInvalidTagEmpty]()},
		{name: "invalid double dash tag", err: configloader.ValidateConfig[validateInvalidTagDoubleDash]()},
		{name: "unexported tagged field", err: configloader.ValidateConfig[validateUnexportedTagged]()},
		{name: "text unmarshaler field", err: configloader.ValidateConfig[validateTextUnmarshaler]()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatalf("ValidateConfig() error = nil")
			}
		})
	}
}
