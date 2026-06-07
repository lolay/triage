# Handoff — concurrency / bounded worker pool

> **Audience:** the agent that will turn the concurrency design into a concrete
> implementation plan (steps, ordering, tests) for `triage`.
> **Status:** design decided & folded into `specs/product.md` (§4, §5, §7.2, §7.3,
> [milestones.md](../milestones.md) m2 s3 + m4 s4, §12). This handoff is the planning brief — **don't
> re-litigate the decision**; plan the build.

## 1. One-paragraph context

`triage` is a read-only, config-driven environment checker (`triage.yaml`) — a
single Go binary replacing copy-pasted `make doctor` shell scripts. Checks are
declarative types (`tool`/`env`/`path`/`one_of`/`command`/`delegate`/`group`),
selected by **profile**, reported with a **severity** model through a grouped
board. Full product spec: `specs/product.md`. Today the spec runs checks
strictly sequentially; this work adds **automatic, bounded concurrency** without
changing any observable output.

## 2. The decision (locked)

- **Automatic, bounded concurrency** via a worker pool. **No `async:` /
  `parallel:` YAML knob** — parallelism is the default.
- **Why it's safe by construction:** every check is **read-only** (§2, §7), so
  there are *no inter-check data dependencies*. Order affects presentation only,
  not correctness. (Contrast a task runner — explicitly a non-goal — where
  `build` must precede `test`. triage has none of that.)
- **Core invariant — "execute concurrently, materialize in list order."**
  Concurrency is a scheduling detail; it must never change a byte of output. A
  check that finishes early does **not** render early. Both the **board line**
  and the **`--command-log` block** for a check are materialized in config
  **list order** (finalize-in-order). Output is therefore independent of
  `--jobs` and of subprocess timing → golden fixtures stay byte-stable.
- **`--jobs <n>` / `-j`** controls pool size. Default ≈ `runtime.NumCPU()`
  (capped). `--jobs 1` = fully sequential.
