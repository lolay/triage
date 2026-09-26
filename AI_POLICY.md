# AI Policy

AI-assisted contributions are welcome. triage ships [`AGENTS.md`](./AGENTS.md) so coding agents can orient themselves, and runs an agent state machine ([`.github/AGENT_TRIAGE.md`](./.github/AGENT_TRIAGE.md)) that can triage, plan, and implement issues. The bar for review and merge is the same as for hand-written code: small, focused, in scope, tested, and explained.

This policy is adapted from [`lolay/nowline`'s](https://github.com/lolay/nowline/blob/main/AI_POLICY.md) and applies across the triage estate (`lolay/triage`, `lolay/triage-action`). The general workflow lives in [`CONTRIBUTING.md`](./CONTRIBUTING.md); this file covers AI-specific expectations only.

## Transparency

Disclose AI involvement at two levels.

**On each AI-assisted commit**, add an `Assisted-by:` trailer naming the specific agent and version. It is a standard Git footer (same shape as `Co-Authored-By:`), so it survives squash-merge and stays grep-able in `git log`.

```
fix version parsing for go1.27rc1-style tool output

firstSemverToken stopped at the "rc" suffix and returned "1.27", which
satisfied ">=1.27.1" constraints it should have failed.

Assisted-by: Claude Opus 5.5
```

Use the agent's own product name and version (`Claude Opus 5.5`, `Claude Sonnet 5`, `GPT-5.5`, `Codex CLI`, `Aider`, ...). Multiple trailers are fine.

**In the PR description**, repeat the same `Assisted-by:` line(s) under the `## AI assistance` section of the [PR template](./.github/PULL_REQUEST_TEMPLATE.md). If the PR is entirely hand-written, write `Assisted-by: None`. Agent-opened PRs are checked for this automatically (`copilot-pr-validate.yml`).

This applies to commits and PRs landed after this file was introduced; earlier history is exempt.

## Accountability

- **A human owns every merge.** Agent PRs are opened by the Copilot coding agent, but a maintainer reviews and clicks Approve + Merge; nothing agent-written lands on green CI alone. For human-opened PRs, you own the PR: you reviewed every line and can explain it without re-prompting.
- **You own the tests, edge cases, and scope fit.** The Apache 2.0 contributor grant applies regardless of who drafted the diff.

## Quality

triage has a deliberately narrow scope ([`specs/product.md` § Goals & non-goals](./specs/product.md)): it checks readiness, it does not install toolchains, run tasks, or execute fixes. AI doesn't lower that bar.

- **Match the existing style.** gofmt, table-driven tests with testify, small functions. See [`CONTRIBUTING.md`](./CONTRIBUTING.md).
- **Discuss before drafting these changes** (issue first, agree the shape, then code):
  - new check types or changes to check semantics,
  - config-schema changes (`schema/triage.schema.json`, `specs/config.md`, `man/triage.5` move together),
  - the `--json` output shape or key order (locked by `TestJSON_KeyOrder`),
  - exit codes or the severity model,
  - anything in `specs/`.
- **Golden output is the regression gate.** If your change moves `internal/cli/testdata/*/stdout.golden`, say so in the PR and justify the new baseline.
- **Stay shell-free.** No implicit `/bin/sh` (see `specs/product.md` § Native-Windows design rule).

## What we will close fast

- Scope expansion past the non-goals in `specs/product.md`.
- Golden-fixture rebaselines with no explanation of the intended output change.
- Missing `Assisted-by:` disclosure when AI was clearly involved.
- PRs that ask the reviewer to do the investigation.

## Licensing

By opening a PR you confirm your contribution can be licensed under [Apache 2.0](./LICENSE), whether hand-written or AI-assisted.
