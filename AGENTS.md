## Build and validate

- Run tasks with `mise run <task>`; do not call `go`, `golangci-lint`, `govulncheck`, or similar tools directly.
- Run `mise run check` before declaring work done.
- `mise run check` is read-only and must leave the tracked tree unchanged.
- `GOTOOLCHAIN=local` makes Go run the toolchain mise installed, so keep the `go` directive in every `go.mod` equal to the `go` pin.
- Bump a tool with `mise upgrade --bump <tool>`, run `mise lock` and `mise run check`, and commit `mise.toml` and `mise.lock` together.
- Prefer targeted `mise run test-pkg ./path` while iterating, then finish with `mise run check`.
