// Package main demonstrates printing effective values alongside provenance sources.
package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	configloader "github.com/jmcampanini/go-config-loader/configloader"
	"github.com/jmcampanini/go-config-loader/configreporter"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/spf13/pflag"
)

type Config struct {
	Name     string            `toml:"name" config:"name" help:"display name"`
	Debug    bool              `toml:"debug" config:"debug" help:"enable debug output"`
	Timeout  time.Duration     `toml:"timeout" config:"timeout" help:"request timeout"`
	Profiles []string          `toml:"profiles" config:"profiles" pflag_singular:"profile" help:"active profiles"`
	Labels   map[string]string `toml:"labels"`
}

var defaults = Config{
	Name:     "default-app",
	Timeout:  5 * time.Second,
	Profiles: []string{"default"},
}

const fileConfig = `
[labels]
"prod.env" = "green"
`

func main() {
	configPath, cleanup, err := writeTempConfigFile()
	must(err)
	defer cleanup()

	flags := pflag.NewFlagSet("provenance-example", pflag.ContinueOnError)
	flags.SetOutput(io.Discard)
	must(pflagloader.Register[Config](flags))
	must(flags.Parse([]string{
		"--name=from-flag",
		"--profiles=flag-a,flag-b",
		"--profile=canary",
	}))

	fileLoader, err := configloader.NewMergeAllFilesLoader[Config](configloader.File(configPath))
	must(err)
	envLoader, err := configloader.NewEnvironmentLoader[Config]("my-app", map[string]string{
		"MY_APP_DEBUG": "true",
	})
	must(err)
	flagLoader, err := pflagloader.NewLoader[Config](flags)
	must(err)

	cfg, report, err := configloader.Load(defaults, fileLoader, envLoader, flagLoader)
	must(err)

	reporter := configreporter.New(cfg, report)
	shortConfigPath := filepath.Base(configPath)
	rows := shortenSources(reporter.ProvenanceRows(), map[string]string{
		configPath: shortConfigPath,
	})

	fmt.Printf("config file source shown as: %s\n\n", shortConfigPath)
	fmt.Println(provenanceTable(reporter.ProvenanceHeaders(), rows))
}

func writeTempConfigFile() (string, func(), error) {
	dir, err := os.MkdirTemp("", "configloader-provenance-")
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

func shortenSources(rows [][]string, replacements map[string]string) [][]string {
	shortened := make([][]string, len(rows))
	for i, row := range rows {
		shortened[i] = append([]string(nil), row...)
		if len(row) < 3 {
			continue
		}
		if replacement, ok := replacements[row[2]]; ok {
			shortened[i][2] = replacement
		}
	}
	return shortened
}

func provenanceTable(headers []string, rows [][]string) string {
	baseStyle := lipgloss.NewStyle().Padding(0, 1)
	headerStyle := baseStyle.Bold(true)

	return table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("240"))).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return headerStyle
			}
			return baseStyle
		}).
		Headers(headers...).
		Rows(rows...).
		String()
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
