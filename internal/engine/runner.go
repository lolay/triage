package engine

import (
	"context"
	"fmt"

	"github.com/lolay/triage/internal/config"
)

// Runner executes the checks in a profile in declaration order.
//
// m2 executes `tool` (LookPath + version probe + semver) and recurses into
// `group` containers; all other check types (env/path/one_of/command/delegate)
// emit a non-failing SeverityInfo skip until their milestone (m3/m5).
type Runner struct {
	opts RunnerOpts
}

// NewRunner creates a Runner with default (real) LookPath and probe functions.
func NewRunner() *Runner { return &Runner{} }

// NewRunnerWith creates a Runner with injected functions (for tests).
func NewRunnerWith(opts RunnerOpts) *Runner { return &Runner{opts: opts} }

// Run executes each check in profile and returns an ordered, depth-annotated
// Result slice. Group headers appear immediately before their children and carry
// the worst-status glyph of the group.
func (r *Runner) Run(profile config.Profile) []Result {
	return r.runChecks(context.Background(), profile, 0)
}

func (r *Runner) runChecks(ctx context.Context, checks config.Profile, depth int) []Result {
	var out []Result
	for _, c := range checks {
		out = append(out, r.runCheck(ctx, c, depth)...)
	}
	return out
}

func (r *Runner) runCheck(ctx context.Context, c config.Check, depth int) []Result {
	sev := resolveSeverity(c)

	switch c.Type {
	case config.TypeTool:
		res := checkTool(ctx, c.Value, c.Constraint, c.Hint, r.opts)
		res.Severity = sev
		res.Depth = depth
		res.Group = c.Group
		res.Kind = KindLeaf
		// Build a friendlier label that includes version detail on pass.
		if res.Pass && c.Constraint != "" {
			res.Label = fmt.Sprintf("%s %s", c.Value, res.Message)
			// Message carries "name version (constraint)" already; use it as label
			res.Label = res.Message
		} else {
			res.Label = c.Value
		}
		return []Result{res}

	case config.TypeGroup:
		return r.runGroup(ctx, c, depth)

	default:
		// All other types (env/path/one_of/command/delegate) are not yet
		// implemented. Emit a non-failing info skip (spec: "parse and render as
		// a non-failing `info` skip until their milestone").
		label := c.Value
		if c.Label != "" {
			label = c.Label
		}
		if label == "" {
			label = c.Type
		}
		return []Result{{
			Label:    label,
			Severity: SeverityInfo,
			Pass:     true,
			Message:  fmt.Sprintf("%s (not yet implemented — deferred to m3/m5)", c.Type),
			Depth:    depth,
			Group:    c.Group,
			Kind:     KindLeaf,
		}}
	}
}

// runGroup executes a structural `group` container: run all children, then
// prepend a header Result carrying the group's worst glyph.
func (r *Runner) runGroup(ctx context.Context, c config.Check, depth int) []Result {
	children := r.runChecks(ctx, c.Items, depth+1)

	// Determine the header's pass/severity from the children.
	headerSev, headerPass := worstStatus(children)

	header := Result{
		Label:    c.Value,
		Severity: headerSev,
		Pass:     headerPass,
		Depth:    depth,
		Kind:     KindHeader,
	}

	out := make([]Result, 0, 1+len(children))
	out = append(out, header)
	out = append(out, children...)
	return out
}

// resolveSeverity converts a Check's Severity string and Required sugar to a
// Severity value.  precedence: explicit Severity > Required sugar > default (Error).
func resolveSeverity(c config.Check) Severity {
	switch c.Severity {
	case "warn":
		return SeverityWarn
	case "info":
		return SeverityInfo
	case "error":
		return SeverityError
	}
	if c.Required != nil && !*c.Required {
		return SeverityWarn
	}
	return SeverityError
}

// worstStatus returns the aggregate pass/severity across a slice of results.
// A group header passes only when all children pass; severity reflects the
// worst-failing (or worst-overall) child severity.
func worstStatus(results []Result) (Severity, bool) {
	sev := SeverityInfo // start at least-severe; tightened by each child
	allPass := true
	for _, r := range results {
		if r.Kind == KindHeader {
			continue // avoid double-counting nested group headers
		}
		if !r.Pass {
			allPass = false
		}
		// Lower numeric value = worse (Error=0, Warn=1, Info=2).
		if r.Severity < sev {
			sev = r.Severity
		}
	}
	return sev, allPass
}
