GO ?= go
NPM ?= npm
VERSION ?= 0.4.2-dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null)
BUILD_TIME ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS = -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(BUILD_TIME)

.PHONY: frontend-install frontend-build test build run-mock build-linux

frontend-install:
	cd frontend && $(NPM) ci

frontend-build:
	cd frontend && $(NPM) run build

test:
	$(GO) test ./...
	cd frontend && $(NPM) run type-check && $(NPM) run lint

build: frontend-build
	$(GO) build -ldflags "$(LDFLAGS)" -o bin/palworld-companion ./cmd/companion

run-mock: frontend-build
	$(GO) run ./cmd/companion --config deploy/config.example.yaml

build-linux: frontend-build
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -ldflags "$(LDFLAGS)" -o bin/palworld-companion-linux-amd64 ./cmd/companion
