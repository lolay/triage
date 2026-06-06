package engine

import (
	"context"
	"fmt"
	"os"

	"github.com/lolay/triage/internal/config"
)

// Runner executes the checks in a profile in declaration order.
//
// m3 adds env, path, one_of, command (explicit interpreter), and platform
// guards. Checks whose platform: list excludes the current OS are omitted
// entirely (no board line, no count). delegate is deferred to m5.
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

// goos returns the effective OS string for platform: guards: opts.GOOS when
// set, otherwise CurrentPlatform().
func (r *Runner) goos() string {
	if r.opts.GOOS != "" {
		return r.opts.GOOS
	}
	return CurrentPlatform()
}

// platformMatches reports whether check c should run on the current OS.
// An empty Platform list means "all platforms".
func platformMatches(c config.Check, goos string) bool {
	if len(c.Platform) == 0 {
		return true
	}
	for _, p := range c.Platform {
		if p == goos {
			return true
		}
	}
	return false
}

func (r *Runner) runCheck(ctx context.Context, c config.Check, depth int) []Result {
	// Platform gate: omit checks whose platform: list excludes the current OS.
	if !platformMatches(c, r.goos()) {
		return nil
	}

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

	case config.TypeEnv:
		lookupEnv := r.opts.LookupEnv
		if lookupEnv == nil {
			lookupEnv = os.LookupEnv
		}
		res := checkEnv(c.Value, c.Unset, c.Matches, c.Hint, lookupEnv)
		res.Label = c.Value
		res.Severity = sev
		res.Depth = depth
		res.Group = c.Group
		res.Kind = KindLeaf
		return []Result{res}

	case config.TypePath:
		base := c.Dir
		if base == "" {
			base = r.opts.BaseDir
		}
		res := checkPath(c.Value, base, c.Hint)
		res.Label = c.Value
		res.Severity = sev
		res.Depth = depth
		res.Group = c.Group
		res.Kind = KindLeaf
		return []Result{res}

	case config.TypeOneOf:
		res := r.checkOneOf(ctx, c, depth)
		return []Result{res}

	case config.TypeCommand:
		res := r.checkCommand(ctx, c)
		label := c.Label
		if label == "" {
			label = c.Value
		}
		res.Label = label
		res.Severity = sev
		res.Depth = depth
		res.Group = c.Group
		res.Kind = KindLeaf
		return []Result{res}

	case config.TypeGroup:
		return r.runGroup(ctx, c, depth)

	default:
		// delegate is deferred to m5; command lands in m3 s2.
		// Emit a non-failing info skip for now.
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

// checkOneOf evaluates a one_of: check and returns a single atomic leaf result.
// Each alternative is evaluated silently (internal to the one_of). The result
// passes if any alternative passes. Platform-filtered alternatives (nil results)
// count as "not available" and do not satisfy the one_of.
func (r *Runner) checkOneOf(ctx context.Context, c config.Check, depth int) Result {
	label := c.Label
	if label == "" {
		label = oneOfLabel(c.OneOf)
	}
	sev := resolveSeverity(c)

	for _, alt := range c.OneOf {
		results := r.runCheck(ctx, alt, depth)
		if len(results) == 0 {
			// Platform-filtered — alternative does not apply.
			continue
		}
		// For a group alternative, results[0] is the header with worst-child
		// status; for a leaf, results[0] is the leaf.
		if results[0].Pass {
			return Result{
				Label:    label,
				Severity: sev,
				Pass:     true,
				Message:  results[0].Message,
				Depth:    depth,
				Group:    c.Group,
				Kind:     KindLeaf,
			}
		}
	}

	msg := fmt.Sprintf("%s: none available", label)
	if c.Hint != "" {
		msg += " — " + c.Hint
	}
	return Result{
		Label:    label,
		Severity: sev,
		Pass:     false,
		Message:  msg,
		Depth:    depth,
		Group:    c.Group,
		Kind:     KindLeaf,
	}
}

// runGroup executes a structural `group` container: run all children, then
// prepend a header Result carrying the group's worst glyph.
// If all children are omitted (platform-filtered), the group header is also
// omitted.
func (r *Runner) runGroup(ctx context.Context, c config.Check, depth int) []Result {
	children := r.runChecks(ctx, c.Items, depth+1)

	// Omit the header when all children were platform-filtered away.
	if len(children) == 0 {
		return nil
	}

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
