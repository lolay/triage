package engine

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/lolay/triage/internal/config"
)

// fakeCmd describes a scripted RunCommand response.
type fakeCmd struct {
	stdout   string
	exitCode int
}

// fakeRunCommand returns a RunCommand injection that maps "interp:script" → response.
// Unrecognised keys return exit 0, empty output.
func fakeRunCommand(table map[string]fakeCmd) func(context.Context, string, string, string, map[string]string) (string, int, error) {
	return func(_ context.Context, interp, script, _ string, _ map[string]string) (string, int, error) {
		key := interp + ":" + script
		if v, ok := table[key]; ok {
			return v.stdout, v.exitCode, nil
		}
		return "", 0, nil
	}
}

// capturingRunCommand wraps fakeRunCommand and records the env map passed to each call.
func capturingRunCommand(table map[string]fakeCmd, captured *map[string]string) func(context.Context, string, string, string, map[string]string) (string, int, error) {
	base := fakeRunCommand(table)
	return func(ctx context.Context, interp, script, dir string, env map[string]string) (string, int, error) {
		*captured = env
		return base(ctx, interp, script, dir, env)
	}
}

func runnerWithCmd(table map[string]fakeCmd) *Runner {
	return NewRunnerWith(RunnerOpts{RunCommand: fakeRunCommand(table)})
}

// ── exit code assertions ──────────────────────────────────────────────────────

func TestCommand_ExitZeroPass(t *testing.T) {
	r := runnerWithCmd(map[string]fakeCmd{"sh:true": {exitCode: 0}})
	results := r.Run(config.Profile{{
		Type:  config.TypeCommand,
		Value: "true",
		Label: "truthy",
	}})
	if len(results) != 1 || !results[0].Pass {
		t.Errorf("want pass on exit 0: %+v", results)
	}
	if results[0].Label != "truthy" {
		t.Errorf("label = %q, want 'truthy'", results[0].Label)
	}
}

func TestCommand_ExitNonZeroFail(t *testing.T) {
	r := runnerWithCmd(map[string]fakeCmd{"sh:false": {exitCode: 1}})
	results := r.Run(config.Profile{{
		Type:  config.TypeCommand,
		Value: "false",
		Label: "falsy",
		Hint:  "fix it",
	}})
	if results[0].Pass {
		t.Errorf("want fail on exit 1")
	}
	if !strings.Contains(results[0].Message, "exit 1") {
		t.Errorf("message should mention exit code: %q", results[0].Message)
	}
	if !strings.Contains(results[0].Message, "fix it") {
		t.Errorf("hint should appear in message: %q", results[0].Message)
	}
}

func TestCommand_ExplicitExitCode_Pass(t *testing.T) {
	wantExit := 42
	r := runnerWithCmd(map[string]fakeCmd{"sh:myscript": {exitCode: 42}})
	results := r.Run(config.Profile{{
		Type:  config.TypeCommand,
		Value: "myscript",
		Label: "special exit",
		Exit:  &wantExit,
	}})
	if !results[0].Pass {
		t.Errorf("want pass when exit matches expected %d: %s", wantExit, results[0].Message)
	}
}

func TestCommand_ExplicitExitCode_Fail(t *testing.T) {
	wantExit := 2
	r := runnerWithCmd(map[string]fakeCmd{"sh:cmd": {exitCode: 1}})
	results := r.Run(config.Profile{{
		Type:  config.TypeCommand,
		Value: "cmd",
		Label: "needs exit 2",
		Exit:  &wantExit,
	}})
	if results[0].Pass {
		t.Errorf("want fail when exit doesn't match expected")
	}
}

// ── contains assertions ───────────────────────────────────────────────────────

func TestCommand_ContainsPass(t *testing.T) {
	r := runnerWithCmd(map[string]fakeCmd{
		"sh:xcode-select -p": {stdout: "/Applications/Xcode.app/Contents/Developer\n", exitCode: 0},
	})
	results := r.Run(config.Profile{{
		Type:     config.TypeCommand,
		Value:    "xcode-select -p",
		Label:    "Xcode installed",
		Contains: "Xcode",
	}})
	if !results[0].Pass {
		t.Errorf("want pass (stdout contains Xcode): %s", results[0].Message)
	}
}

