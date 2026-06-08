# triage — the environment doctor

> `triage` is a single, cross-platform binary that replaces copy-pasted
> `make doctor` shell scripts with one declarative, config-driven environment
> checker — install once, declare prerequisites in `triage.yaml`, run anywhere.
> (Name decided in §10; `drdr`/`bones`/`mccoy`/`doogie` kept as backups.)

---

## 1. Problem

Every repo answers the same question before you can build or ship: **do I have the
right tools, versions, keys, and identities installed?** Today that question is
often answered by `scripts/doctor.sh` + `scripts/doctor.<mode>.conf`, a design
that is sound but distributed by **copy-paste vendoring**, which rots quickly:

- **The "vendored verbatim" file drifts.** The same script is copied into sibling
repos and silently diverges — different md5s, renamed variants, headers that claim
"byte-identical to upstream" long after they've forked.
- **Bespoke checks are re-implemented per repo.** Anything that doesn't fit
`name | min-version | install-hint` (Xcode path/version tier, keychain signing
identities, `xcrun ba-*/altool`, virtualenv paths, age identities) is hand-written
bash that only lives in one repo.
- **Cross-repo delegation is Makefile glue.** Workspace roots loop over child
repos; complex repos delegate to `make secrets-doctor` / `make profiles-doctor`.
The dispatch + aggregation logic is duplicated shell.
- **bash 3.2 tax.** The scripts are written by hand to macOS's stock bash 3.2
(no associative arrays, no `mapfile`, parallel indexed arrays), which is
error-prone and untested.

The logic is good. The **distribution mechanism** is the problem. A single
installable binary eliminates drift, makes bespoke checks declarative and
shareable, and gives cross-repo delegation/aggregation a real implementation
instead of shell glue.

## 2. Goals & non-goals

### Goals

- **One binary, zero vendoring.** `brew install <org>/tap/triage` (dedicated tap;
auto-taps on first install) or download a release binary; no `scripts/doctor.sh`
copied into any repo ever again.
- **Preserve the contract.** Read-only, safe to re-run, with a graded
**severity** model (`error`/`warn`/`info`, §7.1) reported through a `0`/`1`/`2`
exit-code model (§7.3): pass-fail `0`/`1` by default; `--severity` for a
severity-graded `0`/`1`/`2` ladder when callers need to branch on warnings (§8),
keeping the read-only guarantee and
"green = ready" meaning. Which checks run is selected by **profile** (`default`
when omitted, or any other name via `--profile`); severity is orthogonal.
- **Declarative bespoke checks.** Absorb today's hand-written bash (Xcode,
keychain, env vars, file presence) into config-expressible check types, with a
guarded escape hatch + plugin convention for anything truly repo-specific.
- **Cross-platform, native-Windows-ready.** macOS + Linux shipped in v1; the
design is shell-free so native Windows is a later build target, not a rewrite.
- **Zero-churn adoption.** Repos keep `make doctor`; it just calls `triage`.

### Non-goals

- **Not a version manager.** `triage` does not install or pin toolchains. It
*cooperates* with `mise`/`asdf`/`aqua` by reading their config (`.tool-versions`,
`.nvmrc`, `.go-version`, `.terraform-version`) rather than replacing them.
- **Not a task runner.** It checks readiness; it does not build, test, or deploy.
- **v1 does not execute fixes.** It reports what's wrong and (optionally) prints
the commands to fix it. Executing those commands (`--fix`) is a later,
opt-in, guarded milestone (§7).

## 3. Language & distribution

### Language: **Go**

Chosen after comparing Go, Rust, and Python against this tool's workload (spawn
many subprocesses, parse version strings, read a config, print `✓/✗`, run
constantly in pre-commit/CI/interactive, ship as one small binary to
macOS/Linux/native-Windows).

- **Go** — first-class `GOOS=windows` cross-compile from a single CI runner;
`exec.LookPath` is Windows-aware (PATHEXT, `.exe`/`.cmd`); VT colors
auto-enable; ~3–6MB static binary; ~ms cold start; second-scale builds. Winner.
- **Rust** — the credible runner-up and the best native-Windows CLI pedigree
(ripgrep/starship/fd), equal on size/startup. Rejected for *this* tool because
cross-compiling to Windows needs `cargo-xwin`/`cross`/per-OS runners and builds
take minutes — a poor fit for a small, frequently-run utility binary.
- **Python** — ruled out for the binary: its three weakest traits (cold start,
single-binary packaging, native-Windows distribution) are this tool's three
hardest requirements.

