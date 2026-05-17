package pflagloader

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	configloader "github.com/jmcampanini/go-config-loader"
	"github.com/jmcampanini/go-config-loader/internal/configmeta"
	"github.com/spf13/pflag"
)

const singularHelpSuffix = "Adds a single value to the array; empty values are not allowed."

// Register registers canonical long pflags and any pflag_singular aliases for C.
func Register[C any](flags *pflag.FlagSet) error {
	if flags == nil {
		return fmt.Errorf("pflagloader: flag set is nil")
	}

	fields, err := taggedPFlagFields[C]()
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
		if field.SingularTag != "" && flags.Lookup(field.SingularTag) != nil {
			return fmt.Errorf("pflagloader: flag %q already exists", field.SingularTag)
		}
	}

	for _, field := range fields {
		flag := flags.VarPF(newFlagValue(field.Type), field.ConfigTag, "", field.Help)
		if field.Type.Kind() == reflect.Bool {
			flag.NoOptDefVal = "true"
		}

		if field.SingularTag != "" {
			singularFlag := flags.VarPF(newSingularFlagValue(field.Type.Elem()), field.SingularTag, "", singularHelp(field.Help))
			if field.Type.Elem().Kind() == reflect.Bool {
				singularFlag.NoOptDefVal = "true"
			}
		}
	}

	return nil
}

