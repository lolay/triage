# triage Makefile
#
# The single source of truth for build / lint / test / release command
# strings. Humans, local agents, cloud agents, and CI all run the same
# verbs — the CI workflow calls `make ci`, so a green `make ci` locally
# means the same checks CI runs.
#
# Conventions (shared across our Go CLI repos):
#   - `.DEFAULT_GOAL := help`; bare `make` prints the grouped target list.
#   - Self-documenting: a `## comment` after a target shows up in `make help`;
#     a `##@ Section` line renders as a header (section-aware awk, so help
#     can never drift from the targets).
#   - Recipes are tab-indented (a Make requirement) and run under bash
#     (SHELL below) with `set -o pipefail`, so pipelines and `[[ ... ]]`
#     behave the same on macOS and Linux.

SHELL := bash

.DEFAULT_GOAL := help

.PHONY: help init build lint format test vuln ci pre-commit doctor clean snapshot man tag release

# Remote-mutating targets refuse to run without CONFIRM_* (CI sets inline).
define confirm
$(if $(CONFIRM_$(1)),,$(error Set CONFIRM_$(1)=1 to run $@))
endef

# Module + binary coordinates.
MODULE  := github.com/lolay/triage
BINARY  := triage
BIN_DIR := bin

# Build metadata injected into internal/buildinfo via -ldflags. Overridable so
# CI and goreleaser can pass exact values; the defaults serve a local build.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE    ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -s -w \
	-X $(MODULE)/internal/buildinfo.Version=$(VERSION) \
	-X $(MODULE)/internal/buildinfo.Commit=$(COMMIT) \
	-X $(MODULE)/internal/buildinfo.Date=$(DATE)

# MODE selects the dogfood profile for `make doctor` (default | release).
MODE ?= default

##@ Develop

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*?##/ {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2} /^##@/ {printf "\n\033[1m%s\033[0m\n", substr($$0,5)}' $(MAKEFILE_LIST)

init: ## Download Go module dependencies
	go mod download

build: ## Compile the triage binary into bin/ (stamps build metadata)
	go build -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(BINARY) .

lint: ## Static checks: gofmt drift + go vet + golangci-lint (if installed)
	@set -o pipefail; \
	drift="$$(gofmt -l .)"; \
	if [ -n "$$drift" ]; then \
	  echo "gofmt drift detected — run 'make format':"; echo "$$drift"; exit 1; \
	fi
	go vet ./...
	@set -o pipefail; \
	if command -v golangci-lint >/dev/null 2>&1; then \
	  golangci-lint run; \
	else \
	  echo "golangci-lint not found — skipping (install: https://golangci-lint.run; CI runs it)"; \
	fi

format: ## Auto-fix formatting (gofmt -w .)
	gofmt -w .

test: ## Run all tests (race detector on; prints per-package coverage)
	go test -race -cover ./...

vuln: ## Scan dependencies for known vulnerabilities (govulncheck)
	go tool govulncheck ./...

ci: build lint test ## Full pre-push gate: build, lint, test (what CI runs)

pre-commit: ci ## Local gate before committing or pushing (alias of ci)

doctor: build ## Dogfood: run triage against this repo's triage.yaml. MODE=default|release
	@set -o pipefail; \
	if [ "$(MODE)" = "default" ]; then \
	  "$(BIN_DIR)/$(BINARY)"; \
	else \
	  "$(BIN_DIR)/$(BINARY)" --profile "$(MODE)"; \
	fi

clean: ## Remove build artifacts (bin/, dist/)
	rm -rf $(BIN_DIR) dist

##@ Release

snapshot: ## Build a local release snapshot (goreleaser, no publish)
	goreleaser build --snapshot --clean

man: ## Lint hand-authored man pages (mandoc -Tlint, best-effort)
	@set -e; \
	if command -v mandoc >/dev/null 2>&1; then \
	  mandoc -Tlint man/triage.1 man/triage.5; \
	else \
	  echo "mandoc not found — skipping lint (install mandoc to validate man pages)"; \
	fi

tag: ## Create and push git tag VERSION=x.y.z (triggers release workflow)
	$(call confirm,TAG)
	@test -n "$(VERSION)" || { echo "VERSION=x.y.z required" >&2; exit 1; }
	git tag "v$(VERSION)"
	git push origin "v$(VERSION)"

##@ Danger

release: ## Publish release for current tag via goreleaser (tag must exist)
	$(call confirm,RELEASE)
	goreleaser release --clean
