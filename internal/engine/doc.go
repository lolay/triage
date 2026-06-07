// Package engine defines the check execution model and runner.
//
// m3 completes the full check-type surface (spec §5):
//   - env: presence check (optional matches regex; unset mode).
//   - path: glob + ~ expansion, resolved against opts.BaseDir / c.Dir.
//   - one_of: evaluates alternatives silently; emits one atomic leaf result.
//   - command: explicit interpreter (-c), unified {{ name }} template expansion
//     (vars: + built-ins profile/os), optional with_env map (layered over inherited
//     env), bounded stdout capture (captureCap = 256 KiB), exit / contains /
//     matches assertions, and optional CommandLog streaming.
//   - platform: guards on any check/group; non-matching checks are omitted
//     entirely (no board line, no summary count).
//
// m4 adds delegation and concurrency:
//   - delegate: loads a child triage.yaml and recurses, inheriting the parent's
//     CLI vars but using the child's own config vars and BaseDir, with
//     absolute-path cycle detection (a cycle is a fatal config error, exit 3).
//   - Bounded worker pool: siblings run concurrently while results — and any
//     --command-log blocks — are materialized in config list order, so output
//     is byte-identical for any --jobs (execute concurrently, materialize in
//     list order; §7.2). Jobs == 1 uses a dedicated sequential path (the
//     reference oracle); Jobs > 1 schedules siblings into per-index slots and
//     bounds concurrent subprocess spawns with a global semaphore shared across
//     the whole delegate tree. Only subprocess-spawning leaves (tool version
//     probe, command) acquire a token; aggregators never hold one while waiting.
//   - serial: true on a check is a barrier in its sibling list plus a global
//     serial lock so serial checks never overlap across subtrees; on a group it
//     serializes the whole subtree.
//   - Cancellation: RunContext stops scheduling and returns the partial board
//     when ctx is cancelled (CLI maps SIGINT/SIGTERM to exit 130).
//
// RunnerOpts is extended with GOOS (injectable OS, darwin→macos normalised),
// LookupEnv, BaseDir, RunCommand, CommandLog, Profile, Vars, and Jobs. Template
// expansion merges Vars with built-ins profile and os at runtime.
//
// CurrentPlatform() normalises runtime.GOOS; the env var TRIAGE_TEST_GOOS
// overrides it in tests so golden fixtures can pin a specific OS.
//
// m2 highlights still present: tool (LookPath + semver probe), structural
// group containers, injectable LookPath/RunProbe, severity resolution, and
// the go/node probe override table.
//
// Full async TTY pending/back-update streaming is still deferred; the static
// board already renders the nested delegate tree.
package engine
