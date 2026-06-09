# triage examples

These configs are **fictional** — two made-up medical companies, **Vitalink**
(a multi-repo health platform) and **Otoscope** (a standalone macOS app). They
reference no real project; they exist to demonstrate every feature in
[specs/product.md](../specs/product.md) and double as the acceptance set for the
milestone acceptance fixtures (m3 check types, m4 delegation; m6 action dogfoods here).

## Running an example

`triage` loads config from the **current working directory** only (no walk-up).
From this repo:

```sh
triage examples/vitalink-api.yaml
triage examples/otoscope-mac.yaml --profile release
triage examples/vitalink-workspace.yaml
triage examples/vitalink-workspace --no-color
```

Or `cd` into a directory that contains a config and run `triage` with no args.

## Grouping

Examples use **structural `group` containers** (`group:` + `items:`). Checks
without a containing group render at the top level (mixed layout). Nested
sections appear in `otoscope-mac.yaml`.

| File | Shape | Features exercised |
|---|---|---|
| [`vitalink-base.yaml`](vitalink-base.yaml) | shared base | the building block for cross-file `include` |
| [`vitalink-api.yaml`](vitalink-api.yaml) | Go service | `include`, `tool`, `version_from` (`.go-version`), `extends`, **gcloud auth/session** |
| [`vitalink-web.yaml`](vitalink-web.yaml) | Node/pnpm app | `version_from` (`.nvmrc`), `one_of` (pnpm/npm), `severity: warn`, **firebase + gcloud auth/session** |
| [`vitalink-infra.yaml`](vitalink-infra.yaml) | Terraform | `version_from` (`.terraform-version`), empty release profile |
| [`otoscope-mac.yaml`](otoscope-mac.yaml) | macOS Swift app | structural **`group`+`items:`**, `command`+`platform`, `path`, heterogeneous `one_of`, keychain/xcrun/Xcode, `warn` |
| [`vitalink-workspace.yaml`](vitalink-workspace.yaml) / [`vitalink-workspace/`](vitalink-workspace/) | estate root | mixed layout: structural `Workspace` group + top-level `delegate` checks (nested tree output); runnable with path-only member checks |
| [`../triage.yaml`](../triage.yaml) | **this repo** (dogfood) | `tool`, `version_from`, `extends`, `severity: warn` |

## Auth / session checks

A common need beyond "is the tool installed" is "am I **logged in and not
expired**." These are `command` checks that run the CLI's own auth probe and
assert exit 0:

- **gcloud logged in, token valid** — `gcloud auth print-access-token` exits
  non-zero when logged out *or* the token has expired. (`vitalink-api`, `-web`)
- **firebase authenticated** — `firebase login:list` lists a signed-in account
  (`contains: "@"`). (`vitalink-web`)

## Use-case coverage vs. the original bash doctor

- **Config-driven tool inventory** (`name|min|hint`) → `tool` + `version` / `version_from`.
- **`include` directive** → top-level `include:` list (multiple files).
- **`MODE=<profile>`** → `triage --profile <name>`; `default` when omitted;
  examples use `default` / `release` + `extends`/`add`.
- **Bespoke checks** (Xcode path/version, keychain certs, xcrun, a venv dir) →
  `command`+`platform`, `path`. (`otoscope-mac`)
- **Submake secrets doctor** (age identity, sealed blobs) → heterogeneous
  `one_of` + `path`. (`otoscope-mac`)
- **Workspace loop** → `delegate` checks for migrated members; `command` + `dir:` for stragglers; all in profile list order.
  (`vitalink-workspace`)
- **Auth/session validity** → `command` auth probes (`gcloud`/`firebase`).

## Caveat

Most single-file examples assume real tools (`go`, `terraform`, Xcode, …) are
installed — they demonstrate config shape, not guaranteed green boards.
[`vitalink-workspace`](vitalink-workspace/) is self-contained (path checks only)
and should pass on any machine.
