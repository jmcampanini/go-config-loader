BUILD_DIR := .build
COBRA_EXAMPLE_BIN := $(BUILD_DIR)/cobra-example

.PHONY: build check test test-cobra-example lint fmt clean

build:
	go build ./...
	mkdir -p $(BUILD_DIR)
	cd examples/cobra && go build -o ../../$(COBRA_EXAMPLE_BIN) .

check: test lint

test:
	go test ./...
	$(MAKE) test-cobra-example

test-cobra-example:
	cd examples/cobra && go test ./...

lint:
	go mod tidy
	cd examples/cobra && go mod tidy
	golangci-lint run ./...

fmt:
	go fmt ./...
	cd examples/cobra && go fmt ./...

clean:
	rm -rf $(BUILD_DIR) examples/cobra/cobra
	go clean -testcache
	cd examples/cobra && go clean -testcache
