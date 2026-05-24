# Provenance reporting

Shows how to use `configreporter` to render the effective configuration and each field's final source. The output is formatted as a Lip Gloss table.

Run it with:

```sh
go run .
```

It prints a compact provenance table like:

```text
config file source shown as: config.toml

╭────────────────────┬────────────────────────────────┬─────────────╮
│ Path               │ Value                          │ Source      │
├────────────────────┼────────────────────────────────┼─────────────┤
│ debug              │ true                           │ <env>       │
│ labels["prod.env"] │ "green"                        │ config.toml │
│ name               │ "from-flag"                    │ <pflag>     │
│ profiles           │ ["flag-a", "flag-b", "canary"] │ <pflag>     │
│ timeout            │ "5s"                           │ <default>   │
╰────────────────────┴────────────────────────────────┴─────────────╯
```
