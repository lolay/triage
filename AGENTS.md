# AGENTS.md

Orientation for AI coding agents (Copilot coding agent, Claude Code, Cursor,
Codex CLI, Aider) working in **lolay/triage**, the environment-doctor CLI.
Humans start with [`README.md`](./README.md) and
[`CONTRIBUTING.md`](./CONTRIBUTING.md).

## What this repo is

A Go CLI that reads `triage.yaml` and reports missing tools, versions, env
vars, files, and auth probes as a grouped board. Product spec:
[`specs/product.md`](./specs/product.md); config reference:
[`specs/config.md`](./specs/config.md). It lives as a child of
[`lolay/triage-workspace`](https://github.com/lolay/triage-workspace); the
GitHub Action wrapper is [`lolay/triage-action`](https://github.com/lolay/triage-action).

## Build commands

Always use the Makefile — CI runs the same targets:

```bash
make init INSTALL_PACKAGES=1   # go mod download + pinned golangci-lint/actionlint
make build                     # compile bin/triage
make pre-commit                # full gate (alias of make ci): build + lint + test
make vuln                      # govulncheck
make aw-check                  # only if you touched frontmatter in .github/workflows/agent-*.md or the prelude
```

The Go toolchain is pinned in `.go-version`; tool versions are pinned in the
`Makefile` (`GOLANGCI_LINT_VERSION`, `ACTIONLINT_VERSION`, `GORELEASER_VERSION`,
`GH_AW_VERSION`). Don't pin versions anywhere else.

## Hard rules

- **Golden output is a deliberate baseline.** Don't rebaseline
  `internal/cli/testdata/*/stdout.golden` unless the change intends a visible
  output change, and say why in the PR.
- **`--json` shape and key order are a contract** (`TestJSON_KeyOrder`).
- **Config surface moves together:** `schema/triage.schema.json`,
  `specs/config.md`, `man/triage.5`, and an `examples/` config.
- **No implicit shell.** Structured checks spawn tools directly; `command`
  checks name their interpreter and are platform-guarded.
- **Every POSIX fake binary in `testdata/*/bin/` needs a `.bat` companion** so
  tests pass on Windows.
- **The `go` directive in `go.mod` is the consumer floor.** Raise it only as a
  deliberate policy change, never as part of a dependency bump.
- **Releases are maintainer-only.** Don't edit `release.yml`, tag, or cut a
  release (`specs/releasing.md`).

## Changelog

User-visible changes need a Keep-a-Changelog entry under `## [Unreleased]` in
`CHANGELOG.md` (see `.cursor/rules/changelog.mdc`).

## AI assistance

Follow [`AI_POLICY.md`](./AI_POLICY.md): `Assisted-by: <agent + version>` on
every commit and in the PR's `## AI assistance` section. Agent-flow PRs also
need `Closes #N` and the plan reproduced under `## Plan from issue`
([`.github/AGENT_TRIAGE.md`](./.github/AGENT_TRIAGE.md)).

## Dependency PRs (Renovate)

If you are asked to fix a failing Renovate PR: keep the update, make the
smallest change that gets `make pre-commit` green, regenerate files with
Makefile targets (`go mod tidy`, `make aw-compile`) rather than by hand, and
explain in a comment instead of forcing a fix when the update needs a design
decision.

## Workspace parent

When working across repos, read the workspace
[`AGENTS.md`](https://github.com/lolay/triage-workspace/blob/main/AGENTS.md):
commits belong in this repo's history, not the workspace wrapper.
