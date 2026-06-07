# triage — Milestones

Sequenced **parity-first**: m1–m2 stand up the tool; m3–m5 reach feature-parity
with real-world bash doctor scripts (validated via `examples/` + golden fixtures);
m6 ships CI adoption via sibling **`lolay/triage-action`**; m7 is forward-looking. Each
milestone is shippable. **Repo migration is not a milestone in this repo.**

> Tags per the model-tier convention: `[deep]` architecture/ambiguous,
> `[exec]` repo-aware implementation, `[fast]` mechanical/fully-spec'd.

### ~~m1 — Project, CLI skeleton & README~~ ✓

- s1 — [deep] Repo bootstrap: Go module + `.go-version`, layout (`cmd/triage`, `internal/`), root `Makefile` (§11 conventions: `help` default, self-documenting `##`/`##@`, `build`/`test`/`lint`/`doctor`/`ci`/`pre-commit`/`clean`/`init`), CI calls `make ci`, `goreleaser` stub (no publish yet). Community files (LICENSE/README/CONTRIBUTING/COC/SECURITY/SUPPORT) already exist (§11); `.github/` = workflows + CODEOWNERS only — no issue/PR templates (§12)
- s2 — [exec] `cobra` CLI shell: flat `triage` (default = run checks), optional `[config]` positional, `--version`, global flags (`--profile`, `--json`, `--quiet`, `--strict`, `--severity`, `--no-color`, `--command-log`, `--verbose`, `--no-update-check`); exit-code plumbing (pass-fail `0`/`1`/`3` default; `--severity` → `0`/`1`/`2`/`3`) (§4 CLI). **`--only` deferred** (§7.3)
- s3 — [exec] README already drafted (§11) — keep in sync as flags/commands land
- s4 — [fast] Golden-output harness + first end-to-end test (empty config → clean board)

### ~~m2 — Config & the `tool` check (single-repo MVP)~~ ✓

- s1 — [deep] `triage.yaml` schema + loader (`goccy/go-yaml`): mapping root, top-level profile keys (no `profiles:` wrapper; no bare checks at root), `include`, optional empty `default`, `extends`/`add`, `version_from`, type-as-key checks; publish a JSON Schema for editors
- s2 — [exec] `tool` check: presence via `exec.LookPath`, version extraction (incl. `go version`-style overrides), `version` (npm-style ranges) + `version_from` via `Masterminds/semver`
- s3 — [exec] Severity model (`error`/`warn`/`info` + `required` sugar, `--strict`); grouped output (`group` containers + remediation summary); pending `[…]` + TTY back-update for slow checks (§7.2). Renderer **materializes results in list order, independent of execution order** (finalize-in-order) so it's ready for concurrency (§7.2) with no rewrite

### ~~m3 — Full check types~~ ✓

- s1 — [exec] `env`, `path`, `one_of`, `platform` check types
- s2 — [exec] `command` escape hatch (explicit interpreter + `platform` guard, **no implicit shell**); subprocess I/O: stream-to-discard default, bounded assertion scan, `--command-log` tee, `--verbose` stderr replay (§5)
- s3 — [exec] Validate complex real-world checks in config (Xcode path/version, keychain certs, `xcrun`, virtualenv paths) against golden fixtures

### ~~m4 — Delegation & concurrency~~ ✓

- s1 — [deep] `delegate` check type + recursive nesting (checks list order, pass/fail per child)
- s2 — [exec] Delegate tree renderer: nested board under pending summary line, stream child lines as they complete (extends §7.2 pending renderer)
- s3 — [exec] **Deferred.** `triage-<name>` PATH-plugin discovery + contract was dropped from m4 per the owner; the delegate/workspace golden fixtures it would have carried ship with s2 instead. Revisit post-m4 if a real plugin need appears.
- s4 — [deep] Bounded worker pool + `--jobs`/`-j` (default ≈ NumCPU, `1` = sequential): run local checks **and** sibling delegates concurrently; `serial:` opt-out on check/group; render and `--command-log` materialize in **list order** (per-probe spool → ordered concat) so golden output is timing-independent (§5, §7.2). Highest payoff here because delegate-heavy workspaces dominate runtime

### m5 — Release & distribution

