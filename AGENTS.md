## Make targets

- `make test`: Run after code or test changes. Runs uncached race-enabled tests for the main module and standalone example modules.
- `make lint`: Run after code or public API changes. Runs the pinned `golangci-lint` for root and standalone example modules; it never rewrites files.
- `make fmt` / `make fmt-check`: `fmt` formats root and standalone example modules; `fmt-check` verifies formatting without changing files.
- `make tidy` / `make tidy-check`: `tidy` applies `go mod tidy` to root and standalone example modules; `tidy-check` verifies dependency metadata without changing it.
- `make vuln`: Run govulncheck against the main module.
- `make build`: Run when changing build paths, examples, or CLI integration. Builds all packages and standalone example binaries into `.build/`.
- `make check`: Run before handing off changes. Executes `fmt-check` + `tidy-check` + `lint` + `test` + `run-example-modules` + `build` + `vuln`; this is the required final validation and is read-only — it must leave the tracked tree unchanged. CI verifies this.
- `make clean`: Run only to clear generated build/test artifacts.

- Tools run through pinned `go.mod` tool declarations via `go tool`; never rely on globally installed golangci-lint or govulncheck.
- Prefer targeted `go test ./path` while iterating, then finish with `make check`.
