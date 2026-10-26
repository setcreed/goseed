GO ?= go
VERSION ?= dev
COMMIT ?= $(shell git rev-parse --verify HEAD 2>/dev/null || printf unknown)
LINT := .tools/bin/golangci-lint
LINT_VERSION := v2.14.0
GO_PACKAGES := ./...

# Project caches are disposable; no build or runtime input depends on them.
export GOCACHE := $(CURDIR)/.tmp/go-cache
export GOMODCACHE := $(CURDIR)/.tmp/mod-cache
export GOLANGCI_LINT_CACHE := $(CURDIR)/.tmp/lint-cache
export GOWORK := off
export GOTOOLCHAIN := local

.DEFAULT_GOAL := check
.PHONY: build install package verify-package test race vet lint fmt fmt-check check contract-check tools compatibility compatibility-cross compatibility-db protocol-tools protocol-check template-smoke template-container template-db template-redis template-deployment template-deployment-db template-deployment-nginx

build:
	$(GO) build -mod=readonly -trimpath -ldflags '-X main.version=$(VERSION) -X main.commit=$(COMMIT)' -o bin/goseed ./cmd/goseed

# Install explicitly into a caller-selected Go bin directory.
install:
	@test -n "$(GOBIN)" || { printf '%s\n' 'Set GOBIN to the directory where goseed should be installed.'; exit 1; }
	@test "$(VERSION)" != dev || { printf '%s\n' 'Set VERSION to a release-style version, for example v0.1.0.'; exit 1; }
	python3 scripts/distribution.py validate-version "$(VERSION)"
	@mkdir -p "$(GOBIN)"
	$(GO) build -mod=readonly -trimpath -ldflags '-X main.version=$(VERSION) -X main.commit=$(COMMIT)' -o "$(GOBIN)/goseed" ./cmd/goseed

package:
	python3 scripts/distribution.py package --version "$(VERSION)" --goos "$(GOOS)" --goarch "$(GOARCH)" --output .tmp/dist

verify-package:
	@test -n "$(ARCHIVE)" || { printf '%s\n' 'Set ARCHIVE to the package archive to verify.'; exit 1; }
	python3 scripts/distribution.py verify "$(ARCHIVE)"

test:
	$(GO) test -mod=readonly -timeout=40m $(GO_PACKAGES)

race:
	$(GO) test -mod=readonly -race -timeout=40m $(GO_PACKAGES)

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

template-smoke:
	$(GO) test -mod=readonly -count=1 -timeout=10m ./tests/integration -run '^TestResourceGeneratedProjectSmoke$$'

template-container:
	GOSEED_TEST_DOCKER=1 $(GO) test -mod=readonly -count=1 -timeout=15m -v ./tests/integration -run '^Test(DefaultProject|NoDatabaseProject|CasbinFileContainer|GRPCContainer|DualContainer|GatewayContainer)$$'

template-db:
	python3 scripts/template-db.py

template-redis:
	python3 scripts/template-redis.py

template-deployment: template-deployment-nginx
	docker build --platform linux/arm64 -f tests/integration/deployment.Dockerfile -t goseed-d11-systemd-test:local tests/integration
	GOSEED_TEST_DOCKER=1 $(GO) test -mod=readonly -count=1 -timeout=10m -v ./tests/integration -run '^TestSingleHostDeploymentContainer$$'

template-deployment-db:
	GOSEED_DEPLOYMENT_BACKUP_ONLY=1 python3 scripts/template-db.py

template-deployment-nginx:
	GOSEED_TEST_DOCKER=1 $(GO) test -mod=readonly -count=1 -timeout=5m -v ./tests/integration -run '^TestSingleHostNginxContainer$$'

# Explicit installation only; build/test/check never install global tools.
tools:
	GOBIN=$(CURDIR)/.tools/bin $(GO) install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(LINT_VERSION)
