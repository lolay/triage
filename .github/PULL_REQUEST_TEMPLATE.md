<!--
Thanks for opening a PR. Quick reminders:

- One logical change per PR.
- Run `make pre-commit` before pushing.
- New check types, config-schema changes, `--json` shape, and exit-code changes are
  discuss-first: open an issue to agree the shape before coding.

See CONTRIBUTING.md for the full workflow.
-->

## Summary

<!-- What does this change do, in one or two sentences? -->

## Motivation

<!-- Why is this change needed? Link the issue. -->

Closes #

## How I tested this

<!--
For example:
- `make pre-commit` passes locally.
- Added a table-driven case in `internal/engine/..._test.go` that fails without this patch.
- Golden output unchanged (or: rebaselined `internal/cli/testdata/<case>/stdout.golden` because ...).
-->

## AI assistance

<!--
Required. Name the specific agent + version, one per line, or "None" if the PR is entirely hand-written.
Each AI-assisted commit also needs an Assisted-by: trailer. See AI_POLICY.md.
-->

Assisted-by: <e.g. Claude Opus 5.5, GPT-5.5, or "None">

## Checklist

- [ ] I ran `make pre-commit` locally.
- [ ] I added or updated tests where the change affects observable behavior.
- [ ] I updated `CHANGELOG.md`, `specs/`, `schema/`, and man pages where the change affects observable behavior.
- [ ] I disclosed any AI assistance above with an `Assisted-by:` line (see [AI_POLICY.md](../AI_POLICY.md)).