- s1 — [exec] `goreleaser` build matrix (macOS+Linux × arm64/x64 for v1) + GitHub
  release assets + manpage; release pipeline promotes **floating git tags**
  (`v0.3`, `v0`) alongside exact `v0.3.1` on `lolay/triage`
- s2 — [exec] Homebrew tap formula + automated version bump on release (goreleaser → direct tap commit on `lolay/homebrew-tap`)
- s3 — ~~[fast] `curl | sh` installer~~ **dropped** — tap + GitHub release assets only
- s4 — [fast] Lightweight update banner: cached GitHub `releases/latest` check,
  one-liner above board, `TRIAGE_NO_UPDATE_CHECK` / `--no-update-check` (§7.2)

### m6 — GitHub Action (`lolay/triage-action`)

First-party composite action in a **sibling repo** (`lolay/triage-action`) so CI
can run `triage` without hand-rolled install scripts and the action is eligible
for **GitHub Marketplace** (requires `action.yml` at the **repository root** —
subdirectory actions under `lolay/triage/action` are not listed).

```yaml
- uses: lolay/triage-action@v0.3
  with:
    profile: ci
```

Layout in **`lolay/triage-action`:** `action.yml` at repo root (+ install helpers).
The action downloads the **`triage` CLI** from **`lolay/triage` GitHub release
assets** — not from the action repo's tag contents.

#### Release & versioning (two repos, coordinated semver)

| Repo | What tags version |
| --- | --- |
| **`lolay/triage`** | Go CLI — source + goreleaser binaries (m5) |
| **`lolay/triage-action`** | Composite action wrapper only (m6) |

**Coordination:** ship matching semver together (e.g. CLI `v0.3.1` on `lolay/triage`
and action `v0.3.1` on `lolay/triage-action` in the same release beat). Document
the pairing in both READMEs.

**Default CLI pin:** derive from the action ref (`github.action_ref` → `v0.3.1` →
download `triage_0.3.1_…` from `lolay/triage` releases). Optional input
**`version`** overrides the CLI release when needed.

**Floating tags** on **`lolay/triage-action`** (GitHub does not semver-resolve):

| Tag | Role |
| --- | --- |
| `v0.3.1` | Exact action release |
| `v0.3` | **Retagged** to latest `0.3.x` action on each patch |
| `v0` | **Retagged** to latest `0.x` (optional) |

Promote `v0.3` / `v0` in the **triage-action** release workflow when `v0.3.1`
ships. **`lolay/triage`** has its own floating tags for CLI/tap consumers (m5).

**Consumer guidance:**

```yaml
- uses: lolay/triage-action@v0.3.1   # exact pin
- uses: lolay/triage-action@v0.3     # latest 0.3.x action (floating minor)
- uses: lolay/triage-action@<sha>    # dogfood / PR CI only
```

Do not document `@main` for external consumers. Publish to **GitHub Marketplace**
from `lolay/triage-action` once the Developer Agreement is accepted.

- s1 — [deep] Action contract: inputs (`profile`, `config`, `version`, `args` for
  passthrough flags like `--strict`/`--severity`/`--json`); default CLI = action
  ref → `lolay/triage` release asset (linux/macos × arm64/amd64); forward exit
  codes (`0`/`1`/`3`) unchanged
- s2 — [exec] Bootstrap **`lolay/triage-action`** (root `action.yml` + install step);
  dogfood here via `uses: lolay/triage-action@v…`; cross-link READMEs; list on
  Marketplace
- s3 — [fast] Floating-tag release workflow for triage-action; coordinated semver
  with `lolay/triage`; example workflow for `profile: ci` / `--json`

### m7 — Forward-looking (deferred)

- s1 — [exec] Native Windows: add `windows/{amd64,arm64}` to the matrix, CI on Windows, `scoop`/`winget`
- s2 — [deep] `--fix`: per-check `fix=`, `--dry-run`, confirmation, safety model
- s3 — [fast] `go install <module>@latest` path
- s4 — [exec] Cooperate-with-version-managers polish (`version_from` for `.tool-versions`/`mise.toml`); optional `optional`-tier UX refinements
- s5 — [fast] **`--only` output filter** (deferred from v1): severity and/or group
  filtering; grammar TBD after dogfood — single `--only <value>` vs split flags (§7.3)
