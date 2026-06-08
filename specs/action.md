# triage GitHub Action — contract reference

Canonical reference for the **`lolay/triage-action`** composite action: its
inputs, outputs, CLI-version resolution, OS/arch + checksum mapping, and
exit-code forwarding. The action **wraps the `triage` CLI** — it installs the
binary from [`lolay/triage`](https://github.com/lolay/triage) GitHub release
assets and runs a profile in CI.

Design rationale lives in [`product.md`](product.md) §3 (distribution) and
[`milestones.md`](milestones.md) m6; CLI flags/exit codes in
[`product.md`](product.md) §7; release/versioning ordering in
[`releasing.md`](releasing.md). This file is the **contract** the
`lolay/triage-action` repo implements (its `action.yml` / `install.sh` /
`run.sh` realize the shapes finalized in [§7](#7-finalized-file-shapes)); the
action repo's README links here.

> **Two repos, one contract.** The CLI source and release binaries live in
> `lolay/triage`; the composite-action wrapper lives in `lolay/triage-action`.
> The action downloads the **CLI from `lolay/triage` releases**, never from the
> action repo's own tag contents. See [§6](#6-coordinated-versioning--floating-tags).

---

## 1. Usage

Pin the action with a floating or exact tag (GitHub does not semver-resolve
action refs — you pick the granularity):

```yaml
- uses: lolay/triage-action@v0      # latest 0.x      (major floating)
- uses: lolay/triage-action@v0.3    # latest 0.3.x    (minor floating — preferred pre-1.0)
- uses: lolay/triage-action@v0.3.1  # exact           (immutable)
```

Common CI step:

```yaml
- uses: lolay/triage-action@v0.3
  with:
    profile: ci
    strict: true
    json: true
    command-log: .triage/commands.log
```

Pre-1.0, prefer `@v0.3` (minor): `0.x` minor bumps may carry breaking changes,
so `@v0` tracks them all. Post-1.0, `@v1` (major) becomes the idiomatic default.
Do **not** document `@main` for external consumers; `@<sha>` is for dogfood / PR
CI only.

---

## 2. Inputs

All inputs are optional. Boolean inputs default `false` and map to a bare flag
when `true`. String inputs default empty (`''`); empty means "unset" — the
action omits the corresponding CLI flag/argument rather than passing a blank
value. **Typed inputs cover the CI-relevant flags; `args` is the escape hatch
for anything else (and forward-compat with future CLI flags).**

| Input | Required | Type / default | Maps to | Purpose |
| --- | --- | --- | --- | --- |
| `version` | Optional | string `''` | — (install) | **Optional override.** Empty (default) → the CLI version pinned by the action release (the `VERSION` file), so the action tag you pick selects the CLI. Set it only to install a *different* CLI, e.g. `0.3.1` (no leading `v`). See [§4](#4-cli-version-resolution-the-crux). |
| `profile` | Optional | string `''` | `--profile <profile>` | Profile to run. Omit for the default profile. |
| `config` | Optional | string `''` | `triage … <config>` (positional, **last**) | Path to a `triage.yaml` (file or directory). Omit → the CLI's normal discovery in the working directory (`triage.yaml` / `triage.yml` / `.triage.yaml` / `.triage.yml`). |
| `strict` | Optional | bool `false` | `--strict` | Treat warnings as errors. |
| `severity` | Optional | bool `false` | `--severity` | Toggle (on/off, **not** a level): switch from pass-fail to the graded exit ladder — `0` clean / `1` warn-only / `2` error / `3` config — so a job can tell warnings from errors. See [§5](#5-exit-code-forwarding). |
| `json` | Optional | bool `false` | `--json` | Emit machine-readable JSON. |
| `quiet` | Optional | bool `false` | `--quiet` | Suppress passing checks; show only `[!]`/`[✗]` + summary. |
| `verbose` | Optional | bool `false` | `--verbose` | Replay failed-check output to stderr. |
| `command-log` | Optional | string `''` | `--command-log <path>` | Path to write the per-check command log (upload it as an artifact). Empty disables. |
| `jobs` | Optional | string `''` | `--jobs <n>` | Max concurrent checks (bounded worker pool). Empty → CLI default (≈ NumCPU); `1` = fully sequential. |
| `working-directory` | Optional | string `'.'` | run step `working-directory` | Directory `triage` runs in. |
| `args` | Optional | string `''` | passthrough | Escape hatch for any other / future flags, space-separated (e.g. `--var key=val --no-color`). Word-split intentionally. |

**Argv order** (`run.sh`): `triage [--profile P] [bool/typed flags…] [args…] [config]`.
The optional `config` positional is always **last** (CLI rule,
[`product.md`](product.md) §4).

**Why no `github-token` input:** release-asset downloads come from public
`releases/download/...` URLs that need no auth, and the committed `VERSION` file
(below) always resolves the version locally — so the action never calls the
GitHub API and needs no token. Auto-handled flags (`--no-color`,
`--no-update-check`) are omitted too: the CLI already disables color and the
update banner in CI / non-TTY (`product.md` §7.2). Reach for `args` if you ever
need them.

---

## 3. Outputs

| Output | Source | Meaning |
| --- | --- | --- |
| `version` | install step | The resolved triage CLI version that was installed (e.g. `0.3.1`). |
| `exit-code` | run step | The **raw** exit code returned by `triage` (see [§5](#5-exit-code-forwarding)). |

Reference an output downstream:

```yaml
- id: triage
  uses: lolay/triage-action@v0.3
  with:
    profile: ci
    severity: true
  continue-on-error: true
- run: echo "triage installed ${{ steps.triage.outputs.version }}, exit ${{ steps.triage.outputs.exit-code }}"
```

---

## 4. CLI version resolution (the crux)

A **floating** action tag (`v0.3`, `v0`) or a SHA does **not** by itself map to a
CLI release. The committed `VERSION` file is what makes floating action tags
deterministic.

Resolution order (first match wins):

1. **`version` input** — used verbatim (e.g. `0.3.1`).
2. **`VERSION` file** at the action repo root — written/bumped by the
   triage-action release workflow at tag time, so a floating tag `v0.3` points
   at the commit whose `VERSION` is, say, `0.3.1`. **This is the normal,
   deterministic path** for every released tag (exact, minor, or major) and for
   dogfood (`uses: ./` checks out the file).
3. Otherwise **fail fast** with a clear message (set the `version` input or
   commit a `VERSION` file).

> Resolution is intentionally **local and tokenless**: no `github.action_ref`
> parsing, no `releases/latest` API call, no `github-token`. GitHub does not
> semver-resolve action refs and `github.action_ref` reports the *requested* ref
> (`v0.3`), not the concrete version behind it — the `VERSION` file is the
> single source of truth.

**Normally you don't set `version`.** The action tag is the selector: `@v0.3.1`
(and the floating `@v0.3` / `@v0` that point at it) carry a `VERSION` of `0.3.1`,
so you get CLI 0.3.1 with no extra input — the CLI and action move together
(coordinated semver, [§6](#6-coordinated-versioning--floating-tags)). The
`version` input exists only to run a CLI *different* from the action's pinned
default, e.g.:

- try a newer CLI patch before the paired action release ships;
- pin an exact CLI patch while floating the action tag;
- roll back a bad CLI release without changing the action.

The typed inputs map to the **paired** CLI's flag surface, so overriding to a CLI
from a different major line (where flags may have changed) is the consumer's
responsibility.

---

## 5. Exit-code forwarding

triage's exit code is **forwarded unchanged** — a job using this action
succeeds or fails exactly as `triage` would on the same machine.

**Severity model (set in `triage.yaml`, not the action).** Every check declares a
`severity` — `error` (default; a failure fails the run), `warn` (recommended), or
`info` (FYI only) — see [`config.md`](config.md) "Severity". The `strict` and
`severity` inputs do **not** pick a level; they only change how those check
outcomes become a process exit code:

- **default** (both off) — pass-fail: only `error` failures count (exit `1`);
  `warn`/`info` print but the run still exits `0`.
- **`strict: true`** — `warn` is escalated to `error`, so warnings fail the run.
- **`severity: true`** — graded ladder (table below): a `warn`-only run exits `1`
  and an `error` run exits `2`, so the job can tell them apart via `exit-code`.

| triage mode | Codes | Job result under forwarding |
| --- | --- | --- |
| pass-fail (default) | `0` ok / `1` errors / `3` config-or-usage | `1`/`3` fail the job |
| `severity: true` | `0` clean / `1` warnings-only / `2` errors / `3` config | `1`/`2`/`3` fail the job |

**No `fail-on` knob in v1** — forwarding stays pure. Note the sharp edge: with
`severity: true`, exit `1` means *warnings only* and will fail the job.
Consumers who want to **branch** on warnings rather than fail set
`continue-on-error: true` and inspect the `exit-code` output (see
[§3](#3-outputs)).

(Exit-code semantics are owned by the CLI — [`product.md`](product.md) §7.3. The
action neither remaps nor suppresses them.)

---

## 6. Coordinated versioning & floating tags

The action and CLI ship **matching semver together** (e.g. CLI `v0.3.1` on
`lolay/triage` and action `v0.3.1` on `lolay/triage-action` in the same release
beat). The action release runs **after** the CLI release is fully green
([`releasing.md`](releasing.md) "remote pushes last").

Floating tags on `lolay/triage-action` (GitHub does not semver-resolve refs):

| Tag | Role |
| --- | --- |
| `vX.Y.Z` | Exact action release (immutable). |
| `vX.Y` | Retagged to the latest `X.Y.*` action on each patch; pairs with the bumped `VERSION`. |
| `vX` | Retagged to the latest `X.*` action (major floating, e.g. `v0`). |

The `VERSION` bump and both floating-tag promotions (`vX.Y` **and** `vX`) happen
in the triage-action release workflow (Wave 5 of the m6 plan), mirroring the
CLI's `promote-floating-tags` ordering.

---

## 7. Finalized file shapes

The shapes below are the **Wave 1 output** — the contract Wave 2 implements at
the root of `lolay/triage-action`. They follow the `taiki-e/install-action`
model: composite + bash, tool-cache aware, no committed `dist/`, only
`bash` + `curl` + `tar`/`unzip` assumed on the runner.

### 7.1 `action.yml`

```yaml
name: triage
description: >-
  Run triage — the environment doctor — in CI. Installs the triage CLI from
  lolay/triage releases (tool-cache aware) and runs the selected profile.
author: lolay
branding:
  icon: check-circle
  color: green

inputs:
  version:
    description: triage CLI version to install (e.g. 0.3.1). Defaults to the version pinned by this action release (VERSION file).
    required: false
    default: ''
  profile:
    description: Profile to run (triage --profile). Omit for the default profile.
    required: false
    default: ''
  config:
    description: Path to triage.yaml (file or directory). Omit to use the working directory's discovered config.
    required: false
    default: ''
  strict:
    description: Treat warnings as errors (triage --strict).
    required: false
    default: 'false'
  severity:
    description: Use the severity-graded exit ladder (triage --severity).
    required: false
    default: 'false'
  json:
    description: Emit machine-readable JSON (triage --json).
    required: false
    default: 'false'
  quiet:
    description: Suppress passing checks; show only warnings/errors (triage --quiet).
    required: false
    default: 'false'
  verbose:
    description: Replay failed-check output to stderr (triage --verbose).
    required: false
    default: 'false'
  command-log:
    description: Path to write the per-check command log (triage --command-log <path>). Empty disables it.
    required: false
    default: ''
  jobs:
    description: Max concurrent checks (triage --jobs). Empty uses the CLI default (~NumCPU); 1 = sequential.
    required: false
    default: ''
  working-directory:
    description: Directory to run triage in.
    required: false
    default: '.'
  args:
    description: Escape hatch for any other/future triage flags, space-separated (e.g. "--jobs 2 --var key=val").
    required: false
    default: ''

outputs:
  version:
    description: Resolved triage CLI version that was installed.
    value: ${{ steps.install.outputs.version }}
  exit-code:
    description: Raw exit code returned by triage.
    value: ${{ steps.run.outputs.exit-code }}

runs:
  using: composite
  steps:
    - id: install
      shell: bash
      run: bash "${GITHUB_ACTION_PATH}/install.sh"
      env:
        INPUT_VERSION: ${{ inputs.version }}
    - id: run
      shell: bash
      working-directory: ${{ inputs.working-directory }}
      run: bash "${GITHUB_ACTION_PATH}/run.sh"
      env:
        INPUT_PROFILE: ${{ inputs.profile }}
        INPUT_CONFIG: ${{ inputs.config }}
        INPUT_STRICT: ${{ inputs.strict }}
        INPUT_SEVERITY: ${{ inputs.severity }}
        INPUT_JSON: ${{ inputs.json }}
        INPUT_QUIET: ${{ inputs.quiet }}
        INPUT_VERBOSE: ${{ inputs.verbose }}
        INPUT_COMMAND_LOG: ${{ inputs.command-log }}
        INPUT_JOBS: ${{ inputs.jobs }}
        INPUT_ARGS: ${{ inputs.args }}
```

### 7.2 `install.sh` — resolve → download → verify → cache → addPath

```bash
#!/usr/bin/env bash
set -euo pipefail
REPO="lolay/triage"

# 1. Resolve version: input > committed VERSION file (deterministic for floating tags).
version="${INPUT_VERSION:-}"
if [[ -z "$version" && -f "${GITHUB_ACTION_PATH}/VERSION" ]]; then
  version="$(tr -d ' \n' < "${GITHUB_ACTION_PATH}/VERSION")"
fi
[[ -n "$version" ]] || {
  echo "::error::no triage CLI version: set the 'version' input or commit a VERSION file"; exit 1
}

# 2. Map runner OS/arch -> goreleaser asset naming.
case "$RUNNER_OS" in
  Linux) os=linux ;; macOS) os=darwin ;; Windows) os=windows ;;
  *) echo "::error::unsupported OS $RUNNER_OS"; exit 1 ;;
esac
case "$RUNNER_ARCH" in
  X64) arch=amd64 ;; ARM64) arch=arm64 ;;
  *) echo "::error::unsupported arch $RUNNER_ARCH"; exit 1 ;;
esac
ext=tar.gz; [[ "$os" == windows ]] && ext=zip

# 3. tool-cache lookup (warm runners skip the re-download). Assets are public — no token.
tool_dir="${RUNNER_TOOL_CACHE}/triage/${version}/${arch}"
if [[ ! -x "${tool_dir}/triage" && ! -x "${tool_dir}/triage.exe" ]]; then
  asset="triage_${version}_${os}_${arch}.${ext}"
  base="https://github.com/${REPO}/releases/download/v${version}"
  tmp="$(mktemp -d)"
  curl --proto '=https' --tlsv1.2 -fsSL -o "${tmp}/${asset}"        "${base}/${asset}"
  curl --proto '=https' --tlsv1.2 -fsSL -o "${tmp}/checksums.txt"   "${base}/checksums.txt"
  # Verify: sha256sum on Linux, shasum -a 256 on macOS.
  ( cd "$tmp" && grep " ${asset}\$" checksums.txt \
      | { command -v sha256sum >/dev/null && sha256sum -c - || shasum -a 256 -c -; } )
  mkdir -p "$tool_dir"
  if [[ "$ext" == zip ]]; then unzip -q "${tmp}/${asset}" -d "$tool_dir";
  else tar -xzf "${tmp}/${asset}" -C "$tool_dir"; fi
fi

# 4. Put it on PATH and export the resolved version.
echo "$tool_dir" >> "$GITHUB_PATH"
echo "version=$version" >> "$GITHUB_OUTPUT"
```

### 7.3 `run.sh` — build argv → exec → forward exit code

```bash
#!/usr/bin/env bash
set -uo pipefail
cmd=(triage)
[[ -n "${INPUT_PROFILE:-}" ]]       && cmd+=(--profile "$INPUT_PROFILE")
[[ "${INPUT_STRICT:-}"   == true ]] && cmd+=(--strict)
[[ "${INPUT_SEVERITY:-}" == true ]] && cmd+=(--severity)
[[ "${INPUT_JSON:-}"     == true ]] && cmd+=(--json)
[[ "${INPUT_QUIET:-}"    == true ]] && cmd+=(--quiet)
[[ "${INPUT_VERBOSE:-}"  == true ]] && cmd+=(--verbose)
[[ -n "${INPUT_COMMAND_LOG:-}" ]]   && cmd+=(--command-log "$INPUT_COMMAND_LOG")
[[ -n "${INPUT_JOBS:-}" ]]          && cmd+=(--jobs "$INPUT_JOBS")
# shellcheck disable=SC2206  # intentional word-split for the escape-hatch flags
[[ -n "${INPUT_ARGS:-}" ]]          && cmd+=($INPUT_ARGS)
[[ -n "${INPUT_CONFIG:-}" ]]        && cmd+=("$INPUT_CONFIG")   # optional positional, LAST
"${cmd[@]}"; code=$?
echo "exit-code=$code" >> "$GITHUB_OUTPUT"
exit "$code"
```

---

## 8. Asset mapping (from `lolay/triage` goreleaser)

The install step depends on the goreleaser asset naming
([`.goreleaser.yaml`](../.goreleaser.yaml)); keep these in lockstep with any CLI
release change.

| Concern | Value |
| --- | --- |
| Archive name | `triage_<version>_<os>_<arch>.<ext>` (`<version>` has **no** leading `v`) |
| `<os>` | `darwin` / `linux` / `windows` (from `RUNNER_OS` `macOS`/`Linux`/`Windows`) |
| `<arch>` | `amd64` / `arm64` (from `RUNNER_ARCH` `X64`/`ARM64`; error otherwise) |
| `<ext>` | `tar.gz` (unix) / `zip` (windows) |
| Checksums | `checksums.txt` alongside the archive; verified before extract |
| Download base | `https://github.com/lolay/triage/releases/download/v<version>/` (public, no auth) |
| Tool-cache key | `${RUNNER_TOOL_CACHE}/triage/<version>/<arch>` |

Windows asset mapping is defined for completeness; **Windows CI runtime support
ships with the CLI's native-Windows milestone (m7)**. Linux and macOS
(amd64/arm64) are the v1 runner targets.

---

## See also

- [`product.md`](product.md) — §3 distribution, §7 CLI flags/exit codes
- [`milestones.md`](milestones.md) — m6 scope, coordinated semver, floating tags
- [`releasing.md`](releasing.md) — release ordering ("remote pushes last")
- [`.goreleaser.yaml`](../.goreleaser.yaml) — asset matrix the install step targets
```