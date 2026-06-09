# Changelog

All notable changes to `triage` are recorded here.
Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

---

## [Unreleased]

### Changed
- `triage.yaml` `release` profile: replaced `cosign` (no call site) with `mandoc`
  (lints hand-authored man pages before release); `mandoc` is `error` severity so
  `make doctor MODE=release` blocks on a missing install rather than silently
  shipping unvalidated man pages.

---

## [v0.3.0] — 2026-06-08

### Added
- `specs/action.md` — canonical contract for the `lolay/triage-action` composite
  action: typed inputs for the CI-relevant flags (`strict`, `severity`, `json`,
  `quiet`, `verbose`, `command-log`, `jobs`, …) plus an `args` escape hatch, outputs,
  tokenless CLI version resolution (`version` input → root `VERSION` file → fail
  fast), OS/arch + checksum asset mapping, exit-code forwarding (no `fail-on` in
  v1), coordinated semver/floating tags (`vX.Y` + `vX`), and the finalized
  `action.yml` / `install.sh` / `run.sh` shapes.
- GitHub Actions CI dogfood job using `lolay/triage-action@v0` (`.github/workflows/ci.yml`).
- `ci` profile in `triage.yaml` (extends `default`, adds `gh` for CI workflows).
- README **CI / GitHub Actions** section with `lolay/triage-action@v0.3` example.

### Changed
- `specs/product.md` §3 and `specs/milestones.md` m6 cross-link the new
  `specs/action.md` contract.

### Fixed
- `internal/updatecheck`: struct field ordering, permission bits (0o750/0o600),
  filepath.Clean on cache reads, shadowed `err` variable.

---

## [Unreleased] — m5: release & distribution pipeline

_2026-06-07 · commit `afddce6`_

### Added
- GoReleaser v2 config: darwin/linux/windows × amd64/arm64 builds, GitHub release
  assets (windows archives as `.zip`), and man pages bundled in all archives.
- Binary-download Homebrew formula (`Formula/triage.rb`) generated at release time
  via `scripts/publish-formula.sh` and committed to `lolay/homebrew-tap`.
- GitHub Actions release workflow (`.github/workflows/release.yml`):
  tag-triggered `goreleaser release` job, formula publish step, and
  `promote-floating-tags` job (`vX.Y`, `vX`) sequenced strictly after the tap commit.
- `make tag VERSION=x.y.z` and guarded `make release` (`CONFIRM_RELEASE=1`)
  and `make publish-formula` (`CONFIRM_PUBLISH_FORMULA=1`) targets; `make man`
  runs `mandoc -Tlint` over both man pages.
- `make snapshot` runs `goreleaser release --snapshot` (full archives +
  checksums locally, no publish).
- Man pages `man/triage.1` (CLI reference) and `man/triage.5` (config file
  format), hand-authored mdoc; mandoc lint clean.
- Update-availability banner: `internal/updatecheck` fetches GitHub
  `releases/latest`, caches to XDG cache dir (24 h TTL, 300 ms timeout),
  compares semver, and prints a one-line hint. Suppressed in CI, JSON mode,
  non-TTY, `--no-update-check`, and `TRIAGE_NO_UPDATE_CHECK=1`.
- `specs/config.md` — canonical prose reference for `triage.yaml` format,
  check types, composition, vars, and severity.
- `specs/releasing.md` — maintainer guide: marketplace-last ordering rule,
  phase diagram, required secrets, floating/pre-release tag rules, tap
  bootstrap, and post-release verification.

### Changed
- Homebrew install command: `brew install lolay/tap/triage` (cross-platform
  binary-download formula; `brew upgrade` picks up new releases on macOS, Linux,
  and WSL2).
- `specs/product.md §4–§5` trimmed to one-paragraph summaries that link to
  `specs/config.md`; full check-type and config-model detail lives there now.
- `triage.yaml` header comment updated to point to `specs/config.md`.
- `README.md`: install section updated for Homebrew formula, Releasing section
  added linking to `specs/releasing.md`.

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
