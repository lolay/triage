---
description: "Phase 3 (fast model) of the triage-estate agent state machine. Triggered by agent-exec label. Same shape as agent-deep but uses Sonnet and enforces an exec-grade-work check (bounded file count, short approach, no protected files). Delegates to a Copilot coding-agent session that writes the code and opens a PR."
on:
  issues:
    types: [labeled]
if: github.event.label.name == 'agent-exec'
engine:
  id: copilot
  model: claude-sonnet-5
imports:
  - shared/agentic-prelude.md
safe-outputs:
  assign-to-agent:
    model: claude-sonnet-5
  add-comment:
    max: 1
permissions:
  issues: read
  pull-requests: none
  contents: read
timeout-minutes: 15
---

{{#runtime-import .github/agent-prompts/agent-exec.md}}
