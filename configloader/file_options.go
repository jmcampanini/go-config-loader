package configloader

import "fmt"

// FileLoaderOption configures config file loaders.
type FileLoaderOption interface {
	applyFileLoaderOption(*fileLoaderOptions)
}

type unknownKeyPolicy int

const (
	unknownKeyError unknownKeyPolicy = iota
	unknownKeyIgnore
	unknownKeyWarn
)

// Warning describes a non-fatal config loading issue.
type Warning struct {
	Source  string
	Message string
}

// IgnoreUnknownKeys configures file loaders to ignore unknown config file keys.
func IgnoreUnknownKeys() FileLoaderOption {
	return fileLoaderOptionFunc(func(opts *fileLoaderOptions) {
		opts.unknownKeyPolicy = unknownKeyIgnore
	})
}

// WarnUnknownKeys configures file loaders to report unknown config file keys as warnings.
func WarnUnknownKeys() FileLoaderOption {
	return fileLoaderOptionFunc(func(opts *fileLoaderOptions) {
		opts.unknownKeyPolicy = unknownKeyWarn
	})
}

type fileLoaderOptionFunc func(*fileLoaderOptions)

func (f fileLoaderOptionFunc) applyFileLoaderOption(opts *fileLoaderOptions) {
	f(opts)
}

type fileLoaderOptions struct {
	unknownKeyPolicy unknownKeyPolicy
}

func resolveFileLoaderOptions(options []FileLoaderOption) (fileLoaderOptions, error) {
	resolved := fileLoaderOptions{unknownKeyPolicy: unknownKeyError}
	for i, option := range options {
		if option == nil {
			return fileLoaderOptions{}, fmt.Errorf("configloader: file loader option at index %d is nil", i)
		}
		option.applyFileLoaderOption(&resolved)
	}
	return resolved, nil
}