Libraries (proposed): `spf13/cobra` (root command + flags; optional positional
config path; no `check` subcommand), `goccy/go-yaml` (config — better
errors than `gopkg.in/yaml.v3`; JSON parses for free), `Masterminds/semver/v3`
(npm-style `version:` range matching), `muesli/termenv` or `fatih/color`
(Windows-safe color), `goreleaser` (release + tap + scoop).

### Distribution

Three tiers — what's available when:

- **Always:** build from source. Clone the repo and `make build` (see §11). The
root Makefile is the source of truth for local dev, agents, and CI — same verbs
everywhere.
- **Primary (released):** dedicated Homebrew tap (`brew install <org>/tap/triage`
— not homebrew-core; auto-taps on first install) — a cross-platform
binary-download formula (`on_macos`/`on_linux`; WSL2 uses the Linux build) —
plus GitHub release binaries (goreleaser; six-target matrix + manpages).
- **CI (m6):** sibling repo [`lolay/triage-action`](https://github.com/lolay/triage-action)
  — root `action.yml` (GitHub Marketplace–eligible); `uses: lolay/triage-action@v<tag>`;
  installs CLI from `lolay/triage` release assets ([milestones.md](milestones.md) m6).
  Contract: [`action.md`](action.md).
- **Eventually:** `go install <module>@latest` — deferred, not a v1 path.
- **Later:** `curl … | sh` installer; `scoop` once Windows binaries ship — the
single Windows package-manager channel (`winget` / `chocolatey` deferred).

### Native-Windows design rule (applies now, even though Windows ships later)

**No implicit shell.** Native Windows has no `/bin/sh`, so:

1. The **structured check types** (§5) are primary — they spawn tools directly
  via `exec.LookPath`/`exec.Command` with **no shell**, so they behave
   identically on every OS.
2. The `command` escape hatch is **explicitly-interpreted and platform-guarded**
  (the config names `sh`/`pwsh` and gates it `platform: macos`), never an
   implicit `/bin/sh`.

## 4. Config model — `triage.yaml`

One `triage.yaml` per repo replaces the per-mode `doctor.*.conf` files. YAML
chosen because it's already ubiquitous for nested project config and far less
verbose than TOML for lists of checks; JSON works for free (YAML is a JSON
superset).

**Canonical format reference:** profiles, composition, check types, fields,
severity, and discovery rules live in **[`specs/config.md`](config.md)**.
Editor validation: [`schema/triage.schema.json`](../schema/triage.schema.json).
Worked examples: [`examples/`](../examples/) and the dogfood
[`triage.yaml`](../triage.yaml).

Design choices (the *why*):

- **Type-as-key checks** — each check names its type as the YAML key (`tool:`,
  `env:`, …); optional fields attach to any check.
- **Two composition axes** — `include:` (merge files, in order) and `extends:`
  (inherit profiles) are deliberately separate mechanisms; collisions are
  flagged at load time, later wins.
- **Profiles at the root** — no `profiles:` wrapper; only `include` and `vars`
  are reserved top-level keys besides profile names. `default` is special
  (implicit when `--profile` is omitted).
- **List order = render order** — execution may be concurrent (§7.2) but output
  always materializes in config list order.
- **No walk-up** — entry config must be in cwd or an explicit path (see CLI
  below). One entry file per run; compose via `include:` / `extends:` only.

### CLI — flat invocation (no `check` subcommand)

The binary does **one job**: run environment checks. There is no `triage check`
subcommand — that was Cobra boilerplate, not user value. Other CLIs put `doctor`
*on* a tool (`flutter doctor`); **`triage` is the doctor**, so the default
action *is* the check run.

```text
triage                          # triage.{yaml,yml} or .triage.{yaml,yml} in cwd
triage --profile release        # stricter preflight
triage path/to/triage.yaml      # explicit config file
triage path/to/repo/            # directory → triage.{yaml,yml}/.triage.{yaml,yml} inside
triage --version                # print version and exit
triage --help                   # flags + usage
```

**Default action:** unless `--version` / `--help`, `triage`
loads a config and runs the active profile. All other behavior is flags on that
same command (§7.3).

**Config path (optional positional):** zero or one argument.

- **Omitted:** look for `triage.yaml`, `triage.yml`, `.triage.yaml`, then
  `.triage.yml` (in that precedence order) in the **current working directory
  only** — no parent-directory walk-up.
- **Directory argument:** look for the same names, in the same order, inside that
  directory.
- **File argument:** load that path directly (any name/extension).

**Config not found:** print a short message (e.g. `no triage.yaml, triage.yml,
.triage.yaml, .triage.yml in <cwd>`), then print the **same usage text as
`--help`**, and exit `3`. Do not run checks.

More than one positional argument → usage error (exit `3`).

**No other subcommands in v1.** Deferred (m7): `--fix`, self-update — still flags or separate entry points TBD
(§7.4), not a growing subcommand tree.

## 5. Check types

Eight declarative check types absorb today's bespoke bash doctor scripts. All
are read-only. Full field reference per type, examples, and workspace patterns:
**[`specs/config.md`](config.md)**.

| Type | Purpose |
| --- | --- |
| `tool` | Command present; optional `version` or `version_from` |
| `env` | Env var set or unset; optional `matches` regex |
| `path` | File/dir exists (glob ok) |
| `one_of` | At least one of N alternatives present |
| `platform` | Guard on any check or group — runs only on matching OS |
| `command` | Explicitly-interpreted snippet; assert exit/stdout (no implicit shell) |
| `delegate` | Load and run another `triage.yaml` from `dir:` (nested tree output) |
| `group` | Named container for other checks via `items:` |

**Command subprocess I/O** — `command` checks and `tool` version probes stream
stdout/stderr to discard by default; `--command-log` opt-in tee; bounded assertion
scan; `--verbose` stderr replay on failure. Full rules: §7 and [`config.md`](config.md).

**Severity** (`error` / `warn` / `info`, orthogonal to profile): §7.1 and
[`config.md`](config.md).

**Cross-repo:** `delegate` for nested configs; `command` + `dir:` for
`make doctor` stragglers (flat line in parent board). See [`config.md`](config.md)
and [`examples/`](examples/).

## 6. Plugins

- **`triage-<name>` plugins (git-style)** — `triage` discovers executables named
  `triage-secrets`, `triage-profiles`, … on `PATH` and runs them in-process.
  Repo-specific readiness can also be reached via `command:` + `dir:` (e.g.
  `make secrets-doctor`) or a plugin wrapper — m4 validates both paths.

## 7. Severity, output & the hint-vs-fix ladder

### 7.1 Severity: required vs optional (`error` / `warn` / `info`)

Many doctor scripts are strictly pass/fail and express "needed always vs needed
to ship" only through `MODE`. That's too coarse — e.g. a formatter omitted
entirely because it isn't CI-gated, when what you really want is a non-failing
reminder. `triage` adds a per-check **severity** that is
**orthogonal to profile**: the profile selects *which* checks run; severity
decides *how a failure is treated*. (The name says it: the tool **triages** —
sorts findings by urgency.)


| `severity`        | Meaning                                  | Glyph | Fails the run?         |
| ----------------- | ---------------------------------------- | ----- | ---------------------- |
| `error` (default) | Required — blocks the dev loop / release | `[✗]` | Yes → exit 1           |
| `warn`            | Optional / recommended — nice to have    | `[!]` | No (unless `--strict`) |
| `info`            | FYI only — surfaced, never judged        | `[ℹ]` | No                     |


```yaml
- tool: swiftformat
  severity: warn             # show [!] if missing; don't fail `make doctor`
  hint: brew install swiftformat
```

- `required = true|false` is accepted as sugar (`true` → `error`, `false` → `warn`).
- `--strict` promotes every `warn` to `error` (CI can demand a clean board).
- This replaces the old "just omit non-gated tools" workaround: include them as
`warn`/`info` so they're visible without being fatal.

### 7.2 Output — grouped board

Human output is a grouped board. Sections come from **structural `group`**
containers (`group:` + `items:` in config). Each section prints a **header line**
with the group's name and worst-status glyph; child checks (and nested group
headers) indent underneath. Checks without a containing `group` render at the
top level. Each non-pass line carries its actionable hint.

