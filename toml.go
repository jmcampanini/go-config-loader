package configloader

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/jmcampanini/go-config-loader/internal/configmeta"
)

type tomlKeyNode struct {
	present  bool
	children map[string]*tomlKeyNode
}

func newFilesLoader[C any](files []string, pickLast bool, opts ...FileLoaderOption) (ConfigLoader[C], error) {
	if err := ValidateConfig[C](); err != nil {
		return nil, err
	}
	fileOpts, err := resolveFileLoaderOptions(opts)
	if err != nil {
		return nil, err
	}
	filesCopy := make([]string, len(files))
	for i, file := range files {
		path, err := normalizeFilePath(file)
		if err != nil {
			return nil, fmt.Errorf("configloader: file path at index %d: %w", i, err)
		}
		filesCopy[i] = path
	}

	return func(base C) (C, Updates, error) {
		if len(filesCopy) == 0 {
			return base, Updates{}, nil
		}

		if pickLast {
			for i := len(filesCopy) - 1; i >= 0; i-- {
				file := filesCopy[i]
				exists, err := fileExists(file)
				if err != nil {
					return base, nil, err
				}
				if !exists {
					continue
				}
				loaded, updates, warnings, err := loadOneTomlFile(base, file, fileOpts)
				if err != nil {
					return base, nil, err
				}
				emitFileLoaderWarnings(warnings, fileOpts)
				return loaded, updates, nil
			}
			return base, Updates{}, nil
		}

		config := base
		updates := Updates{}
		var warnings []Warning
		for _, file := range filesCopy {
			exists, err := fileExists(file)
			if err != nil {
				return base, nil, err
			}
			if !exists {
				continue
			}
			loaded, fileUpdates, fileWarnings, err := loadOneTomlFile(config, file, fileOpts)
			if err != nil {
				return base, nil, err
			}
			config = loaded
			warnings = append(warnings, fileWarnings...)
			for path, source := range fileUpdates {
				updates[path] = source
			}
		}
		// Delay callbacks until success so warnings describe config that was applied.
		emitFileLoaderWarnings(warnings, fileOpts)
		return config, updates, nil
	}, nil
}

func normalizeFilePath(path string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("file path is empty")
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("make file path %q absolute: %w", path, err)
	}
	return abs, nil
}

func fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, fmt.Errorf("configloader: stat file %q: %w", path, err)
}

// NewRequiredFileLoader constructs a loader for one required config file.
func NewRequiredFileLoader[C any](file string, opts ...FileLoaderOption) (ConfigLoader[C], error) {
	if err := ValidateConfig[C](); err != nil {
		return nil, err
	}
	fileOpts, err := resolveFileLoaderOptions(opts)
	if err != nil {
		return nil, err
	}
	path, err := normalizeFilePath(file)
	if err != nil {
		return nil, err
	}

	return func(base C) (C, Updates, error) {
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return base, nil, fmt.Errorf("configloader: required config file %q does not exist", path)
			}
			return base, nil, fmt.Errorf("configloader: stat required config file %q: %w", path, err)
		}
		if info.IsDir() {
			return base, nil, fmt.Errorf("configloader: required config file %q is a directory", path)
		}
		loaded, updates, warnings, err := loadOneTomlFile(base, path, fileOpts)
		if err != nil {
			return base, nil, err
		}
		// Delay callbacks until success so warnings describe config that was applied.
		emitFileLoaderWarnings(warnings, fileOpts)
		return loaded, updates, nil
	}, nil
}

func loadOneTomlFile[C any](base C, file string, opts fileLoaderOptions) (C, Updates, []Warning, error) {
	config := base
	metadata, err := toml.DecodeFile(file, &config)
	if err != nil {
		return base, nil, nil, fmt.Errorf("configloader: load config file %q: %w", file, err)
	}

	warnings, err := inspectUnknownTomlKeys(file, metadata, opts)
	if err != nil {
		return base, nil, nil, err
	}

	updates := Updates{}
	root := tomlPresenceTree(metadata.Keys())
	if err := collectTomlUpdates(reflect.ValueOf(config), root, "", file, updates); err != nil {
		return base, nil, nil, fmt.Errorf("configloader: inspect config file %q: %w", file, err)
	}
	return config, updates, warnings, nil
}

func inspectUnknownTomlKeys(file string, metadata toml.MetaData, opts fileLoaderOptions) ([]Warning, error) {
	if opts.unknownKeyPolicy == UnknownKeyIgnore {
		return nil, nil
	}
	undecoded := metadata.Undecoded()
	if len(undecoded) == 0 {
		return nil, nil
	}
	message := "contains unknown keys: " + tomlKeysString(undecoded)
	switch opts.unknownKeyPolicy {
	case UnknownKeyError:
		return nil, fmt.Errorf("configloader: config file %q %s", file, message)
	case UnknownKeyWarn:
		return []Warning{{Source: file, Message: message}}, nil
	default:
		return nil, fmt.Errorf("configloader: invalid unknown key policy %d", opts.unknownKeyPolicy)
	}
}

