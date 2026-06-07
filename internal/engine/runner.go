package engine

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"sync"

	"github.com/lolay/triage/internal/config"
)

// maxAutoJobs caps the auto-selected worker count. Checks are I/O-bound
// subprocess waits, so NumCPU is already conservative; the cap keeps the
// default polite on big machines (spec §7.2).
const maxAutoJobs = 8

// Runner executes the checks in a profile in declaration order.
//
// m3 adds env, path, one_of, command (explicit interpreter), and platform
// guards. Checks whose platform: list excludes the current OS are omitted
// entirely (no board line, no count). m4 adds delegate: a delegate loads a
// child config and recurses, inheriting the parent's CLI vars but using the
// child's own config vars, with absolute-path cycle detection.
//
// m4 s4 adds a bounded worker pool: siblings run concurrently while results are
// always materialized in list order (execute concurrently, materialize in list
// order — §7.2). The whole delegate tree shares one Runner.shared (semaphore,
// global serial lock, first fatal error), so the subprocess budget is global.
type Runner struct {
	// vars is the effective template var map (config + CLI + built-ins).
	vars map[string]string
	// visited is the set of resolved absolute config paths on the current
	// ancestor chain; a revisit is a delegate cycle. Children receive a copy
	// (path-based, so sibling/diamond delegates to the same config are allowed).
	visited map[string]bool
	// shared is the run-global coordination state, shared by pointer across the
	// entire delegate tree. Initialized on the root in RunContext.
	shared *sharedState
	opts   RunnerOpts
}

// sharedState is the run-global coordination shared across the whole delegate
// tree: the subprocess semaphore, the global serial lock, and the first fatal
// config error. A single instance is created on the root Runner and threaded
// (by pointer) into every delegate child runner.
type sharedState struct {
	// fatal is the first fatal config error seen; guarded by fatalMu because it
	// may be written from sibling goroutines.
	fatal error
	// sem bounds concurrent subprocess spawns; nil means sequential (jobs == 1),
	// in which case the dedicated in-list-order path is used as the oracle.
	sem chan struct{}
	// serialMu serializes serial: checks so they never overlap across subtrees.
	serialMu sync.Mutex
	fatalMu  sync.Mutex
}

func (s *sharedState) recordFatal(err error) {
	s.fatalMu.Lock()
	defer s.fatalMu.Unlock()
	if s.fatal == nil {
		s.fatal = err
	}
}

// acquire/release bracket a single subprocess spawn against the semaphore.
// They are no-ops in the sequential path (sem == nil), so instant checks and
// aggregators never consume a token.
func (r *Runner) acquire() {
	if r.shared != nil && r.shared.sem != nil {
		r.shared.sem <- struct{}{}
	}
}

func (r *Runner) release() {
	if r.shared != nil && r.shared.sem != nil {
		<-r.shared.sem
	}
}

// NewRunner creates a Runner with default (real) LookPath and probe functions.
func NewRunner() *Runner { return &Runner{} }

// NewRunnerWith creates a Runner with injected functions (for tests).
func NewRunnerWith(opts RunnerOpts) *Runner { return &Runner{opts: opts} }

// defaultJobs resolves Jobs==0 to min(NumCPU, maxAutoJobs), never below 1.
func defaultJobs() int {
	return max(min(runtime.NumCPU(), maxAutoJobs), 1)
}

// Run executes each check in profile against a background context. It is the
// stable entry point kept for the many call sites that don't thread a context;
// RunContext is the cancellation-aware variant.
func (r *Runner) Run(profile config.Profile) []Result {
	return r.RunContext(context.Background(), profile)
}

// RunContext executes each check in profile and returns an ordered,
// depth-annotated Result slice. Group headers appear immediately before their
// children and carry the worst-status glyph of the group. Execution may be
// concurrent (bounded by Jobs), but results — and any --command-log blocks —
// are materialized in config list order. Cancelling ctx (SIGINT/SIGTERM) stops
// scheduling new work and returns the partial board.
func (r *Runner) RunContext(ctx context.Context, profile config.Profile) []Result {
	r.vars = r.effectiveVars()
	if r.visited == nil {
		r.visited = map[string]bool{}
	}
	// Seed the entry config so a child delegating back to the root is caught.
	if r.opts.ConfigPath != "" {
		r.visited[absOrSelf(r.opts.ConfigPath)] = true
	}
	if r.shared == nil {
		jobs := r.opts.Jobs
		if jobs <= 0 {
			jobs = defaultJobs()
		}
		r.shared = &sharedState{}
		if jobs > 1 {
			r.shared.sem = make(chan struct{}, jobs)
		}
	}

	results := r.runChecks(ctx, profile, 0)

	// Materialize --command-log blocks in list order so the log file is
	// byte-identical regardless of --jobs. The root's results slice contains
	// every leaf in the whole tree (delegate children included), in order.
	if r.opts.CommandLog != nil {
		for i := range results {
			if e := results[i].cmdLog; e != nil {
				r.opts.CommandLog.WriteBlock(e.label, e.cwd, e.run, e.output)
			}
		}
	}
	return results
}