func TestCommand_ContainsFail(t *testing.T) {
	r := runnerWithCmd(map[string]fakeCmd{
		"sh:xcode-select -p": {stdout: "/Library/Developer/CommandLineTools\n", exitCode: 0},
	})
	results := r.Run(config.Profile{{
		Type:     config.TypeCommand,
		Value:    "xcode-select -p",
		Label:    "Xcode installed",
		Contains: "Xcode",
		Hint:     "switch to Xcode.app",
	}})
	if results[0].Pass {
		t.Errorf("want fail (stdout lacks Xcode)")
	}
	if !strings.Contains(results[0].Message, "does not contain") {
		t.Errorf("message should say does not contain: %q", results[0].Message)
	}
	if !strings.Contains(results[0].Message, "switch to Xcode.app") {
		t.Errorf("hint should appear: %q", results[0].Message)
	}
}

// ── matches assertions ────────────────────────────────────────────────────────

func TestCommand_MatchesPass(t *testing.T) {
	r := runnerWithCmd(map[string]fakeCmd{
		"sh:xcodebuild -version": {stdout: "Xcode 26.0\nBuild version 26A242\n", exitCode: 0},
	})
	results := r.Run(config.Profile{{
		Type:    config.TypeCommand,
		Value:   "xcodebuild -version",
		Label:   "Xcode 26+",
		Matches: `Xcode (2[6-9]|[3-9][0-9])`,
	}})
	if !results[0].Pass {
		t.Errorf("want pass — stdout matches Xcode 26+: %s", results[0].Message)
	}
}

func TestCommand_MatchesFail(t *testing.T) {
	r := runnerWithCmd(map[string]fakeCmd{
		"sh:xcodebuild -version": {stdout: "Xcode 15.4\n", exitCode: 0},
	})
	results := r.Run(config.Profile{{
		Type:    config.TypeCommand,
		Value:   "xcodebuild -version",
		Label:   "Xcode 26+",
		Matches: `Xcode (2[6-9]|[3-9][0-9])`,
		Hint:    "upgrade Xcode",
	}})
	if results[0].Pass {
		t.Errorf("want fail — Xcode 15 doesn't match >=26")
	}
	if !strings.Contains(results[0].Message, "does not match") {
		t.Errorf("message should say does not match: %q", results[0].Message)
	}
	if !strings.Contains(results[0].Message, "upgrade Xcode") {
		t.Errorf("hint should appear: %q", results[0].Message)
	}
}

func TestCommand_BadMatchesRegex(t *testing.T) {
	r := runnerWithCmd(map[string]fakeCmd{"sh:cmd": {stdout: "output", exitCode: 0}})
	results := r.Run(config.Profile{{
		Type:    config.TypeCommand,
		Value:   "cmd",
		Label:   "bad regex",
		Matches: `[invalid`,
	}})
	if results[0].Pass {
		t.Errorf("want fail on bad regex")
	}
	if !strings.Contains(results[0].Message, "invalid matches regex") {
		t.Errorf("message should mention bad regex: %q", results[0].Message)
	}
}

// ── {{profile}} expansion ─────────────────────────────────────────────────────

func TestCommand_ProfileExpansion(t *testing.T) {
	r := NewRunnerWith(RunnerOpts{
		Profile:    "release",
		RunCommand: fakeRunCommand(map[string]fakeCmd{"sh:make doctor MODE=release": {exitCode: 0}}),
	})
	results := r.Run(config.Profile{{
		Type:  config.TypeCommand,
		Value: "make doctor MODE={{profile}}",
		Label: "doctor",
	}})
	if !results[0].Pass {
		t.Errorf("want pass after {{profile}} expansion: %s", results[0].Message)
	}
}

// ── interp selection ──────────────────────────────────────────────────────────

func TestCommand_InterpPwsh(t *testing.T) {
	r := NewRunnerWith(RunnerOpts{
		RunCommand: fakeRunCommand(map[string]fakeCmd{"pwsh:Get-Date": {exitCode: 0}}),
	})
	results := r.Run(config.Profile{{
		Type:   config.TypeCommand,
		Value:  "Get-Date",
		Label:  "date",
		Interp: "pwsh",
	}})
	if !results[0].Pass {
		t.Errorf("want pass with pwsh interp: %s", results[0].Message)
	}
}

// ── over-cap behavior ─────────────────────────────────────────────────────────

func TestCommand_OverCapFail(t *testing.T) {
	huge := strings.Repeat("x", captureCap) // exactly captureCap bytes
	r := NewRunnerWith(RunnerOpts{
		RunCommand: func(_ context.Context, _, _, _ string, _ map[string]string) (string, int, error) {
			return huge, 0, nil
		},
	})
	results := r.Run(config.Profile{{
		Type:     config.TypeCommand,
		Value:    "noisycmd",
		Label:    "noisy",
		Contains: "target",
	}})
	if results[0].Pass {
		t.Errorf("want fail when output hits cap")
	}
	if !strings.Contains(results[0].Message, "exceeded") {
		t.Errorf("message should mention exceeded cap: %q", results[0].Message)
	}
	if !strings.Contains(results[0].Message, "--command-log") {
		t.Errorf("message should mention --command-log: %q", results[0].Message)
	}
}