- **`serial: true`** — optional opt-*out* field on any check or `group`. Pins
  that check (or a group's direct children) to run sequentially. Reach for it
  only on **shared scarce resources** (exclusive lock, rate-limited/flaky auth
  endpoint, keychain) or a `command` escape hatch with a side effect another
  check observes. Default everywhere else is parallel.
- **Delegates** are the natural coarse-grained unit and the highest-value place
  to parallelize (each is a whole child board, often `make doctor`). Sibling
  delegates run concurrently with each other and with local checks; each child
  config runs its own pool internally.
- **`group` stays display-only.** Grouping is a board concept, **not** a
  scheduling boundary — putting checks in a group must not silently change how
  they schedule. (Authors get `serial:` on the group if they want serialization.)

## 3. The two hard parts (plan these carefully)

### 3a. Renderer (§7.2)
Today's "pending `[…]` → back-update in place" model assumes **one active line
at a time**. With N workers, many checks are in flight at once.
- **Chosen approach (v1): finalize-in-order.** Run concurrently, but reveal /
  finalize lines top-down in list order — a check that finishes early waits its
  turn to render. Keeps essentially today's renderer + golden output; you lose
  "multiple spinners at once."
- **Deferred polish:** a multi-line live region (several `[…]` active
  simultaneously). Nicer for big delegate trees; more renderer work. Not v1.
- **Readiness:** m2 s3 builds the renderer **finalize-in-order from day one**
  (materialize in list order, execution-order-independent) so enabling the pool
  at m4 needs no renderer rewrite. Verify the plan honors this.

### 3b. `--command-log` under concurrency (§5)
§5's `--command-log` is a stream-through tee where "each probe writes one block"
— a **single-writer** assumption. Naive shared-file writes from N workers
interleave line-by-line → garbled log.
- **Chosen approach: per-probe spool → ordered concatenation.** Each running
  probe streams its combined stdout+stderr to its **own** spool file as bytes
  arrive (no shared-file interleaving, **no in-memory buffering** — preserves the
  §5 "stream-through, not load-then-write" memory guarantee; disk-bound so no
  byte cap needed). When a probe finalizes, append its block to the real log
  **in list order** (early finishers wait), then discard its spool.
- On-disk **format unchanged** (one coherent `label`/`cwd`/`run:`/`---`/output
  block per probe); log still **truncated once at run start**.
- Precedent: GNU `make -j --output-sync=target`.
- `--verbose` stderr replay follows the same list-order, one-block discipline.
- Discard default (no flag): zero contention — each worker streams to
  `/dev/null` independently.
- Author redirect escape hatch (`noisy-tool > .triage/x.log`): author owns
  coherence if two parallel probes target the same path — one documented
  sentence, not triage's problem.

## 4. Spec sections already updated (read these)

- **§4** — "list order" reworded as **render order**; execution may be concurrent.
- **§5** — `serial` added to the optional-field reference; new "Under
  concurrency" command-log bullet (per-probe spool → ordered concat); `--verbose`
  + `delegate` wording updated for concurrency.
- **§7.2** — new **"Concurrency (bounded worker pool)"** subsection; in-flight
  progress reworded to "rendered in list order."
- **§7.3** — `--jobs <n>` / `-j` flag row added.
- **[milestones.md](../milestones.md)** — m2 s3 notes finalize-in-order renderer; **new m4 s4** = the worker
  pool deliverable.
- **§12** — locked "Concurrency" decision bullet.

## 5. Suggested milestone placement (confirm or adjust)

- **m2 s3:** renderer built finalize-in-order (no live pool yet — execution still
  effectively serial, but the *rendering contract* is in place).
- **m3 s2:** `--command-log` written to the per-probe-spool model so it's correct
  once the pool turns on (even with one worker it's a degenerate case).
- **m4 s4 [deep]:** turn on the bounded worker pool — `--jobs`/`-j`, `serial:`
  handling, concurrent local checks + sibling delegates, `--json`
  `status: running` while in flight. This is where parallelism pays off
  (delegate-heavy workspaces dominate runtime).
- **Open call to confirm with the owner:** ship the live pool in **v1** vs defer
  to **m4** as above. Recommendation = m4 (the contract is designed in earlier,
  so the speedup can land without churn when delegates exist to parallelize).

## 6. Invariants the plan must preserve (acceptance criteria)

1. **Output is `--jobs`-invariant.** Same board, same `--command-log` bytes, same
   `--json` for any `-j` value and any subprocess timing. (Golden tests must run
   at `-j 1` *and* `-j N` and match — add this to the golden harness.)
2. **Read-only guarantee intact.** No check mutates state (unchanged from today).
3. **No implicit shell** (§3 native-Windows rule) — the worker pool spawns the
   same way regardless of OS.
4. **Memory safety** — no unbounded in-RAM buffering of subprocess output under
   concurrency (per-probe spool to disk; assertion scan stays the §5 256 KiB
   bounded prefix, per worker).
5. **Bounded fan-out** — never spawn more than `--jobs` subprocesses at once,
   including across nested delegate pools (decide: shared global semaphore vs
   per-level pools — see open questions).
6. **Deterministic exit codes** — concurrency must not affect the
   `0`/`1`/`2`/`3` model (§7.3); aggregate after all checks complete.
7. **Cancellation / fail-fast** — define behavior on Ctrl-C and on first error
   (see open questions); in-flight subprocesses must be terminated cleanly.

## 7. Open questions for the planning agent to resolve (and propose answers)

- **Global semaphore vs per-pool limits for nested delegates.** A single global
  worker budget (recommended — true cap on total subprocesses) vs each delegate
  spawning its own pool (simpler, can over-subscribe). Pick one and justify.
- **Default `--jobs` value & cap.** `NumCPU`? `NumCPU` capped at e.g. 8? These
  are mostly I/O-bound subprocess waits, so a higher cap than CPU count may be
  fine — propose a value and reasoning.
- **Fail-fast vs run-to-completion.** triage is a doctor — likely **run
  everything** so the board is complete (don't abort siblings on first error).
  Confirm; define `--strict` interaction.
- **`serial:` semantics precisely.** Does `serial:` on check X mean "X runs alone
  (a barrier — drain in-flight, run X, resume)" or just "X is not itself eligible
  for the pool / runs on the main path in list position"? Recommend the
  **barrier** reading for the scarce-resource use case; spec the exact ordering.
- **`serial:` on a `group`** — serialize only direct children, or the whole
  subtree (incl. nested groups/delegates)? Recommend whole subtree; confirm.
- **`--json` `status: running`** emission cadence under concurrency (already
  allowed by §7.2/§7.3) — define when running entries are flushed for non-TTY.
- **Cancellation** — propagate `context.Context` to every subprocess; on signal,
  cancel and mark unfinished checks. Spec the exit code for interrupted runs.

## 8. Tagging convention (for the plan you produce)

Use the repo's model-tier tags on each step:
`[deep]` = architecture/ambiguous · `[exec]` = repo-aware implementation ·
`[fast]` = mechanical/fully-spec'd. (Matches [milestones.md](../milestones.md).)
