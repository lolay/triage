package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/lolay/triage/internal/config"
)

// defaultRunCommand executes an explicitly-interpreted command snippet and
// captures combined stdout+stderr up to captureCap bytes (spec §5). It is used
// as the default for RunnerOpts.RunCommand; a 30-second timeout is applied
// unless the incoming context already carries a shorter deadline.
//
// env pairs are appended after os.Environ(); duplicate keys use the last value
// (Go exec semantics), so with_env overrides inherited variables.
func defaultRunCommand(ctx context.Context, interp, script, dir string, env map[string]string) (string, int, error) {
	tctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(tctx, interp, "-c", script)
	if dir != "" {
		cmd.Dir = dir
	}
	if len(env) > 0 {
		cmd.Env = mergeEnv(os.Environ(), env)
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

// mergeEnv appends env overrides (sorted by key) after the inherited environment.
func mergeEnv(inherited []string, overrides map[string]string) []string {
	if len(overrides) == 0 {
		return inherited
	}
	keys := make([]string, 0, len(overrides))
	for k := range overrides {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, len(inherited), len(inherited)+len(keys))
	copy(out, inherited)
	for _, k := range keys {
		out = append(out, k+"="+overrides[k])
	}
	return out
}

// expandProfileInMap returns a copy of m with {{profile}} replaced in each value.
func expandProfileInMap(m map[string]string, profile string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = strings.ReplaceAll(v, "{{profile}}", profile)
	}
	return out
}

// envPrefix formats injected env pairs as a paste-safe shell prefix (sorted
// keys). Values containing spaces or special characters are single-quoted so
// the log line can be copy-pasted back into a shell.
func envPrefix(env map[string]string) string {
	if len(env) == 0 {
		return ""
	}
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + shellQuote(env[k])
	}
	return strings.Join(parts, " ") + " "
}

// shellQuote wraps s in single quotes if it contains any character that would
// require quoting in POSIX sh. Simple values (alphanumeric + a few safe
// punctuation chars) are returned as-is.
func shellQuote(s string) string {
	safe := true
	for _, r := range s {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.' || r == '/') {
			safe = false
			break
		}
	}
	if safe {
		return s
	}
	// Single-quote the value; escape any embedded single quotes as '\''.
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
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
	cmdEnv := expandProfileInMap(c.WithEnv, r.opts.Profile)

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

	stdout, exitCode, err := runCmd(ctx, interp, script, dir, cmdEnv)

	// Write to CommandLog regardless of pass/fail.
	if r.opts.CommandLog != nil {
		runLine := fmt.Sprintf("%s%s -c %q", envPrefix(cmdEnv), interp, script)
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
