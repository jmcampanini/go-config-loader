# Plan: pflag singular slice aliases

## Goal

Add an explicit pflag-only singular alias for slice fields, without changing TOML keys, environment variable names, or the canonical config field/value.

Example:

```go
type Config struct {
    Profiles []string `toml:"profiles" config:"profiles" pflag_singular:"profile" help:"profile names"`
}
```

Supported CLI forms:

```text
--profiles=a,b      # canonical CSV/list form
--profile a         # singular form, one value
--profile=a,b       # for []string: one raw value "a,b"
```

## Decisions

- `pflag_singular:"name"` is explicit. The library will not infer singular names from plural names.
- The tag is pflag-only. It does not affect TOML or environment loading.
- The tag requires a canonical `config` tag on the same field.
- The tag is only valid on slices whose element type is a supported scalar pflag/env type.
- Arrays, maps, structs, pointers, interfaces, and nested non-scalar elements are invalid for `pflag_singular`.
- The singular pflag name is validated with the same kebab-case rules as `config` tags.
- Singular values are trimmed with `strings.TrimSpace`.
- A trimmed-empty singular value is invalid.
- Singular values are not CSV-split.
  - For `[]string`, append the trimmed raw string.
  - For `[]int`, `[]bool`, `[]time.Duration`, etc., parse the trimmed value as one scalar.
- Canonical slice pflag behavior remains unchanged:
  - comma-separated values
  - whitespace trimmed per item
  - repeated canonical flags append
  - empty canonical value means empty slice
- If canonical and singular flags are both present, values are combined in deterministic waves:
  1. all canonical `--profiles` values
  2. all singular `--profile` values, in the order pflag saw that singular flag
- No global command-line ordering is guaranteed between canonical and singular flags.
- Combined values are deduped while preserving first-seen order within the deterministic wave ordering.
- Pflag-loaded slice values replace lower-priority config values; they do not append to defaults, files, or env values.
- If only singular values are provided, the resulting slice is built from those singular values.
- If `--profiles=` and one or more `--profile` values are provided, the final slice is the singular values after dedupe.
- If `--profiles=` is provided without singular values, the final slice is empty.
- For singular `[]bool` flags, follow pflag bool convention:
  - `--flag` appends `true`
  - `--flag=false` appends `false`
  - `--flag=` is invalid because empty singular values are rejected

## Help text

`pflagloader.Register` will auto-register the singular pflag and reuse the canonical field help text, appending a terse note such as:

```text
Adds a single value to the array; empty values are not allowed.
```

Example help entries:

```text
--profiles stringSlice   profile names
--profile string         profile names Adds a single value to the array; empty values are not allowed.
```

## Validation and collision rules

Reject during `pflagloader.Register` / `pflagloader.NewLoader` validation:

- `pflag_singular` on a field without `config`
- `pflag_singular` on a non-slice field
- `pflag_singular` on a slice with unsupported element type
- empty `pflag_singular` value
- `pflag_singular:"-"`
- non-kebab-case singular names
- singular name equal to the canonical `config` name on the same field
- singular name colliding with any canonical config pflag name
- singular name colliding with another singular pflag name
- canonical or singular name already registered on the target `FlagSet`
- `pflag_singular` on unexported fields

Root `configloader.ValidateConfig` does not need to understand `pflag_singular`; this validation is pflag-specific.

## Implementation outline

1. Add pflag-specific field metadata collection.
   - Reuse existing config metadata where possible.
   - Include canonical config tag, help tag, optional singular tag, Go path, field index, and field type.
   - Scan recursively through exported nested structs.
   - Detect misuse of `pflag_singular`, including fields without `config` tags.

2. Refactor `pflagloader.Register`.
   - Validate all canonical and singular names before registering anything.
   - Register canonical flags with existing behavior.
   - For fields with `pflag_singular`, register an additional singular flag.
   - For singular bool element flags, set `NoOptDefVal = "true"`.

3. Add a singular pflag value type.
   - Store repeated singular values as trimmed strings.
   - Reject trimmed-empty values in `Set`.
   - Do not split on commas.
   - Expose collected values to `NewLoader` without relying on comma-joined `String()` output.

4. Refactor `pflagloader.NewLoader`.
   - Validate and locate both canonical and singular flags.
   - If neither canonical nor singular changed for a field, leave the base config unchanged.
   - If canonical changed, parse canonical value using existing `configmeta.ParseText` behavior.
   - If singular changed, parse each stored singular value as one scalar element.
   - For slice fields with both forms, combine canonical wave then singular wave.
   - Dedupe combined slice values preserving first-seen order.
   - Set the config field once and mark provenance as `<pflag>`.

5. Documentation updates.
   - README source-specific behavior section.
   - pflagloader package docs if needed.
   - Mention no auto-unpluralization and no global ordering guarantee between canonical/singular flags.

6. Tests.
   - Singular string appends one raw value.
   - Singular string trims whitespace.
   - Singular string rejects empty and whitespace-only values.
   - Singular string with comma remains one value.
   - Singular int parses one scalar.
   - Singular int with comma errors.
   - Singular duration parses one scalar.
   - Singular bool supports `--flag` as `true` and `--flag=false` as `false`.
   - Mixed canonical/singular uses wave ordering.
   - Deduping applies across canonical and singular values.
   - Singular-only replaces lower-priority/default slice values.
   - Canonical empty plus singular values yields singular values.
   - Canonical empty alone yields empty slice.
   - Invalid tag placement and invalid names are rejected.
   - Name collisions are rejected.
   - Existing FlagSet collisions are rejected.
   - Help text includes the singular note.

## Out of scope / consumer responsibilities

- No automatic plural-to-singular inference.
- No singular environment variable aliases.
- No TOML aliases.
- No global CLI ordering guarantee between canonical and singular flags.
- No multiple singular aliases for one field initially.
- Cobra persistent/inherited flag-set composition remains the caller's responsibility.
