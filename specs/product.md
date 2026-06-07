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
— not homebrew-core; auto-taps on first install) plus GitHub release binaries
(goreleaser; multi-platform single binary + manpages).
- **CI (m6):** sibling repo [`lolay/triage-action`](https://github.com/lolay/triage-action)
  — root `action.yml` (GitHub Marketplace–eligible); `uses: lolay/triage-action@v<tag>`;
  installs CLI from `lolay/triage` release assets ([milestones.md](milestones.md) m6).
- **Eventually:** `go install <module>@latest` — deferred, not a v1 path.
- **Later:** `curl … | sh` installer; `scoop` / `winget` once Windows binaries
ship.

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
chosen because it's already ubiquitous for nested project config (`project.yml`,
`pnpm-workspace.yaml`, GitHub Actions workflows) and far less verbose than TOML
for lists of checks; JSON works for free (YAML is a JSON superset).

```yaml
# Root is always a mapping. Checks live under profile keys — never at the root.
# Single-profile repo:
default:
  - tool: git
    hint: https://git-scm.com
  - tool: go
    version_from: .go-version
    hint: https://go.dev/dl

# Multi-profile / include — `include` is reserved; every other key is a profile:
include:
  - base.yaml

default:
  - tool: go
    version_from: .go-version
    hint: https://go.dev/dl

release:
  extends: [default]
  add:
    - tool: sops
      hint: brew install sops

# `default` may be omitted (treated as empty). This is valid in an included file:
# release:
#   extends: [default]
#   add:
#     - tool: cosign
```

Two composition axes — deliberately separate names (they're different operations):

- **`include: [files…]`** — top-level list, processed **in order**. Each file
merges its profile content into the composite config. **Collisions are
flagged:** when a later file redefines a profile or check already present, the
loader emits a load-time warning naming both sides (file + key); the later
definition wins. Intentional overrides stay possible; silent shadowing does not.
Check identity: same type key + primary value (`tool: go`, `group: Core toolchain`,
`delegate: vitalink-api`, `command` on `label:`, …). Cross-file DRY.
Cycle/diamond-safe.
- **`extends: [profiles…]`** — per-profile, inherits one or more **profiles**
within the already-merged config (override semantics; same collision-flag rule
when a child check shadows a parent). `release` extends `default`.
Cycle/diamond-safe.

Other keys:

- **Type-as-key.** Each check's *type key* (`tool:`/`env:`/`path:`/`one_of:`/
`command:`/`delegate:`) carries its primary value; `severity`, `group`,
`platform`, `hint` are optional fields on any check (see §5, §7.1). All checks
in a profile's check list are **ordered** — delegates are peers, not a separate
phase. That order is the **render order**: the engine may execute checks
concurrently (bounded worker pool, §7.2) but always *materializes* results in
list order, so output never depends on timing.
- **Profiles at the root — no `profiles:` wrapper.** Root is always a **mapping**.
Checks never sit at the top level — only under profile keys. Parsing rule:
  1. **`include`** — reserved. Composes other files (processed first, in order).
  2. **`vars`** — reserved. A mapping of reusable string values referenced in
     check fields via `{{ name }}` (see below). Merged across `include:` files
     (later wins; collisions warned). Overridable at runtime with
     `--var name=value` (repeatable; CLI wins over config). Built-in vars
     `profile` and `os` are injected by the engine and cannot be defined under
     `vars:` (ignored with a warning).
  3. **Any other key** — a **profile name**. Value is a check **list**, or
     `{ extends: […], add: […] }` when inheriting. Use **`add:`** (not `checks:`)
     with `extends`.
  4. **`default` is the only special profile name** — it is the profile `triage`
     runs when `--profile` is omitted. **`default` may be omitted** from a file
     (treated as an empty profile). Included files may define only non-`default`
     profiles (e.g. `release:`) when `default` is supplied by an earlier
     `include`.
  5. **Unlimited arbitrary profile names** — `release`, `ci`, `publish`, or any
     other key that is not `include`, `vars`, or `default` (e.g. `release` is a
     convention, not a built-in). Select with `triage --profile <name>`.