Legacy: a bare **`group: Foo` string field** on a check still rolls checks into
the same section at render time (flat grouping, no nesting) — useful for small
configs; structural `group` containers are preferred when sections nest.

```
triage (profile: default)

[✓] Core toolchain
    [✓] git 2.49.0
    [✓] go 1.26.3 (>= 1.26)
[!] Formatting
    [!] swiftformat not found — brew install swiftformat
[✗] Signing (macOS)
    [✗] Apple Distribution cert missing — make keychain-import

✗ 1 error, 1 warning, 4 ok — fix the [✗] items above
To fix, run:
    make keychain-import
```

The trailing **remediation summary** (read-only) aggregates the hints into a
copy-paste block — it *prints* commands, never runs them (see 7.4).

#### Concurrency (bounded worker pool)

Checks are **read-only** (§2, §7), so they have no inter-check data dependencies
and are safe to run in any order or simultaneously. The engine runs them through
a **bounded worker pool** (default size ≈ `runtime.NumCPU()`, capped; overridable
with `--jobs <n>` / `-j`). `--jobs 1` forces fully sequential execution.

**Core rule — execute concurrently, materialize in list order.** Concurrency is
a scheduling detail only; it never changes what the user sees. A check that
finishes early does **not** render early — its final board line *and* its
`--command-log` block (§5) are materialized in config **list order**. This keeps
human output and golden fixtures byte-stable regardless of `--jobs` or subprocess
timing.

