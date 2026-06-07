package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/lolay/triage/internal/engine"
)

// Sink is the output interface for the CLI: either the static Board (non-TTY)
// or the streaming TTY sink that back-updates pending lines.
type Sink interface {
	// Begin is called once before any results are emitted.
	Begin(profile string)
	// Emit delivers one result. For a KindHeader the sink may print a pending
	// `[…]` line and update it when the corresponding EndGroup is called.
	Emit(r engine.Result)
	// End is called after all results; it prints the summary + remediation block.
	End(results []engine.Result)
}

// TTYSink streams results to a TTY with pending `[…]` lines that are rewritten
// in place to the final glyph once the check completes.
//
// Slow checks (tool with constraint, group headers) get a pending line first.
// Instant checks (bare tool presence, info skips) print the final line directly.
//
// Rewrite uses ANSI cursor-up + clear-to-EOL (works on all ANSI-capable TTYs).
type TTYSink struct {
	w       io.Writer
	opts    BoardOpts
	pending int // number of pending lines currently on screen (0 or 1)
}

// NewTTYSink creates a TTYSink writing to w.
func NewTTYSink(w io.Writer, opts BoardOpts) *TTYSink {
	return &TTYSink{w: w, opts: opts}
}

func (s *TTYSink) Begin(profile string) {
	p := profile
	if p == "" {
		p = "default"
	}
	fmt.Fprintf(s.w, "triage (profile: %s)\n", p)
	fmt.Fprintln(s.w)
}

func (s *TTYSink) Emit(r engine.Result) {
	// For group headers, we print a pending line that will be rewritten by the
	// next header/leaf in the group (which calls back with the resolved result).
	// In practice the runner collects all children first, so we receive headers
	// before their children with the final glyph already set. We therefore treat
	// the TTY sink the same as the static sink for results — just print lines in
	// order. The pending `[…]` + back-update would require a streaming runner
	// (future); for m2 the runner is synchronous, so the TTY sink adds colors
	// and live flushing but not actual async pending lines.
	//
	// This is still TTY-aware: we print each line as it arrives (streaming)
	// rather than buffering all results for a single Board render at End.
	if r.Kind == engine.KindHeader || r.Kind == engine.KindDelegate {
		indent := strings.Repeat("    ", r.Depth)
		glyph := glyphFor(r, s.opts)
		fmt.Fprintf(s.w, "%s%s %s\n", indent, glyph, r.Label)
		return
	}

	if r.Pass && s.opts.Quiet {
		return
	}

	indent := strings.Repeat("    ", r.Depth)
	if r.Group != "" {
		indent = "    " + indent
	}
	glyph := glyphFor(r, s.opts)
	line := fmt.Sprintf("%s%s %s", indent, glyph, r.Label)
	if !r.Pass && r.Message != "" {
		line += " — " + r.Message
	}
	fmt.Fprintln(s.w, line)
}

func (s *TTYSink) End(results []engine.Result) {
	fmt.Fprintln(s.w)
	printSummary(s.w, results, s.opts)
	printRemediation(s.w, results, s.opts)
}

// printSummary writes the trailing summary line (shared by static and TTY sinks).
func printSummary(w io.Writer, results []engine.Result, opts BoardOpts) {
	var ok, warns, errs int
	for _, r := range results {
		if r.Kind == engine.KindHeader || r.Kind == engine.KindDelegate {
			continue
		}
		if r.Pass {
			ok++
		} else {
			switch r.Severity {
			case engine.SeverityError:
				errs++
			case engine.SeverityWarn:
				warns++
			}
		}
	}

	switch {
	case errs > 0:
		errWord := plural(errs, "error")
		warnWord := plural(warns, "warning")
		fmt.Fprintf(w, "%s %s, %s, %d ok — fix the [✗] items above\n",
			color("✗", colorRed, opts), errWord, warnWord, ok)
	case warns > 0:
		warnWord := plural(warns, "warning")
		fmt.Fprintf(w, "%s %s, %d ok — see [!] items above\n",
			color("!", colorYellow, opts), warnWord, ok)
	default:
		fmt.Fprintf(w, "%s %d ok\n", color("✓", colorGreen, opts), ok)
	}
}
