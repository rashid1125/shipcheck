VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS  := -s -w -X github.com/rashid1125/launchsafe/cmd.Version=$(VERSION)
BINARY   := launchsafe
DIST     := dist

.PHONY: build test vet fmt clean install cross help

## build: build binary for the current platform
build:
	go build -ldflags="$(LDFLAGS)" -o $(BINARY) .

## install: install to $GOPATH/bin (or ~/go/bin)
install:
	go install -ldflags="$(LDFLAGS)" .

## test: run all tests
test:
	go test -race ./...

## vet: run go vet
vet:
	go vet ./...

## fmt: format all Go files
fmt:
	gofmt -w .

## cross: build for all platforms into ./dist/
cross:
	mkdir -p $(DIST)
	GOOS=linux   GOARCH=amd64  go build -ldflags="$(LDFLAGS)" -o $(DIST)/$(BINARY)-linux-amd64       .
	GOOS=linux   GOARCH=arm64  go build -ldflags="$(LDFLAGS)" -o $(DIST)/$(BINARY)-linux-arm64       .
	GOOS=darwin  GOARCH=amd64  go build -ldflags="$(LDFLAGS)" -o $(DIST)/$(BINARY)-darwin-amd64      .
	GOOS=darwin  GOARCH=arm64  go build -ldflags="$(LDFLAGS)" -o $(DIST)/$(BINARY)-darwin-arm64      .
	GOOS=windows GOARCH=amd64  go build -ldflags="$(LDFLAGS)" -o $(DIST)/$(BINARY)-windows-amd64.exe .
	@echo "\nBuilt binaries:"
	@ls -lh $(DIST)/

## clean: remove built binaries
clean:
	rm -f $(BINARY)
	rm -rf $(DIST)/

## help: show this help
help:
	@grep -E '^##' Makefile | sed 's/## //'
