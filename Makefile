BUILD_DIR := .build
COBRA_EXAMPLE_BIN := $(BUILD_DIR)/cobra-example
COBRA_SLICES_EXAMPLE_BIN := $(BUILD_DIR)/cobra-slices-example
PROVENANCE_EXAMPLE_BIN := $(BUILD_DIR)/provenance-example
EXAMPLE_MODULES := examples/cobra examples/cobra-slices examples/provenance

.PHONY: build check test test-example-modules lint fmt clean

build:
	go build ./...
	mkdir -p $(BUILD_DIR)
	cd examples/cobra && go build -o ../../$(COBRA_EXAMPLE_BIN) .
	cd examples/cobra-slices && go build -o ../../$(COBRA_SLICES_EXAMPLE_BIN) .
	cd examples/provenance && go build -o ../../$(PROVENANCE_EXAMPLE_BIN) .

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
	rm -rf $(BUILD_DIR) examples/cobra/cobra examples/cobra-slices/cobra-slices examples/provenance/provenance
	go clean -testcache
	for module in $(EXAMPLE_MODULES); do \
		(cd $$module && go clean -testcache); \
	done
