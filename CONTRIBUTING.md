# Contributing to triage

Thanks for your interest in improving triage. This guide covers setup, the
build/test loop, and the workflow we expect for pull requests.

## Ground rules

- Be respectful — all participation is governed by [`CODE_OF_CONDUCT.md`](./CODE_OF_CONDUCT.md).
- Read the design first. triage is spec-first; [`specs/product.md`](./specs/product.md)
  is the source of truth for the config schema, check types, severity model, and
  milestones. Open an issue before large changes so we can align on the spec.
- Security issues do **not** go in public issues — see [`SECURITY.md`](./SECURITY.md).

## Development setup

triage is a single Go binary. The root `Makefile` is the source of truth — humans,
agents, and CI all run the same `make <target>` verbs (`make help` lists them).

```sh
git clone https://github.com/lolay/triage.git
cd triage
make build        # compile the binary
make test         # unit + golden-output tests
make ci           # full pre-push gate (what CI runs)
make doctor       # dogfood: triage checks its own environment (see triage.yaml)
```

You need the toolchain triage checks for itself — run `make doctor` (or
`triage` once built) and install whatever it flags. The Go version is
pinned in `.go-version`; `make install-tools` installs golangci-lint and
actionlint at the Makefile's pins.

### AI assistance and the agent flow

AI-assisted contributions are welcome under [`AI_POLICY.md`](./AI_POLICY.md)
(`Assisted-by:` on every AI-assisted commit and in the PR). Agents start from
[`AGENTS.md`](./AGENTS.md). Issues can opt into an agent first pass (a checkbox
in the issue templates); the triage → plan → implement → review state machine,
its labels, and how to override it are documented in
[`.github/AGENT_TRIAGE.md`](./.github/AGENT_TRIAGE.md). Every agent PR still
needs a maintainer's Approve + Merge.

### Dependency updates

Tool versions live in exactly one place each: Go in `.go-version`,
golangci-lint, actionlint, goreleaser, and the gh-aw compiler in the `Makefile`
(CI reads them from there).
[Renovate](./renovate.json) keeps them — plus Go modules and GitHub Actions —
current via the shared preset in
[`.github/renovate-shared.json`](./.github/renovate-shared.json), which
`lolay/triage-action` also extends: one grouped minor/patch PR a week
(auto-merge once CI is green), majors individually after a 30-day cooldown, and
a separate hand-reviewed PR for a new Go minor or a gh-aw compiler bump. The
`go` directive in `go.mod` is the from-source consumer floor and is deliberately
excluded — raise it by hand as policy work. Renovate PRs are assigned to the
Copilot coding agent, and when CI fails on one, `renovate-autofix.yml` turns off
its auto-merge and asks Copilot to fix it; a maintainer then reviews and merges.

## Making changes

- **Match the surrounding code** — naming, structure, and idiom. Keep functions
  small and readable.
- **Add a check type or flag?** Update the schema and the JSON Schema, add golden
  tests, and add a row to the relevant `examples/` config so the feature is
  exercised. Update [`specs/product.md`](./specs/product.md) in the same PR.
- **New behavior needs tests.** Write table-driven unit tests that assert with
  [testify](https://github.com/stretchr/testify) — `require` for fatal
  preconditions (e.g. before indexing a slice), `assert` for independent checks —
  and keep golden output fixtures for end-to-end stdout. Prefer these over
  hand-rolled `if got != want { t.Errorf(...) }` assertions.
- **Keep it cross-platform.** No implicit shell — structured checks spawn tools
  directly via `exec.LookPath`/`exec.Command`; `command` checks name their
  interpreter and are platform-guarded (see spec §3, §5).
- Run `make lint` and `make test` before pushing.

## Commit and PR conventions

- Commits: imperative mood, ≤72-char subject, no trailing period
  (`add one_of check type`).
- One feature or fix per PR; squash-and-merge is preferred.
- Describe the change clearly in the PR. CI must be green **and** a maintainer
  must approve before merge — nothing lands on green CI alone.

## Licensing of contributions

By contributing, you agree that your contributions will be licensed under the
project's [Apache 2.0 license](./LICENSE). You retain copyright on your
contributions; Apache 2.0 grants the project and its users the necessary rights.
