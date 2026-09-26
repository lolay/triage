# maintainer-only

**Situation.** This issue has been parked outside the agent flow. No agent workflow will fire on it. A maintainer classified it as outside the automated flow — either intentionally or as a temporary pause.

**What to do.** Handle it manually, close it, or re-admit it to the agent flow whenever the time is right.

**To re-admit.** Add `agent-triage`. The cleanup workflow removes `maintainer-only` automatically, and triage re-fires.

**What to emit.** Nothing required. Add `agent-triage` only if you want the agent flow to resume from the beginning.