- **`vars:` + `{{ name }}` templates.** Reference a var in any check string
  field (`tool`, `version`, `path`, `command`, `label`, `hint`, `dir`,
  `with_env` values, …) with `{{ var_name }}` (whitespace inside braces is
  allowed). Precedence: config `vars:` < `--var` CLI override < built-ins
  (`profile`, `os`). Expansion is **fail-closed**: an undefined `{{ name }}`,
  **and** any malformed token — a `{{` that does not begin a well-formed
  `{{ name }}` (e.g. `{{ 1bad }}`, `{{ a-b }}`, `{{}}`, an unclosed `{{`) —
  fails the check with a clear error on the board rather than passing through as
  literal text. A name is a letter or underscore followed by letters, digits, or
  underscores. **Vars are not recursive** — a `{{ name }}` inside a `vars:` value
  is **not** re-expanded; values are substituted in a single pass. (Var-in-var
  indirection is intentionally a non-feature today and could be added later via
  load-time, dependency-ordered resolution.) Example:

```yaml
vars:
  region: us-west-2
  tool_prefix: fake

default:
  - tool: "{{ tool_prefix }}-git"
  - command: 'echo {{ region }}'
    label: region check
    contains: us-west-2
```

Run `triage --var region=eu …` to override `region` without editing the file.
- **Config file names:** `triage.yaml` (canonical), `triage.yml`, `.triage.yaml`
(hidden), or `.triage.yml` — all accepted, in that precedence order. `.yaml` is
the recommended spelling; the `.yml` variants are accepted for convenience (e.g.
for users coming from GitHub Actions). One **entry** file per run — no per-mode files, no `triage/`
config directory, no multi-root discovery. Large or shared setups compose via
**`include:`** (merge YAML files) and **`extends:`** (inherit profiles within
the merged config); that is the only multi-file story. **No walk-up:** the entry
file must live in the **current working directory** where `triage` is run, unless
you pass an explicit path (§4 CLI). A **JSON Schema** ships for editor
autocomplete + validation, which also blunts YAML's type/`Norway` footguns.
- **Worked examples:** see `examples/` for representative repo shapes and the
repo's own dogfood `triage.yaml`.

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

These absorb today's bespoke bash declaratively. All are read-only.


| Type       | Checks                                                                                                              | Replaces today's…                                                        |
| ---------- | ------------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------ |
| `tool`     | command present; optional `version` or `version_from`                                                              | the whole `doctor.*.conf`                                                |
| `env`      | env var set or unset; optional `matches` regex on value                                                             | `SOPS_AGE_KEY` / `ASC_KEY_ID` checks                                     |
| `path`     | file/dir exists (glob ok)                                                                                           | `.venv-models`, `private_keys/*.p8`                                      |
| `one_of`   | at least one of N alternatives present                                                                              | pnpm-or-npm, venv-or-system python                                       |
| `platform` | a **field/guard** on any check or group — runs only on matching `os`/`arch`                                         | `if macOS` blocks                                                        |
| `command`  | run an **explicitly-interpreted**, platform-guarded snippet; assert exit 0 (default) or stdout `contains`/`matches` | Xcode path/version, `security find-identity | grep 'Apple Distribution'` |
| `delegate` | load and run another `triage.yaml` from `dir:` (nested tree output, §7.2)                                            | workspace / monorepo member configs                                      |
| `group`    | named container for other checks (and nested groups) via `items:` (§7.2)                                             | explicit sections in long profiles                                     |


**Field reference (optional fields on any check):** `severity` (§7.1), `platform`
(`macos`/`linux`/`windows`), `hint`, `fix` (m7), `dir` (working directory — see
below), `serial` (opt out of concurrency for this check or group — §7.2). Legacy
**`group:` string field** on a check (output-only shorthand) is
still accepted; prefer a structural **`group`** container when nesting.
Type-specific:

