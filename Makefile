.PHONY: dev test smoke build

GO_ENV = GOPATH=$(CURDIR)/.cache/go-path GOCACHE=$(CURDIR)/.cache/go-build GOMODCACHE=$(CURDIR)/.cache/go-mod

dev:
	$(GO_ENV) go run ./cmd/openhome

test:
	$(GO_ENV) go test ./internal/... ./cmd/...

smoke:
	$(GO_ENV) go run ./cmd/smoke

build:
	mkdir -p bin
	$(GO_ENV) go build -o bin/openhome ./cmd/openhome
