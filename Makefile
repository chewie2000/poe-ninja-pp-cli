.PHONY: build test lint install clean

build:
	go build -o bin/poe-ninja-pp-cli ./cmd/poe-ninja-pp-cli

test:
	go test ./...

lint:
	golangci-lint run

install:
	go install ./cmd/poe-ninja-pp-cli

clean:
	rm -rf bin/

build-mcp:
	go build -o bin/poe-ninja-pp-mcp ./cmd/poe-ninja-pp-mcp

install-mcp:
	go install ./cmd/poe-ninja-pp-mcp

build-all: build build-mcp
