# originator-input

**Situation.** The agent reached a point where it needs information only you (the issue filer) have. It posted a `## What I need from you` comment explaining what's missing — a repro step, a design preference, or a constraint it couldn't infer from the issue body.

**What to do.**

1. Read the agent's comment for the specific questions.
2. Reply in a comment with the requested information.
3. Remove the `originator-input` label and add `agent-plan` to resume. The plan workflow re-fires automatically; the cleanup workflow removes `originator-input` when you add `agent-plan`.

**If it's not worth pursuing.** Add `maintainer-only` to park the issue outside the agent flow, or close it.

**What to emit.** `agent-plan` (to resume) — or `maintainer-only` (to park) — or close the issue.