// ── with_env ──────────────────────────────────────────────────────────────────

func TestCommand_WithEnv_Injected(t *testing.T) {
	var got map[string]string
	r := NewRunnerWith(RunnerOpts{
		RunCommand: capturingRunCommand(map[string]fakeCmd{"sh:echo hi": {stdout: "hi\n", exitCode: 0}}, &got),
	})
	results := r.Run(config.Profile{{
		Type:    config.TypeCommand,
		Value:   "echo hi",
		Label:   "greet",
		WithEnv: map[string]string{"FOO": "bar"},
	}})
	if !results[0].Pass {
		t.Errorf("want pass: %s", results[0].Message)
	}
	if got["FOO"] != "bar" {
		t.Errorf("env FOO = %q, want bar", got["FOO"])
	}
}

func TestCommand_WithEnv_ProfileExpansionInValue(t *testing.T) {
	var got map[string]string
	r := NewRunnerWith(RunnerOpts{
		Profile:    "release",
		RunCommand: capturingRunCommand(map[string]fakeCmd{"sh:cmd": {exitCode: 0}}, &got),
	})
	r.Run(config.Profile{{
		Type:    config.TypeCommand,
		Value:   "cmd",
		Label:   "mode",
		WithEnv: map[string]string{"MODE": "{{profile}}"},
	}})
	if got["MODE"] != "release" {
		t.Errorf("MODE = %q, want release", got["MODE"])
	}
}

func TestCommand_WithEnv_MultipleKeys(t *testing.T) {
	var got map[string]string
	r := NewRunnerWith(RunnerOpts{
		RunCommand: capturingRunCommand(map[string]fakeCmd{"sh:cmd": {exitCode: 0}}, &got),
	})
	r.Run(config.Profile{{
		Type:  config.TypeCommand,
		Value: "cmd",
		Label: "multi",
		WithEnv: map[string]string{
			"ALPHA": "1",
			"BETA":  "2",
		},
	}})
	if got["ALPHA"] != "1" || got["BETA"] != "2" {
		t.Errorf("env = %#v, want ALPHA=1 BETA=2", got)
	}
}

func TestCommand_WithEnv_CommandLogPrefix(t *testing.T) {
	logPath := t.TempDir() + "/commands.log"
	cl, _ := OpenCommandLog(logPath)
	defer func() { _ = cl.Close() }()

	r := NewRunnerWith(RunnerOpts{
		CommandLog: cl,
		RunCommand: fakeRunCommand(map[string]fakeCmd{"sh:echo hi": {stdout: "hi\n", exitCode: 0}}),
	})
	r.Run(config.Profile{{
		Type:    config.TypeCommand,
		Value:   "echo hi",
		Label:   "greet",
		WithEnv: map[string]string{"FOO": "bar"},
	}})
	if err := cl.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading log: %v", err)
	}
	if !strings.Contains(string(content), "FOO=bar sh -c") {
		t.Errorf("log should contain env prefix: %q", string(content))
	}
}

func TestDefaultRunCommand_WithEnvOverridesInherited(t *testing.T) {
	t.Setenv("TRIAGE_OVERRIDE_TEST", "inherited")
	stdout, exitCode, err := defaultRunCommand(
		t.Context(),
		"sh",
		`echo "$TRIAGE_OVERRIDE_TEST"`,
		"",
		map[string]string{"TRIAGE_OVERRIDE_TEST": "injected"},
	)
	if err != nil {
		t.Fatalf("defaultRunCommand: %v", err)
	}
	if exitCode != 0 {
		t.Fatalf("exit = %d, want 0", exitCode)
	}
	if strings.TrimSpace(stdout) != "injected" {
		t.Errorf("stdout = %q, want injected", stdout)
	}
}

