# triage.yaml — config reference

Canonical reference for the `triage.yaml` file format. Design rationale and
CLI/output behavior live in [`product.md`](product.md). Editor validation:
[`schema/triage.schema.json`](../schema/triage.schema.json). Worked examples:
[`examples/`](../examples/) and the dogfood [`triage.yaml`](../triage.yaml).

After `brew install lolay/tap/triage` (or any package-manager install), `man 5
triage` shows this reference in man-page form; `man triage` covers the CLI.

---

## File structure

Root is always a **mapping**. Checks live under **profile keys** — never at
the root.

```yaml
# Single-profile repo
version: 1
default:
  - tool: git
    hint: https://git-scm.com
  - tool: go
    version_from: .go-version
    hint: https://go.dev/dl

# Multi-profile / include
include:
  - base.yaml

default:
  - tool: go
    version_from: .go-version

release:
  extends: [default]
  add:
    - tool: sops
      hint: brew install sops
```

### Reserved top-level keys

| Key | Role |
| --- | --- |
| `version` | Optional integer. Only `1` is supported; absent means `1`. Future format changes will bump this. |
| `include` | List of YAML files to merge (processed first, in order) |
| `vars` | Mapping of reusable string values for `{{ name }}` templates |
| Any other key | A **profile name** (`default`, `release`, `ci`, …) |

### Profiles

- **`default`** — the profile `triage` runs when `--profile` is omitted. May be
  omitted (treated as empty).
- **Any other name** — select with `triage --profile <name>`.
- Profile value is either a **check list**, or `{ extends: […], add: […] }` when
  inheriting. Use **`add:`** (not `checks:`) with `extends`.

### Composition

Two axes — deliberately separate names:

- **`include: [files…]`** — merge files in order. Collisions emit load-time
  warnings; later wins. Check identity: same type key + primary value (`tool: go`,
  `group: Core toolchain`, `delegate: api`, `command` on `label:`, …).
  Cycle/diamond-safe.
- **`extends: [profiles…]`** — per-profile inheritance within the merged config.
  Same collision-flag rule when a child check shadows a parent. Cycle/diamond-safe.

### Type-as-key checks

Each check's *type key* (`tool:` / `env:` / `path:` / …) carries its primary
value. Optional fields (`severity`, `platform`, `hint`, …) apply to any check.
All checks in a profile list are **ordered** — that order is the **render order**
(execution may be concurrent; output materializes in list order).

### Config file names

`triage.yaml` (canonical), `triage.yml`, `.triage.yaml`, `.triage.yml` — in
that precedence order. One entry file per run; compose via `include:` and
`extends:` only. **No walk-up:** the entry file must be in the **current working
directory** unless you pass an explicit path.

### Vars and templates

Reference a var in any check string field with `{{ var_name }}` (whitespace
inside braces allowed).

Precedence: config `vars:` < `--var name=value` CLI override < built-ins
(`profile`, `os`).

Expansion is **fail-closed**: undefined or malformed `{{ … }}` tokens fail the
check on the board. Names: letter/underscore followed by letters, digits,
underscores. **Vars are not recursive** — no re-expansion inside `vars:` values.

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

Built-in `profile` and `os` cannot be defined under `vars:` (ignored with a
warning).

---

## Severity

Orthogonal to profile: profile selects *which* checks run; severity decides *how
a failure is treated*.

| `severity` | Meaning | Glyph | Fails the run? |
| --- | --- | --- | --- |
| `error` (default) | Required | `[✗]` | Yes → exit `1` |
| `warn` | Recommended | `[!]` | No (unless `--strict`) |
| `info` | FYI only | `[ℹ]` | No |

```yaml
- tool: swiftformat
  severity: warn
  hint: brew install swiftformat
```

- `required: true|false` sugar (`true` → `error`, `false` → `warn`).
- `--strict` promotes every `warn` to `error`.

---

## Common fields (any check)

| Field | Purpose |
| --- | --- |
| `severity` | `error` / `warn` / `info` (see above) |
| `platform` | `macos` / `linux` / `windows` — string or list; skip on non-matching OS |
| `hint` | Human-readable remediation shown on failure |
| `dir` | Working directory for the check (default: config directory) |
| `serial` | Opt out of concurrency for this check or group |