// NewLoader constructs a loader that overlays changed pflags onto a config value.
func NewLoader[C any](flags *pflag.FlagSet) (configloader.ConfigLoader[C], error) {
	if flags == nil {
		return nil, fmt.Errorf("pflagloader: flag set is nil")
	}

	fields, err := taggedPFlagFields[C]()
	if err != nil {
		return nil, err
	}

	return func(base C) (C, configloader.LoadReport, error) {
		type parsedFlag struct {
			field pflagField
			value reflect.Value
		}

		parsed := make([]parsedFlag, 0, len(fields))
		for _, field := range fields {
			canonicalFlag := flags.Lookup(field.ConfigTag)
			if canonicalFlag == nil {
				return base, configloader.LoadReport{}, fmt.Errorf("pflagloader: expected flag %q for field %s is missing", field.ConfigTag, field.GoPath)
			}

			var singularFlag *pflag.Flag
			if field.SingularTag != "" {
				singularFlag = flags.Lookup(field.SingularTag)
				if singularFlag == nil {
					return base, configloader.LoadReport{}, fmt.Errorf("pflagloader: expected flag %q for field %s is missing", field.SingularTag, field.GoPath)
				}
			}

			if !canonicalFlag.Changed && (singularFlag == nil || !singularFlag.Changed) {
				continue
			}

			value, err := parseFlagValue(field, canonicalFlag, singularFlag)
			if err != nil {
				return base, configloader.LoadReport{}, err
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

		return config, configloader.LoadReport{Updates: updates}, nil
	}, nil
}

type pflagField struct {
	configmeta.Field
	SingularTag string
}

func taggedPFlagFields[C any]() ([]pflagField, error) {
	var zero C
	t := reflect.TypeOf(zero)
	if t == nil || t.Kind() != reflect.Struct {
		return nil, configmeta.ValidateType(t)
	}

	singularTags := make(map[[2]string]string)
	if err := collectSingularTags(t, "", true, nil, singularTags); err != nil {
		return nil, err
	}

	canonicalFields, err := configmeta.TaggedScalarFields[C]()
	if err != nil {
		return nil, err
	}

	fields := make([]pflagField, len(canonicalFields))
	for i, field := range canonicalFields {
		fields[i] = pflagField{Field: field, SingularTag: singularTags[[2]string{field.GoPath, field.ConfigTag}]}
	}
	if err := validatePFlagNameCollisions(fields); err != nil {
		return nil, err
	}
	return fields, nil
}

func collectSingularTags(t reflect.Type, prefix string, addressable bool, stack []reflect.Type, out map[[2]string]string) error {
	if t == reflect.TypeOf(time.Time{}) {
		return nil
	}
	for _, seen := range stack {
		if seen == t {
			return nil
		}
	}
	stack = append(stack, t)

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		path := joinPath(prefix, strings.ToLower(f.Name))
		singularTag, hasSingular := f.Tag.Lookup("pflag_singular")

		if f.PkgPath != "" {
			if hasSingular {
				return fmt.Errorf("pflagloader: unexported field %s has pflag_singular tag", displayPath(path))
			}
			continue
		}

		configTag, hasConfig := f.Tag.Lookup("config")
		if hasSingular {
			if !addressable {
				return fmt.Errorf("pflagloader: field %s has pflag_singular tag in unsupported nested type", displayPath(path))
			}
			if !hasConfig {
				return fmt.Errorf("pflagloader: field %s has pflag_singular tag without config tag", displayPath(path))
			}
			if err := validateSingularTag(path, configTag, singularTag, f.Type); err != nil {
				return err
			}
			out[[2]string{path, configTag}] = singularTag
		}

		if !hasConfig {
			if err := collectNestedSingularTags(f.Type, path, addressable, stack, out); err != nil {
				return err
			}
		}
	}
	return nil
}

func collectNestedSingularTags(t reflect.Type, prefix string, addressable bool, stack []reflect.Type, out map[[2]string]string) error {
	switch t.Kind() {
	case reflect.Struct:
		return collectSingularTags(t, prefix, addressable, stack, out)
	case reflect.Slice, reflect.Array:
		return collectNestedSingularTags(t.Elem(), prefix+"[]", false, stack, out)
	case reflect.Map:
		return collectNestedSingularTags(t.Elem(), prefix+"[\"...\"]", false, stack, out)
	default:
		return nil
	}
}

func validateSingularTag(path, configTag, singularTag string, typ reflect.Type) error {
	if singularTag == "" {
		return fmt.Errorf("pflagloader: field %s has empty pflag_singular tag", displayPath(path))
	}
	if singularTag == "-" {
		return fmt.Errorf("pflagloader: field %s has invalid pflag_singular tag %q", displayPath(path), singularTag)
	}
	if !configmeta.IsKebabCase(singularTag) {
		return fmt.Errorf("pflagloader: field %s has invalid pflag_singular tag %q", displayPath(path), singularTag)
	}
	if typ.Kind() != reflect.Slice {
		return fmt.Errorf("pflagloader: field %s has pflag_singular tag on non-slice type %s", displayPath(path), typ)
	}
	if !configmeta.IsScalar(typ.Elem()) {
		return fmt.Errorf("pflagloader: field %s has pflag_singular tag on unsupported slice element type %s", displayPath(path), typ.Elem())
	}
	if singularTag == configTag {
		return fmt.Errorf("pflagloader: field %s has pflag_singular tag %q matching its config tag", displayPath(path), singularTag)
	}
	return nil
}

func validatePFlagNameCollisions(fields []pflagField) error {
	canonicalNames := make(map[string]pflagField, len(fields))
	for _, field := range fields {
		canonicalNames[field.ConfigTag] = field
	}

	singularNames := make(map[string]pflagField)
	for _, field := range fields {
		if field.SingularTag == "" {
			continue
		}
		if owner, ok := canonicalNames[field.SingularTag]; ok {
			return fmt.Errorf("pflagloader: pflag_singular tag %q on field %s collides with config tag on field %s", field.SingularTag, field.GoPath, owner.GoPath)
		}
		if prev, ok := singularNames[field.SingularTag]; ok {
			return fmt.Errorf("pflagloader: duplicate pflag_singular tag %q on fields %s and %s", field.SingularTag, prev.GoPath, field.GoPath)
		}
		singularNames[field.SingularTag] = field
	}
	return nil
}

func parseFlagValue(field pflagField, canonicalFlag, singularFlag *pflag.Flag) (reflect.Value, error) {
	if field.SingularTag == "" {
		value, err := configmeta.ParseText(canonicalFlag.Value.String(), field.Type)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("pflagloader: flag %q for field %s: %w", field.ConfigTag, field.GoPath, err)
		}
		return value, nil
	}

	combined := reflect.MakeSlice(field.Type, 0, 0)
	if canonicalFlag.Changed {
		canonicalValue, err := configmeta.ParseText(canonicalFlag.Value.String(), field.Type)
		if err != nil {
			return reflect.Value{}, fmt.Errorf("pflagloader: flag %q for field %s: %w", field.ConfigTag, field.GoPath, err)
		}
		combined = reflect.AppendSlice(combined, canonicalValue)
	}

	if singularFlag != nil && singularFlag.Changed {
		texts, err := singularTexts(singularFlag)
		if err != nil {
			return reflect.Value{}, err
		}
		for _, text := range texts {
			value, err := configmeta.ParseScalar(text, field.Type.Elem())
			if err != nil {
				return reflect.Value{}, fmt.Errorf("pflagloader: flag %q for field %s: %w", field.SingularTag, field.GoPath, err)
			}
			combined = reflect.Append(combined, value)
		}
	}

	return dedupeSlice(combined), nil
}

