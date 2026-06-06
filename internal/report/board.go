package report

import (
	"fmt"
	"io"
	"strings"

	"github.com/lolay/triage/internal/engine"
)

// BoardOpts controls human board rendering.
type BoardOpts struct {
	Profile string // profile name shown in the header line
	NoColor bool
	Quiet   bool // suppress passing checks (show failures only)
	IsTTY   bool // used by the TTY sink; static board ignores it
}

// Board writes the complete human-readable board to w (header, results, summary,
// remediation block). Used for non-TTY output; the TTY streaming path uses Sink.
func Board(w io.Writer, results []engine.Result, opts BoardOpts) {
	profile := opts.Profile
	if profile == "" {
		profile = "default"
	}

	fmt.Fprintf(w, "triage (profile: %s)\n", profile)
	fmt.Fprintln(w)
	renderResults(w, results, opts)
	fmt.Fprintln(w)
	printSummary(w, results, opts)
	printRemediation(w, results, opts)
}

// renderResults emits one line per result in declaration order.
// Structural group headers are printed at their depth; legacy `group:` string
// fields get a synthetic header on first appearance with the group's worst glyph.
func renderResults(w io.Writer, results []engine.Result, opts BoardOpts) {
	lastLegacyGroup := ""

	for i := range results {
		r := results[i]

		if r.Kind == engine.KindHeader {
			indent := strings.Repeat("    ", r.Depth)
			glyph := glyphFor(r, opts)
			fmt.Fprintf(w, "%s%s %s\n", indent, glyph, r.Label)
			lastLegacyGroup = ""
			continue
		}

		// Legacy flat-section: emit a synthetic header on first appearance.
		if r.Group != "" && r.Group != lastLegacyGroup {
			glyph := worstGlyphForGroup(r.Group, results, opts)
			fmt.Fprintf(w, "%s %s\n", glyph, r.Group)
			lastLegacyGroup = r.Group
		}

		if r.Pass && opts.Quiet {
			continue
		}

		indent := strings.Repeat("    ", r.Depth)
		if r.Group != "" {
			indent = "    " + indent
		}

		glyph := glyphFor(r, opts)
		line := fmt.Sprintf("%s%s %s", indent, glyph, r.Label)
		if !r.Pass && r.Message != "" {
			line += " — " + r.Message
		}
		fmt.Fprintln(w, line)
	}
}

// printRemediation writes the "To fix, run:" block of deduped hints.
func printRemediation(w io.Writer, results []engine.Result, opts BoardOpts) {
	var hints []string
	seen := map[string]bool{}
	for _, r := range results {
		if r.Kind == engine.KindHeader {
			continue
		}
		if r.Pass || r.Severity == engine.SeverityInfo {
			continue
		}
		hint := extractHint(r.Message)
		if hint == "" || seen[hint] {
			continue
		}
		seen[hint] = true
		hints = append(hints, hint)
	}
	if len(hints) == 0 {
		return
	}
	fmt.Fprintln(w, "To fix, run:")
	for _, h := range hints {
		fmt.Fprintf(w, "    %s\n", h)
	}
}

// extractHint pulls the hint portion from a failure message ("... — <hint>").
func extractHint(msg string) string {
	const sep = " — "
	idx := strings.LastIndex(msg, sep)
	if idx < 0 {
		return ""
	}
	return strings.TrimSpace(msg[idx+len(sep):])
}

// glyphFor returns the bracketed status glyph for a result, optionally colored.
func glyphFor(r engine.Result, opts BoardOpts) string {
	if r.Pass {
		if r.Severity == engine.SeverityInfo {
			return color("[ℹ]", colorCyan, opts)
		}
		return color("[✓]", colorGreen, opts)
	}
	switch r.Severity {
	case engine.SeverityWarn:
		return color("[!]", colorYellow, opts)
	case engine.SeverityInfo:
		return color("[ℹ]", colorCyan, opts)
	default:
		return color("[✗]", colorRed, opts)
	}
}

// worstGlyphForGroup scans results for the given legacy-group name and returns
// the glyph for the worst-status result in that group.
func worstGlyphForGroup(group string, results []engine.Result, opts BoardOpts) string {
	worst := engine.Result{Pass: true, Severity: engine.SeverityInfo, Kind: engine.KindLeaf}
	for _, r := range results {
		if r.Kind == engine.KindHeader || r.Group != group {
			continue
		}
		if !r.Pass && worst.Pass {
			worst = r
			continue
		}
		if !r.Pass && r.Severity < worst.Severity {
			worst = r
		}
	}
	return glyphFor(worst, opts)
}

// plural returns "N word" / "N words".
func plural(n int, word string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, word)
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// ANSI color codes.
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
)

func color(s, code string, opts BoardOpts) string {
	if opts.NoColor || !opts.IsTTY {
		return s
	}
	return code + s + colorReset
}
