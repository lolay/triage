package engine

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	require.Len(t, results, 1, "want 1 result: %+v", results)
	assert.True(t, results[0].Pass, "want pass on exit 0: %+v", results)
	assert.Equal(t, "truthy", results[0].Label)
}

func TestCommand_ExitNonZeroFail(t *testing.T) {
	r := runnerWithCmd(map[string]fakeCmd{"sh:false": {exitCode: 1}})
	results := r.Run(config.Profile{{
		Type:  config.TypeCommand,
		Value: "false",
		Label: "falsy",
		Hint:  "fix it",
	}})
	require.NotEmpty(t, results)
	assert.False(t, results[0].Pass, "want fail on exit 1")
	assert.Contains(t, results[0].Message, "exit 1", "message should mention exit code")
	assert.Contains(t, results[0].Message, "fix it", "hint should appear in message")
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
	require.NotEmpty(t, results)
	assert.True(t, results[0].Pass, "want pass when exit matches expected %d: %s", wantExit, results[0].Message)
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
	require.NotEmpty(t, results)
	assert.False(t, results[0].Pass, "want fail when exit doesn't match expected")
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
	require.NotEmpty(t, results)
	assert.True(t, results[0].Pass, "want pass (stdout contains Xcode): %s", results[0].Message)
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
	require.NotEmpty(t, results)
	assert.False(t, results[0].Pass, "want fail (stdout lacks Xcode)")
	assert.Contains(t, results[0].Message, "does not contain", "message should say does not contain")
	assert.Contains(t, results[0].Message, "switch to Xcode.app", "hint should appear")
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
	require.NotEmpty(t, results)
	assert.True(t, results[0].Pass, "want pass — stdout matches Xcode 26+: %s", results[0].Message)
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
	require.NotEmpty(t, results)
	assert.False(t, results[0].Pass, "want fail — Xcode 15 doesn't match >=26")
	assert.Contains(t, results[0].Message, "does not match", "message should say does not match")
	assert.Contains(t, results[0].Message, "upgrade Xcode", "hint should appear")
}

func TestCommand_BadMatchesRegex(t *testing.T) {
	r := runnerWithCmd(map[string]fakeCmd{"sh:cmd": {stdout: "output", exitCode: 0}})
	results := r.Run(config.Profile{{
		Type:    config.TypeCommand,
		Value:   "cmd",
		Label:   "bad regex",
		Matches: `[invalid`,
	}})
	require.NotEmpty(t, results)
	assert.False(t, results[0].Pass, "want fail on bad regex")
	assert.Contains(t, results[0].Message, "invalid matches regex", "message should mention bad regex")
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
	require.NotEmpty(t, results)
	assert.True(t, results[0].Pass, "want pass after {{profile}} expansion: %s", results[0].Message)
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
	require.NotEmpty(t, results)
	assert.True(t, results[0].Pass, "want pass with pwsh interp: %s", results[0].Message)
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
	require.NotEmpty(t, results)
	assert.False(t, results[0].Pass, "want fail when output hits cap")
	assert.Contains(t, results[0].Message, "exceeded", "message should mention exceeded cap")
	assert.Contains(t, results[0].Message, "--command-log", "message should mention --command-log")
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
	require.NotEmpty(t, results)
	assert.True(t, results[0].Pass, "want pass: %s", results[0].Message)
	assert.Equal(t, "bar", got["FOO"], "env FOO")
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
	assert.Equal(t, "release", got["MODE"], "MODE")
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
	assert.Equal(t, "1", got["ALPHA"], "env = %#v", got)
	assert.Equal(t, "2", got["BETA"], "env = %#v", got)
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
	require.NoError(t, cl.Close())

	content, err := os.ReadFile(logPath)
	require.NoError(t, err, "reading log")
	assert.Contains(t, string(content), "FOO=bar sh -c", "log should contain env prefix")
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
	require.NoError(t, err, "defaultRunCommand")
	require.Equal(t, 0, exitCode, "exit")
	assert.Equal(t, "injected", strings.TrimSpace(stdout), "stdout")
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
		assert.Equalf(t, tc.want, shellQuote(tc.in), "shellQuote(%q)", tc.in)
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
	assert.True(t, found, "mergeEnv missing FOO=new: %v", got)
}

// ── one-line excerpt ──────────────────────────────────────────────────────────

func TestOneLineExcerpt_Empty(t *testing.T) {
	assert.Empty(t, oneLineExcerpt(""), "empty input should return empty")
}

func TestOneLineExcerpt_SingleLine(t *testing.T) {
	assert.Equal(t, "hello", oneLineExcerpt("hello"))
}

func TestOneLineExcerpt_MultiLine(t *testing.T) {
	assert.Equal(t, "first", oneLineExcerpt("first\nsecond\nthird"))
}

func TestOneLineExcerpt_Truncate(t *testing.T) {
	long := strings.Repeat("a", 90)
	got := oneLineExcerpt(long)
	assert.LessOrEqual(t, len(got), 83, "excerpt too long") // 80 chars + 3-byte "…"
	assert.True(t, strings.HasSuffix(got, "…"), "should end with ellipsis: %q", got)
}

// ── CommandLog ────────────────────────────────────────────────────────────────

func TestCommandLog_NilSafe(t *testing.T) {
	var cl *CommandLog
	cl.WriteBlock("lbl", "/dir", "sh -c true", nil)
	assert.NoError(t, cl.Close(), "nil Close should not error")
	assert.Empty(t, cl.Path(), "nil Path should return empty string")
}

func TestCommandLog_WriteAndClose(t *testing.T) {
	path := t.TempDir() + "/commands.log"
	cl, err := OpenCommandLog(path)
	require.NoError(t, err, "OpenCommandLog")
	cl.WriteBlock("my label", "/some/dir", "sh -c echo hi", []byte("hi\n"))
	require.NoError(t, cl.Close(), "Close")
	content, err := os.ReadFile(path)
	require.NoError(t, err, "reading log")
	assert.Contains(t, string(content), "my label", "log should contain label")
	assert.Contains(t, string(content), "hi", "log should contain output")
}

func TestCommandLog_Truncated(t *testing.T) {
	path := t.TempDir() + "/commands.log"

	cl1, _ := OpenCommandLog(path)
	cl1.WriteBlock("run1", "", "cmd", []byte("first run output"))
	require.NoError(t, cl1.Close(), "Close cl1")

	// Second open should truncate, not append.
	cl2, _ := OpenCommandLog(path)
	cl2.WriteBlock("run2", "", "cmd", []byte("second run output"))
	require.NoError(t, cl2.Close(), "Close cl2")

	content, _ := os.ReadFile(path)
	assert.NotContains(t, string(content), "first run output", "second open should truncate")
	assert.Contains(t, string(content), "second run output", "second run output missing")
}

func TestCommandLog_MkdirParents(t *testing.T) {
	path := t.TempDir() + "/deep/nested/commands.log"
	cl, err := OpenCommandLog(path)
	require.NoError(t, err, "OpenCommandLog should create parent dirs")
	require.NoError(t, cl.Close(), "Close")
	_, err = os.Stat(path)
	assert.NoError(t, err, "log file should exist")
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
	require.NoError(t, cl.Close(), "Close")

	content, err := os.ReadFile(logPath)
	require.NoError(t, err, "reading log")
	assert.Contains(t, string(content), "greet", "log should contain label 'greet'")
	assert.Contains(t, string(content), "hello", "log should contain output 'hello'")
}