- **Automatic, not declared.** There is **no `async:` / `parallel:` YAML knob** —
  parallelism is the default because read-only makes it always safe. Authors opt
  *out*, never in.
- **`serial: true`** (optional field on any check or `group`) — pin this check
  (or every direct child of this group) to run sequentially, after prior work,
  for the rare case of a **shared scarce resource** (exclusive lock,
  rate-limited/flaky auth endpoint, keychain) or a `command` escape hatch with a
  side effect another check observes. Default is parallel; reach for `serial`
  only on real contention.
- **Delegates** are the natural coarse-grained unit — sibling `delegate` checks
  (and their subtrees) run concurrently with each other and with local checks;
  each child config runs its own pool internally.
- **`group` stays display-only.** Grouping is for the board, not a concurrency
  boundary — putting checks in a group must not silently change how they
  schedule. Set `serial:` on the group when you genuinely want its children
  serialized.

#### In-flight progress (pending checks)

Checks are **rendered in list order** (execution may be concurrent — see
*Concurrency* above): a finished check waits its turn to reveal/finalize, so the
board is a single coherent top-down stream rather than many flickering live
lines. The board should show **what is running** without streaming raw
subprocess output (§5) and without a full-screen TUI — line-at-a-time updates
only (pre-commit / `flutter doctor` style, not Bubble Tea). (A multi-line live
region — several `[…]` active at once — is optional polish, later.)

**Instant checks** — `env`, `path`, bare `tool` presence (no version probe), and
checks skipped by `platform:` — print the **final** `[✓]`/`[✗]`/`[!]` line only
when done (typically sub-second; no pending line).

**Slow checks** — `command`, `tool` with a version probe, `delegate`, and any
check that spawns a subprocess expected to take noticeable time — on **TTY**
stdout:

1. Print a **pending** line first: `[…] <label>` (respecting group/delegate indent).
2. Run the check (subprocess I/O still stream-to-discard or `--command-log`; §5).
3. When done, **rewrite the pending line in place** to the final glyph +
   detail/hint via ANSI cursor-up + clear-to-EOL.

While a slow check runs, the user sees activity (the `[…]` line) even though
child stdout/stderr is not echoed. For a long `command`, that is the only
in-flight signal unless `--command-log` / `--verbose` is set.

**Non-TTY / piped stdout / `--json`:** no cursor back-update. Emit each check's
**final** line only, or include per-check `status: running|pass|fail|warn` in
JSON while work is in flight. CI logs stay clean and deterministic.

