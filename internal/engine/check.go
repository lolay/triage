package engine

// Severity grades the significance of a check failure.
type Severity int

const (
	// SeverityError marks a fatal prerequisite failure.
	SeverityError Severity = iota
	// SeverityWarn marks a non-fatal issue; the project may still be usable.
	SeverityWarn
	// SeverityInfo is informational only.
	SeverityInfo
)

// ResultKind distinguishes group header lines from leaf check lines.
type ResultKind int

const (
	// KindLeaf is a single check result line.
	KindLeaf ResultKind = iota
	// KindHeader is a group header carrying the worst-status glyph of its children.
	KindHeader
	// KindDelegate is a delegate summary carrying the worst glyph of a child subtree.
	KindDelegate
)

// Result records the outcome of a single check execution or a group header.
//
// Fields are ordered for struct alignment, not by topic. Notable ones:
//   - Message: pass = found detail; fail = what went wrong + optional hint.
//   - Output: bounded excerpt of the subprocess combined output (capped at
//     captureCap bytes), set on failure for --verbose replay and --json detail
//     enrichment. Empty for instant checks (env, path, tool presence).
//   - Depth: nesting depth (0 = top level). Kind: leaf/header/delegate.
//   - cmdLog: data needed to materialize a --command-log block. Stashed during
//     (possibly concurrent) execution and written by RunContext in list order,
//     so the log bytes are identical regardless of --jobs. nil when no log is active.
type Result struct {
	cmdLog   *cmdLogEntry
	Label    string
	Message  string
	Output   string
	Severity Severity
	Depth    int
	Kind     ResultKind
	Pass     bool
}

// cmdLogEntry is one deferred --command-log block, written in list order.
type cmdLogEntry struct {
	label  string
	cwd    string
	run    string
	output []byte
}
