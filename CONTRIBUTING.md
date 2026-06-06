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
pinned in `.go-version`.

## Making changes

- **Match the surrounding code** — naming, structure, and idiom. Keep functions
  small and readable.
- **Add a check type or flag?** Update the schema and the JSON Schema, add golden
  tests, and add a row to the relevant `examples/` config so the feature is
  exercised. Update [`specs/product.md`](./specs/product.md) in the same PR.
- **New behavior needs tests.** Prefer table-driven unit tests and golden output
  fixtures over ad-hoc assertions.
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
