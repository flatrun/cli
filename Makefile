.PHONY: help deps build test test-installer fmt fmt-check vet lint lint-install test-coverage clean qa

BINARY_NAME=flatrun
VERSION?=$(shell cat VERSION 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
LDFLAGS=-ldflags "-X github.com/flatrun/cli/internal/command.Version=$(VERSION) -X github.com/flatrun/cli/internal/command.BuildTime=$(BUILD_TIME) -X github.com/flatrun/cli/internal/command.GitCommit=$(GIT_COMMIT)"

help:
	@echo "FlatRun CLI - Build commands"
	@echo ""
	@echo "make deps          - Download dependencies"
	@echo "make build         - Build CLI binary"
	@echo "make test          - Run unit tests"
	@echo "make test-installer - Install and run a pinned published release"
	@echo "make test-coverage - Run tests with coverage report"
	@echo "make fmt           - Format code with gofmt"
	@echo "make fmt-check     - Check gofmt formatting"
	@echo "make vet           - Run go vet"
	@echo "make lint          - Run golangci-lint"
	@echo "make qa            - Run fmt-check, vet, lint, and tests"
	@echo "make clean         - Clean build artifacts"

deps:
	go mod download
	go mod tidy

build:
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/flatrun

test:
	go test ./...

test-installer:
	sh scripts/install-test.sh

test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

fmt:
	go fmt ./...

fmt-check:
	@test -z "$$(gofmt -l .)" || (echo "Run gofmt on:" && gofmt -l . && exit 1)

vet:
	go vet ./...

lint:
	@command -v golangci-lint > /dev/null 2>&1 && golangci-lint run ./... || \
	(test -x "$$(go env GOPATH)/bin/golangci-lint" && "$$(go env GOPATH)/bin/golangci-lint" run ./... || \
	(echo "golangci-lint not found. Run 'make lint-install' first." && exit 1))

lint-install:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

qa: fmt-check vet lint test

clean:
	rm -f $(BINARY_NAME)
	rm -f coverage.out coverage.html
	go clean
