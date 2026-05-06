# go-config-loader

`go-config-loader` loads a Go struct from layered configuration sources and records provenance for each field. Start with application defaults, create one or more loaders, then call `Load`. Loaders run in the order provided, so pass lower-priority sources first and higher-priority sources last.

A common precedence order is:

```text
flags > environment > TOML files > defaults
```

Which means loaders are passed as:

```text
TOML files, then environment, then flags
```

`Load` returns the final config plus an `Updates` map. `Updates` maps canonical Go field paths, such as `server.port` or `labels["env"]`, to the source that last set that value.

The root package supports defaults, TOML files, environment variables, validation, and file-path helpers. CLI flag support lives in `github.com/jmcampanini/go-config-loader/pflagloader` so the root package does not depend on pflag or Cobra.

## Configuration structs

Config types must be non-pointer structs. Exported fields are the configuration surface.

Supported field types:

- `string`
- `bool`
- signed and unsigned integer types
- floating-point types
- `time.Duration`
- nested structs containing supported fields
- slices and arrays of supported element types
- `map[string]T` where `T` is supported

`config:"..."` tags define the external name used for environment variables and pflags. For example, `config:"api-url"` maps to `MY_APP_API_URL` with the `my-app` env prefix and to the `--api-url` flag. Only scalar leaf fields with a `config` tag are loaded from env or pflags. TOML loading uses `toml:"..."` tags when present, otherwise Go field names. Provenance paths are always derived from Go field names, not tags.

## Canonical layered loading

```go
type ServerConfig struct {
    Host string
    Port int `config:"port" help:"listen port"`
}

type Config struct {
    Name    string        `config:"name" help:"display name"`
    Debug   bool          `config:"debug" help:"enable debug output"`
    Timeout time.Duration `config:"timeout" help:"request timeout"`
    Server  ServerConfig
}

var defaults = Config{
    Name:    "my-app",
    Timeout: 5 * time.Second,
    Server:  ServerConfig{Host: "localhost", Port: 8080},
}

func registerFlags(flags *pflag.FlagSet) error {
    // Register before Cobra or pflag parses command-line args.
    return pflagloader.Register[Config](flags)
}

func loadConfig(flags *pflag.FlagSet) (Config, configloader.Updates) {
    // Ignoring errors for brevity.
    helper, _ := configloader.NewFileHelper("my-app", "config.toml")

    // Example config.toml:
    // name = "from-file"
    // timeout = "10s"
    //
    // [server]
    // host = "0.0.0.0"
    // port = 9090
    files := configloader.Files(
        helper.XDGConfigFile(),
        helper.HomeBoth(),
        helper.WalkCwdToHomeBoth(),
        helper.CwdBoth(),
    )

    fileLoader, _ := configloader.NewMergeAllFilesLoader[Config](files)
    envLoader, _ := configloader.NewEnvironmentLoader[Config]("my-app", configloader.OSEnv())
    flagLoader, _ := pflagloader.NewLoader[Config](flags)

    // Load order is low to high priority: files, env, flags.
    cfg, updates, _ := configloader.Load(defaults, fileLoader, envLoader, flagLoader)

    // updates["server.port"] shows the final source for Config.Server.Port.
    return cfg, updates
}
```

## Loading from a custom file location

```go
func loadCustomConfig(path string) (Config, configloader.Updates) {
    // Example config.toml:
    // name = "custom"
    // debug = true
    fileLoader, _ := configloader.NewMergeAllFilesLoader[Config](configloader.File(path))
    cfg, updates, _ := configloader.Load(defaults, fileLoader)
    return cfg, updates
}
```

See the `examples` package for testable examples and `examples/cobra` for isolated Cobra CLI integration.
