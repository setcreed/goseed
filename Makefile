GO ?= go
VERSION ?= dev
LINT := .tools/bin/golangci-lint
LINT_VERSION := v2.14.0
GO_PACKAGES := ./...

# Project caches are disposable; no build or runtime input depends on them.
export GOCACHE := $(CURDIR)/.tmp/go-cache
export GOMODCACHE := $(CURDIR)/.tmp/mod-cache
export GOLANGCI_LINT_CACHE := $(CURDIR)/.tmp/lint-cache
export GOWORK := off
export GOTOOLCHAIN := go1.26.4

.DEFAULT_GOAL := check
.PHONY: build test race vet lint fmt fmt-check check contract-check tools compatibility compatibility-cross compatibility-db protocol-tools protocol-check template-container template-db

build:
	$(GO) build -mod=readonly -trimpath -ldflags '-X main.version=$(VERSION)' -o bin/goseed ./cmd/goseed

test:
	$(GO) test -mod=readonly $(GO_PACKAGES)

race:
	$(GO) test -mod=readonly -race $(GO_PACKAGES)

vet:
	$(GO) vet -mod=readonly $(GO_PACKAGES)

fmt:
	gofmt -w cmd internal tests/compatibility tests/integration

fmt-check:
	@files="$$(gofmt -l cmd internal tests/compatibility tests/integration)"; if [ -n "$$files" ]; then printf '%s\n' "$$files"; exit 1; fi

lint:
	@test -x $(LINT) || { printf '%s\n' 'Run make tools to install the pinned project-local linter.'; exit 1; }
	@$(LINT) version | grep -F 'version $(subst v,,$(LINT_VERSION)) ' >/dev/null || { printf '%s\n' 'Linter version does not match the pinned version.'; exit 1; }
	$(LINT) run ./...

check: contract-check fmt-check test vet lint

contract-check:
	python3 scripts/check-contract.py

compatibility:
	cd tests/compatibility && CGO_ENABLED=0 $(GO) test -mod=readonly -v -timeout=2m ./...

compatibility-cross:
	mkdir -p .tmp/compatibility-cross/linux-amd64 .tmp/compatibility-cross/linux-arm64 .tmp/compatibility-cross/windows-amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) -C tests/compatibility test -mod=readonly -c -o $(CURDIR)/.tmp/compatibility-cross/linux-amd64/ ./...
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $(GO) -C tests/compatibility test -mod=readonly -c -o $(CURDIR)/.tmp/compatibility-cross/linux-arm64/ ./...
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) -C tests/compatibility test -mod=readonly -c -o $(CURDIR)/.tmp/compatibility-cross/windows-amd64/ ./...

compatibility-db:
	python3 scripts/compatibility.py

protocol-tools:
	python3 scripts/protocol-tools.py

protocol-check:
	python3 scripts/protocol-check.py

template-container:
	GOSEED_TEST_DOCKER=1 $(GO) test -mod=readonly -count=1 -timeout=15m -v ./tests/integration -run '^TestDefaultProject$$'

template-db:
	python3 scripts/template-db.py

# Explicit installation only; build/test/check never install global tools.
tools:
	GOBIN=$(CURDIR)/.tools/bin $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(LINT_VERSION)
