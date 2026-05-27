package configloader

import (
	"reflect"

	"github.com/jmcampanini/go-config-loader/internal/configmeta"
)

// ConfigLoader overlays one configuration source onto a base config value.
//
// The returned report describes only the source loaded by this loader. Load
// combines loader reports and adds default provenance.
type ConfigLoader[C any] func(base C) (config C, report LoadReport, err error)

// Updates maps canonical config paths to the source that last provided them.
type Updates map[string]string

// LoadReport describes metadata produced while loading configuration.
type LoadReport struct {
	// Updates maps canonical config paths to the source that last provided them.
	Updates Updates
	// LoadedFiles lists absolute config file paths successfully parsed, deduped in load order.
	LoadedFiles []string
	// Warnings contains non-fatal load diagnostics in occurrence order.
	Warnings []Warning
}

const (
	// SourceDefault identifies values originating from application defaults.
	SourceDefault = "<default>"
	// SourceEnv identifies values originating from environment variables.
	SourceEnv = "<env>"
)

// ValidateConfig validates the shape and tags of C.
func ValidateConfig[C any]() error {
	return configmeta.ValidateConfigType[C]()
}

// Load applies loaders to defaults in order and returns final config and report.
//
// If Load returns an error, the config is the zero value and the report is
// empty; both should be ignored.
func Load[C any](defaults C, loaders ...ConfigLoader[C]) (config C, report LoadReport, err error) {
	if err := ValidateConfig[C](); err != nil {
		return config, LoadReport{}, err
	}

	config = defaults
	paths, err := configmeta.DefaultLeafPathsOf(reflect.ValueOf(defaults))
	if err != nil {
		var zero C
		return zero, LoadReport{}, err
	}

	report.Updates = make(Updates, len(paths))
	for _, path := range paths {
		report.Updates[path] = SourceDefault
	}

	for _, loader := range loaders {
		loadedConfig, loaderReport, err := loader(config)
		if err != nil {
			var zero C
			return zero, LoadReport{}, err
		}
		config = loadedConfig
		mergeLoadReport(&report, loaderReport)
	}

	return config, report, nil
}

func mergeLoadReport(dst *LoadReport, src LoadReport) {
	if dst.Updates == nil && len(src.Updates) > 0 {
		dst.Updates = Updates{}
	}
	for path, source := range src.Updates {
		dst.Updates[path] = source
	}
	dst.LoadedFiles = appendUniqueStrings(dst.LoadedFiles, src.LoadedFiles...)
	dst.Warnings = append(dst.Warnings, src.Warnings...)
}

func appendUniqueStrings(dst []string, values ...string) []string {
	if len(values) == 0 {
		return dst
	}
	seen := make(map[string]struct{}, len(dst)+len(values))
	for _, value := range dst {
		seen[value] = struct{}{}
	}
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		dst = append(dst, value)
	}
	return dst
}
