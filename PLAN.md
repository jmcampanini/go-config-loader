# PLAN: pflag slice compatibility

## Goal

Make `pflagloader.NewLoader` correctly consume existing Cobra/pflag slice flags, especially flags registered manually with helpers like:

```go
flags.StringSliceVar(&profiles, "profiles", nil, "profile names")
```

The loaded config should use the logical slice value already parsed by pflag, not pflag's display-oriented `Value.String()` representation.

## Problem summary

pflag slice flags expose two different views of their value:

1. the parsed Go value, for example:

   ```go
   []string{"work", "vpn"}
   ```

2. a display string, for example:

   ```text
   [work,vpn]
   ```

`pflagloader.NewLoader` currently parses `canonicalFlag.Value.String()` as canonical input. For scalar flags this is fine. For pflag slice flags like `StringSliceVar`, this can produce incorrect values such as:

```go
[]string{"[work", "vpn]"}
```

Instead, for pflag slice values, GoConfigLoader should read the parsed slice from pflag.

## Chosen approach

Use pflag's `pflag.SliceValue` interface for canonical slice flags.

For canonical flag loading:

```text
If the config field is a slice and the pflag value implements pflag.SliceValue:
    read sliceValue.GetSlice()
    parse each returned item as the config slice element type
    apply GoConfigLoader's existing slice normalization/dedupe behavior

Otherwise:
    preserve existing behavior by parsing flag.Value.String()
```

This is preferred because `pflag.SliceValue` is pflag's intended API for list flags and avoids reparsing display strings.

## Implementation shape

### 1. Add a helper for canonical flag values

Add a helper in `pflagloader/pflagloader.go`, likely near `parseFlagValue`:

```go
func parseCanonicalFlagValue(field pflagField, flag *pflag.Flag) (reflect.Value, error)
```

The helper should:

- detect slice config fields with `field.Type.Kind() == reflect.Slice`
- check whether `flag.Value` implements `pflag.SliceValue`
- when it does, call `GetSlice()`
- parse each string returned by `GetSlice()` using `configmeta.ParseScalar`
- trim whitespace before scalar parsing to preserve GoConfigLoader's current slice normalization behavior
- dedupe the final slice preserving first occurrence
- when `pflag.SliceValue` is not implemented, fall back to:

  ```go
  configmeta.ParseText(flag.Value.String(), field.Type)
  ```

### 2. Update both canonical parsing call sites

`parseFlagValue` currently parses canonical flags directly in two paths:

1. fields without `pflag_singular`
2. the canonical portion of fields with `pflag_singular`

Both should use the new helper so behavior is consistent.

### 3. Preserve singular flag behavior

Do not change the existing singular flag parsing logic:

```go
singularTexts(singularFlag)
```

Only the canonical flag portion needs the new slice-aware parsing.

### 4. Preserve existing behavior

The change should preserve:

- scalar flag loading
- unchanged flag behavior
- provenance using canonical GoConfigLoader paths like `profiles`
- flags registered by `pflagloader.Register`
- `pflag_singular` behavior
- existing slice dedupe semantics preserving first occurrence

## Tests to add

Add tests in `pflagloader/pflagloader_test.go`.

### 1. Existing `StringSliceVar` is loaded correctly

Register the flag manually:

```go
var profiles []string
flags.StringSliceVar(&profiles, "profiles", nil, "profile names")
```

Parse:

```text
--profiles=work,vpn --profiles=personal,work
```

Expected loaded config:

```go
config{Profiles: []string{"work", "vpn", "personal"}}
```

Expected provenance:

```go
report.Updates["profiles"] == pflagloader.SourcePFlag
```

This proves repeated pflag slice values and deduplication work.

### 2. Existing `StringSliceVar` unchanged flag does not update

Register the same manual flag but parse no args.

Expected:

- base config remains unchanged
- `report.Updates` is empty

This proves unchanged flags remain non-invasive.

### 3. Optional stronger CSV quoting test

Parse something like:

```text
--profiles=work,"vpn,home"
```

Expected:

```go
[]string{"work", "vpn,home"}
```

This proves GoConfigLoader is using pflag's parsed slice value rather than reparsing pflag's display string.

## Validation

While iterating:

```sh
go test ./pflagloader
```

Before handoff:

```sh
make check
```

## Acceptance criteria

The original repro should change from:

```go
[]string{"[work", "vpn", "personal", "work]"}
```

to:

```go
[]string{"work", "vpn", "personal"}
```

and provenance should still report:

```go
report.Updates["profiles"] == pflagloader.SourcePFlag
```
