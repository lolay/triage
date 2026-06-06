package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

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
	if !r.Pass {
		t.Errorf("want pass, got fail: %s", r.Message)
	}
	if !strings.Contains(r.Message, "FOO=bar") {
		t.Errorf("message should contain value: %q", r.Message)
	}
}

func TestCheckEnv_SetMode_EmptyValueCounts(t *testing.T) {
	r := checkEnv("FOO", false, "", "", fakeEnv(map[string]string{"FOO": ""}))
	if !r.Pass {
		t.Errorf("empty value should count as set: %s", r.Message)
	}
}

func TestCheckEnv_SetMode_Absent(t *testing.T) {
	r := checkEnv("FOO", false, "", "export FOO=x", fakeEnv(nil))
	if r.Pass {
		t.Errorf("want fail when var is absent")
	}
	if !strings.Contains(r.Message, "not set") {
		t.Errorf("message should say not set: %q", r.Message)
	}
	if !strings.Contains(r.Message, "export FOO=x") {
		t.Errorf("hint should appear in message: %q", r.Message)
	}
}

func TestCheckEnv_SetMode_Matches_Pass(t *testing.T) {
	r := checkEnv("FOO", false, `^prod`, "", fakeEnv(map[string]string{"FOO": "production"}))
	if !r.Pass {
		t.Errorf("want pass: value matches regex: %s", r.Message)
	}
}

func TestCheckEnv_SetMode_Matches_Fail(t *testing.T) {
	r := checkEnv("FOO", false, `^prod`, "set FOO=prod-*", fakeEnv(map[string]string{"FOO": "staging"}))
	if r.Pass {
		t.Errorf("want fail: value does not match regex")
	}
	if !strings.Contains(r.Message, "does not match") {
		t.Errorf("message should mention mismatch: %q", r.Message)
	}
	if !strings.Contains(r.Message, "set FOO=prod-*") {
		t.Errorf("hint should appear: %q", r.Message)
	}
}

func TestCheckEnv_SetMode_BadRegex(t *testing.T) {
	r := checkEnv("FOO", false, `[invalid`, "", fakeEnv(map[string]string{"FOO": "x"}))
	if r.Pass {
		t.Errorf("want fail on bad regex")
	}
	if !strings.Contains(r.Message, "invalid matches regex") {
		t.Errorf("message should mention bad regex: %q", r.Message)
	}
}

func TestCheckEnv_UnsetMode_Absent(t *testing.T) {
	r := checkEnv("SECRET", true, "", "", fakeEnv(nil))
	if !r.Pass {
		t.Errorf("want pass when var is absent in unset mode: %s", r.Message)
	}
}

func TestCheckEnv_UnsetMode_Present(t *testing.T) {
	r := checkEnv("SECRET", true, "", "", fakeEnv(map[string]string{"SECRET": "oops"}))
	if r.Pass {
		t.Errorf("want fail when var is set in unset mode")
	}
	if !strings.Contains(r.Message, "expected unset") {
		t.Errorf("message should say expected unset: %q", r.Message)
	}
}

// ── path check ────────────────────────────────────────────────────────────────

func TestCheckPath_FilePresent(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "present.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := checkPath("present.txt", dir, "")
	if !r.Pass {
		t.Errorf("want pass for existing file: %s", r.Message)
	}
}

func TestCheckPath_FileAbsent(t *testing.T) {
	dir := t.TempDir()
	r := checkPath("absent.txt", dir, "create it")
	if r.Pass {
		t.Errorf("want fail for missing file")
	}
	if !strings.Contains(r.Message, "not found") {
		t.Errorf("message should say not found: %q", r.Message)
	}
	if !strings.Contains(r.Message, "create it") {
		t.Errorf("hint should appear: %q", r.Message)
	}
}