func dedupeSlice(value reflect.Value) reflect.Value {
	if value.Len() < 2 {
		return value
	}

	seen := make(map[any]struct{}, value.Len())
	deduped := reflect.MakeSlice(value.Type(), 0, value.Len())
	for i := 0; i < value.Len(); i++ {
		item := value.Index(i)
		key := item.Interface()
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		deduped = reflect.Append(deduped, item)
	}
	return deduped
}

func singularTexts(flag *pflag.Flag) ([]string, error) {
	value, ok := flag.Value.(interface{ Values() []string })
	if !ok {
		return nil, fmt.Errorf("pflagloader: flag %q was not registered by pflagloader.Register", flag.Name)
	}
	return value.Values(), nil
}

func singularHelp(help string) string {
	return help + " " + singularHelpSuffix
}

func newFlagValue(typ reflect.Type) pflag.Value {
	if typ.Kind() == reflect.Slice {
		return &sliceValue{typ: typ}
	}
	return &scalarValue{typ: typ}
}

func newSingularFlagValue(elem reflect.Type) pflag.Value {
	return &singularValue{elem: elem}
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
	return scalarTypeName(v.typ)
}

func (v *scalarValue) IsBoolFlag() bool {
	return v != nil && v.typ != nil && v.typ.Kind() == reflect.Bool
}

type sliceValue struct {
	typ   reflect.Type
	texts []string
}

func (v *sliceValue) Set(text string) error {
	v.texts = append(v.texts, text)
	return nil
}

func (v *sliceValue) String() string {
	if v == nil {
		return ""
	}
	return strings.Join(v.texts, ",")
}

func (v *sliceValue) Type() string {
	if v == nil || v.typ == nil || v.typ.Kind() != reflect.Slice {
		return "slice"
	}
	return scalarTypeName(v.typ.Elem()) + "Slice"
}

type singularValue struct {
	elem  reflect.Type
	texts []string
}

func (v *singularValue) Set(text string) error {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return fmt.Errorf("empty values are not allowed")
	}
	v.texts = append(v.texts, trimmed)
	return nil
}

func (v *singularValue) String() string {
	if v == nil {
		return ""
	}
	return strings.Join(v.texts, ",")
}

func (v *singularValue) Type() string {
	if v == nil || v.elem == nil {
		return "value"
	}
	return scalarTypeName(v.elem)
}

func (v *singularValue) IsBoolFlag() bool {
	return v != nil && v.elem != nil && v.elem.Kind() == reflect.Bool
}

func (v *singularValue) Values() []string {
	if v == nil || len(v.texts) == 0 {
		return nil
	}
	return append([]string(nil), v.texts...)
}

func scalarTypeName(typ reflect.Type) string {
	if typ == reflect.TypeOf(time.Duration(0)) {
		return "duration"
	}
	return typ.Kind().String()
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
