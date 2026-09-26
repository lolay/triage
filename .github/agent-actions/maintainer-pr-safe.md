# maintainer-pr-safe

**Situation.** Agent review judged this PR low-risk and on-plan. The diff is non-empty, the implementation matches the plan, CI is green (or in progress), and no notable concerns were flagged. A brief human review is all that's needed before merging.

**What to do.**

1. Read the agent-review comment for the assessment summary.
2. Skim the diff for anything the agent may have missed.
3. If it looks good: **Approve + Merge** (squash) in the GitHub UI. Because Copilot is the PR author and you are not, Approve is enabled.

**If you find a problem.**

- Add `maintainer-pr-review` with inline comments describing the concern — this signals closer attention is needed and gives the agent or filer a target to address.
- Or close the PR and relabel the linked issue:
  - `originator-input` — to request more info from the filer.
  - `maintainer-only` — to park the issue outside the agent flow.
  - `agent-triage` — to restart from triage with new context.

**Nothing auto-merges.** Every merge is a deliberate maintainer action in the GitHub UI.
