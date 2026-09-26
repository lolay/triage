---
description: "Phase 4 of the triage-estate agent state machine. Triggered when a PR carrying the copilot-pr metadata label is opened or updated. The copilot-pr label is stamped by copilot-pr-stamp.yml (the single chokepoint that checks bot identity). bots: is retained as defense-in-depth for gh-aw's pre-activation gate. Reads the diff, plan comment, CI status, and repo house rules, then emits exactly one PR label: maintainer-pr-safe (confident — a maintainer merges) or maintainer-pr-review (needs human review). Judgment-only — cannot modify the PR diff."
on:
  pull_request:
    types: [opened, synchronize, ready_for_review]
  bots:
    - "copilot-swe-agent[bot]"
    - "Copilot"
if: contains(github.event.pull_request.labels.*.name, 'copilot-pr')
engine:
  id: copilot
  model: claude-sonnet-4.5
imports:
  - shared/agentic-prelude.md
safe-outputs:
  add-comment:
    max: 1
permissions:
  issues: read
  pull-requests: read
  contents: read
  statuses: read
  checks: read
timeout-minutes: 15
---

{{#runtime-import .github/agent-prompts/agent-review.md}}
