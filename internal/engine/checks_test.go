package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lolay/triage/internal/config"
)

// ── env check ─────────────────────────────────────────────────────────────────

func fakeEnv(env map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		v, ok := env[name]
		return v, ok
	}
}

func TestCheckEnv_SetMode_Present(t *testing.T) {
	r := checkEnv("FOO", false, "", "", fakeEnv(map[string]string{"FOO": "bar"}))
	assert.True(t, r.Pass, "want pass, got fail: %s", r.Message)
	assert.Contains(t, r.Message, "FOO=bar", "message should contain value")
}

func TestCheckEnv_SetMode_EmptyValueCounts(t *testing.T) {
	r := checkEnv("FOO", false, "", "", fakeEnv(map[string]string{"FOO": ""}))
	assert.True(t, r.Pass, "empty value should count as set: %s", r.Message)
}

func TestCheckEnv_SetMode_Absent(t *testing.T) {
	r := checkEnv("FOO", false, "", "export FOO=x", fakeEnv(nil))
	assert.False(t, r.Pass, "want fail when var is absent")
	assert.Contains(t, r.Message, "not set", "message should say not set")
	assert.Contains(t, r.Message, "export FOO=x", "hint should appear in message")
}

func TestCheckEnv_SetMode_Matches_Pass(t *testing.T) {
	r := checkEnv("FOO", false, `^prod`, "", fakeEnv(map[string]string{"FOO": "production"}))
	assert.True(t, r.Pass, "want pass: value matches regex: %s", r.Message)
}

func TestCheckEnv_SetMode_Matches_Fail(t *testing.T) {
	r := checkEnv("FOO", false, `^prod`, "set FOO=prod-*", fakeEnv(map[string]string{"FOO": "staging"}))
	assert.False(t, r.Pass, "want fail: value does not match regex")
	assert.Contains(t, r.Message, "does not match", "message should mention mismatch")
	assert.Contains(t, r.Message, "set FOO=prod-*", "hint should appear")
}

func TestCheckEnv_SetMode_BadRegex(t *testing.T) {
	r := checkEnv("FOO", false, `[invalid`, "", fakeEnv(map[string]string{"FOO": "x"}))
	assert.False(t, r.Pass, "want fail on bad regex")
	assert.Contains(t, r.Message, "invalid matches regex", "message should mention bad regex")
}

func TestCheckEnv_UnsetMode_Absent(t *testing.T) {
	r := checkEnv("SECRET", true, "", "", fakeEnv(nil))
	assert.True(t, r.Pass, "want pass when var is absent in unset mode: %s", r.Message)
}

func TestCheckEnv_UnsetMode_Present(t *testing.T) {
	r := checkEnv("SECRET", true, "", "", fakeEnv(map[string]string{"SECRET": "oops"}))
	assert.False(t, r.Pass, "want fail when var is set in unset mode")
	assert.Contains(t, r.Message, "expected unset", "message should say expected unset")
}

// ── path check ────────────────────────────────────────────────────────────────

func TestCheckPath_FilePresent(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "present.txt")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0o644))
	r := checkPath("present.txt", dir, "")
	assert.True(t, r.Pass, "want pass for existing file: %s", r.Message)
}

func TestCheckPath_FileAbsent(t *testing.T) {
	dir := t.TempDir()
	r := checkPath("absent.txt", dir, "create it")
	assert.False(t, r.Pass, "want fail for missing file")
	assert.Contains(t, r.Message, "not found", "message should say not found")
	assert.Contains(t, r.Message, "create it", "hint should appear")
}

func TestCheckPath_GlobMatch(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"key1.p8", "key2.p8"} {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte("k"), 0o644))
	}
	r := checkPath("*.p8", dir, "")
	assert.True(t, r.Pass, "want pass for glob matching multiple files: %s", r.Message)
}

func TestCheckPath_GlobNoMatch(t *testing.T) {
	dir := t.TempDir()
	r := checkPath("*.p8", dir, "")
	assert.False(t, r.Pass, "want fail when no files match glob")
}

func TestCheckPath_Absolute(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "abs.txt")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0o644))
	// Absolute pattern: effectiveBase should be ignored.
	r := checkPath(f, "/some/other/dir", "")
	assert.True(t, r.Pass, "want pass for absolute existing path: %s", r.Message)
}

