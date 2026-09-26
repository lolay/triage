---
description: "Phase 1 of the triage-estate agent state machine. Reads an issue labeled agent-triage and emits exactly one label: agent-plan (proceed to planning) or maintainer-only (stop). Judgment-only — cannot open PRs or modify code."
on:
  issues:
    types: [labeled]
  skip-bots:
    - "renovate[bot]"
    - "dependabot[bot]"
if: github.event.label.name == 'agent-triage'
engine:
  id: copilot
  model: claude-sonnet-5
imports:
  - shared/agentic-prelude.md
safe-outputs:
  add-comment:
    max: 1
permissions:
  issues: read
  pull-requests: none
  contents: read
timeout-minutes: 10
---

{{#runtime-import .github/agent-prompts/agent-triage.md}}
