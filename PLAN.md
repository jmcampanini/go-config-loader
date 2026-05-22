# Plan: Add Effective Values to Provenance Rows

## Outcomes

- Provenance output includes the effective final value for each reported field.
- Existing provenance rows change from two columns to three columns:
  - `Path`
  - `Value`
  - `Source`
- Values come from the final loaded config stored in `configreporter.Reporter`, not from the raw source that provided the value.
- Provenance path identity remains unchanged for now: paths stay canonical Go-derived paths such as `server.port` and `labels["prod"]`.
- TOML-specific display paths are out of scope for this change.

## API Shape

### Existing API, changed behavior

```go
func (r Reporter[C]) ProvenanceHeaders() []string
func (r Reporter[C]) ProvenanceRows() [][]string
```

### New header behavior

```go
reporter.ProvenanceHeaders()
// []string{"Path", "Value", "Source"}
```

### New row behavior

```go
reporter.ProvenanceRows()
// [][]string{
//   {"debug", "true", "<env>"},
//   {"name", "\"my-app\"", "<pflag>"},
//   {"server.port", "9090", "/tmp/config.toml"},
// }
```

This intentionally breaks consumers that expect exactly two columns.

## Expectations

- Rows remain sorted by provenance path, matching current behavior.
- The `Path` column remains the canonical provenance key from `LoadReport.Updates`.
- The `Source` column remains unchanged.
- The new `Value` column reflects the effective value after all loaders and precedence rules have been applied.
- Map paths should resolve correctly, including quoted keys:

```text
labels["prod"]
servers["prod"].port
```

- Slice and array values should be displayed in a readable, TOML-like form.
- Strings should be quoted so empty strings and whitespace are clear.
- Durations should display as their string form, for example `"5s"`.
- If a provenance path cannot be resolved against the final config, the row should still be returned with a clear placeholder value such as `<unavailable>` rather than panicking.

