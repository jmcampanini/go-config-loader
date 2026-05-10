package configloader

import "fmt"

// FileLoaderOption configures config file loaders.
type FileLoaderOption interface {
	applyFileLoaderOption(*fileLoaderOptions)
}

// UnknownKeyPolicy controls how config file loaders handle unknown keys.
type UnknownKeyPolicy int

const (
	// UnknownKeyError rejects config files that contain unknown keys.
	UnknownKeyError UnknownKeyPolicy = iota
	// UnknownKeyIgnore ignores unknown keys and applies known keys.
	UnknownKeyIgnore
	// UnknownKeyWarn reports unknown keys as warnings and applies known keys.
	UnknownKeyWarn
)

// Warning describes a non-fatal config loading issue.
type Warning struct {
	Source  string
	Message string
}

// WarningHandler receives config file loader warnings synchronously.
type WarningHandler func(Warning)

// WithUnknownKeys sets the policy for unknown config file keys.
func WithUnknownKeys(policy UnknownKeyPolicy) FileLoaderOption {
	return fileLoaderOptionFunc(func(opts *fileLoaderOptions) {
		opts.unknownKeyPolicy = policy
	})
}

// WithWarningHandler sets the warning callback used by config file loaders.
func WithWarningHandler(handler WarningHandler) FileLoaderOption {
	return fileLoaderOptionFunc(func(opts *fileLoaderOptions) {
		opts.warningHandler = handler
	})
}

type fileLoaderOptionFunc func(*fileLoaderOptions)

func (f fileLoaderOptionFunc) applyFileLoaderOption(opts *fileLoaderOptions) {
	f(opts)
}

type fileLoaderOptions struct {
	unknownKeyPolicy UnknownKeyPolicy
	warningHandler   WarningHandler
}

func resolveFileLoaderOptions(options []FileLoaderOption) (fileLoaderOptions, error) {
	resolved := fileLoaderOptions{unknownKeyPolicy: UnknownKeyError}
	for i, option := range options {
		if option == nil {
			return fileLoaderOptions{}, fmt.Errorf("configloader: file loader option at index %d is nil", i)
		}
		option.applyFileLoaderOption(&resolved)
	}

	switch resolved.unknownKeyPolicy {
	case UnknownKeyError, UnknownKeyIgnore, UnknownKeyWarn:
		// valid
	default:
		return fileLoaderOptions{}, fmt.Errorf("configloader: invalid unknown key policy %d", resolved.unknownKeyPolicy)
	}
	if resolved.unknownKeyPolicy == UnknownKeyWarn && resolved.warningHandler == nil {
		return fileLoaderOptions{}, fmt.Errorf("configloader: unknown-key warning policy requires a warning handler")
	}
	return resolved, nil
}
