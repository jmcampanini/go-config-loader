# Slice values

Shows how slice fields are loaded from defaults, TOML, environment variables, and pflags. The example demonstrates comma-separated values, repeated singular pflags, and the last-occurrence rule: when an item repeats in an environment or flag list, only its last position is kept. TOML lists are loaded verbatim.

Run it with:

```sh
go run .
```

It prints:

```text
default_only_items=[default] source=<default>
file_items=[file-a file-b file-a] source=/tmp/.../config.toml
env_items=[env-b env-a] source=<env>
flag_items=[flag-b flag-a canary] source=<pflag>
```

The flag list combines `--flag-items` values first, then each `--flag-item` in parse order, so `--flag-item=flag-a --flag-items=flag-a,flag-b --flag-item=canary` becomes `flag-a, flag-b, flag-a, canary` before the last-occurrence rule reduces it to `flag-b, flag-a, canary`.
