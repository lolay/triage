<p align="center">
  <strong>triage — the environment doctor.</strong>
</p>

> **tldr** — Command utility to check you have all the prerequisites you need
> for any build, CI, and tools to succeed.

## What is triage?

`triage` reads a declarative [`triage.yaml`](./triage.yaml) in your repo and
reports what's present, what's missing, and exactly how to fix it — in a grouped
✓/✗/! board. Install once via the
[`lolay/homebrew-tap`](https://github.com/lolay/homebrew-tap); use it in any
project that declares its prerequisites in config.

```
triage (profile: default)

[✓] Core toolchain
    [✓] git 2.49.0
    [✓] go 1.26.3 (>= 1.26)
[!] Lint
    [!] golangci-lint not found — brew install golangci-lint
[✗] Signing (macOS)
    [✗] Apple Distribution cert missing — make keychain-import

✗ 1 error, 1 warning, 4 ok — fix the [✗] items above
```

## Why triage?

- **One binary, not a script.** No per-repo shell to copy, fork, and drift.
- **Declarative checks.** Tools, versions, env vars, files, "one of these",
  platform-guarded commands, and auth/session probes — all in `triage.yaml`,
  no bespoke bash.
- **Graded severity.** `error` / `warn` / `info` in output; exit `0`/`1` by
  default (pass/fail). `--severity` maps exit codes to severity: `0`/`1`/`2`.
- **Delegation.** A `delegate` check loads a child `triage.yaml` (nested board with
  a live `[…]` summary) — a peer in the profile list. Slow checks show `[…]` while
  running, then flip to `[✓]`/`[✗]` on a TTY. `make doctor` stragglers use
  `command` + `dir:` (flat line, same pending behavior).
- **Fast by default.** Checks are read-only, so they run through a bounded
  worker pool (`--jobs`/`-j`, default ≈ NumCPU) — sibling delegates and local
  checks in parallel. Output always **materializes in list order**, so it's
  byte-identical at any `--jobs` (`-j 1` is fully sequential). Pin a check or
  group with `serial: true` for a shared scarce resource (lock, rate-limited
  auth, keychain).
- **Cross-platform.** macOS, Linux, and WSL2 via Homebrew; native Windows
  `.exe` ships on GitHub Releases (scoop/winget packaging in m7).

## Quick start

```sh
# Released: install from the dedicated tap (not homebrew-core)
brew install lolay/tap/triage

# Or download a release tarball from GitHub Releases
# https://github.com/lolay/triage/releases
# WSL2 uses the Linux formula above; native Windows: grab triage_*.zip from Releases

# Pre-release / hacking: build from source (make is the source of truth)
git clone https://github.com/lolay/triage.git && cd triage
make build

# Run checks in any repo that has triage.yaml
cd your-repo
triage                                 # reads triage.yaml in the current directory
triage --profile release               # stricter preflight before you ship
triage path/to/triage.yaml             # explicit config file or directory
```

After `brew install lolay/tap/triage` (or extracting a release tarball with the
included man pages), `man triage` shows the CLI manual and `man 5 triage` shows
the full config file format. Man page sources: [`man/`](./man/).

Run from the directory that contains the config (or pass the path). `triage` does
not search parent directories.

## Configuration

Add `triage.yaml` (or `triage.yml` / `.triage.yaml` / `.triage.yml`) in the
directory where you run `triage`
(typically your repo root — `cd` there first, or pass the path). Root is always a mapping:
**`include`** (reserved top-level key) or profile keys. **`default`** is the only
special profile (used when `--profile` is omitted); any other name is valid
(`release`, `ci`, …). Checks live under profiles — never at the root. `default`
may be omitted (empty). `extends` + `add:` inherits; `include` composes files in
order (collisions flagged; later wins):

```yaml
default:
  - tool: go
    version_from: .go-version
  - tool: golangci-lint
    severity: warn

# Or multi-profile / shared base:
include:
  - base.yaml

release:
  extends: [default]
  add:
    - command: gcloud auth print-access-token
      label: gcloud logged in (token valid, not expired)
      platform: [macos, linux]
      hint: gcloud auth login
```

See the full config reference in [`specs/config.md`](./specs/config.md) (and
`man 5 triage` after install). JSON Schema:
[`schema/triage.schema.json`](./schema/triage.schema.json). Worked examples:
[`examples/`](./examples).

## Exit codes

Default: `0` pass · `1` fail — CI and scripts can fail fast with no exit-code
guards.

**`--severity`** opts into a severity-graded exit ladder so callers can branch on
warnings vs errors:

| Code | Meaning |
|---|---|
| `0` | Clean — all checks pass |
| `1` | Warnings only |
| `2` | Errors (≥1 `error`) |
| `3` | Config/usage error |

`--strict` escalates warnings to errors. `--json` emits a machine-readable
report (each result carries `depth` + `kind` so the delegate tree is
reconstructable); `--quiet` hides passes. `--jobs`/`-j` sets the worker-pool
size (default ≈ NumCPU, capped at 8; `1` = fully sequential; values below 1 are
a usage error). An interrupted run (Ctrl-C / SIGTERM) prints the partial board
and exits `130`. On a TTY,
`triage` may show a one-line notice when a newer release is available (cached
GitHub check; `TRIAGE_NO_UPDATE_CHECK=1` or `--no-update-check` to disable).
Subprocess
output from `command` checks is not printed by default; use `--command-log` to
stream probe output to a file (overwritten each run; blocks are written in list
order so the file is identical regardless of `--jobs`), or `--verbose` to replay
failures on stderr.


## CI / GitHub Actions

Use the composite action in your workflows to run `triage` with a profile from
your repo (or a checked-out config):

```yaml
- uses: lolay/triage-action@v0.3
  with:
    profile: ci
    json: true
```

Pin to a major (`@v0`), minor (`@v0.3`), or exact tag as your policy requires.
Contract and inputs: [`specs/action.md`](./specs/action.md).

## Releasing

Maintainers: all artifacts build and validate first; **remote and marketplace
pushes happen last, in order** (GitHub Release → Homebrew tap → floating tags)
so a partial failure never leaves one channel updated while another is not.
Full process: [`specs/releasing.md`](./specs/releasing.md).

## Status

**Pre-release / spec-first.** The design is captured in
[`specs/product.md`](./specs/product.md) and the milestones there (m1–m7) track
implementation. The configs in [`examples/`](./examples) and the dogfood
[`triage.yaml`](./triage.yaml) are the spec made concrete and the in-repo
acceptance set. Per-repo migration is out of scope for those milestones. The CLI
is not yet published.

## Contributing

Bug reports, feature requests, and pull requests are welcome. Start with
[`CONTRIBUTING.md`](./CONTRIBUTING.md) for setup and the expected workflow.
Security issues follow [`SECURITY.md`](./SECURITY.md). All participation is
governed by our [`CODE_OF_CONDUCT.md`](./CODE_OF_CONDUCT.md). Design lives under
[`specs/`](./specs) — start there before touching code.

## License

Apache 2.0 — see [`LICENSE`](./LICENSE).