- `command` needs a `label:` (the snippet isn't a friendly name); assertion
defaults to **exit 0**, or set `contains:` / `matches:` (stdout) / `exit:`
(expected code); `interp:` picks `sh`/`pwsh` (default `sh`, never implicit).
Template vars (`{{ name }}`) expand in the command string and all other check
string fields (see §4 `vars:`). Built-in `{{ profile }}` and `{{ os }}` are
always available. Optional
**`dir:`** sets the working directory before the command runs — the right way to
invoke a member repo's `make doctor` (or any other script) without treating it as
a triage delegate. Optional **`with_env:`** injects a map of environment
variables for that command only: pairs are layered over the inherited process
environment (later keys override inherited values of the same name). Values support
`{{ name }}` expansion; triage does not perform `$VAR` interpolation — the shell
still does that inside the snippet. Example:

```yaml
- command: terraform validate
  label: terraform config valid
  with_env:
    TF_IN_AUTOMATION: "1"
    MODE: "{{ profile }}"
```

This also covers **auth/session validity** — run the CLI's
own auth probe and let exit 0 mean "logged in and not expired" (e.g.
`gcloud auth print-access-token`, `firebase login:list` + `contains: "@"`).
- **Command subprocess I/O** — `command` checks and `tool` version probes spawn
  subprocesses; their stdout/stderr must **never** pollute the human board and
  must **not** be buffered unboundedly in memory (noisy or huge output is a
  memory hazard). Rules:
  - **Default (no flags):** stream stdout+stderr to **discard** (`/dev/null`) as bytes
    arrive. The board shows only triage's `[✓]`/`[✗]` line (+ optional one-line
    fail excerpt). **No command log file is written.**
  - **`--command-log [path]`** (opt-in): tee the combined stream to a log file as
    it arrives (stream-through, not load-then-write). The log file is **truncated
    at the start of each run** (overwrite, not append across runs). Within a run,
    each probe writes one block: `label`, `cwd`, expanded `run:` line, `---`, then
    raw output. Path defaults to `.triage/commands.log` when the flag is given
    without a value. Parent dirs are created as needed. **Off by default** —
    omit the flag for normal use.
  - **Under concurrency (`--jobs > 1`, §7.2):** "stream-through" means
    **per-probe spool, then ordered concatenation** — never one shared file with
    many writers. Each running probe streams its combined output to its **own**
    spool as bytes arrive (no cross-probe interleaving, no in-memory buffering),
    and when the probe finalizes, its block is appended to the log **in config
    list order** (early finishers wait their turn; the spool is then discarded).
    The on-disk format is unchanged — one coherent `label`/`cwd`/`run:`/`---`
    /output block per probe — and the log is still truncated once at run start.
  - **Assertions:** `contains:` / `matches:` scan a **bounded prefix** of
    captured stdout (default cap **256 KiB** per check). Beyond the cap, the
    check fails with a message to re-run with `--command-log` (or use shell
    redirect below). Exit-code-only checks need no capture beyond the process
    handle.
  - **On failure:** board may show a **one-line excerpt**; `--json` `detail`
    carries excerpt + `command_log` path when `--command-log` was set.
  - **`--verbose`:** on failure, replay the matching log block (or captured
    excerpt) to **stderr** so it does not fight the board on stdout. Under
    concurrency, replays follow the same list-order, one-block-at-a-time
    discipline so blocks never interleave.
  - **Author escape hatch (POSIX):** redirect inside the `command:` string when
    a named artifact is wanted regardless of triage flags — triage still sees
    whatever the shell leaves on stdout/stderr after redirection:
    ```yaml
    - command: noisy-tool diagnose > .triage/diagnose.log 2>&1
      label: diagnose ran cleanly
    ```
    Assertions run against the post-redirect streams only; the file is the
    author's responsibility.
