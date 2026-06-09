package cli_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lolay/triage/internal/cli"
)

// update regenerates golden files when passed to `go test -run TestGolden -update`.
var update = flag.Bool("update", false, "regenerate golden files")

type e2eCase struct {
	setEnv   map[string]string
	name     string
	binDir   string
	goos     string
	args     []string
	unsetEnv []string
	wantExit int
	noGolden bool
}

var e2eCases = []e2eCase{
	{
		// empty-config: default: [] → clean board, exit 0.
		name:     "empty-config",
		wantExit: 0,
	},
	{
		// yml-config: discovery accepts the triage.yml spelling, not just
		// triage.yaml. default: [] → clean board, exit 0.
		name:     "yml-config",
		wantExit: 0,
	},
	{
		// not-found: directory exists but contains no triage.yaml → exit 3.
		name:     "not-found",
		wantExit: 3,
		noGolden: true,
	},
	{
		// passing-grouped: structural group with two tools (one version-checked).
		// bin/fake-git and bin/fake-go are the fake executables on PATH.
		name:     "passing-grouped",
		args:     []string{"--no-color"},
		wantExit: 0,
	},
	{
		// warn-board: one tool present, one warn-severity tool missing → exit 0
		// (pass-fail default mode: warn does not fail).
		name:     "warn-board",
		args:     []string{"--no-color"},
		wantExit: 0,
	},
	{
		// missing-tool: one tool present, one required tool missing → exit 1.
		name:     "missing-tool",
		args:     []string{"--no-color"},
		wantExit: 1,
	},

	// ── m3 fixtures ───────────────────────────────────────────────────────────

	{
		// env-checks: set/matches/unset/missing env vars.
		// TRIAGE_SET_VAR and TRIAGE_MATCH_VAR are set; TRIAGE_UNSET_VAR is
		// absent (unset: true → pass); TRIAGE_MISSING_VAR is absent (set mode
		// → fail) → exit 1.
		name:     "env-checks",
		args:     []string{"--no-color"},
		wantExit: 1,
		setEnv: map[string]string{
			"TRIAGE_SET_VAR":   "foo",
			"TRIAGE_MATCH_VAR": "hello-world",
		},
		unsetEnv: []string{"TRIAGE_UNSET_VAR", "TRIAGE_MISSING_VAR"},
	},
	{
		// path-checks: present file, glob match, and missing path → exit 1.
		name:     "path-checks",
		args:     []string{"--no-color"},
		wantExit: 1,
	},
	{
		// one-of: first case passes (fake-alpha found), second fails (both
		// missing) → exit 1.
		name:     "one-of",
		args:     []string{"--no-color"},
		wantExit: 1,
	},
	{
		// platform-skip: pinned to linux; the macos group is omitted entirely.
		// Only the linux tool (fake-linux-tool) appears → exit 0.
		name:     "platform-skip",
		args:     []string{"--no-color"},
		wantExit: 0,
		goos:     "linux",
	},
	{
		// command-checks: echo pass, matches pass, exit-fail → exit 1.
		name:     "command-checks",
		args:     []string{"--no-color"},
		wantExit: 1,
	},
	{
		// realworld: composite with group/env/one_of/command/platform using fake
		// bin scripts. Pinned to macos so the Xcode group runs.
		name:     "realworld",
		args:     []string{"--no-color"},
		wantExit: 0,
		goos:     "macos",
		setEnv: map[string]string{
			"TRIAGE_FAKE_TOKEN": "abc",
			"TRIAGE_FAKE_KEY":   "secret",
		},
	},
	{
		// vars: template expansion in tool name, with_env, profile builtin;
		// unknown {{ typo }} fails → exit 1.
		name:     "vars",
		args:     []string{"--no-color"},
		wantExit: 1,
	},

	// ── m4 fixtures ───────────────────────────────────────────────────────────

	{
		// delegate-tree: nested delegates with groups, one failing leaf.
		name:     "delegate-tree",
		args:     []string{"--no-color"},
		wantExit: 1,
	},
	{
		// delegate-cycle: A→B→A re-enters a config on the ancestor chain → exit 3.
		name:     "delegate-cycle",
		args:     []string{"--no-color"},
		wantExit: 3,
		noGolden: true,
	},
}

