package configloader

import (
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/jmcampanini/go-config-loader/internal/configmeta"
)

// NewEnvironmentLoader constructs a loader that reads config-tagged scalar and scalar-slice fields from env.
func NewEnvironmentLoader[C any](prefix string, env map[string]string) (ConfigLoader[C], error) {
	if !configmeta.IsKebabCase(prefix) {
		return nil, fmt.Errorf("configloader: invalid environment prefix %q", prefix)
	}

	fields, err := configmeta.TaggedScalarFields[C]()
	if err != nil {
		return nil, err
	}

	envCopy := make(map[string]string, len(env))
	for key, value := range env {
		envCopy[key] = value
	}

	type envField struct {
		field configmeta.Field
		name  string
	}
	tracked := make([]envField, len(fields))
	for i, field := range fields {
		name := strings.ToUpper(prefix + "_" + field.ConfigTag)
		name = strings.ReplaceAll(name, "-", "_")
		tracked[i] = envField{field: field, name: name}
	}

	return func(base C) (C, LoadReport, error) {
		type parsedEnv struct {
			field configmeta.Field
			value reflect.Value
		}

		parsed := make([]parsedEnv, 0, len(tracked))
		for _, item := range tracked {
			raw, ok := envCopy[item.name]
			if !ok {
				continue
			}
			value, err := configmeta.ParseText(raw, item.field.Type)
			if err != nil {
				return base, LoadReport{}, fmt.Errorf("configloader: environment variable %s for field %s: %w", item.name, item.field.GoPath, err)
			}
			parsed = append(parsed, parsedEnv{field: item.field, value: value})
		}

		config := base
		root := reflect.ValueOf(&config).Elem()
		updates := make(Updates, len(parsed))
		for _, item := range parsed {
			root.FieldByIndex(item.field.Index).Set(item.value)
			updates[item.field.GoPath] = SourceEnv
		}

		return config, LoadReport{Updates: updates}, nil
	}, nil
}

// OSEnv returns the current process environment as a map.
func OSEnv() map[string]string {
	env := make(map[string]string)
	for _, entry := range os.Environ() {
		for i := 0; i < len(entry); i++ {
			if entry[i] == '=' {
				env[entry[:i]] = entry[i+1:]
				break
			}
		}
	}
	return env
}
