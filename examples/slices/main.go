// Package main demonstrates slice loading from files, environment variables, and pflags.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/jmcampanini/go-config-loader/configloader"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/spf13/pflag"
)

type Config struct {
	DefaultOnlyProfiles []string `toml:"default_only_profiles"`
	FileProfiles        []string `toml:"file_profiles"`
	EnvProfiles         []string `toml:"env_profiles" config:"env-profiles" help:"profiles loaded from environment"`
	FlagProfiles        []string `toml:"flag_profiles" config:"flag-profiles" pflag_singular:"flag-profile" help:"profiles loaded from pflags"`
}

var defaults = Config{
	DefaultOnlyProfiles: []string{"default"},
	FileProfiles:        []string{"default"},
	EnvProfiles:         []string{"default"},
	FlagProfiles:        []string{"default"},
}

const fileConfig = `
file_profiles = ["file-a", "file-b"]
`

func main() {
	configPath, cleanup, err := writeTempConfigFile()
	must(err)
	defer cleanup()

	flags := pflag.NewFlagSet("slices-example", pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	must(pflagloader.Register[Config](flags))
	must(flags.Parse([]string{
		"--flag-profiles=flag-a,flag-b",
		"--flag-profile=flag-b",
		"--flag-profile=canary",
	}))

	fileLoader, err := configloader.NewMergeAllFilesLoader[Config](configloader.File(configPath))
	must(err)
	envLoader, err := configloader.NewEnvironmentLoader[Config]("slices-demo", map[string]string{
		"SLICES_DEMO_ENV_PROFILES": "env-a, env-b, env-a",
	})
	must(err)
	flagLoader, err := pflagloader.NewLoader[Config](flags)
	must(err)

	cfg, report, err := configloader.Load(defaults, fileLoader, envLoader, flagLoader)
	must(err)

	fmt.Printf("default_only_profiles=%v source=%s\n", cfg.DefaultOnlyProfiles, report.Updates["defaultonlyprofiles"])
	fmt.Printf("file_profiles=%v source=%s\n", cfg.FileProfiles, report.Updates["fileprofiles"])
	fmt.Printf("env_profiles=%v source=%s\n", cfg.EnvProfiles, report.Updates["envprofiles"])
	fmt.Printf("flag_profiles=%v source=%s\n", cfg.FlagProfiles, report.Updates["flagprofiles"])
}

func writeTempConfigFile() (string, func(), error) {
	dir, err := os.MkdirTemp("", "configloader-slices-")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup: %v\n", err)
		}
	}

	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(fileConfig), 0o600); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return path, cleanup, nil
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
