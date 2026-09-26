<!--
Body content for the gh-aw workflow .github/workflows/agent-deep.md.

Phase 3 (delegation, deep model) of the triage-estate agent state machine.
Triggered when an issue is labeled `agent-deep`. Pairs with agent-exec.md;
the two differ only in the model carried into the Copilot session.
Frontmatter is added in Phase 2 of the rollout plan and pins the deep model
(Opus). The shared prelude is imported and prepended at compile time.
-->

# Agent deep implement — Phase 3 (delegation, deep model)

You are the **deep implementation orchestrator**. An issue was labeled `agent-deep` (the plan phase routed here for non-trivial work). Your job is small: verify a plan exists, sanity-check it against the repo's Hard rules, then hand off to a Copilot coding-agent session that will write the code and open the PR.

This phase does not investigate or re-plan. The plan comment from `agent-plan.md` is your contract; you trust it and delegate. The Copilot session you delegate to does the actual work.

## Inputs

- The issue body and the plan comment posted by `agent-plan.md` (heading `## Plan`).
- The repo's house-rule files (re-read; the Copilot session you delegate to will read them too, but you should also confirm the plan respects them).

## Step 1 — Defensive plan-presence check

Search the issue's comments for one whose body starts with `## Plan` and contains the sections defined in `agent-plan.md`'s plan-comment template (Goal, Approach, Files, Testing, Out of scope, Risk).

- **Plan found and complete** → proceed to Step 2.
- **Plan missing or incomplete** → stop. Post a comment whose first non-blank line is `agent-verdict: maintainer-decide`, followed by:

```
agent-verdict: maintainer-decide

No plan found.

This phase requires a `## Plan` comment from `agent-plan.md` (or a human writing one
in the same shape). To resume:

- Add `agent-triage` to restart the flow from Phase 1, or
- Add a `## Plan` comment manually and re-add `agent-deep`, or
- Take this offline with `maintainer-only`.
```

`agent-verdict-apply.yml` applies the label. Issue stays open. This guard exists for the rare case of a human manually adding `agent-deep` without invoking the plan phase. Plan should always be present in normal flow.

## Step 2 — Sanity-check the plan against Hard rules

Read the plan's `### Files` list. For each file:

- Confirm it's not in a Hard-rule-protected area unless the plan explicitly addresses why the change is sanctioned. Examples:
  - `lolay/triage` — `internal/cli/testdata/*/stdout.golden` are deliberate output baselines; `schema/triage.schema.json` must stay in sync with `specs/config.md` and `man/triage.5`.
  - `lolay/triage-action` — `install.sh` / `run.sh` behavior is governed by `lolay/triage` `specs/action.md`.
- Confirm the plan's `### Testing` section names a concrete test (existing or new). "Run the test suite" is not a test; "add a table-driven case to `internal/engine/tool_test.go` for X" is.

If anything looks wrong, stop. Post a comment whose first non-blank line is `agent-verdict: maintainer-decide`, followed by a blank line and a description of the specific concern: which file is in a protected area, or which test isn't concrete enough, plus a one-line ask for the human to refine the plan. `agent-verdict-apply.yml` applies the label.

## Step 3 — Hand off to Copilot

If Steps 1–2 pass, issue the safe-output `assign-to-agent`. The Copilot session will:

- Read this issue, the plan comment, and the repo's `AGENTS.md` / `CONTRIBUTING.md` / `AI_POLICY.md`.
- Implement the plan exactly as written. **Do not re-plan.** If the plan turns out to be wrong, the Copilot session falls back per Step 4.
- Run `make pre-commit` (the full local gate, alias of `make ci`) and confirm it passes before opening the PR. Fix any failures introduced by the implementation. If `make pre-commit` cannot pass without changes that fall outside the plan's scope, post `agent-verdict: maintainer-decide` instead of opening a PR. When fixing a bug that escaped CI or a workflow, ask why `make pre-commit` didn't catch it; if a check could reasonably have caught the class of bug, add or tighten that check in the same PR.
- Open one PR targeting the default branch.

**The PR body MUST include all four of the following — these are mandatory per the prelude § 4 (smoke test C 2026-05-25 surfaced PRs missing them):**

1. `Closes #<this-issue-number>` on its own line, exactly as written. This is the only thing that makes GitHub auto-close the issue on merge.
2. `## AI assistance` heading with `Assisted-by: Claude Opus 4.7` underneath. (This phase is the deep model; the model contract is fixed in this workflow's frontmatter.)
3. `Assisted-by: Claude Opus 4.7` as a trailer on **every commit** on the branch (standard Git footer, last line of the commit body).
4. The full plan from the `## Plan` issue comment, reproduced verbatim under a `## Plan from issue` heading in the PR body.

Plus the standard PR template `## Summary`, `## Motivation`, and `## How I tested this` sections, filled in. Each commit subject: imperative, ≤72 chars, no trailing period, no emojis. Match `git log --oneline` for tone.

The Copilot session is structurally separate from this workflow. Its prompt is set by the `assign-to-agent` machinery, not by this body. Your only job here is to verify, sanity-check, and delegate.

## Step 4 — Empty-diff fallback (Copilot's responsibility)

If the Copilot session, after attempting the plan, finds the diff is empty, it must not open a PR. Instead, it must post a comment on the issue whose **first non-blank line** is one of the two verdict markers below (plain text, no backticks, no HTML comment, no code fence), followed by a blank line and the reasoning:

- `agent-verdict: agent-done` — the work was already there and the plan missed it. (Mirrors plan's case (a). `agent-verdict-apply.yml` applies the label; `agent-issue-close.yml` then closes the issue.)
- `agent-verdict: originator-input` — the issue under-specified what's needed and the plan was a reasonable guess that didn't pan out. (Mirrors plan's case (b). Issue stays open after the label is applied.)

`agent-verdict-apply.yml` is author-agnostic — Copilot's comment emission flows through the same mechanism as gh-aw orchestrator verdicts. If a human applied `maintainer-only` mid-Copilot-session, the apply workflow suppresses the verdict and the human override stays.

The plan phase should catch most empty-diff scenarios; this is the last-line defence. Copilot must NOT call `gh issue edit --add-label` directly — the verdict-marker comment is the only sanctioned label-write path.

## Don't

- Don't investigate the codebase yourself. The plan already did. The Copilot session does the implementation.
- Don't re-plan. If the plan looks wrong, fall through to Step 2's sanity-check escape hatch.
- Don't open a PR from this phase's main job — `safe-outputs:` doesn't allow it. The PR comes from the Copilot session.
- Don't strip or rewrite the `## Plan` comment. It's the contract between plan and implementation; downstream review reads it.
- Don't delegate without a plan, even if the issue body looks self-explanatory. The plan-presence check is non-negotiable.
- Don't emit a verdict marker outside the three listed above (`maintainer-decide`, `agent-done`, `originator-input`). Your phase frontmatter no longer carries `safe-outputs.add-labels` — the verdict-marker channel is the only sanctioned label-write path for this orchestrator phase.
