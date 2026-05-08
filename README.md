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

`Load` returns the final config plus an `Updates` map. `Updates` maps canonical Go field paths, such as `server.port` or `labels["env"]`, to the source that last set that value. File sources are reported as absolute paths.

The root package supports defaults, TOML files, environment variables, validation, and file-path helpers. CLI flag support lives in `github.com/jmcampanini/go-config-loader/pflagloader` so the root package does not depend on pflag or Cobra. Reporting helpers live in `github.com/jmcampanini/go-config-loader/configreporter`.

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

`config:"..."` tags define the external name used for environment variables and pflags. For example, `config:"api-url"` maps to `MY_APP_API_URL` with the `my-app` env prefix and to the `--api-url` flag. Only scalar leaf fields and slices of scalar leaf fields with a `config` tag are loaded from env or pflags. TOML loading uses `toml:"..."` tags when present, otherwise Go field names. Provenance paths are always derived from Go field names, not tags.

Slice env/pflag contract: values are comma-separated, surrounding whitespace is trimmed from each item, duplicate items are removed while preserving first-seen order, repeated pflags append before deduping, an empty value means an empty slice, and commas inside values are not escaped. Scalar fields are not split.

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

## Reporting effective configuration and provenance

`configreporter` formats already-loaded config values and provenance metadata. It does not load or mutate configuration.

```go
func reportConfig(cfg Config, updates configloader.Updates, out io.Writer) error {
    reporter := configreporter.New(cfg, updates)

    if err := reporter.WriteTOML(out); err != nil {
        return err
    }

    headers := reporter.ProvenanceHeaders() // []string{"Path", "Source"}
    rows := reporter.ProvenanceRows()       // sorted [][]string path/source pairs

    // headers and rows can be passed to a table renderer such as Lip Gloss.
    _, _ = headers, rows
    return nil
}
```

`reporter.TOML()` returns the effective config as `[]byte`. TOML output uses normal TOML tags and omits fields tagged `toml:"-"`.

## Loading from a custom file location

For an optional custom candidate path, compose the path with the existing file-list loaders:

```go
func loadCustomConfig(path string) (Config, configloader.Updates) {
    fileLoader, _ := configloader.NewMergeAllFilesLoader[Config](configloader.File(path))
    cfg, updates, _ := configloader.Load(defaults, fileLoader)
    return cfg, updates
}
```

For an explicit `--config` flag that must point to an existing file and should skip all other external sources, use a required file loader:

```go
func loadOnlyExplicitConfig(path string) (Config, configloader.Updates, error) {
    fileLoader, err := configloader.NewRequiredFileLoader[Config](path)
    if err != nil {
        return Config{}, nil, err
    }
    return configloader.Load(defaults, fileLoader)
}
```

If your CLI wants `--config` to replace discovered config files but still allow environment variables and other flags to override values, keep composing loaders in precedence order:

```go
fileLoader, _ := configloader.NewRequiredFileLoader[Config](path)
envLoader, _ := configloader.NewEnvironmentLoader[Config]("my-app", configloader.OSEnv())
flagLoader, _ := pflagloader.NewLoader[Config](flags)
cfg, updates, err := configloader.Load(defaults, fileLoader, envLoader, flagLoader)
```

See the `examples` package for testable examples, `examples/cobra` for isolated Cobra CLI integration, and `examples/cobra-slices` for Cobra slice integration.