- **`dir:`** on any check — resolve relative `path:` values and run `command:`
from that directory. Defaults to the directory containing the loaded config
(or the cwd triage was invoked from). A `command` with `dir:` is one **atomic**
check in the parent board (`[✓] vitalink-web`), not a nested tree.
- `platform` accepts a string or a list (`platform: macos`,
`platform: [macos, linux]`); `os` values `macos`/`linux`/`windows`.
- `one_of` takes either tool names (`one_of: [pnpm, npm]`) **or** a list of
sub-checks (`one_of: [{env: SOPS_AGE_KEY}, {path: ~/.config/sops/age/keys.txt}]`).
- `env` — assert presence or absence of a variable (pick one mode per check):
  - **Set (default):** `env: VAR_NAME` — variable must be present. Optional
    `matches:` (regex on the value). An empty value (`VAR=`) counts as set.
  - **Unset:** `env: VAR_NAME` + `unset: true` — variable must **not** be in the
    environment (`os.LookupEnv` not found). Fails if set, even to an empty string.
    Mutually exclusive with `matches:` (loader warns if both).
  - Typical uses for `unset: true`: dev profiles that must not inherit a prod
    secret (`DATABASE_URL`), or tooling that breaks when a stray export is present.
- `path` supports globs.
- `tool` version fields (pick **one** per check; loader warns if both):
  - **`version`** — npm-style semver range, quoted in YAML. Examples: `">=1.26"`,
    `"^1.26.0"`, `"~1.2.0"`, `">=1.26 <2"`, `"1.x"`. Supports `^` `~` `>=` `<=`
    `>` `<` `=` and comma/range combinations — same syntax npm/pnpm/Cargo use in
    `package.json` / `Cargo.toml`.
  - **`version_from`** — read a pin file (`.go-version`, `.nvmrc`,
    `.terraform-version`, `.tool-versions`, …). A single bare version in the file
    (e.g. `1.26`) is treated as `>=1.26`; if the file already contains a range,
    use it as-is. Keeps the constraint in one place with your version manager.

**Example mappings** — typical bespoke bash checks map cleanly:

- Xcode `xcode-select -p` contains `Xcode` + version ≥ N → `command` + `platform: macos`
- Code-signing certs in keychain → `command` (`security find-identity`) + `platform: macos`
- `xcrun` tool presence → `command` (`xcrun --find`) + `platform: macos`
- Virtualenv / model dir present → `path`
- `python3` ≥ x → `tool`
- Repo-specific secrets/profiles readiness → **plugin** (§6)
- Workspace member on triage → `delegate` check; straggler on `make doctor` →
`command` + `dir:` (§5)

- **`group`** — container type; the type-key value is the section **name**.
  **`items:`** holds child checks (`tool`, `command`, `delegate`, …) and nested
  **`group`** entries. Optional `platform:` on the container skips the whole
  section on non-matching OS. Output: section header with the group's worst glyph,
  children indented one level (§7.2). Groups may nest arbitrarily deep.
  ```yaml
  - group: Core toolchain
    items:
      - tool: git
      - tool: go
        version_from: .go-version
  - group: Signing (macOS)
    platform: macos
    items:
      - command: security find-identity -v -p codesigning
        label: Apple Distribution certificate
        contains: Apple Distribution
  ```
- **`delegate`** — the type key value is the display name. Required **`dir:`**
  (member path). Optional **`config:`** — path to the child config, default
  `triage.yaml` under `dir:`. Loads and runs the child in-process; active
  `--profile` is forwarded. One pass/fail for the whole child subtree; human
  output uses the **delegate tree** (§7.2). Child configs may include their own
  `delegate` checks (same rules, nested arbitrarily deep). Sibling delegates
  **execute concurrently** with each other and with local checks (§7.2) and are
  **rendered** depth-first in list order. No auto-discovery; cycles (A→B→A) →
  config error (exit `3`). No dedup across levels.

```yaml
default:
  - tool: make
    group: Workspace
  - command: make doctor MODE={{profile}}
    label: vitalink-web
    dir: vitalink-web
    group: Members
  - delegate: vitalink-api
    dir: vitalink-api
  - delegate: vitalink-infra
    dir: vitalink-infra
```

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
`go install` deferred (m7); scoop/winget later (§3, §11).
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