---

## Check types

| Type | Checks |
| --- | --- |
| `tool` | Command present; optional `version` or `version_from` |
| `env` | Env var set or unset; optional `matches` regex |
| `path` | File/dir exists (glob ok) |
| `one_of` | At least one of N alternatives present |
| `platform` | Guard on any check or group — runs only on matching OS |
| `command` | Explicitly-interpreted snippet; assert exit/stdout |
| `delegate` | Load and run another `triage.yaml` from `dir:` |
| `group` | Named container via `items:` |

### `tool`

```yaml
- tool: go
  version_from: .go-version
  hint: https://go.dev/dl
- tool: node
  version: ">=20"
```

Version fields (pick **one**; loader warns if both):

- **`version`** — npm-style semver range, quoted in YAML (`">=1.26"`, `"^1.26.0"`,
  `"~1.2.0"`, `"1.x"`, …).
- **`version_from`** — read a pin file (`.go-version`, `.nvmrc`, `.tool-versions`,
  …). Bare version → `>=` that version; existing range used as-is.

### `env`

```yaml
- env: SOPS_AGE_KEY
- env: DATABASE_URL
  unset: true
- env: ASC_KEY_ID
  matches: '^[A-Z0-9]+$'
```

- **Set (default):** variable must be present. Optional `matches:` regex. Empty
  value counts as set.
- **Unset:** `unset: true` — must not be in environment. Mutually exclusive with
  `matches:`.

### `path`

```yaml
- path: .venv
- path: private_keys/*.p8
```

Supports globs. Relative paths resolve from `dir:` or config directory.

### `one_of`

Tool names or sub-checks:

```yaml
- one_of: [pnpm, npm]
- one_of:
    - env: SOPS_AGE_KEY
    - path: ~/.config/sops/age/keys.txt
```

### `command`

Requires `label:`. No implicit shell — `interp:` picks `sh`/`pwsh` (default `sh`).

```yaml
- command: security find-identity -v -p codesigning
  label: Apple Distribution certificate
  contains: Apple Distribution
  platform: macos
  hint: make keychain-import
- command: terraform validate
  label: terraform config valid
  with_env:
    TF_IN_AUTOMATION: "1"
    MODE: "{{ profile }}"
```

Assertions (pick one mode):

- **exit 0** (default)
- `contains:` / `matches:` on stdout (bounded scan)
- `exit:` expected exit code

Optional **`with_env:`** — map layered over process env for this command only.
Values support `{{ name }}` expansion; no `$VAR` interpolation by triage.

Auth/session probes: run the tool's own check (`gcloud auth print-access-token`,
`firebase login:list` + `contains: "@"`, …).

### `delegate`

```yaml
- delegate: vitalink-api
  dir: vitalink-api
  config: triage.yaml   # optional; default triage.yaml under dir:
```

Display name is the type-key value. `--profile` is forwarded. Nested delegates
allowed; cycles → config error (exit `3`).

### `group`

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

Container type; `items:` holds child checks and nested groups. Optional
`platform:` skips the whole section on non-matching OS.

Checks without a containing `group` render at the top level. A profile may mix
top-level checks and `group` containers in any order.

---

## Workspace patterns

```yaml
default:
  - group: Workspace
    items:
      - tool: make
  - group: Members
    items:
      - command: make doctor MODE={{profile}}
        label: vitalink-web
        dir: vitalink-web
  - delegate: vitalink-api
    dir: vitalink-api
  - delegate: vitalink-infra
    dir: vitalink-infra
```

- **`delegate`** — nested `triage.yaml` with tree output.
- **`command` + `dir:`** — flat line in parent board (e.g. straggler on
  `make doctor`).

Typical bash → config mappings:

| Today | triage |
| --- | --- |
| Xcode path/version | `command` + `platform: macos` |
| Keychain certs | `command` (`security find-identity`) |
| Virtualenv dir | `path` |
| python3 ≥ x | `tool` with `version` |
| Workspace member | `delegate` |

---

## See also

- [`product.md`](product.md) — goals, CLI flags, output, exit codes, concurrency
- [`schema/triage.schema.json`](../schema/triage.schema.json) — JSON Schema
- [`examples/`](../examples/) — illustrative repo shapes
