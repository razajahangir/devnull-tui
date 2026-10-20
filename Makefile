.PHONY: build test lint vuln check

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS = -s -w -X main.version=$(VERSION)

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o devnull ./cmd/devnull

test:
	go test -race ./...

lint:
	golangci-lint run

vuln:
	govulncheck ./...

check: build test
	gofmt -l . | tee /tmp/gofmt.out
	test ! -s /tmp/gofmt.out
	go vet ./...
	@command -v golangci-lint >/dev/null && golangci-lint run || echo "skip golangci-lint (not installed)"
	@command -v govulncheck >/dev/null && govulncheck ./... || echo "skip govulncheck (not installed)"