func TestCheckPath_HomeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	// Point at the home dir itself (always exists).
	r := checkPath("~", "", "")
	assert.True(t, r.Pass, "want pass: ~ expands to %s which exists: %s", home, r.Message)
}

// ── one_of check ──────────────────────────────────────────────────────────────

func TestRunner_OneOf_FirstPasses(t *testing.T) {
	runner := NewRunnerWith(RunnerOpts{
		LookPath:  fakeLookPath("pnpm"),
		LookupEnv: fakeEnv(nil),
	})
	results := runner.Run(config.Profile{{
		Type:  config.TypeOneOf,
		Label: "package manager",
		OneOf: []config.Check{
			{Type: config.TypeTool, Value: "pnpm"},
			{Type: config.TypeTool, Value: "npm"},
		},
	}})
	require.Len(t, results, 1, "want 1 atomic result")
	assert.True(t, results[0].Pass, "want pass (pnpm found): %s", results[0].Message)
	assert.Equal(t, "package manager", results[0].Label)
}

func TestRunner_OneOf_SecondPasses(t *testing.T) {
	runner := NewRunnerWith(RunnerOpts{
		LookPath:  fakeLookPath("npm"),
		LookupEnv: fakeEnv(nil),
	})
	results := runner.Run(config.Profile{{
		Type:  config.TypeOneOf,
		Label: "package manager",
		OneOf: []config.Check{
			{Type: config.TypeTool, Value: "pnpm"},
			{Type: config.TypeTool, Value: "npm"},
		},
	}})
	require.Len(t, results, 1, "want 1 atomic result")
	assert.True(t, results[0].Pass, "want pass (npm found): %s", results[0].Message)
}

func TestRunner_OneOf_NonePass(t *testing.T) {
	runner := NewRunnerWith(RunnerOpts{
		LookPath:  fakeLookPath(),
		LookupEnv: fakeEnv(nil),
	})
	results := runner.Run(config.Profile{{
		Type:  config.TypeOneOf,
		Label: "package manager",
		Hint:  "install pnpm or npm",
		OneOf: []config.Check{
			{Type: config.TypeTool, Value: "pnpm"},
			{Type: config.TypeTool, Value: "npm"},
		},
	}})
	require.Len(t, results, 1, "want 1 atomic result")
	assert.False(t, results[0].Pass, "want fail when no alternative passes")
	assert.Contains(t, results[0].Message, "none available", "message should say none available")
	assert.Contains(t, results[0].Message, "install pnpm or npm", "hint should appear")
}

func TestRunner_OneOf_DerivedLabel_Tools(t *testing.T) {
	runner := NewRunnerWith(RunnerOpts{LookPath: fakeLookPath()})
	results := runner.Run(config.Profile{{
		Type: config.TypeOneOf,
		OneOf: []config.Check{
			{Type: config.TypeTool, Value: "git"},
			{Type: config.TypeTool, Value: "gh"},
		},
	}})
	require.NotEmpty(t, results)
	assert.Contains(t, results[0].Label, "git", "derived label should contain tool names")
	assert.Contains(t, results[0].Label, "gh", "derived label should contain tool names")
}

func TestRunner_OneOf_EnvAlternative(t *testing.T) {
	runner := NewRunnerWith(RunnerOpts{
		LookPath:  fakeLookPath(),
		LookupEnv: fakeEnv(map[string]string{"SOPS_AGE_KEY": "AGE-secret"}),
	})
	results := runner.Run(config.Profile{{
		Type:  config.TypeOneOf,
		Label: "age identity",
		OneOf: []config.Check{
			{Type: config.TypeEnv, Value: "SOPS_AGE_KEY"},
			{Type: config.TypeEnv, Value: "SOPS_AGE_KEY_FILE"},
		},
	}})
	require.NotEmpty(t, results)
	assert.True(t, results[0].Pass, "want pass (SOPS_AGE_KEY found): %s", results[0].Message)
}

// ── platform gate ─────────────────────────────────────────────────────────────

func TestRunner_PlatformGate_Omit(t *testing.T) {
	runner := NewRunnerWith(RunnerOpts{
		GOOS:     "linux",
		LookPath: fakeLookPath(),
	})
	results := runner.Run(config.Profile{
		{Type: config.TypeTool, Value: "git", Platform: []string{"macos"}},
	})
	assert.Empty(t, results, "want 0 results (platform-filtered)")
}

