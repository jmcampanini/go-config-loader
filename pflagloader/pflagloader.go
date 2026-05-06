package pflagloader

import (
	"fmt"
	"reflect"
	"time"

	configloader "github.com/jmcampanini/go-config-loader"
	"github.com/jmcampanini/go-config-loader/internal/configmeta"
	"github.com/spf13/pflag"
)

// Register registers one long pflag for every config-tagged scalar field of C.
func Register[C any](flags *pflag.FlagSet) error {
	if flags == nil {
		return fmt.Errorf("pflagloader: flag set is nil")
	}

	fields, err := configmeta.TaggedScalarFields[C]()
	if err != nil {
		return err
	}

	for _, field := range fields {
		if field.Help == "" {
			return fmt.Errorf("pflagloader: field %s with config tag %q is missing help tag", field.GoPath, field.ConfigTag)
		}
		if flags.Lookup(field.ConfigTag) != nil {
			return fmt.Errorf("pflagloader: flag %q already exists", field.ConfigTag)
		}
	}

	for _, field := range fields {
		flag := flags.VarPF(&scalarValue{typ: field.Type}, field.ConfigTag, "", field.Help)
		if field.Type.Kind() == reflect.Bool {
			flag.NoOptDefVal = "true"
		}
	}

	return nil
}

// NewLoader constructs a loader that overlays changed pflags onto a config value.
func NewLoader[C any](flags *pflag.FlagSet) (configloader.ConfigLoader[C], error) {
	if flags == nil {
		return nil, fmt.Errorf("pflagloader: flag set is nil")
	}

	fields, err := configmeta.TaggedScalarFields[C]()
	if err != nil {
		return nil, err
	}

	return func(base C) (C, configloader.Updates, error) {
		type parsedFlag struct {
			field configmeta.Field
			value reflect.Value
		}

		parsed := make([]parsedFlag, 0, len(fields))
		for _, field := range fields {
			flag := flags.Lookup(field.ConfigTag)
			if flag == nil {
				return base, nil, fmt.Errorf("pflagloader: expected flag %q for field %s is missing", field.ConfigTag, field.GoPath)
			}
			if !flag.Changed {
				continue
			}
			value, err := configmeta.ParseScalar(flag.Value.String(), field.Type)
			if err != nil {
				return base, nil, fmt.Errorf("pflagloader: flag %q for field %s: %w", field.ConfigTag, field.GoPath, err)
			}
			parsed = append(parsed, parsedFlag{field: field, value: value})
		}

		config := base
		root := reflect.ValueOf(&config).Elem()
		updates := make(configloader.Updates, len(parsed))
		for _, item := range parsed {
			root.FieldByIndex(item.field.Index).Set(item.value)
			updates[item.field.GoPath] = SourcePFlag
		}

		return config, updates, nil
	}, nil
}

type scalarValue struct {
	typ  reflect.Type
	text string
}

func (v *scalarValue) Set(text string) error {
	v.text = text
	return nil
}

func (v *scalarValue) String() string {
	if v == nil {
		return ""
	}
	return v.text
}

func (v *scalarValue) Type() string {
	if v == nil || v.typ == nil {
		return "value"
	}
	if v.typ == reflect.TypeOf(time.Duration(0)) {
		return "duration"
	}
	return v.typ.Kind().String()
}

func (v *scalarValue) IsBoolFlag() bool {
	return v != nil && v.typ != nil && v.typ.Kind() == reflect.Bool
}