func emitFileLoaderWarnings(warnings []Warning, opts fileLoaderOptions) {
	if opts.warningHandler == nil {
		return
	}
	for _, warning := range warnings {
		opts.warningHandler(warning)
	}
}

func tomlKeysString(keys []toml.Key) string {
	parts := make([]string, len(keys))
	for i, key := range keys {
		parts[i] = key.String()
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func tomlPresenceTree(keys []toml.Key) *tomlKeyNode {
	root := &tomlKeyNode{children: map[string]*tomlKeyNode{}}
	for _, key := range keys {
		node := root
		for _, segment := range key {
			if node.children == nil {
				node.children = map[string]*tomlKeyNode{}
			}
			child := node.children[segment]
			if child == nil {
				child = &tomlKeyNode{children: map[string]*tomlKeyNode{}}
				node.children[segment] = child
			}
			node = child
		}
		node.present = true
	}
	return root
}

func collectTomlUpdates(v reflect.Value, node *tomlKeyNode, path, source string, updates Updates) error {
	if node == nil {
		return nil
	}

	t := v.Type()
	if configmeta.IsScalar(t) || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		if node.present && path != "" {
			updates[path] = source
		}
		return nil
	}

	switch t.Kind() {
	case reflect.Struct:
		return collectTomlStructUpdates(v, node, path, source, updates)
	case reflect.Map:
		return collectTomlMapUpdates(v, node, path, source, updates)
	default:
		return fmt.Errorf("field %s uses unsupported type %s", displayConfigPath(path), t)
	}
}

func collectTomlStructUpdates(v reflect.Value, node *tomlKeyNode, path, source string, updates Updates) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" || field.Anonymous {
			continue
		}
		child := tomlStructChild(node, field)
		if child == nil {
			continue
		}
		fieldPath := joinConfigPath(path, strings.ToLower(field.Name))
		if err := collectTomlUpdates(v.Field(i), child, fieldPath, source, updates); err != nil {
			return err
		}
	}
	return nil
}

func collectTomlMapUpdates(v reflect.Value, node *tomlKeyNode, path, source string, updates Updates) error {
	for keyString, child := range node.children {
		if keyString == "" {
			return fmt.Errorf("field %s contains empty map key", displayConfigPath(path))
		}

		key := reflect.ValueOf(keyString).Convert(v.Type().Key())
		value := v.MapIndex(key)
		if !value.IsValid() {
			continue
		}

		mapPath := path + "[" + strconv.Quote(keyString) + "]"
		if child.present {
			if err := collectValueLeafUpdates(value, mapPath, source, updates); err != nil {
				return err
			}
			continue
		}

		if err := collectTomlUpdates(value, child, mapPath, source, updates); err != nil {
			return err
		}
	}
	return nil
}

func collectValueLeafUpdates(v reflect.Value, path, source string, updates Updates) error {
	t := v.Type()
	if configmeta.IsScalar(t) || t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		updates[path] = source
		return nil
	}

	switch t.Kind() {
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			if field.PkgPath != "" || field.Anonymous {
				continue
			}
			fieldPath := joinConfigPath(path, strings.ToLower(field.Name))
			if err := collectValueLeafUpdates(v.Field(i), fieldPath, source, updates); err != nil {
				return err
			}
		}
		return nil
	case reflect.Map:
		keys := v.MapKeys()
		sort.Slice(keys, func(i, j int) bool { return keys[i].String() < keys[j].String() })
		for _, key := range keys {
			keyString := key.String()
			if keyString == "" {
				return fmt.Errorf("field %s contains empty map key", displayConfigPath(path))
			}
			mapPath := path + "[" + strconv.Quote(keyString) + "]"
			if err := collectValueLeafUpdates(v.MapIndex(key), mapPath, source, updates); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("field %s uses unsupported type %s", displayConfigPath(path), t)
	}
}

func tomlStructChild(node *tomlKeyNode, field reflect.StructField) *tomlKeyNode {
	name, ok := tomlFieldName(field)
	if !ok {
		return nil
	}
	var matched *tomlKeyNode
	for key, child := range node.children {
		if strings.EqualFold(key, name) {
			matched = mergeTomlKeyNodes(matched, child)
		}
	}
	return matched
}

func mergeTomlKeyNodes(dst, src *tomlKeyNode) *tomlKeyNode {
	if src == nil {
		return dst
	}
	if dst == nil {
		dst = &tomlKeyNode{children: map[string]*tomlKeyNode{}}
	}
	dst.present = dst.present || src.present
	for key, srcChild := range src.children {
		dst.children[key] = mergeTomlKeyNodes(dst.children[key], srcChild)
	}
	return dst
}

func tomlFieldName(field reflect.StructField) (string, bool) {
	tag, ok := field.Tag.Lookup("toml")
	if !ok {
		return field.Name, true
	}
	name := strings.SplitN(tag, ",", 2)[0]
	if name == "-" {
		return "", false
	}
	if name == "" {
		return field.Name, true
	}
	return name, true
}

func joinConfigPath(prefix, segment string) string {
	if prefix == "" {
		return segment
	}
	return prefix + "." + segment
}

func displayConfigPath(path string) string {
	if path == "" {
		return "<root>"
	}
	return path
}
