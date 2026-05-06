package configloader

import (
	"reflect"

	"github.com/jmcampanini/go-config-loader/internal/configmeta"
)

// ConfigLoader overlays one configuration source onto a base config value.
type ConfigLoader[C any] func(base C) (config C, updates Updates, err error)

// Updates maps canonical config paths to the source that last provided them.
type Updates map[string]string

const (
	// SourceDefault identifies values originating from application defaults.
	SourceDefault = "<default>"
	// SourceEnv identifies values originating from environment variables.
	SourceEnv = "<env>"
	// SourceUnknownFile identifies values originating from an unspecified file.
	SourceUnknownFile = "<file>"
)

// ValidateConfig validates the shape and tags of C.
func ValidateConfig[C any]() error {
	return configmeta.ValidateConfigType[C]()
}

// Load applies loaders to defaults in order and returns final config and provenance.
func Load[C any](defaults C, loaders ...ConfigLoader[C]) (config C, updates Updates, err error) {
	if err := ValidateConfig[C](); err != nil {
		return config, nil, err
	}

	config = defaults
	paths, err := configmeta.DefaultLeafPathsOf(reflect.ValueOf(defaults))
	if err != nil {
		return config, nil, err
	}

	updates = make(Updates, len(paths))
	for _, path := range paths {
		updates[path] = SourceDefault
	}

	for _, loader := range loaders {
		loadedConfig, loaderUpdates, err := loader(config)
		if err != nil {
			return config, updates, err
		}
		config = loadedConfig
		for path, source := range loaderUpdates {
			updates[path] = source
		}
	}

	return config, updates, nil
}
