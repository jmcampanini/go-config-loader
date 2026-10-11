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
	DefaultOnlyItems []string `toml:"default_only_items"`
	FileItems        []string `toml:"file_items"`
	EnvItems         []string `toml:"env_items" config:"env-items" help:"items loaded from environment"`
	FlagItems        []string `toml:"flag_items" config:"flag-items" pflag_singular:"flag-item" help:"items loaded from pflags"`
}

var defaults = Config{
	DefaultOnlyItems: []string{"default"},
	FileItems:        []string{"default"},
	EnvItems:         []string{"default"},
	FlagItems:        []string{"default"},
}

const fileConfig = `
file_items = ["file-a", "file-b", "file-a"]
`

func main() {
	configPath, cleanup, err := writeTempConfigFile()
	must(err)
	defer cleanup()

	flags := pflag.NewFlagSet("slices-example", pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	must(pflagloader.Register[Config](flags))
	must(flags.Parse([]string{
		"--flag-item=flag-a",
		"--flag-items=flag-a,flag-b",
		"--flag-item=canary",
	}))

	fileLoader, err := configloader.NewMergeAllFilesLoader[Config](configloader.File(configPath))
	must(err)
	envLoader, err := configloader.NewEnvironmentLoader[Config]("slices-demo", map[string]string{
		"SLICES_DEMO_ENV_ITEMS": "env-a, env-b, env-a",
	})
	must(err)
	flagLoader, err := pflagloader.NewLoader[Config](flags)
	must(err)

	cfg, report, err := configloader.Load(defaults, fileLoader, envLoader, flagLoader)
	must(err)

	fmt.Printf("default_only_items=%v source=%s\n", cfg.DefaultOnlyItems, report.Updates["defaultonlyitems"])
	fmt.Printf("file_items=%v source=%s\n", cfg.FileItems, report.Updates["fileitems"])
	fmt.Printf("env_items=%v source=%s\n", cfg.EnvItems, report.Updates["envitems"])
	fmt.Printf("flag_items=%v source=%s\n", cfg.FlagItems, report.Updates["flagitems"])
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
