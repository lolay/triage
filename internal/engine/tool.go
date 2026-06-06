package engine

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"time"

	semver "github.com/Masterminds/semver/v3"
)

// probeOverride holds a custom probe command and an optional version extractor
// for tools that don't follow the conventional `<tool> --version` output.
type probeOverride struct {
	// args replaces the default []string{"--version"}.
	args []string
	// extract pulls the semver token from the output line; nil means the default
	// firstSemverToken extractor is used.
	extract func(line string) string
}

// probeOverrides maps a tool name to its probe overrides.
var probeOverrides = map[string]probeOverride{
	// `go version` → "go version go1.26.3 linux/amd64"
	"go": {
		args: []string{"version"},
		extract: func(line string) string {
			// strip the "go" prefix: "go1.26.3" → "1.26.3"
			re := regexp.MustCompile(`\bgo(\d+\.\d+(?:\.\d+)?)\b`)
			if m := re.FindStringSubmatch(line); m != nil {
				return m[1]
			}
			return ""
		},
	},
	// `node --version` → "v20.11.0" — standard but has a leading 'v'
	"node": {
		args: []string{"--version"},
	},
}

// firstSemverToken extracts the first semver-like token (e.g. "1.26.3",
// "v1.26.3", "1.26") from a line.
func firstSemverToken(line string) string {
	re := regexp.MustCompile(`v?(\d+\.\d+(?:\.\d+)?)`)
	if m := re.FindStringSubmatch(line); m != nil {
		return m[1]
	}
	return ""
}

// RunnerOpts holds injected functions for testability.
type RunnerOpts struct {
	// LookPath resolves a tool name to its path; defaults to exec.LookPath.
	LookPath func(name string) (string, error)
	// RunProbe executes a probe command and returns combined stdout+stderr;
	// defaults to a real subprocess with a 10s timeout.
	RunProbe func(ctx context.Context, name string, args []string) (string, error)
}

func defaultLookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func defaultRunProbe(_ context.Context, name string, args []string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	return string(out), err
}

// checkTool runs a single tool check: presence + optional version constraint.
//
// Lookup and probing are done via the injected opts functions so unit tests can
// run without real executables.
func checkTool(ctx context.Context, name, constraint, hint string, opts RunnerOpts) Result {
	lookPath := opts.LookPath
	if lookPath == nil {
		lookPath = defaultLookPath
	}
	runProbe := opts.RunProbe
	if runProbe == nil {
		runProbe = defaultRunProbe
	}

	_, err := lookPath(name)
	if err != nil {
		msg := fmt.Sprintf("%s not found", name)
		if hint != "" {
			msg += " — " + hint
		}
		return Result{
			Label:    name,
			Severity: SeverityError,
			Pass:     false,
			Message:  msg,
		}
	}

	if constraint == "" {
		return Result{
			Label:    name,
			Severity: SeverityError,
			Pass:     true,
			Message:  name,
		}
	}

	parsed, err := semver.NewConstraint(constraint)
	if err != nil {
		return Result{
			Label:    name,
			Severity: SeverityError,
			Pass:     false,
			Message:  fmt.Sprintf("%s: invalid constraint %q: %v", name, constraint, err),
		}
	}

	override, hasOverride := probeOverrides[name]
	probeArgs := []string{"--version"}
	if hasOverride && override.args != nil {
		probeArgs = override.args
	}
	extract := firstSemverToken
	if hasOverride && override.extract != nil {
		extract = override.extract
	}

	raw, _ := runProbe(ctx, name, probeArgs)
	versionStr := ""
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if tok := extract(line); tok != "" {
			versionStr = tok
			break
		}
	}

	if versionStr == "" {
		msg := fmt.Sprintf("%s: could not parse version from probe output", name)
		if hint != "" {
			msg += " — " + hint
		}
		return Result{
			Label:    name,
			Severity: SeverityError,
			Pass:     false,
			Message:  msg,
		}
	}

	// Strip leading 'v' for Masterminds/semver.
	cleanVer := strings.TrimPrefix(versionStr, "v")
	ver, err := semver.NewVersion(cleanVer)
	if err != nil {
		msg := fmt.Sprintf("%s: could not parse version %q: %v", name, versionStr, err)
		return Result{
			Label:    name,
			Severity: SeverityError,
			Pass:     false,
			Message:  msg,
		}
	}

	if parsed.Check(ver) {
		return Result{
			Label:    name,
			Severity: SeverityError,
			Pass:     true,
			Message:  fmt.Sprintf("%s %s (%s)", name, cleanVer, constraint),
		}
	}

	msg := fmt.Sprintf("%s %s — need %s", name, cleanVer, constraint)
	if hint != "" {
		msg += " — " + hint
	}
	return Result{
		Label:    name,
		Severity: SeverityError,
		Pass:     false,
		Message:  msg,
	}
}

// devNull is an io.Writer that discards all output (subprocess I/O default).
var devNull io.Writer = io.Discard
