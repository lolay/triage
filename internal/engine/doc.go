// Package engine defines the check execution model and runner.
//
// m2 delivers the first real execution pipeline (spec §5–§7.2):
//   - Result is extended with Depth, Group, and Kind (KindLeaf / KindHeader)
//     so the board renderer can emit structural group headers with their
//     worst-status glyph.
//   - Runner walks the active profile recursively: it executes `tool` checks
//     (LookPath + optional version probe + semver matching via
//     Masterminds/semver/v3), recurses into structural `group` containers, and
//     emits a non-failing SeverityInfo skip for all other types
//     (env/path/one_of/command/delegate) until their milestone (m3/m5).
//   - Runner is injectable (RunnerOpts: LookPath, RunProbe) so unit tests run
//     without relying on the host environment.
//   - Severity is resolved from each Check's Severity string ("error"/"warn"/
//     "info") or the `required: true|false` sugar (spec §7.1); `--strict`
//     escalation lives in the CLI layer (internal/cli/exit.go).
//   - The `go version` probe override (parse `go1.26.3` from stdout) and a
//     node leading-v normaliser are included in the override table.
//
// Delegate nesting (m5), command execution (m3), and full TTY pending/back-update
// (async runner, deferred) are out of scope for m2.
package engine