func TestRunner_PlatformGate_Match(t *testing.T) {
	runner := NewRunnerWith(RunnerOpts{
		GOOS:     "linux",
		LookPath: fakeLookPath("git"),
	})
	results := runner.Run(config.Profile{
		{Type: config.TypeTool, Value: "git", Platform: []string{"linux"}},
	})
	require.Len(t, results, 1, "want 1 result on matching platform")
	assert.True(t, results[0].Pass, "want passing result on matching platform")
}

func TestRunner_PlatformGate_GroupOmitted(t *testing.T) {
	runner := NewRunnerWith(RunnerOpts{GOOS: "linux", LookPath: fakeLookPath()})
	results := runner.Run(config.Profile{{
		Type:     config.TypeGroup,
		Value:    "macOS only",
		Platform: []string{"macos"},
		Items:    []config.Check{{Type: config.TypeTool, Value: "xcodebuild"}},
	}})
	assert.Empty(t, results, "want 0 results (group platform-filtered)")
}

func TestRunner_PlatformGate_ChildrenFilteredGroupOmitted(t *testing.T) {
	// Group itself has no platform guard, but all children do.
	runner := NewRunnerWith(RunnerOpts{GOOS: "linux", LookPath: fakeLookPath()})
	results := runner.Run(config.Profile{{
		Type:  config.TypeGroup,
		Value: "Mixed",
		Items: []config.Check{
			{Type: config.TypeTool, Value: "xcodebuild", Platform: []string{"macos"}},
		},
	}})
	assert.Empty(t, results, "want 0 results (all children filtered → group omitted)")
}

func TestRunner_PlatformGate_EnvCheck(t *testing.T) {
	runner := NewRunnerWith(RunnerOpts{
		GOOS:      "linux",
		LookupEnv: fakeEnv(map[string]string{"FOO": "bar"}),
	})
	// env check with no platform: passes; env check with platform: macos is omitted.
	results := runner.Run(config.Profile{
		{Type: config.TypeEnv, Value: "FOO"},
		{Type: config.TypeEnv, Value: "FOO", Platform: []string{"macos"}},
	})
	require.Len(t, results, 1, "want 1 result (second omitted)")
	assert.True(t, results[0].Pass, "want pass for FOO=bar: %s", results[0].Message)
}

func TestRunner_OneOf_PlatformFilteredAlternative(t *testing.T) {
	// One alternative is platform-filtered (macos), the other is a passing env.
	// On linux: the macos tool is filtered, the env passes.
	runner := NewRunnerWith(RunnerOpts{
		GOOS:      "linux",
		LookPath:  fakeLookPath(),
		LookupEnv: fakeEnv(map[string]string{"MY_KEY": "val"}),
	})
	results := runner.Run(config.Profile{{
		Type:  config.TypeOneOf,
		Label: "key source",
		OneOf: []config.Check{
			{Type: config.TypeTool, Value: "security", Platform: []string{"macos"}},
			{Type: config.TypeEnv, Value: "MY_KEY"},
		},
	}})
	require.Len(t, results, 1, "want pass via env alternative: %+v", results)
	assert.True(t, results[0].Pass, "want pass via env alternative: %+v", results)
}

func TestExpandTilde(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}

	assert.Equal(t, home, expandTilde("~"))
	assert.Equal(t, filepath.Join(home, "sub/path"), expandTilde("~/sub/path"))
	assert.Equal(t, "/absolute/path", expandTilde("/absolute/path"))
}

func TestOneOfLabel_nonToolAlternatives(t *testing.T) {
	t.Parallel()

	got := oneOfLabel([]config.Check{
		{Type: config.TypeEnv, Value: "val1"},
		{Type: config.TypeEnv, Value: "val2"},
	})
	assert.Equal(t, "env: val1 or env: val2", got)
}

func TestResolveSeverity_info(t *testing.T) {
	t.Parallel()

	got := resolveSeverity(config.Check{Severity: "info"})
	assert.Equal(t, SeverityInfo, got)
}

func TestCurrentPlatform_override(t *testing.T) {
	t.Setenv("TRIAGE_TEST_GOOS", "linux")
	assert.Equal(t, "linux", CurrentPlatform())

	t.Setenv("TRIAGE_TEST_GOOS", "windows")
	assert.Equal(t, "windows", CurrentPlatform())
}