**Not in v1:** full-screen spinners, progress bars with known totals, or
streaming hook output to the board. Braille/dot animation on the active pending
line is optional polish later; the required contract is `[…]` → final glyph
back-update on TTY.

**Example** (TTY, mid-run — `gcloud` probe still going):

```
triage (profile: release)

[✓] Core toolchain
    [✓] git 2.49.0
[…] gcloud logged in (token valid, not expired)
[…] vitalink-api
    [✓] Toolchain
        [✓] go 1.26.3 (>= 1.26)
```

#### Delegate tree output

When a profile includes `delegate` checks, human output is a **nested tree** on
top of the pending-check rules above. Each delegate gets one **summary line at
the top of its block**; the child's full board prints indented underneath; when
the child finishes, the summary line is **back-updated** in place to `[✓]` or
`[✗]`.

**Per delegate (at any nesting depth):**

1. Print a **pending** summary line first: `[…] <name>` (indent = delegate depth).
2. Run the child (local checks — each using the pending rules above — then nested
   delegates with the same pattern).
3. Stream the child's **finished** check lines **indented one level** under the
   summary line as the child completes them.
4. When the child's subtree is done, **rewrite the summary line** in the terminal
   (`[…]` → `[✓]` or `[✗]`) via ANSI cursor-up + clear-to-EOL. Only when stdout
   is a TTY; see fallback above.

**Example** (after all back-updates complete):

```
triage (profile: default)

[✓] Workspace
    [✓] make
[✓] vitalink-api
    [✓] Toolchain
        [✓] go 1.26.3 (>= 1.26)
    [✗] vitalink-infra
        [✗] terraform 1.14.2 — need >= 1.15.4

✗ 1 error, 3 ok — fix the [✗] items above
```

While `vitalink-api` is running, the user sees `[…] vitalink-api` with lines
appearing under it; the `[…]` flips to `[✓]` only after `vitalink-api` and all
of *its* delegates finish. Nested delegates (`vitalink-infra` under `vitalink-api`)
use the same pattern at their indent depth.

**`--json`** mirrors the tree structurally (`delegates[]` with nested `checks` /
`delegates`) and may emit `status: running` while a delegate subtree is active.

**`command` + `dir:`** (e.g. `make doctor`) stays a **flat** line in the parent
board — no nested child tree — but still uses the **pending** `[…]` → final glyph
back-update like any other slow `command` check. Only `delegate` checks get the
indented subtree treatment.

#### Update availability (lightweight)

On interactive human runs, `triage` may print a **one-line update notice** above
the board when a **newer release** exists. This is informational only — never
changes exit codes or blocks checks.

**Mechanism:**

- Compare embedded build version (`triage --version`) to
  `GET /repos/lolay/triage/releases/latest` (GitHub API), with a **short timeout**
  (≤300ms) and **disk cache** (default TTL **24h**, under XDG cache dir).
- **Do not** shell out to `brew update` / `brew outdated` on each run (too slow).
- On fetch failure, timeout, or parse error: **silent** — no banner, no error.

**When shown (all required):**

- Human board mode (not `--json`)
- stdout is a **TTY**
- Not in CI (`CI`, `GITHUB_ACTIONS`, and other common CI env vars → skip)
- Installed build is a **release** (not `dev` / dirty / missing version metadata)
- Cache miss or cache entry older than TTL

**When skipped:**

- `--json`, `--no-update-check`, or `TRIAGE_NO_UPDATE_CHECK=1`
- Non-TTY / piped stdout
- CI environments
- Dev/local `make build` binaries (no release version)

**Copy (example):**

```
[!] triage 0.4.0 available (you have 0.3.1). Update: brew upgrade triage

triage (profile: default)
…
```

**Update hint:** default `brew upgrade triage` when `brew list triage` succeeds
(tap install); otherwise `brew install lolay/tap/triage` or the release URL from
README. Best-effort detection only — wrong hint is acceptable; the version line
is the important part.

### 7.3 Flags & exit codes


