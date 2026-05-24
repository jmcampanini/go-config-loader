BUILD_DIR := .build
EXAMPLE_MODULES := examples/basic examples/cobra examples/provenance examples/slices

.PHONY: build check test test-example-modules lint fmt clean

build:
	go build ./...
	mkdir -p $(BUILD_DIR)
	for module in $(EXAMPLE_MODULES); do \
		name=$$(basename $$module); \
		(cd $$module && go build -o ../../$(BUILD_DIR)/$$name-example .); \
	done

check: test lint

test:
	go test ./...
	$(MAKE) test-example-modules

test-example-modules:
	for module in $(EXAMPLE_MODULES); do \
		(cd $$module && go test ./...); \
	done

lint:
	go mod tidy
	for module in $(EXAMPLE_MODULES); do \
		(cd $$module && go mod tidy); \
	done
	golangci-lint run ./...
	for module in $(EXAMPLE_MODULES); do \
		(cd $$module && golangci-lint run ./...); \
	done

fmt:
	go fmt ./...
	for module in $(EXAMPLE_MODULES); do \
		(cd $$module && go fmt ./...); \
	done

clean:
	rm -rf $(BUILD_DIR) examples/basic/basic examples/cobra/cobra examples/provenance/provenance examples/slices/slices
	go clean -testcache
	for module in $(EXAMPLE_MODULES); do \
		(cd $$module && go clean -testcache); \
	done
