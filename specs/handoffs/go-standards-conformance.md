# Handoff — Go standards conformance

> **Audience:** any agent or developer continuing work on this repo.
> **Status:** complete and fully committed (4/4 groups, June 2026). This
> document records what changed, what was decided, and what residual work
> remains open for a future pass.

## 1. What was done

Four groups of conformance work landed as four sequential commits. Each group
was `make ci` green before commit; commits are the review gate.

| Commit | Group | Nature |
| --- | --- | --- |
| `c3aaf28` | Group 1 — structure & cleanup | Move `cmd/triage/main.go` → `./main.go`; remove `devNull`, `StaticSink`/`NewStaticSink`, `mergeVarMaps`, and the `Check` interface (all dead/speculative). |
| `b330f8a` | Group 2 — linters | Enable `modernize`, `errorlint`, `revive`, `bodyclose`, `govet enable-all` in `.golangci.yml`; fix all 58 surfaced findings. Add `TestJSON_KeyOrder` to lock `--json` key order against accidental fieldalignment reorder. |
| `fcb433f` | Group 3 — security & CI | Enable `gosec`; triage every finding individually (see §3). Harden `command_log.go` to `0o600`/`0o750`. Wire `govulncheck` as a `go.mod` tool dependency; add `make vuln` target; add `go mod verify` + `make vuln` to CI. Add `-cover` to `make test`. |
| `88d5770` | Group 4 — testify | Add `testify v1.11.1`; convert all 10 `*_test.go` files to `require`/`assert`; update `CONTRIBUTING.md` convention. |

## 2. Locked decisions

### Entrypoint layout
Single-binary repos put `main.go` at the repo root; `cmd/` is reserved for
multi-binary repos. Do not reintroduce a `cmd/triage/` directory.

### Linter set (`.golangci.yml`)
The active linter set is: `default: standard` (errcheck, govet, ineffassign,
staticcheck, unused) + `misspell`, `modernize`, `errorlint`, `revive`,
`bodyclose`, `gosec`. `govet.enable-all: true`.

**Do not disable gosec globally.** Every production finding was individually
justified — see the per-line `//nolint:gosec // G…:` comments in:
- `internal/engine/command.go` — G204: operator-authored `command:` checks
- `internal/engine/tool.go` — G204: probing operator-declared tools
- `internal/config/load.go` ×2 — G304: reading operator-supplied config + `version_from` pin
- `internal/engine/command_log.go` — G304: writing operator-supplied `--command-log` path

Test files (`*_test.go`) are excluded from gosec via a scoped
`linters.exclusions` rule in `.golangci.yml` — fixture writes to `t.TempDir()`
are not production attack surface.

**`--json` key order** (`internal/report/json.go`): `JSONResult` carries a
`//nolint:govet` to prevent `fieldalignment` from reordering its fields.
`encoding/json` emits struct keys in declaration order; the documented
`name`/`kind`/`depth`/`severity`/`status`/`detail` order is spec §7.3.
`TestJSON_KeyOrder` enforces this. Do not reorder `JSONResult` fields without
updating the test and the spec.

### command_log permissions
`OpenCommandLog` creates the parent directory at `0o750` and the file at
`0o600`. The log records run lines including `with_env` values (potential
secrets). Do not loosen these to `0o755`/`0o644`.

### govulncheck wiring
`govulncheck` is a `go.mod` tool dependency (`tool golang.org/x/vuln/cmd/govulncheck`).
Run it via `go tool govulncheck ./...` or `make vuln`. CI runs it on every
push/PR via `make vuln`. Do not remove the `tool` directive from `go.mod`.

### Test framework
All unit tests use `github.com/stretchr/testify/require` (fatal preconditions,
before indexing slices) and `github.com/stretchr/testify/assert` (independent
checks). The `CONTRIBUTING.md` convention matches. Do not add new tests using
bare `if … t.Errorf` patterns. Golden-fixture end-to-end tests in
`internal/cli/e2e_test.go` remain — testify wraps the byte comparison but the
golden files and `--jobs` invariance loop are unchanged.

## 3. Coverage snapshot (Group 3 baseline)

| Package | Coverage |
| --- | --- |
| `internal/cli` | 72.5% |
| `internal/config` | 86.4% |
| `internal/engine` | 87.6% |
| `internal/report` | 13.6% |

`internal/report` is low because the board renderer writes directly to a TTY
writer; the golden tests exercise it only through the CLI integration path. No
hard coverage threshold is enforced in CI yet — see residual items.

## 4. Residual / open items (not done)

These were explicitly deferred and recorded in the plan's out-of-scope section:

- **Coverage 80% gate.** Coverage is measured and printed on every `make test`
  run. Enforcing a hard threshold in CI (`go test -coverprofile … | fail if <
  80%`) was deferred to avoid surprise failures. When adding it, note that
  `internal/report` will need attention first.
- **`t.Parallel()` / `testing/synctest`.** Tests are not parallelized.
  `concurrency_test.go` uses real atomics and `time.Sleep` for the semaphore
  and serial-barrier assertions; `synctest` was evaluated and left aside.
- **Table-driven refactor of one-off tests.** Several tests (especially in
  `command_test.go` and `checks_test.go`) could be collapsed into tables.
  Deferred — conversion to testify was the main structural change and further
  refactoring was not warranted.
- **Example test functions.** No `Example*` functions were added; not planned.

## 5. How the toolchain is invoked

```sh
make build       # compile; stamps Version/Commit/Date via -ldflags
make test        # go test -race -cover ./...  (coverage prints per-package)
make lint        # gofmt drift check + go vet + golangci-lint
make vuln        # go tool govulncheck ./...
make ci          # build + lint + test  (what CI runs)
```

CI additionally runs `go mod verify` before `make ci` and `make vuln` after.
golangci-lint is installed in CI from the pinned v2.12.2 script; locally it is
invoked by `make lint` only when present on `PATH` (graceful skip otherwise).

## 6. Tagging convention

Plan and commit messages use the repo's model-tier tags:
`[deep]` = architecture/ambiguous · `[exec]` = repo-aware implementation ·
`[fast]` = mechanical/fully-spec'd.
