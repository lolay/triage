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

.PHONY: help init install-tools aw-compile aw-check build lint format test vuln ci pre-commit doctor clean snapshot man tag publish-formula publish-scoop release \
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

# Pinned tool versions — the single source of truth. CI installs golangci-lint
# and actionlint via `make install-tools` and reads the goreleaser pin via
# `make print-GORELEASER_VERSION`; Renovate bumps both lines (custom managers in
# .github/renovate-shared.json). Keep the `NAME ?= vX.Y.Z` shape intact.
GOLANGCI_LINT_VERSION ?= v2.14.0
ACTIONLINT_VERSION ?= 1.7.12
GORELEASER_VERSION ?= v2.18.2
GH_AW_VERSION ?= v0.74.8

# Where install-tools puts binaries (must be on PATH).
TOOLS_BIN ?= $(shell go env GOPATH)/bin

# Maximum recent runs to fetch for gh-runs-list / gh-runs-watch.
GH_LIMIT ?= 50

##@ Develop

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z0-9_.-]+:.*?##/ {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2} /^##@/ {printf "\n\033[1m%s\033[0m\n", substr($$0,5)}' $(MAKEFILE_LIST)

init: ## Download Go module dependencies (INSTALL_PACKAGES=1 also runs install-tools)
	go mod download
	@if [ "$(INSTALL_PACKAGES)" = "1" ]; then $(MAKE) --no-print-directory install-tools; fi

install-tools: ## Install pinned golangci-lint + actionlint into TOOLS_BIN (no-op when already pinned)
	@set -o pipefail; \
	want="$(GOLANGCI_LINT_VERSION)"; want="$${want#v}"; \
	have="$$("$(TOOLS_BIN)/golangci-lint" version --short 2>/dev/null || true)"; \
	if [ "$$have" = "$$want" ]; then \
	  echo "golangci-lint $$want already installed in $(TOOLS_BIN)"; \
	else \
	  echo "Installing golangci-lint $(GOLANGCI_LINT_VERSION) into $(TOOLS_BIN)..."; \
	  curl -sSfL "https://raw.githubusercontent.com/golangci/golangci-lint/$(GOLANGCI_LINT_VERSION)/install.sh" \
	    | sh -s -- -b "$(TOOLS_BIN)" "$(GOLANGCI_LINT_VERSION)"; \
	fi; \
	if [ "$$("$(TOOLS_BIN)/actionlint" -version 2>/dev/null | head -1)" = "$(ACTIONLINT_VERSION)" ]; then \
	  echo "actionlint $(ACTIONLINT_VERSION) already installed in $(TOOLS_BIN)"; \
	else \
	  echo "Installing actionlint $(ACTIONLINT_VERSION) into $(TOOLS_BIN)..."; \
	  GOBIN="$(TOOLS_BIN)" go install "github.com/rhysd/actionlint/cmd/actionlint@v$(ACTIONLINT_VERSION)"; \
	fi

# The compiler runs with GitHub API lookups blocked on purpose: it then uses the
# action pins built into GH_AW_VERSION instead of resolving floating tags live,
# so the same sources always compile to byte-identical lock files (aw-check can
# diff them). Bumping GH_AW_VERSION is how the pins move.
aw-compile: ## Recompile gh-aw agent workflows (agent-*.md -> .lock.yml) with the pinned GH_AW_VERSION
	@set -euo pipefail; \
	bin="$(TOOLS_BIN)/gh-aw"; \
	if [ "$$("$$bin" version 2>/dev/null | awk '{print $$NF}')" != "$(GH_AW_VERSION)" ]; then \
	  case "$$(uname -s)" in Linux) os=linux ;; Darwin) os=darwin ;; *) echo "unsupported OS: $$(uname -s)" >&2; exit 1 ;; esac; \
	  case "$$(uname -m)" in x86_64|amd64) arch=amd64 ;; arm64|aarch64) arch=arm64 ;; *) echo "unsupported arch: $$(uname -m)" >&2; exit 1 ;; esac; \
	  echo "Installing gh-aw $(GH_AW_VERSION) into $(TOOLS_BIN)..."; \
	  tmp="$$(mktemp -d)"; trap 'rm -rf "$$tmp"' EXIT; \
	  base="https://github.com/github/gh-aw/releases/download/$(GH_AW_VERSION)"; \
	  curl -sSfL -o "$$tmp/$$os-$$arch" "$$base/$$os-$$arch"; \
	  curl -sSfL -o "$$tmp/checksums.txt" "$$base/checksums.txt"; \
	  (cd "$$tmp" && grep " $$os-$$arch\$$" checksums.txt | { command -v sha256sum >/dev/null && sha256sum -c - || shasum -a 256 -c -; }); \
	  mkdir -p "$(TOOLS_BIN)"; install -m 0755 "$$tmp/$$os-$$arch" "$$bin"; \
	fi; \
	HTTPS_PROXY=http://127.0.0.1:9 HTTP_PROXY=http://127.0.0.1:9 NO_PROXY= \
	  "$$bin" compile --no-check-update

aw-check: aw-compile ## Fail if the gh-aw lock files are stale (CI runs this)
	@changes="$$(git status --porcelain --untracked-files=all -- .github/workflows .github/aw)"; \
	if [ -n "$$changes" ]; then \
	  echo "$$changes"; \
	  echo "gh-aw lock files are out of date — run 'make aw-compile' and commit the result" >&2; exit 1; \
	fi

build: ## Compile the triage binary into bin/ (stamps build metadata)
	go build -ldflags '$(LDFLAGS)' -o $(BIN_DIR)/$(BINARY) .

lint: ## Static checks: gofmt drift + go vet + golangci-lint + actionlint (required; run make install-tools)
	@set -o pipefail; \
	drift="$$(gofmt -l .)"; \
	if [ -n "$$drift" ]; then \
	  echo "gofmt drift detected — run 'make format':"; echo "$$drift"; exit 1; \
	fi
	go vet ./...
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
	  echo "golangci-lint not found — run 'make install-tools' to install" >&2; exit 1; \
	fi
	@want="$(GOLANGCI_LINT_VERSION)"; have="$$(golangci-lint version --short 2>/dev/null)"; \
	if [ "$$have" != "$${want#v}" ]; then \
	  echo "warning: golangci-lint $$have on PATH, pinned $(GOLANGCI_LINT_VERSION) — run 'make install-tools'" >&2; \
	fi
	golangci-lint run
	@if ! command -v actionlint >/dev/null 2>&1; then \
	  echo "actionlint not found — run 'make install-tools' to install" >&2; exit 1; \
	fi
	actionlint

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

# Print a Makefile variable, e.g. `make -s print-GORELEASER_VERSION` (CI reads
# tool pins this way so they live only here).
print-%:
	@echo '$($*)'

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
