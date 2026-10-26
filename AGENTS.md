## Build and validate

- Run tasks with `mise run <task>`; do not call `go`, `golangci-lint`, `govulncheck`, `markdownlint-cli2`, or similar tools directly.
- Run `mise run check` before declaring work done.
- `mise run check` is read-only and must leave the tracked tree unchanged.
- `GOTOOLCHAIN=local` makes Go run the toolchain mise installed. The root `go.mod` `go` directive is the oldest Go release line the library supports, written as `1.N.0`, and the example modules use the same floor. Keep it at or below the `go` pin. Raise it only when the library drops a Go release line or when the code or a dependency needs a newer Go.
- Bump a tool with `mise upgrade --bump <tool>`, run `mise lock` and `mise run check`, and commit `mise.toml`, `mise.lock`, and `.mise/locks/` together.
- Prefer targeted `mise run test-pkg ./path` while iterating, then finish with `mise run check`.