| Flag                                     | Effect                                                                                    |
| ---------------------------------------- | ----------------------------------------------------------------------------------------- |
| `--profile <name>`                       | Select profile (replaces `MODE=`)                                                         |
| `--json`                                 | Machine/agent-readable: per-check `{group, name, severity, status, detail}` + counts (`status` may be `running` while in flight) |
| `--quiet`                                | Suppress `[✓]` passes; show only `[!]`/`[✗]` + summary                                    |
| `--strict`                               | Treat `warn` as `error` (warnings fail the run)                                           |
| `--severity`                             | Severity-graded exit ladder — `0`/`1`/`2`/`3` so callers can branch on warnings vs errors |
| `--jobs <n>` / `-j`                      | Max concurrent checks (bounded worker pool; default ≈ NumCPU). `--jobs 1` = fully sequential (§7.2) |
| `--no-color`                             | Disable color (auto when stdout is not a TTY)                                             |
| `--command-log` `[path]`                 | Stream each probe's output to a log file (default `.triage/commands.log`); **truncated each run**; off by default |
| `--verbose`                              | On check failure, replay command output (or log excerpt) to **stderr**                  |
| `--no-update-check`                      | Skip cached GitHub release staleness banner (§7.2)                                      |

**Output filtering (v1):** `--quiet` only. **`--only`** (filter by severity or
group name) is **deferred** — grammar and delegate-tree behavior need a pass once
the board is stable in real use; revisit in m7 ([milestones.md](milestones.md)).

**Exit codes** — pass-fail `0`/`1`/`3` by default; `--severity` enables the full
severity ladder:


| Mode                | Codes           | Behavior                                                        |
| ------------------- | --------------- | --------------------------------------------------------------- |
| pass-fail (default) | `0`/`1`/`3`     | Warnings print `[!]` but exit `0`; only `error` checks exit `1` |
| `--severity`        | `0`/`1`/`2`/`3` | Exit reflects severity — clean / warnings only / errors         |


`--severity` ladder:


| Code | Meaning                                                                                                                     |
| ---- | --------------------------------------------------------------------------------------------------------------------------- |
| `0`  | Clean — all checks pass (no `warn`, no `error`)                                                                             |
| `1`  | Warnings only — ≥1 `warn`, 0 `error`                                                                                        |
| `2`  | Errors — ≥1 `error`                                                                                                         |
| `3`  | Config/usage error — couldn't run the checks (distinct from "errors found"; the CLI overrides cobra's default `1` for this) |


`--strict` escalates `warn` to the `error` tier. In pass-fail mode, warn-only
runs exit `0`; with `--severity`, they exit `1`. See §8 for `make doctor` wiring.

### 7.4 The hint → summary → `--fix` ladder

Three rungs, increasing in power and risk:

1. **Hint (v1, always-on, read-only).** Each `[!]`/`[✗]` prints what's wrong +
  the install hint. Today's behavior — *a message, not an action*.
2. **Remediation summary (v1, read-only).** Aggregates all hints into one
  copy-paste block. It **prints** commands; it never runs them.
3. `**--fix` (deferred milestone, mutating, opt-in).** Actually *executes* the
  remediation. Requires a per-check `fix = "<command>"` (distinct from the
   human-readable `hint`), `--dry-run`, and confirmation. `**--fix` ≠ hint:**
   the hint *says* what to do; `--fix` *does* it.

## 8. Make integration & migration

- **Entry point unchanged.** Default pass-fail exit means warnings surface but
don't break the build — no shell guard needed:
  ```make
  doctor:
      triage --profile $(MODE)
  ```
  Every existing `make doctor [MODE=]` keeps working; users notice only that the
  vendored scripts are gone. (Add `--severity` if a CI job wants to branch on
  warnings; add `--strict` if warnings should fail.)
- **Rollout (adopter playbook):** add `triage` to a repo → switch `make doctor` →
delete vendored `scripts/doctor*.sh` + `*.conf`. Per-repo migration is **out of
scope** for this project's milestones ([milestones.md](milestones.md)) — done in consuming repos, not tracked
here.

## 9. Milestones

> Moved to **[specs/milestones.md](milestones.md)**.

## 10. Naming

**Decision: `triage`** (binary, repo, and formula). Researched June 2026.

Rationale:

