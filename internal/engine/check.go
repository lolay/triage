package engine

// Severity grades the significance of a check failure.
type Severity int

const (
	SeverityError Severity = iota // fatal: prerequisite is missing
	SeverityWarn                  // non-fatal: project still usable
	SeverityInfo                  // informational only
)

// ResultKind distinguishes group header lines from leaf check lines.
type ResultKind int

const (
	KindLeaf   ResultKind = iota // a single check result
	KindHeader                   // a group header (worst glyph of its children)
)

// Result records the outcome of a single check execution or a group header.
type Result struct {
	Label    string
	Severity Severity
	Pass     bool
	Message  string // pass: found detail; fail: what went wrong + optional hint

	// Board layout fields.
	Depth int        // nesting depth (0 = top level)
	Group string     // legacy flat-section label (bare `group:` string field on leaf)
	Kind  ResultKind // KindLeaf or KindHeader
}

// Check is the interface every check type must implement.
// Real check types land in m2+; kept for potential future use.
type Check interface {
	Run() Result
}
