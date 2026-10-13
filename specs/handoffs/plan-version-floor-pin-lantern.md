# Minimum vs. recommended versions: triage, standards, nowline

Status: m1 done and pushed on `claude/hopeful-dijkstra-5rz2m9` (triage). m2 (standards) next, waiting on approval.

## Context

**Today triage has no notion of "recommended."** A `tool` check carries one
range (`version` or `version_from`) and one severity for the whole check
(`internal/engine/tool.go` `checkTool`, `internal/engine/runner.go`
`resolveSeverity`). `version_from` turns a bare pin into `>=pin`
(`internal/config/load.go` `constraintFromPin`). The only way to express
"error below 22, warn below 26.2" is two separate `tool: node` checks with
different severities: two probes, two board lines, and a missing tool shows
up as both `[✗]` and `[!]`.

**The consequence is a real bug in both repos, not a hypothetical.**
- nowline `triage.yaml` does `version_from: .nvmrc` → `>=26.2.0` at error.
  A contributor on Node 22 gets a hard `[✗]`, yet `CONTRIBUTING.md` declares
  `>=22` as the supported floor and `ci.yml` runs a Node 22 unit-test cell
  on every PR. (Small correction to the framing: the floor is `engines.node:
  ">=22"`, not 22.2.)
- triage itself: `go.mod` floor is `go 1.26.0`, `.go-version` pin is
  `1.27.1`, and `triage.yaml` errors below 1.27.1. The 0.4.1 changelog
  even calls out the split.

**How other tools model it.** Nearly every ecosystem keeps two separate
declarations, a floor (compat contract, hard) and a pin (what you build and
test with, soft or auto-installed):

| Ecosystem | Floor (hard) | Pin (recommended) | Below pin |
|---|---|---|---|
| Go | `go.mod` `go 1.26` | `toolchain go1.27.1` / `.go-version` | auto-switches toolchain |
| Rust | `Cargo.toml` `rust-version` (error, `--ignore-rust-version`) | `rust-toolchain.toml` | rustup installs it |
| Python/uv | `requires-python` | `.python-version` | uv installs it; errors if pin violates floor |
| Node | `engines.node` (warn, error with `engine-strict`) | `.nvmrc` | nvm/fnm switch on `use` |
| npm `devEngines` | per-entry `onFail: error\|warn\|ignore` | — | one range per entry, severity per entry |
| Android | `minSdk` | `targetSdk` | lint warns on stale target |
| Debian / Homebrew | `Depends` / `depends_on` | `Recommends` / `=> :recommended` | soft |
| flutter / expo doctor | `[✗]` unmet requirement | `[!]` works but not what we expect | warning |

Nobody ships a general N-tier severity ladder. The common shape is exactly
two thresholds, and doctor-style tools render the gap as a warning. That is
what triage should do.

## Recommendation for triage

Add `recommended` and `recommended_from` to `tool` checks, mirroring
`version` / `version_from` (same parsing, same `constraintFromPin`, same
"both set → loader warns, explicit wins" rule). The name matches triage's
existing vocabulary: the severity table already defines `warn` as
"Recommended."

Evaluation, one probe, one board line:

| Result | Severity |
|---|---|
| tool missing | check severity (unchanged) |
| fails `version` | check severity (unchanged) |
| meets `version`, fails `recommended` | `warn`, or the check's severity if lower (`info` stays `info`) |
| meets both | pass |

Board text: pass `node 26.2.0 (>=22, recommended >=26.2.0)`; warn
`node 22.11.0 — recommended >=26.2.0 (minimum >=22 met) — <hint>`.
`--strict` promotes the warn to error with no extra code, which is the
right lever for a "must be on the pin" CI job. `recommended` without
`version` is allowed (warn-only floor); no range-intersection validation
(not worth the code).

Deferred, deliberately: teaching `version_from` to read `package.json`
`engines.<tool>` or the `go.mod` `go`/`toolchain` directives. It would kill
one duplication but adds per-format parsers; the floor changes rarely and
is already a policy-level, multi-file edit. Revisit if a third repo wants it.

### YAML

Every existing config is unchanged. The new keys are optional siblings of
`version` / `version_from`:

```yaml
default:
  # Before (still valid, same meaning): pin is the floor, error below 26.2.0
  - tool: node
    version_from: .nvmrc

  # After: error below 22, warn between 22 and 26.2.0, pass at 26.2.0+
  - tool: node
    version: ">=22"              # minimum, at the check's severity (error)
    recommended_from: .nvmrc     # 26.2.0 -> ">=26.2.0", reported as warn
    hint: nvm install            # shown on both the error and the warning

  # Inline recommended range instead of a pin file
  - tool: go
    version: ">=1.26"
    recommended: ">=1.27.1"

  # Recommended only, no floor: missing tool is still an error,
  # old-but-present is a warning
  - tool: terraform
    recommended_from: .terraform-version

  # Optional tool: missing is warn, and below recommended stays warn
  - tool: golangci-lint
    severity: warn
    recommended: ">=2.14"
```

Board on Node 22.11.0 with the second form:

```
[!] Toolchain
    [!] node 22.11.0 — recommended >=26.2.0 (minimum >=22 met) — nvm install
```

### Backward compatibility

Both directions hold, and the design is chosen for it:

- **Old configs on new triage:** nothing changes. A check without
  `recommended`/`recommended_from` evaluates exactly as today, same
  severity, same message, same JSON shape. `version_from` keeps its
  "bare pin means `>=pin`" meaning; it is not reinterpreted as recommended.
