package configmeta

import (
	"encoding"
	"fmt"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// kebabRE is the accepted spelling for config tags, env prefixes, and app names.
// This is the Go regexp equivalent of the spec's lookahead-based pattern.
var kebabRE = regexp.MustCompile(`^[a-z](?:[a-z0-9]|-[a-z0-9])*$`)

var (
	durationType        = reflect.TypeOf(time.Duration(0))
	timeType            = reflect.TypeOf(time.Time{})
	textUnmarshalerType = reflect.TypeOf((*encoding.TextUnmarshaler)(nil)).Elem()
)

// Field describes a supported scalar field with a direct config tag.
type Field struct {
	GoPath    string
	ConfigTag string
	Help      string
	Index     []int
	Type      reflect.Type
}

// IsKebabCase reports whether s matches the config tag/env prefix spelling.
func IsKebabCase(s string) bool {
	return kebabRE.MatchString(s)
}

// ValidateConfigType validates the generic config type C.
func ValidateConfigType[C any]() error {
	var zero C
	t := reflect.TypeOf(zero)
	if t == nil {
		return fmt.Errorf("configmeta: config type must be a non-pointer struct, got <nil>")
	}
	return ValidateType(t)
}

// ValidateType validates a concrete config type.
func ValidateType(t reflect.Type) error {
	if t == nil {
		return fmt.Errorf("configmeta: config type must be a non-pointer struct, got <nil>")
	}
	if t.Kind() != reflect.Struct {
		return fmt.Errorf("configmeta: config type must be a non-pointer struct, got %s", t)
	}
	if t == timeType {
		return fmt.Errorf("configmeta: config type %s is not supported", t)
	}

	state := validationState{
		tags: make(map[string]string),
	}
	return state.validateStruct(t, "")
}

// TaggedScalarFields returns all exported scalar fields with direct config tags.
func TaggedScalarFields[C any]() ([]Field, error) {
	var zero C
	t := reflect.TypeOf(zero)
	if err := ValidateType(t); err != nil {
		return nil, err
	}

	var fields []Field
	collectTaggedScalars(t, "", nil, &fields)
	return fields, nil
}

// ConcreteLeafPaths returns canonical leaf paths for concrete config values.
// It validates runtime map keys and rejects empty map keys.
func ConcreteLeafPaths[C any](cfg C) ([]string, error) {
	if err := ValidateConfigType[C](); err != nil {
		return nil, err
	}
	return ConcreteLeafPathsOf(reflect.ValueOf(cfg))
}

// ConcreteLeafPathsOf returns canonical leaf paths for v. v must be a valid config struct value.
func ConcreteLeafPathsOf(v reflect.Value) ([]string, error) {
	return concreteLeafPathsOf(v)
}

// DefaultLeafPathsOf returns canonical leaf paths that should receive default provenance.
// v must be a valid config struct value that has already been type-validated.
func DefaultLeafPathsOf(v reflect.Value) ([]string, error) {
	return concreteLeafPathsOf(v)
}

func concreteLeafPathsOf(v reflect.Value) ([]string, error) {
	if !v.IsValid() {
		return nil, fmt.Errorf("configmeta: invalid config value")
	}
	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("configmeta: config value must be a struct, got %s", v.Type())
	}
	var paths []string
	if err := collectConcreteLeafPaths(v, "", &paths); err != nil {
		return nil, err
	}
	return paths, nil
}

type validationState struct {
	tags  map[string]string
	stack []reflect.Type
}

func (s *validationState) validate(t reflect.Type, path string, field *reflect.StructField) error {
	if t == timeType {
		return fmt.Errorf("configmeta: field %s uses unsupported type time.Time", displayPath(path))
	}
	if t == durationType {
		return nil
	}
	if implementsTextUnmarshaler(t) {
		return fmt.Errorf("configmeta: field %s uses unsupported encoding.TextUnmarshaler type %s", displayPath(path), t)
	}

	if field != nil {
		if tag, ok := field.Tag.Lookup("config"); ok {
			if err := s.validateConfigTag(tag, path, t); err != nil {
				return err
			}
		}
	}

	switch t.Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return nil
	case reflect.Struct:
		return s.validateStruct(t, path)
	case reflect.Slice, reflect.Array:
		return s.validate(t.Elem(), path+"[]", nil)
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return fmt.Errorf("configmeta: field %s uses unsupported map key type %s", displayPath(path), t.Key())
		}
		return s.validate(t.Elem(), path+"[\"...\"]", nil)
	case reflect.Pointer, reflect.Interface, reflect.Func, reflect.Chan, reflect.UnsafePointer:
		return fmt.Errorf("configmeta: field %s uses unsupported type %s", displayPath(path), t)
	default:
		return fmt.Errorf("configmeta: field %s uses unsupported type %s", displayPath(path), t)
	}
}

func (s *validationState) validateStruct(t reflect.Type, path string) error {
	for _, seen := range s.stack {
		if seen == t {
			return fmt.Errorf("configmeta: recursive config type %s is not supported", t)
		}
	}
	s.stack = append(s.stack, t)
	defer func() { s.stack = s.stack[:len(s.stack)-1] }()

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		fieldPath := joinPath(path, strings.ToLower(f.Name))

		if f.Anonymous {
			return fmt.Errorf("configmeta: anonymous embedded field %s is not supported", displayPath(fieldPath))
		}

		exported := f.PkgPath == ""
		if !exported {
			if tag, ok := f.Tag.Lookup("config"); ok {
				if tag == "" {
					return fmt.Errorf("configmeta: unexported field %s has empty config tag", displayPath(fieldPath))
				}
				return fmt.Errorf("configmeta: unexported field %s has config tag %q", displayPath(fieldPath), tag)
			}
			continue
		}

		if err := s.validate(f.Type, fieldPath, &f); err != nil {
			return err
		}
	}
	return nil
}

