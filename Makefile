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

.PHONY: help init build lint format test vuln ci pre-commit doctor clean snapshot man tag publish-formula publish-scoop release \
        gh-runs-list gh-runs-watch gh-runs-status

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

# Pinned golangci-lint version — must match the Install step in ci.yml.
GOLANGCI_LINT_VERSION ?= v2.12.2

# Maximum recent runs to fetch for gh-runs-list / gh-runs-watch.
GH_LIMIT ?= 50

##@ Develop

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_.-]+:.*?##/ {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2} /^##@/ {printf "\n\033[1m%s\033[0m\n", substr($$0,5)}' $(MAKEFILE_LIST)

init: ## Download Go module dependencies (INSTALL_PACKAGES=1 also installs golangci-lint)
	go mod download
	@if [ "$(INSTALL_PACKAGES)" = "1" ] && ! command -v golangci-lint >/dev/null 2>&1; then \
	  echo "Installing golangci-lint (latest)..."; \
	  curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh \
	    | sh -s -- -b "$$(go env GOPATH)/bin"; \
	fi

build: ## Compile the triage binary into bin/ (stamps build metadata)
	go build -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(BINARY) .

lint: ## Static checks: gofmt drift + go vet + golangci-lint (required; run make init)
	@set -o pipefail; \
	drift="$$(gofmt -l .)"; \
	if [ -n "$$drift" ]; then \
	  echo "gofmt drift detected — run 'make format':"; echo "$$drift"; exit 1; \
	fi
	go vet ./...
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
	  echo "golangci-lint not found — run 'make init' to install" >&2; exit 1; \
	fi
	golangci-lint run

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

##@ GitHub

gh-runs-list: ## List this repo's in-flight Actions runs (status != completed)
	@out=$$(gh run list --limit $(GH_LIMIT) \
	  --json status,workflowName,headBranch,event,url \
	  --jq '.[] | select(.status != "completed") | "  \(.status)\t\(.workflowName)\t\(.headBranch)\t\(.event)\t\(.url)"' 2>&1) \
	  || { printf '  \033[33m⚠\033[0m gh run list failed (auth? run `gh auth login`)\n'; exit 0; }; \
	if [ -z "$$out" ]; then printf '  \033[2mno active runs\033[0m\n'; \
	else printf '%s\n' "$$out" | column -t -s "$$(printf '\t')"; fi

gh-runs-watch: ## Watch this repo's in-flight Actions runs until each completes
	@ids=$$(gh run list --limit $(GH_LIMIT) --json status,databaseId \
	  --jq '.[] | select(.status != "completed") | .databaseId' 2>/dev/null); \
	if [ -z "$$ids" ]; then printf '  \033[2mno active runs\033[0m\n'; exit 0; fi; \
	for id in $$ids; do \
	  gh run watch "$$id" --compact || printf '  \033[33m⚠\033[0m watch failed for run %s\n' "$$id"; \
	done

gh-runs-status: ## Show pass/fail of the last completed run per workflow
	@out=$$(gh run list --limit $(GH_LIMIT) \
	  --json conclusion,workflowName,headBranch,url,status,updatedAt \
	  --jq '[.[] | select(.status == "completed")] | group_by(.workflowName) | map(sort_by(.updatedAt) | last) | sort_by(.updatedAt) | .[] | (now - (.updatedAt | fromdateiso8601)) as $$age | "\(.conclusion)\t\(.workflowName)\t\(.headBranch)\t\(.url)\t\($$age | floor)"' \
	  2>&1) \
	  || { printf '  \033[33m⚠\033[0m gh run list failed (auth? run `gh auth login`)\n'; exit 0; }; \
	if [ -z "$$out" ]; then printf '  \033[2mno completed runs\033[0m\n'; exit 0; fi; \
	esc=$$(printf '\033'); \
	printf '%s\n' "$$out" | while IFS=$$'\t' read -r conclusion name branch url age_secs; do \
	  if [ "$$conclusion" = "success" ]; then mark="ok"; \
	  elif [ "$$conclusion" = "skipped" ] || [ "$$conclusion" = "neutral" ]; then mark="skip"; \
	  else mark="fail"; fi; \
	  if [ "$$age_secs" -lt 60 ]; then age="$${age_secs}s"; \
	  elif [ "$$age_secs" -lt 3600 ]; then age="$$((age_secs / 60))m"; \
	  elif [ "$$age_secs" -lt 86400 ]; then age="$$((age_secs / 3600))h"; \
	  else age="$$((age_secs / 86400))d"; fi; \
	  printf '%s\t%s\t%s\t%s\t%s\n' "$$mark" "$$name" "$$branch" "$$age" "$$url"; \
	done | column -t -s "$$(printf '\t')" \
	| sed -e "s/^ok  /$${esc}[32m✓$${esc}[0m   /" \
	      -e "s/^skip/$${esc}[2m-$${esc}[0m   /" \
	      -e "s/^fail/$${esc}[31m✗$${esc}[0m   /" \
	      -e 's/^/  /'

##@ Release

snapshot: ## Build a local release snapshot (goreleaser, no publish)
	goreleaser release --snapshot --clean

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

publish-formula: ## Render and push Formula/triage.rb from dist/ (VERSION=x.y.z; CONFIRM_PUBLISH_FORMULA=1)
	$(call confirm,PUBLISH_FORMULA)
	@test -n "$(VERSION)" || { echo "VERSION=x.y.z required" >&2; exit 1; }
	scripts/publish-formula.sh "$(VERSION)" dist

publish-scoop: ## Render and push bucket/triage.json from dist/ (VERSION=x.y.z; CONFIRM_PUBLISH_SCOOP=1)
	$(call confirm,PUBLISH_SCOOP)
	@test -n "$(VERSION)" || { echo "VERSION=x.y.z required" >&2; exit 1; }
	scripts/publish-scoop.sh "$(VERSION)" dist

release: ## Publish release for current tag via goreleaser (tag must exist)
	$(call confirm,RELEASE)
	goreleaser release --clean
