# maintainer-pr-review

**Situation.** Agent review flagged one or more concerns with this PR — see the agent-review comment for the specific list. The concerns may include: diff exceeds plan scope, suspicious patterns, failing or missing CI checks, or a zero-diff (validation failure). The PR is not safe to merge without a careful human review.

**What to do.**

1. Read the agent-review comment for the flagged concerns.
2. Review the diff against those concerns.
3. If the concerns are resolved or acceptable: **Approve + Merge** (squash) in the GitHub UI.
4. If the concerns are not resolved:
   - Request changes with inline comments so the agent or filer can address them.
   - Or close the PR if the direction is wrong.
5. If you close the PR, relabel the linked issue to indicate the next step:
   - `originator-input` — to bounce back to the filer for more info.
   - `maintainer-only` — to park the issue outside the agent flow.
   - `agent-triage` — to restart from triage with new context.

**Nothing auto-merges.** Every merge is a deliberate maintainer action in the GitHub UI.
