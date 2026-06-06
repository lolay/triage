// Package engine defines the check execution model and runner.
//
// m3 completes the full check-type surface (spec §5):
//   - env: presence check (optional matches regex; unset mode).
//   - path: glob + ~ expansion, resolved against opts.BaseDir / c.Dir.
//   - one_of: evaluates alternatives silently; emits one atomic leaf result.
//   - command: explicit interpreter (-c), {{profile}} expansion, bounded
//     stdout capture (captureCap = 256 KiB), exit / contains / matches
//     assertions, and optional CommandLog streaming.
//   - platform: guards on any check/group; non-matching checks are omitted
//     entirely (no board line, no summary count).
//   - delegate: deferred to m5; still emits a non-failing SeverityInfo skip.
//
// RunnerOpts is extended with GOOS (injectable OS, darwin→macos normalised),
// LookupEnv, BaseDir, RunCommand, CommandLog, and Profile. All injections
// default to real OS behaviour so the zero value is always usable in production.
//
// CurrentPlatform() normalises runtime.GOOS; the env var TRIAGE_TEST_GOOS
// overrides it in tests so golden fixtures can pin a specific OS.
//
// m2 highlights still present: tool (LookPath + semver probe), structural
// group containers, injectable LookPath/RunProbe, severity resolution, and
// the go/node probe override table.
//
// Full TTY pending/back-update and delegate-tree execution (m5) are deferred.
package engine
