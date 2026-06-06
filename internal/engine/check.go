package engine

// Severity grades the significance of a check failure.
type Severity int

const (
	SeverityError Severity = iota // fatal: prerequisite is missing
	SeverityWarn                  // non-fatal: project still usable
	SeverityInfo                  // informational only
)

// Result records the outcome of a single check execution.
type Result struct {
	Label    string
	Severity Severity
	Pass     bool
	Message  string // hint or fix command shown on failure
}

// Check is the interface every check type must implement.
// Real check types (tool, command, env, …) land in m2.
type Check interface {
	Run() Result
}
