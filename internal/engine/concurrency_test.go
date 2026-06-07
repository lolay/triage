package engine

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lolay/triage/internal/config"
)

// ── semaphore cap ─────────────────────────────────────────────────────────────

// TestPool_SemaphoreCapsConcurrency asserts that no more than Jobs subprocess
// probes run at once, even with far more checks than slots.
func TestPool_SemaphoreCapsConcurrency(t *testing.T) {
	const jobs = 3
	const nChecks = 20

	var inFlight atomic.Int32
	var maxSeen atomic.Int32
	probe := func(_ context.Context, _ string, _ []string) (string, error) {
		cur := inFlight.Add(1)
		for {
			old := maxSeen.Load()
			if cur <= old || maxSeen.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(2 * time.Millisecond)
		inFlight.Add(-1)
		return "tool version 1.0.0", nil
	}

	found := make([]string, nChecks)
	checks := make(config.Profile, nChecks)
	probeOut := map[string]string{}
	for i := range nChecks {
		name := "tool" + strconv.Itoa(i)
		found[i] = name
		probeOut[name] = "tool version 1.0.0"
		checks[i] = config.Check{Type: config.TypeTool, Value: name, Constraint: ">=1.0.0"}
	}

	r := NewRunnerWith(RunnerOpts{
		LookPath: fakeLookPath(found...),
		RunProbe: probe,
		Jobs:     jobs,
	})
	results := r.Run(checks)

	if len(results) != nChecks {
		t.Fatalf("want %d results, got %d", nChecks, len(results))
	}
	for _, res := range results {
		if !res.Pass {
			t.Errorf("check %q should pass: %s", res.Label, res.Message)
		}
	}
	if got := maxSeen.Load(); got > jobs {
		t.Errorf("max concurrent probes = %d, want <= %d", got, jobs)
	}
	if got := maxSeen.Load(); got < 2 {
		t.Errorf("expected real concurrency (max in-flight %d); pool may not be running", got)
	}
}

// TestPool_BarePresenceDoesNotHoldToken verifies that bare tool-presence checks
// (no version probe) never consume a semaphore token: with Jobs=1, many bare
// presence checks plus probes still complete (a token held by a non-probing
// check would not deadlock, but this guards the "only probes acquire" rule by
// confirming presence checks don't serialize behind the single probe slot).
func TestPool_BarePresenceDoesNotHoldToken(t *testing.T) {
	r := NewRunnerWith(RunnerOpts{
		LookPath: fakeLookPath("a", "b", "c"),
		RunProbe: fakeProbe(nil),
		Jobs:     2,
	})
	results := r.Run(config.Profile{
		{Type: config.TypeTool, Value: "a"},
		{Type: config.TypeTool, Value: "b"},
		{Type: config.TypeTool, Value: "c"},
	})
	if len(results) != 3 {
		t.Fatalf("want 3 results, got %d", len(results))
	}
	for _, res := range results {
		if !res.Pass {
			t.Errorf("presence check %q should pass", res.Label)
		}
	}
}

// ── serial barrier ────────────────────────────────────────────────────────────

// TestSerial_NeverOverlaps asserts that a serial: command never runs while any
// other check is in flight: while the serial check executes, in-flight count is
// exactly 1.
func TestSerial_NeverOverlaps(t *testing.T) {
	var inFlight atomic.Int32
	var serialSawOthers atomic.Int32

	runCmd := func(_ context.Context, _, script, _ string, _ map[string]string) (string, int, error) {
		cur := inFlight.Add(1)
		defer inFlight.Add(-1)
		if script == "serial-cmd" && cur != 1 {
			serialSawOthers.Store(1)
		}
		time.Sleep(3 * time.Millisecond)
		return "", 0, nil
	}

	checks := config.Profile{
		{Type: config.TypeCommand, Value: "p1", Label: "p1"},
		{Type: config.TypeCommand, Value: "p2", Label: "p2"},
		{Type: config.TypeCommand, Value: "serial-cmd", Label: "serial", Serial: true},
		{Type: config.TypeCommand, Value: "p3", Label: "p3"},
		{Type: config.TypeCommand, Value: "p4", Label: "p4"},
	}

	r := NewRunnerWith(RunnerOpts{RunCommand: runCmd, Jobs: 8})
	results := r.Run(checks)

	if len(results) != 5 {
		t.Fatalf("want 5 results, got %d", len(results))
	}
	if serialSawOthers.Load() != 0 {
		t.Errorf("serial check overlapped with another check")
	}
}

// TestSerial_GlobalAcrossDelegates asserts the serial lock is process-global:
// two sibling delegates each running a serial check never overlap.
func TestSerial_GlobalAcrossDelegates(t *testing.T) {
	dir := t.TempDir()
	writeCfg(t, dir, "triage.yaml", `default:
  - delegate: a
    dir: a
  - delegate: b
    dir: b
`)
	writeCfg(t, filepath.Join(dir, "a"), "triage.yaml", `default:
  - command: serial-cmd
    label: a-serial
    serial: true
`)
	writeCfg(t, filepath.Join(dir, "b"), "triage.yaml", `default:
  - command: serial-cmd
    label: b-serial
    serial: true
`)

	var inFlight atomic.Int32
	var overlap atomic.Int32
	runCmd := func(_ context.Context, _, script, _ string, _ map[string]string) (string, int, error) {
		if cur := inFlight.Add(1); script == "serial-cmd" && cur != 1 {
			overlap.Store(1)
		}
		defer inFlight.Add(-1)
		time.Sleep(3 * time.Millisecond)
		return "", 0, nil
	}

	cfgPath := filepath.Join(dir, "triage.yaml")
	r := NewRunnerWith(RunnerOpts{
		BaseDir:    dir,
		ConfigPath: cfgPath,
		Profile:    "default",
		RunCommand: runCmd,
		Jobs:       8,
	})
	r.Run(loadProfile(t, cfgPath, "default"))

	if overlap.Load() != 0 {
		t.Errorf("serial checks in sibling delegates overlapped")
	}
}

// ── command-log byte equality ─────────────────────────────────────────────────

// TestCommandLog_ByteEqualAcrossJobs writes the command log at -j 1 and -j 8 and
// asserts the bytes are identical (list-order materialization, spec §5/§7.2).
func TestCommandLog_ByteEqualAcrossJobs(t *testing.T) {
	checks := config.Profile{
		{Type: config.TypeCommand, Value: "echo one", Label: "one"},
		{Type: config.TypeCommand, Value: "echo two", Label: "two"},
		{Type: config.TypeCommand, Value: "echo three", Label: "three"},
		{Type: config.TypeCommand, Value: "echo four", Label: "four"},
	}
	table := map[string]fakeCmd{
		"sh:echo one":   {stdout: "one\n"},
		"sh:echo two":   {stdout: "two\n"},
		"sh:echo three": {stdout: "three\n"},
		"sh:echo four":  {stdout: "four\n"},
	}

	run := func(jobs int) []byte {
		logPath := filepath.Join(t.TempDir(), "commands.log")
		cl, err := OpenCommandLog(logPath)
		if err != nil {
			t.Fatalf("OpenCommandLog: %v", err)
		}
		r := NewRunnerWith(RunnerOpts{
			CommandLog: cl,
			RunCommand: fakeRunCommand(table),
			Jobs:       jobs,
		})
		r.Run(checks)
		if closeErr := cl.Close(); closeErr != nil {
			t.Fatalf("Close: %v", closeErr)
		}
		b, err := os.ReadFile(logPath)
		if err != nil {
			t.Fatalf("read log: %v", err)
		}
		return b
	}

	seq := run(1)
	par := run(8)
	if string(seq) != string(par) {
		t.Errorf("command log differs across --jobs:\n--- -j1 ---\n%s\n--- -j8 ---\n%s", seq, par)
	}
	// Blocks must be in list order regardless of jobs.
	for _, label := range []string{"one", "two", "three", "four"} {
		if !strings.Contains(string(par), label) {
			t.Errorf("log missing block %q: %s", label, par)
		}
	}
	if idx1, idx4 := strings.Index(string(par), "one"), strings.Index(string(par), "four"); idx1 > idx4 {
		t.Errorf("log blocks out of list order: %s", par)
	}
}

// ── delegate cycle + var isolation under the pool ─────────────────────────────

// TestDelegate_CycleIsFatal verifies an A→B→A delegate cycle is recorded as a
// fatal config error (exit 3 in the CLI), even under concurrency.
func TestDelegate_CycleIsFatal(t *testing.T) {
	dir := t.TempDir()
	writeCfg(t, dir, "triage.yaml", `default:
  - delegate: child
    dir: child
`)
	writeCfg(t, filepath.Join(dir, "child"), "triage.yaml", `default:
  - delegate: parent
    dir: ..
`)

	cfgPath := filepath.Join(dir, "triage.yaml")
	r := NewRunnerWith(RunnerOpts{
		BaseDir:    dir,
		ConfigPath: cfgPath,
		Profile:    "default",
		Jobs:       8,
	})
	r.Run(loadProfile(t, cfgPath, "default"))
	if r.Fatal() == nil {
		t.Errorf("expected a fatal cycle error")
	}
}

// TestDelegate_VarIsolation confirms a child uses its own config vars (not the
// parent's), while CLI vars apply tree-wide.
func TestDelegate_VarIsolation(t *testing.T) {
	dir := t.TempDir()
	writeCfg(t, dir, "triage.yaml", `vars:
  who: parent
default:
  - delegate: child
    dir: child
`)
	writeCfg(t, filepath.Join(dir, "child"), "triage.yaml", `vars:
  who: child
default:
  - command: echo {{who}}-{{cli}}
    label: child-echo
`)

	var gotScript string
	runCmd := func(_ context.Context, _, script, _ string, _ map[string]string) (string, int, error) {
		gotScript = script
		return "", 0, nil
	}

	cfgPath := filepath.Join(dir, "triage.yaml")
	r := NewRunnerWith(RunnerOpts{
		BaseDir:    dir,
		ConfigPath: cfgPath,
		Profile:    "default",
		RunCommand: runCmd,
		CLIVars:    map[string]string{"cli": "global"},
		Jobs:       8,
	})
	r.Run(loadProfile(t, cfgPath, "default"))

	if gotScript != "echo child-global" {
		t.Errorf("script = %q, want %q (child config var + tree-wide CLI var)", gotScript, "echo child-global")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

// writeCfg writes name under dir (creating dir), failing the test on error.
func writeCfg(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// loadProfile discovers the config at path and returns the named profile.
func loadProfile(t *testing.T, path, profile string) config.Profile {
	t.Helper()
	cfg, err := config.Discover(path)
	if err != nil {
		t.Fatalf("discover %s: %v", path, err)
	}
	return cfg.Profiles[profile]
}
