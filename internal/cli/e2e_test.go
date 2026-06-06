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
	wantExit int
	noGolden bool // skip golden stdout comparison; just assert the exit code
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
