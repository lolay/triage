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
- **Cross-platform.** macOS and Linux today; Windows coming soon.

## Quick start

```sh
# Released: install from the dedicated tap (not homebrew-core)
brew install lolay/tap/triage

# Pre-release / hacking: build from source (make is the source of truth)
git clone https://github.com/lolay/triage.git && cd triage
make build

# Run checks in any repo that has triage.yaml
cd your-repo
triage                                 # reads triage.yaml in the current directory
triage --profile release               # stricter preflight before you ship
triage path/to/triage.yaml             # explicit config file or directory
```

Run from the directory that contains the config (or pass the path). `triage` does
not search parent directories.

## Configuration

Add `triage.yaml` (or `.triage.yaml`) in the directory where you run `triage`
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

See the full schema in [`specs/product.md`](./specs/product.md) §4–§7, and one
worked config per repo shape under [`examples/`](./examples).

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
report; `--quiet` hides passes. On a TTY,
`triage` may show a one-line notice when a newer release is available (cached
GitHub check; `TRIAGE_NO_UPDATE_CHECK=1` or `--no-update-check` to disable).
Subprocess
output from `command` checks is not printed by default; use `--command-log` to
stream probe output to a file (overwritten each run), or `--verbose` to replay
failures on stderr.

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
