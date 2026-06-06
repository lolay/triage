package engine

import "github.com/lolay/triage/internal/config"

// Runner executes the checks in a profile in declaration order.
// In m1 all profiles are empty, so Run always returns nil.
type Runner struct{}

// NewRunner creates a Runner.
func NewRunner() *Runner { return &Runner{} }

// Run executes each check in profile and returns ordered results.
// In m1 this always returns a nil slice; real execution lands in m2.
func (r *Runner) Run(_ config.Profile) []Result {
	return nil
}
