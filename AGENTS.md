## Make targets

- `make test`: Run after code or test changes. Verifies the main module and standalone example modules.
- `make lint`: Run after formatting, dependency, or public API changes. Runs `go mod tidy` and `golangci-lint` for root and standalone example modules.
- `make vuln`: Run govulncheck against the main module.
- `make check`: Run before handing off changes. Executes tests, examples, lint, and vulnerability scanning; this is the required final validation.
- `make fmt`: Run after editing Go files if formatting may be needed. Formats root and standalone example modules.
- `make build`: Run when changing build paths, examples, or CLI integration. Builds all packages and standalone example binaries.
- `make clean`: Run only to clear generated build/test artifacts.

- Prefer targeted `go test ./path` while iterating, then finish with `make check`.