- **Maps onto the architecture.** triage = assess and prioritize by urgency —
exactly the severity model (§7.1: `error`/`warn`/`info`, `--strict`). "triage
your environment" *is* what the tool does.
- **Reads instantly** as a health/diagnostic tool to any engineer;
self-explanatory and professional.
- `**doctor` was rejected as the name:** it's a *subcommand* everywhere (`flutter doctor`, `conda doctor`, `wp doctor`, `sf doctor`, `maui doctor`), never a
standalone binary; a bare `doctor` breaks the `<tool> doctor` expectation and
reads as a `brew doctor` typo.
- **Namespace:** no Homebrew formula named `triage` (free); npm `triage` is taken
but unrelated/dead (express router scaffolding); nothing named `triage` on a
typical PATH.

Backups (kept on record, in preference order):

- `**drdr`** — "doctor-doctor": the *Spies Like Us* "Doctor. Doctor. Doctor!"
gag *and* the *doctor of doctors* (it delegates to per-repo doctors). The only
candidate free on **both** brew and npm — fall back here if `triage`'s npm
collision or genericness ever bites. (The working directory is being renamed
`drdr/` → `triage/`; see §12.)
- `**bones*`* — *Star Trek*'s Dr. McCoy ("Bones"); the ship's doctor. Short,
iconic, warm. brew-free; dead npm collision.
- `**mccoy*`* — same lineage, character-name variant; npm namespace empty.
- `**doogie**` — *Doogie Howser, M.D.*; the prodigy doctor. Distinctive; npm empty.

Considered and dropped: `radar` (too generic / already widely used);
`nurse`/`clinic`/`intern`/`aide` (collide with real same-domain tools);
`medic`/`vitals`/`checkup`/`workup` (viable, but no stronger than the above).

## 11. Repository layout & governance

Distributed under **Apache-2.0**. MIT was considered; Apache-2.0 wins for the
explicit patent grant — a common choice for infrastructure CLI tools.

Already in place (created during this design phase):

```
LICENSE                 Apache-2.0
README.md               what/why, install, config, exit codes, status
CONTRIBUTING.md         Go setup, build/test loop, commit + PR conventions, Apache terms
CODE_OF_CONDUCT.md      Contributor Covenant
SECURITY.md             private advisory via GitHub Security Advisories, scope
SUPPORT.md              Discussions / Issues / SECURITY, community model
specs/product.md        this spec
triage.yaml             dogfood config (repo root)
examples/               illustrative multi-repo configs + README
```

Planned in m1 (tracked there):

```
go.mod, .go-version     Go module + pinned toolchain
Makefile                single source of truth for build verbs (see below)
main.go, internal/      CLI + check engine
schema/triage.schema.json   JSON Schema for editor validation
.github/workflows/      ci, release (m1/m5); renovate optional
.github/CODEOWNERS      @GaryRudolph (default owner for all paths)
```

No issue or PR templates — simple repo; plain GitHub Issues/PRs (§12).

**Sibling repo (m6):**

```
lolay/triage-action/    root action.yml; Marketplace listing;
                        uses: lolay/triage-action@v…; installs CLI from lolay/triage releases
```

**Makefile conventions** — same shape as our other Go CLI repos: humans, local
agents, cloud agents, and CI all run the same `make <target>` verbs; workflows
call `make ci` so a green local gate matches CI.