func TestShellQuote(t *testing.T) {
	cases := []struct{ in, want string }{
		{"simple", "simple"},
		{"with space", "'with space'"},
		{"it's", `'it'\''s'`},
		{"PATH=/usr/local/bin", "'PATH=/usr/local/bin'"},
		{"plain123", "plain123"},
		{"under_score", "under_score"},
	}
	for _, tc := range cases {
		if got := shellQuote(tc.in); got != tc.want {
			t.Errorf("shellQuote(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMergeEnv_LastKeyWins(t *testing.T) {
	got := mergeEnv([]string{"FOO=old"}, map[string]string{"FOO": "new"})
	found := false
	for _, e := range got {
		if e == "FOO=new" {
			found = true
		}
		if e == "FOO=old" && found {
			t.Error("FOO=new should appear after FOO=old so override wins")
		}
	}
	if !found {
		t.Errorf("mergeEnv missing FOO=new: %v", got)
	}
}

// ── one-line excerpt ──────────────────────────────────────────────────────────

func TestOneLineExcerpt_Empty(t *testing.T) {
	if oneLineExcerpt("") != "" {
		t.Error("empty input should return empty")
	}
}

func TestOneLineExcerpt_SingleLine(t *testing.T) {
	if got := oneLineExcerpt("hello"); got != "hello" {
		t.Errorf("got %q, want 'hello'", got)
	}
}

func TestOneLineExcerpt_MultiLine(t *testing.T) {
	if got := oneLineExcerpt("first\nsecond\nthird"); got != "first" {
		t.Errorf("got %q, want 'first'", got)
	}
}

func TestOneLineExcerpt_Truncate(t *testing.T) {
	long := strings.Repeat("a", 90)
	got := oneLineExcerpt(long)
	if len(got) > 83 { // 80 chars + 3-byte "…"
		t.Errorf("excerpt too long: %d chars", len(got))
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("should end with ellipsis: %q", got)
	}
}

// ── CommandLog ────────────────────────────────────────────────────────────────

func TestCommandLog_NilSafe(t *testing.T) {
	var cl *CommandLog
	cl.WriteBlock("lbl", "/dir", "sh -c true", nil)
	if closeErr := cl.Close(); closeErr != nil {
		t.Errorf("nil Close should not error: %v", closeErr)
	}
	if cl.Path() != "" {
		t.Error("nil Path should return empty string")
	}
}

func TestCommandLog_WriteAndClose(t *testing.T) {
	path := t.TempDir() + "/commands.log"
	cl, err := OpenCommandLog(path)
	if err != nil {
		t.Fatalf("OpenCommandLog: %v", err)
	}
	cl.WriteBlock("my label", "/some/dir", "sh -c echo hi", []byte("hi\n"))
	if closeErr := cl.Close(); closeErr != nil {
		t.Fatalf("Close: %v", closeErr)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log: %v", err)
	}
	if !strings.Contains(string(content), "my label") {
		t.Errorf("log should contain label: %q", string(content))
	}
	if !strings.Contains(string(content), "hi") {
		t.Errorf("log should contain output: %q", string(content))
	}
}

func TestCommandLog_Truncated(t *testing.T) {
	path := t.TempDir() + "/commands.log"

	cl1, _ := OpenCommandLog(path)
	cl1.WriteBlock("run1", "", "cmd", []byte("first run output"))
	if err := cl1.Close(); err != nil {
		t.Fatalf("Close cl1: %v", err)
	}

	// Second open should truncate, not append.
	cl2, _ := OpenCommandLog(path)
	cl2.WriteBlock("run2", "", "cmd", []byte("second run output"))
	if err := cl2.Close(); err != nil {
		t.Fatalf("Close cl2: %v", err)
	}

	content, _ := os.ReadFile(path)
	if strings.Contains(string(content), "first run output") {
		t.Errorf("second open should truncate; first run content found: %q", string(content))
	}
	if !strings.Contains(string(content), "second run output") {
		t.Errorf("second run output missing: %q", string(content))
	}
}

func TestCommandLog_MkdirParents(t *testing.T) {
	path := t.TempDir() + "/deep/nested/commands.log"
	cl, err := OpenCommandLog(path)
	if err != nil {
		t.Fatalf("OpenCommandLog should create parent dirs: %v", err)
	}
	if err := cl.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("log file should exist: %v", err)
	}
}

// ── command + CommandLog integration ─────────────────────────────────────────

func TestCommand_WritesCommandLog(t *testing.T) {
	logPath := t.TempDir() + "/commands.log"
	cl, _ := OpenCommandLog(logPath)
	defer func() { _ = cl.Close() }()

	r := NewRunnerWith(RunnerOpts{
		CommandLog: cl,
		RunCommand: fakeRunCommand(map[string]fakeCmd{
			"sh:echo hello": {stdout: "hello\n", exitCode: 0},
		}),
	})
	r.Run(config.Profile{{
		Type:  config.TypeCommand,
		Value: "echo hello",
		Label: "greet",
	}})
	if err := cl.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("reading log: %v", err)
	}
	if !strings.Contains(string(content), "greet") {
		t.Errorf("log should contain label 'greet': %q", string(content))
	}
	if !strings.Contains(string(content), "hello") {
		t.Errorf("log should contain output 'hello': %q", string(content))
	}
}
