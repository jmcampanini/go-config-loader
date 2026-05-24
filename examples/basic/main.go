// Package main demonstrates basic layered loading with defaults, TOML, and environment variables.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	configloader "github.com/jmcampanini/go-config-loader"
)

type ServerConfig struct {
	Host string
	Port int `config:"port" help:"server port"`
}

type Config struct {
	Name   string `config:"name" help:"display name"`
	Debug  bool   `config:"debug" help:"enable debug output"`
	Server ServerConfig
}

var defaults = Config{
	Name:   "default-name",
	Debug:  false,
	Server: ServerConfig{Host: "localhost", Port: 8080},
}

const fileConfig = `
name = "from-file"

[server]
host = "0.0.0.0"
`

func main() {
	configPath, cleanup, err := writeTempConfigFile()
	must(err)
	defer cleanup()

	fileLoader, err := configloader.NewMergeAllFilesLoader[Config](configloader.File(configPath))
	must(err)

	envLoader, err := configloader.NewEnvironmentLoader[Config]("my-app", map[string]string{
		"MY_APP_NAME":  "from-env",
		"MY_APP_DEBUG": "true",
	})
	must(err)

	cfg, report, err := configloader.Load(defaults, fileLoader, envLoader)
	must(err)

	fmt.Printf("name=%s source=%s\n", cfg.Name, report.Updates["name"])
	fmt.Printf("debug=%t source=%s\n", cfg.Debug, report.Updates["debug"])
	fmt.Printf("server.host=%s source=%s\n", cfg.Server.Host, report.Updates["server.host"])
	fmt.Printf("server.port=%d source=%s\n", cfg.Server.Port, report.Updates["server.port"])
}

func writeTempConfigFile() (string, func(), error) {
	dir, err := os.MkdirTemp("", "configloader-basic-")
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