func (s *validationState) validateConfigTag(tag, path string, t reflect.Type) error {
	if tag == "" {
		return fmt.Errorf("configmeta: field %s has empty config tag", displayPath(path))
	}
	if tag == "-" {
		return fmt.Errorf("configmeta: field %s has invalid config tag %q", displayPath(path), tag)
	}
	if !IsKebabCase(tag) {
		return fmt.Errorf("configmeta: field %s has invalid config tag %q", displayPath(path), tag)
	}
	if !IsScalar(t) {
		return fmt.Errorf("configmeta: field %s has config tag on unsupported type %s", displayPath(path), t)
	}
	if prev, exists := s.tags[tag]; exists {
		return fmt.Errorf("configmeta: duplicate config tag %q on fields %s and %s", tag, prev, displayPath(path))
	}
	s.tags[tag] = displayPath(path)
	return nil
}

// IsScalar reports whether t is a supported scalar env/pflag type.
func IsScalar(t reflect.Type) bool {
	if t == durationType {
		return true
	}
	if t == timeType || implementsTextUnmarshaler(t) {
		return false
	}
	switch t.Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}

// ParseScalar parses text into a reflect.Value of the supported scalar type t.
func ParseScalar(text string, t reflect.Type) (reflect.Value, error) {
	if t == durationType {
		value, err := time.ParseDuration(text)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("parse %q as %s: %w", text, t, err)
		}
		return reflect.ValueOf(value), nil
	}

	switch t.Kind() {
	case reflect.String:
		return reflect.ValueOf(text).Convert(t), nil
	case reflect.Bool:
		value, err := strconv.ParseBool(text)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("parse %q as %s: %w", text, t, err)
		}
		return reflect.ValueOf(value).Convert(t), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := strconv.ParseInt(text, 10, t.Bits())
		if err != nil {
			return reflect.Value{}, fmt.Errorf("parse %q as %s: %w", text, t, err)
		}
		parsed := reflect.New(t).Elem()
		parsed.SetInt(value)
		return parsed, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		value, err := strconv.ParseUint(text, 10, t.Bits())
		if err != nil {
			return reflect.Value{}, fmt.Errorf("parse %q as %s: %w", text, t, err)
		}
		parsed := reflect.New(t).Elem()
		parsed.SetUint(value)
		return parsed, nil
	case reflect.Float32, reflect.Float64:
		value, err := strconv.ParseFloat(text, t.Bits())
		if err != nil {
			return reflect.Value{}, fmt.Errorf("parse %q as %s: %w", text, t, err)
		}
		parsed := reflect.New(t).Elem()
		parsed.SetFloat(value)
		return parsed, nil
	default:
		return reflect.Value{}, fmt.Errorf("type %s is not a supported scalar", t)
	}
}

func collectTaggedScalars(t reflect.Type, prefix string, index []int, out *[]Field) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" || f.Anonymous {
			continue
		}
		path := joinPath(prefix, strings.ToLower(f.Name))
		fieldIndex := append(append([]int(nil), index...), i)
		if tag := f.Tag.Get("config"); tag != "" {
			*out = append(*out, Field{GoPath: path, ConfigTag: tag, Help: f.Tag.Get("help"), Index: fieldIndex, Type: f.Type})
			continue
		}
		if f.Type.Kind() == reflect.Struct && f.Type != durationType && f.Type != timeType {
			collectTaggedScalars(f.Type, path, fieldIndex, out)
		}
	}
}

func collectConcreteLeafPaths(v reflect.Value, prefix string, out *[]string) error {
	t := v.Type()
	if IsScalar(t) || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		if prefix != "" {
			*out = append(*out, prefix)
		}
		return nil
	}

	switch t.Kind() {
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.PkgPath != "" || f.Anonymous {
				continue
			}
			path := joinPath(prefix, strings.ToLower(f.Name))
			if err := collectConcreteLeafPaths(v.Field(i), path, out); err != nil {
				return err
			}
		}
		return nil
	case reflect.Map:
		if v.IsNil() || v.Len() == 0 {
			return nil
		}
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
		for _, key := range keys {
			keyString := key.String()
			if keyString == "" {
				return fmt.Errorf("configmeta: field %s contains empty map key", displayPath(prefix))
			}
			path := prefix + "[" + strconv.Quote(keyString) + "]"
			if err := collectConcreteLeafPaths(v.MapIndex(key), path, out); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("configmeta: field %s uses unsupported type %s", displayPath(prefix), t)
	}
}

func implementsTextUnmarshaler(t reflect.Type) bool {
	if t == nil {
		return false
	}
	if t.Implements(textUnmarshalerType) {
		return true
	}
	if t.Kind() != reflect.Pointer {
		return reflect.PointerTo(t).Implements(textUnmarshalerType)
	}
	return false
}

func joinPath(prefix, segment string) string {
	if prefix == "" {
		return segment
	}
	return prefix + "." + segment
}

func displayPath(path string) string {
	if path == "" {
		return "<root>"
	}
	return path
}
