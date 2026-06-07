package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"time"

	semver "github.com/Masterminds/semver/v3"
)

// captureCap is the maximum bytes captured from a subprocess's combined output
// for assertion scanning and --verbose replay (spec §5, 256 KiB per check).
const captureCap = 256 << 10

// probeOverride holds a custom probe command and an optional version extractor
// for tools that don't follow the conventional `<tool> --version` output.
type probeOverride struct {
	// extract pulls the semver token from the output line; nil means the
	// default firstSemverToken extractor is used.
	extract func(line string) string
	// args replaces the default []string{"--version"}.
	args []string
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

	// GOOS is the OS for platform: guards ("macos"/"linux"/"windows").
	// Empty defaults to CurrentPlatform() (runtime.GOOS, darwin→macos).
	GOOS string
	// LookupEnv looks up an environment variable; defaults to os.LookupEnv.
	LookupEnv func(string) (string, bool)
	// BaseDir is the base directory for resolving relative path/dir values.
	// Defaults to the directory of the loaded config file.
	BaseDir string
	// RunCommand executes an explicitly-interpreted command snippet and returns
	// bounded captured stdout, the exit code, and any exec error. Defaults to
	// a real subprocess via the named interpreter with stream-to-discard I/O.
	RunCommand func(ctx context.Context, interp, script, dir string, env map[string]string) (stdout string, exitCode int, err error)
	// CommandLog receives per-probe output blocks when --command-log is active.
	CommandLog *CommandLog
	// Profile is the active profile name, used for {{profile}} expansion.
	Profile string
	// ConfigVars holds the per-config vars: block. A delegated child uses its
	// own ConfigVars (never the parent's), so config vars don't leak across a
	// delegate boundary.
	ConfigVars map[string]string
	// CLIVars holds global --var overrides; they apply to every config in the
	// delegate tree and win over ConfigVars. Built-ins (profile/os) are injected
	// at runtime and win over both.
	CLIVars map[string]string
	// ConfigPath is the absolute path of the entry config file. It seeds the
	// delegate cycle-detection set so a child delegating back to the root config
	// is caught.
	ConfigPath string
	// Jobs is the max number of concurrent subprocess spawns (the bounded worker
	// pool size). 0 means auto = min(NumCPU, 8). 1 forces the dedicated
	// sequential path. The whole delegate tree shares one budget.
	Jobs int
}

// CurrentPlatform returns the normalised runtime OS string: darwin → "macos";
// all other GOOS values ("linux", "windows", …) are returned as-is.
//
// The env var TRIAGE_TEST_GOOS overrides the runtime value so tests can pin a
// specific OS without forking a subprocess or recompiling.
func CurrentPlatform() string {
	if override := os.Getenv("TRIAGE_TEST_GOOS"); override != "" {
		return override
	}
	if runtime.GOOS == "darwin" {
		return "macos"
	}
	return runtime.GOOS
}

func defaultLookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func defaultRunProbe(ctx context.Context, name string, args []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if len(out) > captureCap {
		out = out[:captureCap]
	}
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
	for line := range strings.SplitSeq(raw, "\n") {
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
