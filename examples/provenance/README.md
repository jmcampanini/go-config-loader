# Provenance reporting

Shows how to use `configreporter` to render the effective configuration and each field's final source. The output is formatted as a Lip Gloss table.

Run it with:

```sh
go run .
```

It prints a compact provenance table like:

```text
config file source shown as: config.toml

╭────────────────────┬──────────────────────┬─────────────╮
│ Path               │ Value                │ Source      │
├────────────────────┼──────────────────────┼─────────────┤
│ debug              │ true                 │ <env>       │
│ labels["prod.env"] │ "green"              │ config.toml │
│ name               │ "from-flag"          │ <pflag>     │
│ profiles           │ ["flag-b", "flag-a"] │ <pflag>     │
│ timeout            │ "5s"                 │ <default>   │
╰────────────────────┴──────────────────────┴─────────────╯
```

The `profiles` row shows raw profiles: the loaded list after precedence, with `--profiles` values first, then `--profile` values, reduced to the last occurrence of each name. Deriving effective profiles, such as appending `env_profiles` values and validating names, is application work that happens after loading and is not part of this report.
