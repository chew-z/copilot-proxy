.PHONY: build install test clean lint format

# Build the binary with green tea GC experiment
build:
	GOEXPERIMENT=greenteagc go build -o bin/copilot-proxy .

# Install the binary
install:
	go install .

# Run tests
test:
	go test -v ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Run linter
lint:
	if [ -f run_lint.sh ]; then ./run_lint.sh; else golangci-lint run; fi

# Format code
format:
	if [ -f run_format.sh ]; then ./run_format.sh; else go fmt ./...; fi

# Development
dev: build
	./bin/copilot-proxy serve

# Default target
all: format lint test build
