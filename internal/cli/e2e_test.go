package cli_test

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lolay/triage/internal/cli"
)

// update regenerates golden files when passed to `go test -run TestGolden -update`.
var update = flag.Bool("update", false, "regenerate golden files")

type e2eCase struct {
	name     string
	args     []string // extra args appended after the fixture dir path
	binDir   string   // subdir under testdata/<name> to prepend to PATH; default "bin"
	wantExit int
	noGolden bool              // skip golden stdout comparison; just assert the exit code
	setEnv   map[string]string // env vars to set via t.Setenv before the run
	unsetEnv []string          // env var names to ensure are absent for the run
	goos     string            // sets TRIAGE_TEST_GOOS to pin platform for platform: guards
}

var e2eCases = []e2eCase{
	{
		// empty-config: default: [] → clean board, exit 0.
		name:     "empty-config",
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
}

// TestGolden runs each fixture through ExecuteWith in-process, captures stdout,
// and diffs against testdata/<name>/stdout.golden.
// Pass -update to regenerate all golden files.
func TestGolden(t *testing.T) {
	for _, tc := range e2eCases {
		tc := tc
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
			if err != nil {
				t.Fatalf("abs bin dir: %v", err)
			}
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
				os.Unsetenv(k)
				if existed {
					t.Cleanup(func() { os.Setenv(k, prev) })
				} else {
					t.Cleanup(func() { os.Unsetenv(k) })
				}
			}
			if tc.goos != "" {
				t.Setenv("TRIAGE_TEST_GOOS", tc.goos)
			}

			var stdout, stderr bytes.Buffer
			exitCode := cli.ExecuteWith(args, &stdout, &stderr)

			if exitCode != tc.wantExit {
				t.Errorf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s",
					exitCode, tc.wantExit, stdout.String(), stderr.String())
			}

			if tc.noGolden {
				return
			}

			goldenFile := filepath.Join(fixtureDir, "stdout.golden")
			got := stdout.String()

			if *update {
				if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
					t.Fatalf("mkdir %s: %v", fixtureDir, err)
				}
				if err := os.WriteFile(goldenFile, []byte(got), 0o644); err != nil {
					t.Fatalf("write golden %s: %v", goldenFile, err)
				}
				t.Logf("updated %s", goldenFile)
				return
			}

			wantBytes, err := os.ReadFile(goldenFile)
			if err != nil {
				t.Fatalf("read golden %s: %v\n(run: go test -run TestGolden -update)", goldenFile, err)
			}
			want := string(wantBytes)
			if got != want {
				t.Errorf("stdout mismatch\n--- want ---\n%s\n--- got ---\n%s\n--- diff ---\n%s",
					want, got, lineDiff(got, want))
			}
		})
	}
}

// lineDiff returns a line-by-line diff of got vs want for readable test failures.
func lineDiff(got, want string) string {
	gotLines := strings.Split(got, "\n")
	wantLines := strings.Split(want, "\n")
	n := len(gotLines)
	if len(wantLines) > n {
		n = len(wantLines)
	}
	var sb strings.Builder
	for i := 0; i < n; i++ {
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
