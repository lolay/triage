# maintainer-decide

**Situation.** The agent hit a fork with multiple valid options and cannot pick one on its own — either because the options have significant trade-offs, or because a hard rule in the design spec blocks autonomous routing. It posted a numbered list of options with trade-offs and a recommendation.

**What to do.**

1. Read the agent's comment for the options and recommendation.
2. Pick a routing label based on the complexity and risk of the work:
   - `agent-deep` — Opus model; use for genuinely complex, high-risk, or architecturally significant changes.
   - `agent-exec` — Sonnet model; use for mechanical, bounded, or straightforward changes.
3. Add the chosen label. The cleanup workflow removes `maintainer-decide` automatically, and the matching implement workflow fires.

**Going offline.** Add `maintainer-only` to park the issue outside the agent flow.

**What to emit.** `agent-deep`, `agent-exec`, or `maintainer-only`.
