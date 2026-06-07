# Changelog

All notable changes to `triage` are recorded here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Releases are tagged on the `lolay/triage` repository; m5 wires the release
pipeline. Until then entries are grouped by milestone and refactor pass.

---

## [Unreleased] — Go standards conformance refactor

_2026-06-06 – 2026-06-07 · commits `c3aaf28`…`88d5770`_

A four-group pass bringing the codebase into full conformance with the project's
Go standards. No behaviour changes; `make ci` green throughout.

### Changed
- `main.go` moved from `cmd/triage/main.go` to the repo root (single-binary
  standard layout); `Makefile` and `.goreleaser.yaml` build paths updated.
- `make test` now runs with `-cover`; per-package coverage prints on every run.
- `CONTRIBUTING.md` testing convention updated to require
  `testify require`/`assert` + golden output fixtures.

### Added
- Linters: `modernize`, `errorlint`, `revive`, `bodyclose`, `govet enable-all`
  enabled in `.golangci.yml`.
- Security linter: `gosec` enabled; every production finding triaged
  individually with a scoped `//nolint:gosec // G…:` justification comment.
- `govulncheck` wired as a `go.mod` tool dependency (`go 1.24+` pattern);
  `make vuln` target added.
- CI: `go mod verify` step (before build) and `make vuln` step (after `make ci`)
  run on every push and PR.
- `TestJSON_KeyOrder` locks `--json` output key order against accidental
  `fieldalignment` reorder (spec §7.3).
- `testify v1.11.1` added; all 10 `*_test.go` files converted to
  `require`/`assert` (net −404 test lines; coverage and behaviour unchanged).

### Removed
- Dead/speculative code: `devNull` (`engine/tool.go`), `StaticSink` /
  `NewStaticSink` (`report/sink.go`), `mergeVarMaps` + its test
  (`cli/vars.go`), `Check` interface (`engine/check.go`).

### Fixed
- `command_log.go` directory and file permissions hardened to `0o750`/`0o600`
  (the log can capture `with_env` values which may include secrets).
- Pre-existing `// indirect` markers on `Masterminds/semver/v3` and
  `golang.org/x/term` corrected to direct in `go.mod`.

---

## [m4] — Delegation & concurrency

_2026-06-06 · commits `64745a9`…`27ed00d`_

### Added
- `delegate` check type: recursive nesting into child `triage.yaml` configs;
  delegate results rendered as an indented sub-board.
- Bounded worker pool (`--jobs` / `-j`, default ≈ `NumCPU` capped at 8):
  local checks and sibling delegates run concurrently. `serial: true` opt-out
  field on any check or group for scarce/exclusive resources.
- Results and `--command-log` blocks always materialize in config **list order**
  regardless of execution order — golden output is `--jobs`-invariant.
- Global process-level serial lock so `serial:` checks across sibling delegates
  never overlap.
- `context.Context` cancellation propagated to all subprocesses (clean Ctrl-C).
- `triage.yml` accepted as a config filename alongside `triage.yaml`.

---

## [m3] — Full check types

_2026-06-06 · commit `ec273f2` + `4b1d089` + `4025527`_

### Added
- `env` check: set/unset mode, optional `matches` regex.
- `path` check: exact path, glob (`*.p8`), `~` home expansion.
- `one_of` check: first-passing alternative wins; derived label from child names.
- `command` check: explicit interpreter (`sh`, `pwsh`, …), `--command-log` tee,
  `--verbose` stderr replay, `exit`/`contains`/`matches` assertions, 256 KiB
  output cap with `--command-log` redirect hint on overflow.
- `platform` guard on any check or group (skips on non-matching OS).
- `with_env`: per-command environment injection (overrides inherited env).
- `vars`: template `{{ name }}` expansion across all check fields; `profile` and
  `os` built-ins injected at runtime; `--var key=value` CLI override.

---

## [m2] — Config & the `tool` check

_2026-06-05 · commit `a03f64b`_

### Added
- `triage.yaml` schema and loader (`goccy/go-yaml`): mapping root, top-level
  profile keys, `include` (cycle/diamond-safe), `extends` / `add` inheritance,
  `version_from` pin-file support.
- `tool` check: presence via `exec.LookPath`, version extraction (including
  `go version` / node `v`-prefix overrides), npm-style semver constraints via
  `Masterminds/semver`.
- Severity model: `error` / `warn` / `info`, `required: false` sugar, `--strict`
  flag (warnings become failures).
- Grouped board output with remediation summary; `group` containers.
- Finalize-in-order renderer (results emitted in config list order, independent
  of execution timing — ready for concurrency with no rewrite at m4).
- JSON Schema published for editor auto-complete.
- `--json` structured output (spec §7.3).

---

## [m1] — Project, CLI skeleton & README

_2026-06-05 · commits `479bf3a`…`a03f64b`_

### Added
- Go module (`github.com/lolay/triage`), `.go-version` pinning Go 1.26,
  single-binary layout.
- Root `Makefile`: `help` default, self-documenting `##`/`##@` targets,
  `build` / `test` / `lint` / `doctor` / `ci` / `pre-commit` / `clean` /
  `init` / `snapshot`.
- GitHub Actions CI (`make ci` + `goreleaser check`).
- `goreleaser` stub (no publish yet).
- `cobra` CLI shell: `triage [config]`, `--profile`, `--json`, `--quiet`,
  `--strict`, `--severity`, `--no-color`, `--command-log`, `--verbose`,
  `--no-update-check`, `--version`; exit codes `0`/`1`/`3` (pass-fail) and
  `0`/`1`/`2`/`3` (`--severity` mode).
- Config discovery (`triage.yaml` / `triage.yml` / `.triage.yaml` /
  `.triage.yml`, directory or explicit file argument).
- TTY board renderer with ANSI colour, `--no-color`, pending `[…]` spinner.
- Golden-output test harness; first end-to-end test (empty config → clean board).
- Community files: LICENSE (Apache 2.0), README, CONTRIBUTING, CODE_OF_CONDUCT,
  SECURITY, SUPPORT.
