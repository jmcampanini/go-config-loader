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

`Load` returns the final config plus a `LoadReport`. `LoadReport.Updates` maps canonical Go field paths, such as `server.port` or `labels["env"]`, to the source that last set that value. File sources are reported as absolute paths. `LoadReport.LoadedFiles` lists the absolute config file paths that were successfully parsed, deduped in load order. `LoadReport.Warnings` contains non-fatal load diagnostics such as unknown-key warnings. If `Load` returns an error, the config is the zero value and the report is empty; inspect them only after checking that `err == nil`.

The root package supports defaults, TOML files, environment variables, validation, and file-path helpers. CLI flag support lives in `github.com/jmcampanini/go-config-loader/pflagloader` so the root package does not depend on pflag or Cobra. Reporting helpers live in `github.com/jmcampanini/go-config-loader/configreporter`.

## Configuration structs

A config type must be a non-pointer struct. Exported fields define the configuration surface. Unexported fields are ignored; anonymous embedded fields are unsupported and rejected by validation.

Requirements:

- The root config type must be a struct, not a pointer.
- Exported config fields must use supported types.
- Map keys must be strings.
- Empty map keys are invalid because they cannot be represented in provenance paths.
- Fields tagged `toml:"-"` are excluded from TOML loading and TOML reporting.

Supported field types:

- `string`
- `bool`
- signed and unsigned integer types
- floating-point types
- `time.Duration`
- nested structs containing supported fields
- slices and arrays of supported element types
- `map[string]T` where `T` is supported

Tags:

- `toml:"name"` controls the TOML key for file loading and reporting.
- `toml:"-"` excludes a field from TOML loading and TOML reporting.
- `config:"name"` controls environment variable and canonical pflag names.
- `pflag_singular:"name"` adds an explicit pflag-only singular alias for scalar-slice fields with `config` tags.
- `help:"..."` is required for pflag registration on fields with `config` tags.

Source-specific behavior:

- TOML can load exported fields using `toml` tags or Go field names.
- Environment variables and pflags only load scalar leaf fields and scalar-slice leaf fields with `config` tags.
- Slice env/canonical pflag values are comma-separated, surrounding whitespace is trimmed from each item, duplicate items are removed while preserving first-seen order, repeated pflags append before deduping, an empty value means an empty slice, and commas inside values are not escaped. Scalar fields are not split.
- `pflag_singular` is pflag-only; it does not change TOML keys or environment variable names, and names are never inferred from plurals. Singular pflags append one trimmed scalar value per occurrence, reject empty values, and do not split commas. If both canonical and singular slice pflags are present, canonical values are applied first, then singular values in pflag parse order; no global ordering is guaranteed between the two flag names.
- Provenance paths always use canonical Go field names, not tags.

For example, `Server.Port` is reported as `server.port` even if its TOML key is `listen_port` or its flag is `--port`.

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

func loadConfig(flags *pflag.FlagSet) (Config, configloader.LoadReport) {
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
    cfg, report, _ := configloader.Load(defaults, fileLoader, envLoader, flagLoader)

    // report.Updates["server.port"] shows the final source for Config.Server.Port.
    // report.LoadedFiles lists config files that were parsed.
    return cfg, report
}
```

## Reporting effective configuration and provenance

`configreporter` formats already-loaded config values and provenance metadata. It does not load or mutate configuration.

```go
func reportConfig(cfg Config, report configloader.LoadReport, out io.Writer) error {
    reporter := configreporter.New(cfg, report)

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
func loadCustomConfig(path string) (Config, configloader.LoadReport) {
    fileLoader, _ := configloader.NewMergeAllFilesLoader[Config](configloader.File(path))
    cfg, report, _ := configloader.Load(defaults, fileLoader)
    return cfg, report
}
```

By default, config files are strict: unknown keys are errors. To allow extra keys, choose a permissive unknown-key mode:

```go
// Ignore unknown config file keys.
fileLoader, _ := configloader.NewMergeAllFilesLoader[Config](
    files,
    configloader.IgnoreUnknownKeys(),
)

// Collect unknown config file keys in report.Warnings.
fileLoader, _ = configloader.NewMergeAllFilesLoader[Config](
    files,
    configloader.WarnUnknownKeys(),
)

cfg, report, err := configloader.Load(defaults, fileLoader)
if err != nil {
    return err
}
for _, warning := range report.Warnings {
    log.Printf("%s: %s", warning.Source, warning.Message)
}
_ = cfg
```

For an explicit `--config` flag that must point to an existing file and should skip all other external sources, use a required file loader:

```go
func loadOnlyExplicitConfig(path string) (Config, configloader.LoadReport, error) {
    fileLoader, err := configloader.NewRequiredFileLoader[Config](path)
    if err != nil {
        return Config{}, configloader.LoadReport{}, err
    }
    return configloader.Load(defaults, fileLoader)
}
```

If your CLI wants `--config` to replace discovered config files but still allow environment variables and other flags to override values, keep composing loaders in precedence order:

```go
fileLoader, _ := configloader.NewRequiredFileLoader[Config](path)
envLoader, _ := configloader.NewEnvironmentLoader[Config]("my-app", configloader.OSEnv())
flagLoader, _ := pflagloader.NewLoader[Config](flags)
cfg, report, err := configloader.Load(defaults, fileLoader, envLoader, flagLoader)
```

See the `examples` package for testable examples, `examples/cobra` for isolated Cobra CLI integration, and `examples/cobra-slices` for Cobra slice integration.
