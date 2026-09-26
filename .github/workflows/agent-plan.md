---
description: "Phase 2 of the triage-estate agent state machine. Investigates the codebase for an issue labeled agent-plan and picks exactly one of five terminals: agent-deep, agent-exec, agent-done (resolved without action), originator-input (cannot reproduce), or maintainer-decide (multi-option / hard-rule blocks). Posts a ## Plan comment on the route-to-implement paths. Cannot open PRs."
on:
  issues:
    types: [labeled]
if: github.event.label.name == 'agent-plan'
engine:
  id: copilot
  model: claude-opus-5.5
imports:
  - shared/agentic-prelude.md
safe-outputs:
  add-comment:
    max: 1
permissions:
  issues: read
  pull-requests: none
  contents: read
timeout-minutes: 30
---

{{#runtime-import .github/agent-prompts/agent-plan.md}}
