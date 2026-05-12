// Package main demonstrates using go-config-loader with Cobra.
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
	Name  string `config:"name" help:"display name"`
	Debug bool   `config:"debug" help:"enable debug output"`
	Port  int    `config:"port" help:"listen port"`
}

var defaultConfig = Config{
	Name: "cobra-demo",
	Port: 8080,
}

func newRootCommand(out io.Writer) (*cobra.Command, error) {
	var configFile string

	cmd := &cobra.Command{
		Use:          "cobra-demo",
		Short:        "Cobra integration example for go-config-loader",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fileLoader, err := configloader.NewMergeAllFilesLoader[Config](configloader.File(configFile))
			if err != nil {
				return err
			}

			envLoader, err := configloader.NewEnvironmentLoader[Config]("cobra-demo", configloader.OSEnv())
			if err != nil {
				return err
			}

			flagLoader, err := pflagloader.NewLoader[Config](cmd.Flags())
			if err != nil {
				return err
			}

			cfg, report, err := configloader.Load(defaultConfig, fileLoader, envLoader, flagLoader)
			if err != nil {
				return err
			}

			fmt.Fprintf(out, "name=%s source=%s\n", cfg.Name, report.Updates["name"])
			fmt.Fprintf(out, "debug=%t source=%s\n", cfg.Debug, report.Updates["debug"])
			fmt.Fprintf(out, "port=%d source=%s\n", cfg.Port, report.Updates["port"])
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
