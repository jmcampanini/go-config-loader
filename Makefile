.DEFAULT_GOAL := help

BUILD_DIR := .build
EXAMPLE_MODULES := examples/basic examples/cobra examples/provenance examples/slices

.PHONY: help build test run-example-modules lint fmt fmt-check tidy tidy-check vuln check clean

help: ## Show available targets.
	@printf 'Usage: make <target>\n\nTargets:\n'
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z0-9_-]+:.*## / {printf "  %-14s %s\n", $$1, $$2}' $(MAKEFILE_LIST) | LC_ALL=C sort

build: ## Build all packages and example binaries into .build.
	go build ./...
	@mkdir -p $(BUILD_DIR)
	@for module in $(EXAMPLE_MODULES); do \
		name=$$(basename $$module); \
		(cd $$module && go build -o ../../$(BUILD_DIR)/$$name-example .) || exit 1; \
	done

test: ## Run all tests uncached with the race detector.
	go test -count=1 -race ./...
	@for module in $(EXAMPLE_MODULES); do \
		(cd $$module && go test -count=1 -race ./...) || exit 1; \
	done

run-example-modules: ## Run each example module.
	@for module in $(EXAMPLE_MODULES); do \
		(cd $$module && go run .) || exit 1; \
	done

lint: ## Run static analysis.
	go tool golangci-lint run
	@bin="$$(go tool -n golangci-lint)"; \
	for module in $(EXAMPLE_MODULES); do \
		(cd $$module && "$$bin" run) || exit 1; \
	done

fmt: ## Format Go source files.
	go tool golangci-lint fmt
	@bin="$$(go tool -n golangci-lint)"; \
	for module in $(EXAMPLE_MODULES); do \
		(cd $$module && "$$bin" fmt) || exit 1; \
	done

fmt-check: ## Verify formatting without changing files.
	go tool golangci-lint fmt --diff
	@bin="$$(go tool -n golangci-lint)"; \
	for module in $(EXAMPLE_MODULES); do \
		(cd $$module && "$$bin" fmt --diff) || exit 1; \
	done

tidy: ## Apply go mod tidy to the root and example modules.
	go mod tidy
	@for module in $(EXAMPLE_MODULES); do \
		(cd $$module && go mod tidy) || exit 1; \
	done

tidy-check: ## Verify go.mod and go.sum are tidy without changing them.
	go mod tidy -diff
	@for module in $(EXAMPLE_MODULES); do \
		(cd $$module && go mod tidy -diff) || exit 1; \
	done

vuln: ## Check dependencies and reachable code for known vulnerabilities.
	go tool govulncheck ./...

check: fmt-check tidy-check lint test run-example-modules build vuln ## Run the complete local verification contract.

clean: ## Remove build artifacts and the Go test cache.
	rm -rf $(BUILD_DIR) examples/basic/basic examples/cobra/cobra examples/provenance/provenance examples/slices/slices
	go clean -testcache
	@for module in $(EXAMPLE_MODULES); do \
		(cd $$module && go clean -testcache) || exit 1; \
	done