// Fatal returns the first fatal config error encountered during Run (e.g. a
// delegate cycle), or nil. The CLI maps a non-nil value to ExitUsageError (3).
// Safe to call after Run/RunContext returns (all workers have joined).
func (r *Runner) Fatal() error {
	if r.shared == nil {
		return nil
	}
	r.shared.fatalMu.Lock()
	defer r.shared.fatalMu.Unlock()
	return r.shared.fatal
}

// recordFatal stores the first fatal config error seen during a run.
func (r *Runner) recordFatal(err error) {
	if r.shared != nil {
		r.shared.recordFatal(err)
	}
}

// effectiveVars builds the template variable map: per-config vars overlaid by
// global CLI --var overrides, with built-in profile and os injected last
// (built-ins always win). A delegated child uses its own ConfigVars but
// inherits the parent's CLIVars, so config vars never leak across the boundary.
func (r *Runner) effectiveVars() map[string]string {
	out := make(map[string]string, len(r.opts.ConfigVars)+len(r.opts.CLIVars)+2)
	maps.Copy(out, r.opts.ConfigVars)
	maps.Copy(out, r.opts.CLIVars)
	out["profile"] = r.opts.Profile
	out["os"] = r.goos()
	return out
}

// absOrSelf returns the cleaned absolute form of p, falling back to p itself
// when the working directory can't be resolved.
func absOrSelf(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

// runChecks executes a sibling list and returns their results concatenated in
// list order. When the pool is disabled (Jobs == 1, sem == nil) it uses the
// dedicated sequential path — the reference oracle for --jobs-invariant output.
// Otherwise siblings run as goroutines writing into per-index slots; serial:
// checks act as a barrier (drain in-flight siblings, then run alone under the
// global serial lock).
func (r *Runner) runChecks(ctx context.Context, checks config.Profile, depth int) []Result {
	if r.shared == nil || r.shared.sem == nil {
		return r.runChecksSequential(ctx, checks, depth)
	}

	slots := make([][]Result, len(checks))
	var wg sync.WaitGroup
	for i, c := range checks {
		if ctx.Err() != nil {
			break
		}
		if c.Serial {
			// Barrier: let prior in-flight siblings finish, then run this check
			// alone while holding the process-global serial lock so serial
			// checks never overlap across subtrees.
			wg.Wait()
			r.shared.serialMu.Lock()
			slots[i] = r.runCheck(ctx, c, depth)
			r.shared.serialMu.Unlock()
			continue
		}
		wg.Add(1)
		go func(i int, c config.Check) {
			defer wg.Done()
			slots[i] = r.runCheck(ctx, c, depth)
		}(i, c)
	}
	wg.Wait()

	var out []Result
	for _, s := range slots {
		out = append(out, s...)
	}
	return out
}

// runChecksSequential runs a sibling list in a single goroutine, in list order.
// Used for Jobs == 1 (the oracle) and for the children of a serial: group.
func (r *Runner) runChecksSequential(ctx context.Context, checks config.Profile, depth int) []Result {
	var out []Result
	for _, c := range checks {
		if ctx.Err() != nil {
			break
		}
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
	return slices.Contains(c.Platform, goos)
}

func (r *Runner) runCheck(ctx context.Context, c config.Check, depth int) []Result {
	// Platform gate: omit checks whose platform: list excludes the current OS.
	if !platformMatches(c, r.goos()) {
		return nil
	}

	expanded, err := expandCheck(c, r.vars)
	if err != nil {
		label := c.Label
		if label == "" {
			label = c.Value
		}
		if label == "" {
			label = c.Type
		}
		return []Result{{
			Label:    label,
			Severity: SeverityError,
			Pass:     false,
			Message:  err.Error(),
			Depth:    depth,
			Group:    c.Group,
			Kind:     KindLeaf,
		}}
	}
	c = expanded

	sev := resolveSeverity(c)

	switch c.Type {
	case config.TypeTool:
		// Only the version probe spawns a subprocess, so the semaphore is
		// acquired inside the wrapped RunProbe — a bare presence check never
		// holds a token. The wrapper is a no-op in the sequential path.
		o := r.opts
		o.RunProbe = r.wrapProbe(o.RunProbe)
		res := checkTool(ctx, c.Value, c.Constraint, c.Hint, o)
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

	case config.TypeDelegate:
		return r.runDelegate(ctx, c, depth)

	default:
		// Unknown type key (parse-time rejection should prevent this). Emit a
		// non-failing info skip rather than crashing.
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
			Message:  fmt.Sprintf("%s (unsupported check type)", c.Type),
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

// wrapProbe brackets a tool version probe with the subprocess semaphore. It
// returns base unchanged in the sequential path so tool checks behave exactly
// as before when the pool is off.
func (r *Runner) wrapProbe(base func(context.Context, string, []string) (string, error)) func(context.Context, string, []string) (string, error) {
	if r.shared == nil || r.shared.sem == nil {
		return base
	}
	return func(ctx context.Context, name string, args []string) (string, error) {
		r.acquire()
		defer r.release()
		if base == nil {
			return defaultRunProbe(ctx, name, args)
		}
		return base(ctx, name, args)
	}
}

// runGroup executes a structural `group` container: run all children, then
// prepend a header Result carrying the group's worst glyph.
// If all children are omitted (platform-filtered), the group header is also
// omitted. A serial: group runs its children sequentially (spec §7.2); it is
// already invoked under the global serial lock by the scheduler.
func (r *Runner) runGroup(ctx context.Context, c config.Check, depth int) []Result {
	var children []Result
	if c.Serial {
		children = r.runChecksSequential(ctx, c.Items, depth+1)
	} else {
		children = r.runChecks(ctx, c.Items, depth+1)
	}

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

// runDelegate loads a child config and recurses into it, emitting a summary
// KindDelegate line (worst-child glyph) followed by the child subtree at
// depth+1. The child inherits the parent's CLI vars and active profile but uses
// its own config vars and BaseDir. A missing or unparseable child config yields
// a single failing delegate result (exit 1); a delegate that re-enters a config
// already on the ancestor chain is a fatal cycle (exit 3 via Fatal()).
func (r *Runner) runDelegate(ctx context.Context, c config.Check, depth int) []Result {
	// Child config target: config: if set, else dir:, resolved against BaseDir.
	target := c.Config
	if target == "" {
		target = c.Dir
	}
	resolved := target
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(r.opts.BaseDir, resolved)
	}

	label := c.Value
	if label == "" {
		label = c.Label
	}
	if label == "" {
		label = target
	}
	if label == "" {
		label = "delegate"
	}

	childCfg, err := config.Discover(resolved)
	if err != nil {
		// No subtree was produced, so this is a counted leaf failure (KindLeaf),
		// not an aggregate summary — otherwise the failure would be skipped by an
		// ancestor's worstStatus and by the summary count.
		return []Result{{
			Label:    label,
			Severity: SeverityError,
			Pass:     false,
			Message:  fmt.Sprintf("delegate %q: %v", label, err),
			Depth:    depth,
			Group:    c.Group,
			Kind:     KindLeaf,
		}}
	}

	// Cycle detection on the resolved absolute config path (load() makes Path
	// absolute). A path already on the ancestor chain is a fatal config error.
	absPath := childCfg.Path
	if r.visited[absPath] {
		r.recordFatal(fmt.Errorf("delegate cycle detected: %q re-enters %s", label, absPath))
		return []Result{{
			Label:    label,
			Severity: SeverityError,
			Pass:     false,
			Message:  fmt.Sprintf("delegate %q: cycle detected at %s", label, absPath),
			Depth:    depth,
			Group:    c.Group,
			Kind:     KindLeaf,
		}}
	}

	// Forwarded profile; absent in the child → empty (mirrors top-level).
	childProfile := childCfg.Profiles[r.opts.Profile]
	if childProfile == nil {
		childProfile = config.Profile{}
	}

	// Per-branch visited copy: ancestors + this child (path-based, so diamond
	// delegates to the same config in different branches are allowed).
	childVisited := make(map[string]bool, len(r.visited)+1)
	for k := range r.visited {
		childVisited[k] = true
	}
	childVisited[absPath] = true

	// Child runner: own BaseDir + ConfigVars, inherited CLIVars/Profile/probes.
	childOpts := r.opts
	childOpts.BaseDir = filepath.Dir(absPath)
	childOpts.ConfigVars = childCfg.Vars
	childOpts.ConfigPath = absPath

	// The child shares the parent's run-global state (semaphore, serial lock,
	// fatal error), so the subprocess budget is global across the whole tree
	// and a cycle detected deep in a subtree surfaces at the root.
	child := &Runner{opts: childOpts, visited: childVisited, shared: r.shared}
	child.vars = child.effectiveVars()
	children := child.runChecks(ctx, childProfile, depth+1)

	sev, pass := worstStatus(children)
	summary := Result{
		Label:    label,
		Severity: sev,
		Pass:     pass,
		Depth:    depth,
		Group:    c.Group,
		Kind:     KindDelegate,
	}

	out := make([]Result, 0, 1+len(children))
	out = append(out, summary)
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
		if r.Kind == KindHeader || r.Kind == KindDelegate {
			continue // headers/delegate summaries aggregate children; don't double-count
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
