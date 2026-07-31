.PHONY: build install test lint clean run help

# Build variables
BINARY_NAME := devex
VERSION     := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME  := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS     := -ldflags "-s -w -X main.version=$(VERSION) -X main.buildTime=$(BUILD_TIME)"

## help: Show this help message
help:
	@echo "DevEx CLI — Build Targets"
	@echo "────────────────────────────────────"
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/^## /  /'

## build: Compile the binary to ./bin/
build:
	@mkdir -p bin
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/devex

## install: Build and install to $GOPATH/bin
install:
	go install $(LDFLAGS) ./cmd/devex

## run: Build and run with optional ARGS (e.g., make run ARGS="funding status")
run: build
	./bin/$(BINARY_NAME) $(ARGS)

## test: Run all tests with race detection
test:
	go test -race -coverprofile=coverage.out ./...

## lint: Run golangci-lint
lint:
	golangci-lint run ./...

## clean: Remove build artifacts
clean:
	rm -rf bin/ coverage.out

## mod: Tidy and verify Go modules
mod:
	go mod tidy
	go mod verify
