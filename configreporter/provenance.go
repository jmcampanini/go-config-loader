package configreporter

import (
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

const unavailableProvenanceValue = "<unavailable>"

var durationType = reflect.TypeOf(time.Duration(0))

func formatProvenancePathValue[C any](config C, path string) string {
	value, ok := resolveProvenancePath(reflect.ValueOf(config), path)
	if !ok {
		return unavailableProvenanceValue
	}
	return formatProvenanceValue(value)
}

func resolveProvenancePath(root reflect.Value, path string) (reflect.Value, bool) {
	if path == "" {
		return reflect.Value{}, false
	}

	value := root
	pos := 0
	for pos < len(path) {
		switch path[pos] {
		case '[':
			key, next, ok := parseMapSelector(path, pos)
			if !ok {
				return reflect.Value{}, false
			}
			var resolved bool
			value, resolved = resolveMapKey(value, key)
			if !resolved {
				return reflect.Value{}, false
			}
			pos = next
		case '.':
			return reflect.Value{}, false
		default:
			segment, next := parsePathSegment(path, pos)
			if segment == "" {
				return reflect.Value{}, false
			}
			var resolved bool
			value, resolved = resolveStructField(value, segment)
			if !resolved {
				return reflect.Value{}, false
			}
			pos = next
		}

		if pos == len(path) {
			return value, true
		}

		switch path[pos] {
		case '[':
			continue
		case '.':
			pos++
			if pos == len(path) || path[pos] == '[' {
				return reflect.Value{}, false
			}
		default:
			return reflect.Value{}, false
		}
	}

	return reflect.Value{}, false
}

func parsePathSegment(path string, pos int) (string, int) {
	start := pos
	for pos < len(path) && path[pos] != '.' && path[pos] != '[' {
		pos++
	}
	return path[start:pos], pos
}

func parseMapSelector(path string, pos int) (string, int, bool) {
	if pos >= len(path) || path[pos] != '[' || pos+1 >= len(path) || path[pos+1] != '"' {
		return "", 0, false
	}

	quoteEnd := pos + 2
	escaped := false
	for quoteEnd < len(path) {
		char := path[quoteEnd]
		if escaped {
			escaped = false
			quoteEnd++
			continue
		}
		if char == '\\' {
			escaped = true
			quoteEnd++
			continue
		}
		if char == '"' {
			break
		}
		quoteEnd++
	}
	if quoteEnd >= len(path) || quoteEnd+1 >= len(path) || path[quoteEnd+1] != ']' {
		return "", 0, false
	}

	key, err := strconv.Unquote(path[pos+1 : quoteEnd+1])
	if err != nil {
		return "", 0, false
	}
	return key, quoteEnd + 2, true
}

func resolveStructField(value reflect.Value, segment string) (reflect.Value, bool) {
	value = indirectValue(value)
	if !value.IsValid() || value.Kind() != reflect.Struct {
		return reflect.Value{}, false
	}

	typ := value.Type()
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" || field.Anonymous {
			continue
		}
		if strings.ToLower(field.Name) == segment {
			return value.Field(i), true
		}
	}
	return reflect.Value{}, false
}

func resolveMapKey(value reflect.Value, key string) (reflect.Value, bool) {
	value = indirectValue(value)
	if !value.IsValid() || value.Kind() != reflect.Map || value.Type().Key().Kind() != reflect.String || value.IsNil() {
		return reflect.Value{}, false
	}

	keyValue := reflect.ValueOf(key).Convert(value.Type().Key())
	mapValue := value.MapIndex(keyValue)
	if !mapValue.IsValid() {
		return reflect.Value{}, false
	}
	return mapValue, true
}

func indirectValue(value reflect.Value) reflect.Value {
	for value.IsValid() {
		switch value.Kind() {
		case reflect.Interface, reflect.Pointer:
			if value.IsNil() {
				return reflect.Value{}
			}
			value = value.Elem()
		default:
			return value
		}
	}
	return value
}

func formatProvenanceValue(value reflect.Value) string {
	if !value.IsValid() {
		return unavailableProvenanceValue
	}

	for value.Kind() == reflect.Interface || value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return "<nil>"
		}
		value = value.Elem()
	}

	if value.Type() == durationType {
		return strconv.Quote(time.Duration(value.Int()).String())
	}

	switch value.Kind() {
	case reflect.String:
		return strconv.Quote(value.String())
	case reflect.Bool:
		return strconv.FormatBool(value.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(value.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(value.Uint(), 10)
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(value.Float(), 'g', -1, value.Type().Bits())
	case reflect.Slice, reflect.Array:
		return formatProvenanceList(value)
	case reflect.Map:
		return formatProvenanceMap(value)
	case reflect.Struct:
		return formatProvenanceStruct(value)
	default:
		return unavailableProvenanceValue
	}
}

func formatProvenanceList(value reflect.Value) string {
	if value.Len() == 0 {
		return "[]"
	}

	items := make([]string, 0, value.Len())
	for i := 0; i < value.Len(); i++ {
		items = append(items, formatProvenanceValue(value.Index(i)))
	}
	return "[" + strings.Join(items, ", ") + "]"
}

func formatProvenanceMap(value reflect.Value) string {
	if value.IsNil() || value.Len() == 0 {
		return "{}"
	}
	if value.Type().Key().Kind() != reflect.String {
		return unavailableProvenanceValue
	}

	keys := value.MapKeys()
	sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })

	items := make([]string, 0, len(keys))
	for _, key := range keys {
		items = append(items, strconv.Quote(key.String())+" = "+formatProvenanceValue(value.MapIndex(key)))
	}
	return "{" + strings.Join(items, ", ") + "}"
}

func formatProvenanceStruct(value reflect.Value) string {
	typ := value.Type()
	items := make([]string, 0, typ.NumField())
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if field.PkgPath != "" || field.Anonymous {
			continue
		}
		items = append(items, strings.ToLower(field.Name)+" = "+formatProvenanceValue(value.Field(i)))
	}
	if len(items) == 0 {
		return "{}"
	}
	return "{" + strings.Join(items, ", ") + "}"
}
