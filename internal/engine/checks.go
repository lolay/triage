package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lolay/triage/internal/config"
)

// checkEnv evaluates an env: check (spec §5).
//
// Set mode (unset=false, the default): the variable must be present in the
// environment; an empty value ("") counts as set. Optional matches regex is
// asserted against the value if supplied.
//
// Unset mode (unset=true): the variable must NOT be present at all.
func checkEnv(name string, unset bool, matches, hint string, lookupEnv func(string) (string, bool)) Result {
	val, found := lookupEnv(name)

	if unset {
		if found {
			return Result{
				Pass:    false,
				Message: fmt.Sprintf("%s is set (expected unset)", name),
			}
		}
		return Result{Pass: true, Message: fmt.Sprintf("%s (not set)", name)}
	}

	// Set mode.
	if !found {
		msg := fmt.Sprintf("%s not set", name)
		if hint != "" {
			msg += " — " + hint
		}
		return Result{Pass: false, Message: msg}
	}

	if matches != "" {
		re, err := regexp.Compile(matches)
		if err != nil {
			return Result{
				Pass:    false,
				Message: fmt.Sprintf("%s: invalid matches regex %q: %v", name, matches, err),
			}
		}
		if !re.MatchString(val) {
			msg := fmt.Sprintf("%s value does not match %q", name, matches)
			if hint != "" {
				msg += " — " + hint
			}
			return Result{Pass: false, Message: msg}
		}
	}

	display := val
	if len(display) > 40 {
		display = display[:40] + "…"
	}
	return Result{Pass: true, Message: fmt.Sprintf("%s=%s", name, display)}
}

// checkPath evaluates a path: check (spec §5). pattern may include glob
// wildcards. A leading ~ is expanded to the user's home directory. Relative
// patterns are resolved against effectiveBase (which callers set to c.Dir if
// non-empty, else opts.BaseDir).
func checkPath(pattern, effectiveBase, hint string) Result {
	p := expandTilde(pattern)
	if !filepath.IsAbs(p) && effectiveBase != "" {
		p = filepath.Join(effectiveBase, p)
	}

	matches, err := filepath.Glob(p)
	if err != nil || len(matches) == 0 {
		msg := fmt.Sprintf("%s not found", pattern)
		if hint != "" {
			msg += " — " + hint
		}
		return Result{Pass: false, Message: msg}
	}
	return Result{Pass: true, Message: pattern}
}

// expandTilde replaces a leading ~ with the user's home directory.
func expandTilde(path string) string {
	if path == "~" {
		if h, err := os.UserHomeDir(); err == nil {
			return h
		}
		return path
	}
	if strings.HasPrefix(path, "~/") {
		if h, err := os.UserHomeDir(); err == nil {
			return filepath.Join(h, path[2:])
		}
	}
	return path
}

// oneOfLabel derives a display label for an unlabelled one_of check from its
// alternatives. Tool alternatives use just the tool name; others use type:value.
func oneOfLabel(alts []config.Check) string {
	names := make([]string, 0, len(alts))
	for _, a := range alts {
		switch {
		case a.Label != "":
			names = append(names, a.Label)
		case a.Type == config.TypeTool:
			names = append(names, a.Value)
		default:
			names = append(names, a.Type+": "+a.Value)
		}
	}
	return strings.Join(names, " or ")
}
