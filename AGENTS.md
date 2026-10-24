## Build and validate

- This repository uses mise for tool versions and task execution. Run `mise trust` and `mise install` once in a checkout. Run tasks with `mise run <task>`; do not call `go`, `golangci-lint`, `govulncheck`, or similar tools directly.
- Common tasks (`mise tasks` lists all of them):
  - `mise run check` runs the complete verification contract: `fmt-check`, `tidy-check`, `lint`, `test`, `run-example-modules`, `build`, and `vuln`. **Run this before declaring work done.** It is read-only and must leave the tracked tree unchanged; CI verifies this.
  - `mise run test` runs uncached race-enabled tests for the main module and the standalone example modules.
  - `mise run lint` runs the pinned `golangci-lint` for the root and example modules without rewriting files; `mise run lint-fix` applies its fixes.
  - `mise run fmt` formats the root and example modules; `mise run fmt-check` verifies formatting without changing files.
  - `mise run tidy` applies `go mod tidy` to the root and example modules; `mise run tidy-check` verifies dependency metadata without changing it.
  - `mise run build` builds all packages and the standalone example binaries into `.build/`.
  - `mise run run-example-modules` runs each example module.
  - `mise run vuln` runs govulncheck against the main module.
  - `mise run clean` removes generated build and test artifacts.
- `mise.toml` pins Go, golangci-lint, and govulncheck; `mise.lock` records their checksums. `GOTOOLCHAIN=local` makes Go run the toolchain mise installed, so keep the `go` directive in every `go.mod` equal to the `go` pin. Bump a tool with `mise upgrade --bump <tool>`, run `mise lock` and `mise run check`, and commit `mise.toml` and `mise.lock` together.
- Prefer targeted `mise run test-pkg ./path` while iterating, then finish with `mise run check`.
