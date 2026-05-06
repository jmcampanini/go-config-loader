package configloader_test

import (
	"testing"

	configloader "github.com/jmcampanini/go-config-loader"
)

type phase0Config struct {
	Name string
}

func TestPhase0ExportedAPISignatures(t *testing.T) {
	requireConfigLoader(nil)
	requireUpdates(configloader.Updates{})
	requireValidateFunc(configloader.ValidateConfig[phase0Config])
	requireLoadFunc(configloader.Load[phase0Config])
	requireEnvironmentLoaderFunc(configloader.NewEnvironmentLoader[phase0Config])
	requireOSEnvFunc(configloader.OSEnv)
	requireNewFileHelperFunc(configloader.NewFileHelper)
	requireFilesFunc(configloader.Files)
	requireFileFunc(configloader.File)
	requireMergeAllFilesLoaderFunc(configloader.NewMergeAllFilesLoader[phase0Config])
	requirePickLastFileLoaderFunc(configloader.NewPickLastFileLoader[phase0Config])
}

func requireConfigLoader(configloader.ConfigLoader[phase0Config]) {}
func requireUpdates(configloader.Updates)                         {}
func requireValidateFunc(func() error)                            {}
func requireLoadFunc(func(phase0Config, ...configloader.ConfigLoader[phase0Config]) (phase0Config, configloader.Updates, error)) {
}
func requireEnvironmentLoaderFunc(func(string, map[string]string) (configloader.ConfigLoader[phase0Config], error)) {
}
func requireOSEnvFunc(func() map[string]string)                                      {}
func requireNewFileHelperFunc(func(string, string) (configloader.FileHelper, error)) {}
func requireFilesFunc(func(...[]string) []string)                                    {}
func requireFileFunc(func(string) []string)                                          {}
func requireMergeAllFilesLoaderFunc(func([]string) (configloader.ConfigLoader[phase0Config], error)) {
}
func requirePickLastFileLoaderFunc(func([]string) (configloader.ConfigLoader[phase0Config], error)) {
}

func TestPhase0SourceConstants(t *testing.T) {
	if configloader.SourceDefault != "<default>" {
		t.Fatalf("SourceDefault = %q", configloader.SourceDefault)
	}
	if configloader.SourceEnv != "<env>" {
		t.Fatalf("SourceEnv = %q", configloader.SourceEnv)
	}
	if configloader.SourceUnknownFile != "<file>" {
		t.Fatalf("SourceUnknownFile = %q", configloader.SourceUnknownFile)
	}
}
