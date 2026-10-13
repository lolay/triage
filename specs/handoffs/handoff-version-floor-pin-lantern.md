# Handoff: version floor and pin (recommended versions)

Branch: `claude/hopeful-dijkstra-5rz2m9` in `lolay/triage`, `GaryRudolph/public`, `lolay/nowline`.
Plan: [`plan-version-floor-pin-lantern.md`](./plan-version-floor-pin-lantern.md).

## Done (m1, triage, pushed)

- `7bfd6db` fix: group/delegate headers graded by failing children only. In 0.4.1 a group with a passing
  required check and a failing `warn` check showed `[✗]` and exited 1, which would have defeated the feature.
- `37864b7` feat: `recommended` / `recommended_from` on `tool` checks, with tests, golden fixture
  `internal/cli/testdata/recommended`, schema, `specs/config.md`, `man/triage.5`, example, CHANGELOG.
- `d74c243` dogfood: `triage.yaml` go check is `version: ">=1.26"` + `recommended_from: .go-version`.

Verified: `go test -race` on every package, golangci-lint v2.14.0 clean, end-to-end with a fake `node`
(21 → `[✗]` exit 1, 22 → `[!]` exit 0, 26.2 → `[✓]`; `--strict` and `--severity` exit 1 on the warning;
`--json` is an ordinary `warn` result). 0.4.1 loads the new keys and enforces the minimum only.

## Done (m2, public, pushed)

- `e814822` Add toolchain floor and pin standard: new "Toolchain Floor and Pin" section in
  `standards/architecture.md`, `doctor` row in `makefile.md` points at it, `core.md` / `SKILL.md` index
  mention it (Cursor rule regenerated), `personal-new-project` step 6 records floor and pin separately.
  `make -C agents validate test` passes (only the existing "no plugin version" warnings).

## CI fix (pushed)

- `.go-version` 1.27.1 -> 1.27.2: `make vuln` flagged 8 stdlib vulns in go1.27.1 (net/http, HTTP/2),
  published after main last went green. Folded into this PR at Gary's call.

## Pending

- m3 (nowline) waits on approval and, ideally, a triage release with `recommended_from`.
- Release of triage is maintainer-only; not started.
- Remove `specs/handoffs/*-lantern.md` before the triage PR merges.

## Gotchas

- `make install-tools` can't download golangci-lint here (GitHub release 403 via proxy); build it with
  `GOBIN=$(go env GOPATH)/bin go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.14.0`.
- `make test` fails in this sandbox with `go: no such tool "covdata"` on test-less packages (auto-downloaded
  toolchain); `go test -race ./internal/...` is the real signal.
- Pre-existing, out of scope: a hint-less failing version check prints its failure text under
  "To fix, run:" (queued as a separate task).
