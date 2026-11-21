.PHONY: build test test-race lint fmt fmt-check vet check vulncheck clean

BINARY := appman
PKG := ./...
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -ldflags "-X appman/internal/cli.version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/appman

test:
	go test $(PKG)

test-race:
	go test -race $(PKG)

lint:
	golangci-lint run $(PKG)

fmt:
	golangci-lint fmt $(PKG)

fmt-check:
	golangci-lint fmt --diff $(PKG)

vet:
	go vet $(PKG)

vulncheck:
	govulncheck $(PKG)

check: fmt-check vet lint test

clean:
	rm -f $(BINARY) coverage.out
