# Cobra CLI

Shows a common CLI setup using Cobra: defaults, an optional `--config` TOML file, environment variables, and pflags are loaded in precedence order.

Run it with defaults:

```sh
go run .
```

Or with overrides:

```sh
cat > /tmp/cobra-demo.toml <<'TOML'
name = "from-file"
port = 9090
TOML

COBRA_DEMO_DEBUG=true go run . --config /tmp/cobra-demo.toml --name from-flag
```
