package report

import (
	"fmt"
	"io"

	"github.com/lolay/triage/internal/engine"
)

// BoardOpts controls human board rendering.
type BoardOpts struct {
	Profile string // profile name shown in the header line
	NoColor bool
	Quiet   bool // suppress passing checks (show failures only)
}

// Board writes the human-readable check board to w.
//
// m1 output for an empty profile (0 checks):
//
//	triage (profile: default)
//
//	✓ 0 ok
//
// Group nesting, in-flight pending lines, and the delegate tree land in m2.
func Board(w io.Writer, results []engine.Result, opts BoardOpts) {
	profile := opts.Profile
	if profile == "" {
		profile = "default"
	}

	fmt.Fprintf(w, "triage (profile: %s)\n", profile)
	fmt.Fprintln(w)

	var ok, warns, errs int
	for _, r := range results {
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
	case errs > 0 || warns > 0:
		errWord := "errors"
		if errs == 1 {
			errWord = "error"
		}
		warnWord := "warnings"
		if warns == 1 {
			warnWord = "warning"
		}
		fmt.Fprintf(w, "✗ %d %s, %d %s, %d ok — fix the [✗] items above\n",
			errs, errWord, warns, warnWord, ok)
	default:
		fmt.Fprintf(w, "✓ %d ok\n", ok)
	}
}
