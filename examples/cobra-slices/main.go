// Package main demonstrates loading slice fields with go-config-loader and Cobra.
package main

import (
	"fmt"
	"io"
	"os"

	configloader "github.com/jmcampanini/go-config-loader"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/spf13/cobra"
)

type Config struct {
	DefaultOnlyProfiles []string `toml:"default_only_profiles"`
	FileProfiles        []string `toml:"file_profiles"`
	EnvProfiles         []string `toml:"env_profiles" config:"env-profiles" help:"profiles loaded from environment"`
	FlagProfiles        []string `toml:"flag_profiles" config:"flag-profiles" help:"profiles loaded from pflags"`
}

var defaultConfig = Config{
	DefaultOnlyProfiles: []string{"abc"},
	FileProfiles:        []string{"abc"},
	EnvProfiles:         []string{"abc"},
	FlagProfiles:        []string{"abc"},
}

func newRootCommand(out io.Writer) (*cobra.Command, error) {
	var configFile string

	cmd := &cobra.Command{
		Use:          "cobra-slices-demo",
		Short:        "Cobra slice configuration example for go-config-loader",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			files := []string{}
			if configFile != "" {
				files = configloader.File(configFile)
			}

			fileLoader, err := configloader.NewMergeAllFilesLoader[Config](files)
			if err != nil {
				return err
			}

			envLoader, err := configloader.NewEnvironmentLoader[Config]("cobra-slices-demo", configloader.OSEnv())
			if err != nil {
				return err
			}

			flagLoader, err := pflagloader.NewLoader[Config](cmd.Flags())
			if err != nil {
				return err
			}

			cfg, updates, err := configloader.Load(defaultConfig, fileLoader, envLoader, flagLoader)
			if err != nil {
				return err
			}

			fmt.Fprintf(out, "default_only_profiles=%v source=%s\n", cfg.DefaultOnlyProfiles, updates["defaultonlyprofiles"])
			fmt.Fprintf(out, "file_profiles=%v source=%s\n", cfg.FileProfiles, updates["fileprofiles"])
			fmt.Fprintf(out, "env_profiles=%v source=%s\n", cfg.EnvProfiles, updates["envprofiles"])
			fmt.Fprintf(out, "flag_profiles=%v source=%s\n", cfg.FlagProfiles, updates["flagprofiles"])
			return nil
		},
	}

	cmd.Flags().StringVar(&configFile, "config", "", "path to a TOML config file")
	if err := pflagloader.Register[Config](cmd.Flags()); err != nil {
		return nil, err
	}

	return cmd, nil
}

func main() {
	cmd, err := newRootCommand(os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