// TestGolden runs each fixture through ExecuteWith in-process, captures stdout,
// and diffs against testdata/<name>/stdout.golden.
// Pass -update to regenerate golden files.
func TestGolden(t *testing.T) {
	for _, tc := range e2eCases {
		t.Run(tc.name, func(t *testing.T) {
			fixtureDir := filepath.Join("testdata", tc.name)
			args := append([]string{fixtureDir}, tc.args...)

			// Prepend the fixture's bin/ directory to PATH so fake executables
			// are found by exec.LookPath in the runner.
			binDir := tc.binDir
			if binDir == "" {
				binDir = "bin"
			}
			fakeBin, err := filepath.Abs(filepath.Join(fixtureDir, binDir))
			require.NoError(t, err, "abs bin dir")
			// Only prepend if the dir exists (empty-config / not-found have no bin/).
			oldPath := os.Getenv("PATH")
			if fi, err := os.Stat(fakeBin); err == nil && fi.IsDir() {
				t.Setenv("PATH", fakeBin+string(os.PathListSeparator)+oldPath)
			}

			// Apply per-case env overrides.
			for k, v := range tc.setEnv {
				t.Setenv(k, v)
			}
			for _, k := range tc.unsetEnv {
				prev, existed := os.LookupEnv(k)
				require.NoErrorf(t, os.Unsetenv(k), "Unsetenv(%q)", k)
				if existed {
					t.Cleanup(func() { _ = os.Setenv(k, prev) })
				} else {
					t.Cleanup(func() { _ = os.Unsetenv(k) })
				}
			}
			if tc.goos != "" {
				t.Setenv("TRIAGE_TEST_GOOS", tc.goos)
			}

			goldenFile := filepath.Join(fixtureDir, "stdout.golden")

			if *update {
				var stdout, stderr bytes.Buffer
				exitCode := cli.ExecuteWith(args, &stdout, &stderr)
				assert.Equalf(t, tc.wantExit, exitCode, "exit code\nstderr:\n%s", stderr.String())
				require.NoErrorf(t, os.MkdirAll(fixtureDir, 0o755), "mkdir %s", fixtureDir)
				require.NoErrorf(t, os.WriteFile(goldenFile, stdout.Bytes(), 0o644), "write golden %s", goldenFile)
				t.Logf("updated %s", goldenFile)
				return
			}

			// --jobs invariance (spec §7.2): run the fixture at -j 1 (the
			// dedicated sequential oracle) and -j 8 (the concurrent pool); both
			// must produce byte-identical stdout and the same exit code, and
			// must match the golden.
			for _, jobs := range []int{1, 8} {
				t.Run(fmt.Sprintf("jobs=%d", jobs), func(t *testing.T) {
					runArgs := append(append([]string{}, args...), "--jobs", strconv.Itoa(jobs))
					var stdout, stderr bytes.Buffer
					exitCode := cli.ExecuteWith(runArgs, &stdout, &stderr)

					assert.Equalf(t, tc.wantExit, exitCode, "exit code at -j %d\nstdout:\n%s\nstderr:\n%s",
						jobs, stdout.String(), stderr.String())

					if tc.noGolden {
						return
					}

					got := stdout.String()
					wantBytes, err := os.ReadFile(goldenFile)
					require.NoErrorf(t, err, "read golden %s\n(run: go test -run TestGolden -update)", goldenFile)
					want := string(wantBytes)
					assert.Equalf(t, want, got, "stdout mismatch at -j %d\n--- diff ---\n%s", jobs, lineDiff(got, want))
				})
			}
		})
	}
}

// TestJobsFlagValidation asserts that an explicit --jobs below 1 is a usage
// error (exit 3), while the default (auto) and explicit positive values run.
func TestJobsFlagValidation(t *testing.T) {
	fixture := filepath.Join("testdata", "empty-config")
	cases := []struct {
		name     string
		args     []string
		wantExit int
	}{
		{"zero", []string{fixture, "--jobs", "0"}, cli.ExitUsageError},
		{"negative", []string{fixture, "-j", "-2"}, cli.ExitUsageError},
		{"one", []string{fixture, "--jobs", "1"}, cli.ExitOK},
		{"eight", []string{fixture, "-j", "8"}, cli.ExitOK},
		{"default", []string{fixture}, cli.ExitOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			got := cli.ExecuteWith(tc.args, &stdout, &stderr)
			assert.Equalf(t, tc.wantExit, got, "stderr: %s", stderr.String())
		})
	}
}

func TestExecuteWith_badVar(t *testing.T) {
	fixture := filepath.Join("testdata", "empty-config")
	var stdout, stderr bytes.Buffer
	got := cli.ExecuteWith([]string{fixture, "--var", "NOEQUALS"}, &stdout, &stderr)
	assert.Equal(t, cli.ExitUsageError, got)
	assert.Contains(t, stderr.String(), "triage:")
}

func TestExecuteWith_validVar(t *testing.T) {
	fixture := filepath.Join("testdata", "vars")
	var stdout, stderr bytes.Buffer
	got := cli.ExecuteWith([]string{fixture, "--no-color", "--var", "echo_msg=cli-override"}, &stdout, &stderr)
	assert.Equal(t, cli.ExitFail, got) // unknown {{ typo }} check still fails
	assert.Contains(t, stdout.String(), "cli-override")
}

func TestExecuteWith_verbose(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "triage.yaml")
	require.NoError(t, os.WriteFile(cfg, []byte(`default:
  - command: 'printf verbose-output'
    label: verbose probe
    matches: 'no-match'
`), 0o644))

	var stdout, stderr bytes.Buffer
	got := cli.ExecuteWith([]string{dir, "--no-color", "--verbose"}, &stdout, &stderr)
	assert.Equal(t, cli.ExitFail, got)
	assert.Contains(t, stderr.String(), "--- verbose probe ---")
	assert.Contains(t, stderr.String(), "verbose-output")
}

func TestExecuteWith_unknownFlag(t *testing.T) {
	var stdout, stderr bytes.Buffer
	got := cli.ExecuteWith([]string{"--no-such-flag"}, &stdout, &stderr)
	assert.Equal(t, cli.ExitUsageError, got)
	assert.Contains(t, stderr.String(), "triage:")
}

// lineDiff returns a line-by-line diff of got vs want for readable test failures.
func lineDiff(got, want string) string {
	gotLines := strings.Split(got, "\n")
	wantLines := strings.Split(want, "\n")
	n := max(len(wantLines), len(gotLines))
	var sb strings.Builder
	for i := range n {
		g, w := "", ""
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			fmt.Fprintf(&sb, "line %d: got %q, want %q\n", i+1, g, w)
		}
	}
	return sb.String()
}
