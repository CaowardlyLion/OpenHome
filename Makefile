.PHONY: dev test smoke build

GO ?= /usr/local/go/bin/go
GO_ENV = GOPATH=$(CURDIR)/.cache/go-path GOCACHE=$(CURDIR)/.cache/go-build GOMODCACHE=$(CURDIR)/.cache/go-mod

dev:
	$(GO_ENV) $(GO) run ./cmd/openhome

test:
	$(GO_ENV) $(GO) test ./internal/... ./cmd/...

smoke:
	$(GO_ENV) $(GO) run ./cmd/smoke

build:
	mkdir -p bin
	$(GO_ENV) $(GO) build -o bin/openhome ./cmd/openhome
