package examples

import (
	"fmt"
	"os"
	"path/filepath"

	configloader "github.com/jmcampanini/go-config-loader"
	"github.com/jmcampanini/go-config-loader/pflagloader"
	"github.com/spf13/pflag"
)

type exampleServerConfig struct {
	Host string
	Port int `config:"port" help:"server port"`
}

type exampleConfig struct {
	Name   string `config:"name" help:"display name"`
	Debug  bool   `config:"debug" help:"enable debug output"`
	Server exampleServerConfig
}

func ExampleLoad_fileAndEnvironment() {
	dir, err := os.MkdirTemp("", "configloader-example-")
	if err != nil {
		panic(err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			panic(err)
		}
	}()

	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(`
name = "from-file"

[server]
host = "0.0.0.0"
`), 0o600); err != nil {
		panic(err)
	}

	defaults := exampleConfig{
		Name:   "default-name",
		Debug:  false,
		Server: exampleServerConfig{Host: "localhost", Port: 8080},
	}

	fileLoader, err := configloader.NewMergeAllFilesLoader[exampleConfig]([]string{path})
	if err != nil {
		panic(err)
	}
	envLoader, err := configloader.NewEnvironmentLoader[exampleConfig]("my-app", map[string]string{
		"MY_APP_NAME":  "from-env",
		"MY_APP_DEBUG": "true",
	})
	if err != nil {
		panic(err)
	}

	cfg, report, err := configloader.Load(defaults, fileLoader, envLoader)
	if err != nil {
		panic(err)
	}

	fmt.Println(cfg.Name)
	fmt.Println(cfg.Debug)
	fmt.Println(cfg.Server.Host)
	fmt.Println(cfg.Server.Port)
	fmt.Println(report.Updates["name"])
	fmt.Println(report.Updates["debug"])
	fmt.Println(report.Updates["server.host"] == path)
	fmt.Println(report.Updates["server.port"])

	// Output:
	// from-env
	// true
	// 0.0.0.0
	// 8080
	// <env>
	// <env>
	// true
	// <default>
}

func ExampleLoad_pflags() {
	flags := pflag.NewFlagSet("example", pflag.ContinueOnError)
	if err := pflagloader.Register[exampleConfig](flags); err != nil {
		panic(err)
	}
	if err := flags.Parse([]string{"--name=from-flag", "--port=9090"}); err != nil {
		panic(err)
	}

	flagLoader, err := pflagloader.NewLoader[exampleConfig](flags)
	if err != nil {
		panic(err)
	}

	cfg, report, err := configloader.Load(
		exampleConfig{Server: exampleServerConfig{Host: "localhost", Port: 8080}},
		flagLoader,
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(cfg.Name)
	fmt.Println(cfg.Server.Host)
	fmt.Println(cfg.Server.Port)
	fmt.Println(report.Updates["name"])
	fmt.Println(report.Updates["server.host"])
	fmt.Println(report.Updates["server.port"])

	// Output:
	// from-flag
	// localhost
	// 9090
	// <pflag>
	// <default>
	// <pflag>
}