- `SHELL := bash`; `.DEFAULT_GOAL := help`
- Self-documenting targets: `## comment` after a target, `##@ Section` headers;
  bare `make` prints the grouped list (section-aware `awk`, help can't drift)
- Standard verbs: `init`, `build`, `lint`, `test`, `ci`, `pre-commit`, `doctor`,
  `clean` — `ci` is the full pre-push gate; `pre-commit` aliases it
- `doctor` → `triage` (dogfood `triage.yaml`; `MODE`/`--profile` selects the
  active profile, e.g. `default` or `release`)
- Recipes are tab-indented; bash so `set -o pipefail` behaves the same on macOS
  and Linux

## 12. Decisions (locked)

The fresh session after the `drdr/` → `triage/` rename should treat these as
settled (don't re-litigate; section refs in parens):

- **Name:** `triage`; backups `drdr`/`bones`/`mccoy`/`doogie` (§10).
- **Repo/dir:** `triage` — directory rename `drdr/` → `triage/` is the manual step
the owner runs before restarting; this spec already reads as `triage`.
- **Language:** Go (§3).
- **Distribution:** Apache-2.0; build from source via root Makefile (always);
dedicated Homebrew tap + release binaries (m5); sibling `lolay/triage-action` for
CI + Marketplace (m6);
`go install` deferred (m7); scoop later — single Windows channel, winget/chocolatey deferred (§3, §11).
- **Native Windows:** designed-for now (no implicit shell), shipped in m7 (§3).
- **Config:** YAML; canonical `triage.yaml`, with `triage.yml`, `.triage.yaml`, `.triage.yml` accepted; type-as-key
checks; JSON Schema ships (§4).
- **Composition:** `include` (files, in order) + `extends` (profiles) — distinct
mechanisms (§4); no `triage/` config dir or other multi-config layout.
- **Profiles:** `default` is special (implicit when `--profile` omitted); unlimited
arbitrary names otherwise (`release`, `ci`, …). Only `include` is a reserved
top-level key (§4).
- **Check types:** `tool`/`env`/`path`/`one_of`/`platform`/`command`/`delegate`/
`group` (container with `items:`); auth/session via `command` probes (§5).
- **Severity:** `error`/`warn`/`info`, orthogonal to profile; `required` sugar (§7.1).
- **Exit codes:** pass-fail `0`/`1` by default; `--severity` → `0`/`1`/`2`; `3` =
config; `--strict` escalates warn→error (§7.3).
- **Output:** grouped board; in-flight `[…]` pending lines + TTY back-update for
slow checks; delegate nested tree (§7.2); cached update banner (§7.2);
`--json`/`--quiet`/`--no-color`/`--no-update-check` (§7); `--only` deferred (m7).
- **Concurrency:** checks run through a **bounded worker pool** (`--jobs`/`-j`,
default ≈ NumCPU; `1` = sequential). **Execute concurrently, materialize in list
order** so board + `--command-log` output (and golden fixtures) are
timing-independent. No `async:`/`parallel:` YAML knob — parallel by default
(read-only ⇒ safe); opt out with `serial:` on a check/group. `group` stays
display-only (not a scheduling boundary). `--command-log` spools per-probe then
concatenates in list order. Implementation lands m4 s4; renderer is built
finalize-in-order from m2 s3 (§5, §7.2, [milestones.md](milestones.md)).
- **Command I/O:** subprocess output stream-to-discard by default (no log file);
`--command-log` opt-in stream-through tee (**overwrite log each run**); bounded
assertion scan; POSIX redirect in `command:` as author escape hatch (§5).
- **Remediation:** hint (always) → summary (read-only) → `--fix` (deferred, m7) (§7.4).
- **CI:** sibling `lolay/triage-action` — root `action.yml`, Marketplace-eligible;
installs CLI from `lolay/triage` releases; coordinated semver; floating action
tags `v0.3`/`v0` (m6).
- **Cross-repo:** `delegate` check = nested `triage.yaml`; `command` + `dir:` for
`make doctor` stragglers; all checks in list order; pass/fail per delegate (§5).
- **CLI:** flat `triage` — default action is run checks; config in **cwd** only
(or explicit `[config]` path); missing config → message + `--help` text, exit `3`;
`--version`; no `check` subcommand (§4).
- **Make integration:** `make doctor` calls `triage` (pass-fail default) so
warnings don't break the build (§8).
- **Examples:** illustrative multi-repo shapes in `examples/` (§4).
- **CODEOWNERS:** `@GaryRudolph` owns all paths (§11).
- **`.github/`:** workflows + CODEOWNERS only; no issue/PR templates (§11).
- **Update check:** cached GitHub `releases/latest` vs build version; one-liner +
`brew upgrade triage` hint; skip CI/`--json`/non-TTY/dev builds;
`TRIAGE_NO_UPDATE_CHECK` / `--no-update-check` (§7.2, m5).
- **Output filter:** v1 = `--quiet` only; `--only` (severity/group) deferred to m7
(§7.3).

