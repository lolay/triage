package engine

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/lolay/triage/internal/config"
)

// defaultRunCommand executes an explicitly-interpreted command snippet and
// captures combined stdout+stderr up to captureCap bytes (spec §5). It is used
// as the default for RunnerOpts.RunCommand; a 30-second timeout is applied
// unless the incoming context already carries a shorter deadline.
func defaultRunCommand(ctx context.Context, interp, script, dir string) (string, int, error) {
	tctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(tctx, interp, "-c", script)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if len(out) > captureCap {
		out = out[:captureCap]
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			// Normal non-zero exit — not an exec error.
			return string(out), exitErr.ExitCode(), nil
		}
		return string(out), -1, err
	}
	return string(out), 0, nil
}

// checkCommand evaluates a command: check (spec §5). It expands {{profile}},
// resolves the working directory, invokes the interpreter (default: sh),
// asserts exit code / contains / matches against bounded stdout, and writes
// a block to CommandLog when active.
func (r *Runner) checkCommand(ctx context.Context, c config.Check) Result {
	runCmd := r.opts.RunCommand
	if runCmd == nil {
		runCmd = defaultRunCommand
	}

	interp := c.Interp
	if interp == "" {
		interp = "sh"
	}

	// {{profile}} template expansion.
	script := strings.ReplaceAll(c.Value, "{{profile}}", r.opts.Profile)

	label := c.Label
	if label == "" {
		label = c.Value
	}

	// Resolve working directory: c.Dir overrides opts.BaseDir; relative c.Dir
	// is resolved against opts.BaseDir.
	dir := c.Dir
	switch {
	case dir == "":
		dir = r.opts.BaseDir
	case !filepath.IsAbs(dir) && r.opts.BaseDir != "":
		dir = filepath.Join(r.opts.BaseDir, dir)
	}

	stdout, exitCode, err := runCmd(ctx, interp, script, dir)

	// Write to CommandLog regardless of pass/fail.
	if r.opts.CommandLog != nil {
		runLine := fmt.Sprintf("%s -c %q", interp, script)
		r.opts.CommandLog.WriteBlock(label, dir, runLine, []byte(stdout))
	}

	if err != nil {
		msg := fmt.Sprintf("%s: exec error: %v", label, err)
		if ex := oneLineExcerpt(stdout); ex != "" {
			msg += ": " + ex
		}
		if c.Hint != "" {
			msg += " — " + c.Hint
		}
		return Result{Pass: false, Message: msg, Output: stdout}
	}

	wantExit := 0
	if c.Exit != nil {
		wantExit = *c.Exit
	}
	hasAssertion := c.Contains != "" || c.Matches != ""

	// Check exit code when no stdout assertions, or when c.Exit is explicit.
	if !hasAssertion || c.Exit != nil {
		if exitCode != wantExit {
			msg := fmt.Sprintf("%s: exit %d (want %d)", label, exitCode, wantExit)
			if ex := oneLineExcerpt(stdout); ex != "" {
				msg += ": " + ex
			}
			if c.Hint != "" {
				msg += " — " + c.Hint
			}
			return Result{Pass: false, Message: msg, Output: stdout}
		}
		if !hasAssertion {
			return Result{Pass: true, Message: label}
		}
	}

	// Spec §5: if the captured output hit the cap, fail and direct user to
	// --command-log rather than asserting on a truncated prefix.
	if len(stdout) >= captureCap {
		return Result{
			Pass:    false,
			Message: fmt.Sprintf("%s: output exceeded %d bytes — re-run with --command-log to inspect", label, captureCap),
			Output:  stdout,
		}
	}

	if c.Contains != "" && !strings.Contains(stdout, c.Contains) {
		msg := fmt.Sprintf("%s: output does not contain %q", label, c.Contains)
		if ex := oneLineExcerpt(stdout); ex != "" {
			msg += ": " + ex
		}
		if c.Hint != "" {
			msg += " — " + c.Hint
		}
		return Result{Pass: false, Message: msg, Output: stdout}
	}

	if c.Matches != "" {
		re, err := regexp.Compile(c.Matches)
		if err != nil {
			return Result{
				Pass:    false,
				Message: fmt.Sprintf("%s: invalid matches regex %q: %v", label, c.Matches, err),
			}
		}
		if !re.MatchString(stdout) {
			msg := fmt.Sprintf("%s: output does not match %q", label, c.Matches)
			if ex := oneLineExcerpt(stdout); ex != "" {
				msg += ": " + ex
			}
			if c.Hint != "" {
				msg += " — " + c.Hint
			}
			return Result{Pass: false, Message: msg, Output: stdout}
		}
	}

	return Result{Pass: true, Message: label}
}

// oneLineExcerpt returns the first non-empty line of s, trimmed to 80 chars.
func oneLineExcerpt(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = strings.TrimSpace(s[:idx])
	}
	if len(s) > 80 {
		return s[:80] + "…"
	}
	return s
}