- **New configs on old triage:** `Check.UnmarshalYAML` decodes into
  `checkFields` without strict mode, so 0.4.x silently ignores the new keys
  and runs the floor (`version: ">=22"`) only. You lose the warning, not
  the run. This is why the new fields are siblings rather than turning
  `version` into a mapping (`version: {min: …, recommended: …}`), which
  would be a decode error on every older triage.
- **JSON report:** no new required fields. A below-recommended result is an
  ordinary `pass: false, severity: "warn"` result, so existing consumers
  (exit-code ladder, `--json` readers, group roll-ups) need no changes.
- **Exit codes:** default mode stays pass/fail on errors only, so adding
  `recommended` to a config never turns a passing CI run red. Only
  `--strict` (already "warnings are errors") changes outcome, by design.
- **Schema:** the JSON Schema gains two optional properties; old configs
  still validate. Editors on the old schema will flag the new keys until
  they pick up the new schema, which is cosmetic.

## ~~m1 - triage~~ (done) (`lolay/triage`, branch `claude/hopeful-dijkstra-5rz2m9`)

- `internal/config/config.go`: add `Recommended`, `RecommendedFrom`
  (yaml `recommended`, `recommended_from`) and resolved
  `RecommendedConstraint` to `Check` and the raw field struct.
- `internal/config/load.go` `postProcessCheck`: resolve
  `RecommendedConstraint` with the existing `readVersionFrom`; warn on both
  set and on unreadable pin, same wording pattern as `version_from`.
- `internal/engine/tool.go`: refactor `checkTool` so the probe/parse runs
  once, then checks the minimum, then the recommended constraint; return a
  flag (or a distinct outcome) for "below recommended."
- `internal/engine/runner.go` tool case: when below recommended, set
  severity to `min(sev, SeverityWarn)` instead of `sev`; adjust the pass
  label to include both ranges.
- `schema/triage.schema.json`, `specs/config.md` (`tool` section +
  Severity section note), `man/triage.5`, `README.md` example,
  one `examples/*.yaml` use.
- Dogfood: triage's own `triage.yaml` → `go`: `version: ">=1.26"`,
  `recommended_from: .go-version`.
- Tests (fakes via `RunnerOpts.LookPath`/`RunProbe`, matching
  `tool_test.go`): below floor → error; between → warn; at/above pin →
  pass; `info` check stays `info`; `--strict` promotes; `recommended_from`
  resolution and both-set warning in `load_test.go`; JSON report severity.
- `CHANGELOG.md` `Unreleased` → `Added` entry.

Release (tag + tap bump) waits for your go-ahead, per standards.

## m2 - standards (`GaryRudolph/public`, same branch)

- `standards/architecture.md` (real file under
  `plugins/personal/skills/personal-standards/standards/`): new short
  section "Toolchain floor and pin" after "Starting New Projects."
  Rule: a library or anything with external consumers declares both a
  floor (the compat contract: `engines`, `go.mod` `go`, `requires-python`,
  `rust-version`) and a pin (`.nvmrc`, `.go-version`, `.python-version`,
  `rust-toolchain.toml`). The doctor errors below the floor and warns below
  the pin (`version:` floor + `recommended_from:` pin file). CI exercises
  the floor in at least one cell. Raising the floor is a breaking change;
  bumping the pin is not. An app with no external consumers may use the pin
  as the floor (today's `version_from` alone).
- `standards/makefile.md` "Common tooling" / `doctor` row: one-line pointer
  to the floor/pin rule.
- `plugins/personal/skills/personal-new-project/SKILL.md` §6 "Record the
  baseline": record floor and pin separately and wire both into
  `triage.yaml`.
- Check `public` for a changelog convention before adding an entry.

## m3 - nowline (`lolay/nowline`, same branch; after triage release)

- `triage.yaml`: `node` → `version: ">=22"`, `recommended_from: .nvmrc`;
  `pnpm` → `version: ">=11"` (no recommended: corepack already enforces
  `packageManager` `pnpm@12.8.1`, and that pin isn't in a pin-file format).
- `CONTRIBUTING.md` "How to bump" Tier 1: add `triage.yaml` to the files
  touched when the consumer floor moves; Tier 2 note that `.nvmrc` drives
  the triage warning automatically.
- `.changelog/` fragment per nowline's convention (read
  `.changelog/README.md` first; the `steward` skill applies if a PR opens).

Sequencing: m3 can land before the triage release without breaking anyone
(old triage ignores `recommended_from` and checks `>=22`), but contributors
on old triage get no nudge toward 26 until they upgrade. Landing it after
the release is cleaner.

## Verification

- triage: `make ci` (or the repo's lint + `go test ./...`); then run the
  built binary against a scratch `triage.yaml` with a fake `node` on PATH
  printing `v22.11.0`, `v21.0.0`, `v26.2.0` and confirm `[!]`, `[✗]`,
  `[✓]`, exit codes with and without `--severity` / `--strict`, and `--json`.
- triage dogfood: `make doctor` clean on Go 1.27.1; warn on a fake 1.26.x.
- Backward compat: add a `load_test.go` case proving a config without the
  new keys produces byte-identical board and JSON output to today (golden
  from 0.4.1). Run the released 0.4.1 binary against the new nowline and
  triage configs and confirm it loads with no config error and enforces the
  floor only.
- nowline: `make doctor` with the new triage on Node 26 (clean) and Node 22
  (one `[!]`, exit 0).
- Commit and push each repo's branch per step; no PRs until asked.