func TestCheckPath_GlobMatch(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"key1.p8", "key2.p8"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("k"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r := checkPath("*.p8", dir, "")
	if !r.Pass {
		t.Errorf("want pass for glob matching multiple files: %s", r.Message)
	}
}

func TestCheckPath_GlobNoMatch(t *testing.T) {
	dir := t.TempDir()
	r := checkPath("*.p8", dir, "")
	if r.Pass {
		t.Errorf("want fail when no files match glob")
	}
}

func TestCheckPath_Absolute(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "abs.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Absolute pattern: effectiveBase should be ignored.
	r := checkPath(f, "/some/other/dir", "")
	if !r.Pass {
		t.Errorf("want pass for absolute existing path: %s", r.Message)
	}
}

func TestCheckPath_HomeExpansion(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home dir")
	}
	// Point at the home dir itself (always exists).
	r := checkPath("~", "", "")
	if !r.Pass {
		t.Errorf("want pass: ~ expands to %s which exists: %s", home, r.Message)
	}
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
	if len(results) != 1 {
		t.Fatalf("want 1 atomic result, got %d", len(results))
	}
	if !results[0].Pass {
		t.Errorf("want pass (pnpm found): %s", results[0].Message)
	}
	if results[0].Label != "package manager" {
		t.Errorf("label = %q, want 'package manager'", results[0].Label)
	}
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
	if len(results) != 1 {
		t.Fatalf("want 1 atomic result, got %d", len(results))
	}
	if !results[0].Pass {
		t.Errorf("want pass (npm found): %s", results[0].Message)
	}
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
	if len(results) != 1 {
		t.Fatalf("want 1 atomic result, got %d", len(results))
	}
	if results[0].Pass {
		t.Errorf("want fail when no alternative passes")
	}
	if !strings.Contains(results[0].Message, "none available") {
		t.Errorf("message should say none available: %q", results[0].Message)
	}
	if !strings.Contains(results[0].Message, "install pnpm or npm") {
		t.Errorf("hint should appear: %q", results[0].Message)
	}
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
	if !strings.Contains(results[0].Label, "git") || !strings.Contains(results[0].Label, "gh") {
		t.Errorf("derived label should contain tool names: %q", results[0].Label)
	}
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
	if !results[0].Pass {
		t.Errorf("want pass (SOPS_AGE_KEY found): %s", results[0].Message)
	}
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
	if len(results) != 0 {
		t.Errorf("want 0 results (platform-filtered), got %d", len(results))
	}
}

func TestRunner_PlatformGate_Match(t *testing.T) {
	runner := NewRunnerWith(RunnerOpts{
		GOOS:     "linux",
		LookPath: fakeLookPath("git"),
	})
	results := runner.Run(config.Profile{
		{Type: config.TypeTool, Value: "git", Platform: []string{"linux"}},
	})
	if len(results) != 1 || !results[0].Pass {
		t.Errorf("want 1 passing result on matching platform, got %d results", len(results))
	}
}

func TestRunner_PlatformGate_GroupOmitted(t *testing.T) {
	runner := NewRunnerWith(RunnerOpts{GOOS: "linux", LookPath: fakeLookPath()})
	results := runner.Run(config.Profile{{
		Type:     config.TypeGroup,
		Value:    "macOS only",
		Platform: []string{"macos"},
		Items:    []config.Check{{Type: config.TypeTool, Value: "xcodebuild"}},
	}})
	if len(results) != 0 {
		t.Errorf("want 0 results (group platform-filtered), got %d", len(results))
	}
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
	if len(results) != 0 {
		t.Errorf("want 0 results (all children filtered → group omitted), got %d", len(results))
	}
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
	if len(results) != 1 {
		t.Errorf("want 1 result (second omitted), got %d", len(results))
	}
	if !results[0].Pass {
		t.Errorf("want pass for FOO=bar: %s", results[0].Message)
	}
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
	if len(results) != 1 || !results[0].Pass {
		t.Errorf("want pass via env alternative: %+v", results)
	}
}
