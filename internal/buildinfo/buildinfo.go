// Package buildinfo holds release metadata stamped into the binary at build
// time. The Makefile and goreleaser inject the values via
// -ldflags "-X github.com/lolay/triage/internal/buildinfo.<Var>=...".
package buildinfo

import "fmt"

// Default values used for `go run` and un-stamped `go build` invocations.
// A release build overrides each via -ldflags.
var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// String returns a one-line build summary, e.g.
// "triage v0.1.0 (commit a1b2c3d, built 2026-06-05T00:00:00Z)".
func String() string {
	return fmt.Sprintf("triage %s (commit %s, built %s)", Version, Commit, Date)
}
