package engine

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/lolay/triage/internal/config"
)

// fakeLookPath returns a function that pretends tools in found exist.
func fakeLookPath(found ...string) func(string) (string, error) {
	set := make(map[string]bool, len(found))
	for _, t := range found {
		set[t] = true
	}
	return func(name string) (string, error) {
		if set[name] {
			return "/usr/local/bin/" + name, nil
		}
		return "", errors.New("not found")
	}
}

// fakeProbe returns a function that emits a fixed output for the named tool.
func fakeProbe(outputs map[string]string) func(context.Context, string, []string) (string, error) {
	return func(_ context.Context, name string, _ []string) (string, error) {
		if out, ok := outputs[name]; ok {
			return out, nil
		}
		return "", nil
	}
}

// opts builds RunnerOpts from lookPath and probe maps.
func opts(found []string, probeOutputs map[string]string) RunnerOpts {
	return RunnerOpts{
		LookPath: fakeLookPath(found...),
		RunProbe: fakeProbe(probeOutputs),
	}
}

// ── tool check ───────────────────────────────────────────────────────────────

func TestCheckTool_Present_NoConstraint(t *testing.T) {
	r := checkTool(context.Background(), "git", "", "https://git-scm.com",
		opts([]string{"git"}, nil))
	if !r.Pass {
		t.Errorf("want pass, got fail: %s", r.Message)
	}
	if r.Label != "git" {
		t.Errorf("label = %q, want git", r.Label)
	}
}

func TestCheckTool_Missing(t *testing.T) {
	r := checkTool(context.Background(), "notfound", "", "install it",
		opts(nil, nil))
	if r.Pass {
		t.Errorf("want fail, got pass")
	}
	if !strings.Contains(r.Message, "not found") {
		t.Errorf("message = %q, want 'not found'", r.Message)
	}
	if !strings.Contains(r.Message, "install it") {
		t.Errorf("hint missing from message: %q", r.Message)
	}
}

func TestCheckTool_Missing_NoHint(t *testing.T) {
	r := checkTool(context.Background(), "notfound", "", "",
		opts(nil, nil))
	if r.Pass {
		t.Errorf("want fail")
	}
	if strings.Contains(r.Message, " — ") {
		t.Errorf("no hint: message should not contain ' — ': %q", r.Message)
	}
}

func TestCheckTool_VersionPass(t *testing.T) {
	r := checkTool(context.Background(), "mytool", ">=1.2.0", "upgrade mytool",
		opts([]string{"mytool"}, map[string]string{"mytool": "mytool version 1.3.0"}))
	if !r.Pass {
		t.Errorf("want pass, got: %s", r.Message)
	}
	if !strings.Contains(r.Message, "1.3.0") {
		t.Errorf("message should contain found version: %q", r.Message)
	}
}

func TestCheckTool_VersionFail(t *testing.T) {
	r := checkTool(context.Background(), "mytool", ">=2.0.0", "upgrade mytool",
		opts([]string{"mytool"}, map[string]string{"mytool": "mytool version 1.9.0"}))
	if r.Pass {
		t.Errorf("want fail (version too old)")
	}
	if !strings.Contains(r.Message, "1.9.0") {
		t.Errorf("should mention found version: %q", r.Message)
	}
	if !strings.Contains(r.Message, ">=2.0.0") {
		t.Errorf("should mention required constraint: %q", r.Message)
	}
}

func TestCheckTool_GoVersionOverride(t *testing.T) {
	r := checkTool(context.Background(), "go", ">=1.21.0", "",
		opts([]string{"go"}, map[string]string{"go": "go version go1.26.3 linux/amd64"}))
	if !r.Pass {
		t.Errorf("go version override: want pass, got: %s", r.Message)
	}
	if !strings.Contains(r.Message, "1.26.3") {
		t.Errorf("message should contain 1.26.3: %q", r.Message)
	}
}

func TestCheckTool_GoVersionFail(t *testing.T) {
	r := checkTool(context.Background(), "go", ">=1.30.0", "install newer go",
		opts([]string{"go"}, map[string]string{"go": "go version go1.26.3 linux/amd64"}))
	if r.Pass {
		t.Errorf("want fail: 1.26.3 < 1.30.0")
	}
}

func TestCheckTool_UnparseableVersion(t *testing.T) {
	r := checkTool(context.Background(), "mytool", ">=1.0.0", "",
		opts([]string{"mytool"}, map[string]string{"mytool": "no version info here"}))
	if r.Pass {
		t.Errorf("want fail when version cannot be parsed")
	}
}

func TestCheckTool_NodeLeadingV(t *testing.T) {
	r := checkTool(context.Background(), "node", ">=18.0.0", "",
		opts([]string{"node"}, map[string]string{"node": "v20.11.0"}))
	if !r.Pass {
		t.Errorf("node v20.11.0 should satisfy >=18.0.0: %s", r.Message)
	}
}

// ── runner integration ────────────────────────────────────────────────────────

func TestRunner_EmptyProfile(t *testing.T) {
	r := NewRunner().Run(config.Profile{})
	if len(r) != 0 {
		t.Errorf("want 0 results, got %d", len(r))
	}
}

func TestRunner_ToolPresent(t *testing.T) {
	runner := NewRunnerWith(opts([]string{"git"}, nil))
	results := runner.Run(config.Profile{
		{Type: config.TypeTool, Value: "git", Hint: "https://git-scm.com"},
	})
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	if !results[0].Pass {
		t.Errorf("want pass: %s", results[0].Message)
	}
	if results[0].Depth != 0 {
		t.Errorf("depth = %d, want 0", results[0].Depth)
	}
}

func TestRunner_ToolMissing(t *testing.T) {
	runner := NewRunnerWith(opts(nil, nil))
	results := runner.Run(config.Profile{
		{Type: config.TypeTool, Value: "missing", Hint: "install missing"},
	})
	if results[0].Pass {
		t.Errorf("want fail")
	}
	if results[0].Severity != SeverityError {
		t.Errorf("severity = %v, want Error", results[0].Severity)
	}
}

func TestRunner_SeverityWarn(t *testing.T) {
	runner := NewRunnerWith(opts(nil, nil))
	results := runner.Run(config.Profile{
		{Type: config.TypeTool, Value: "optional", Severity: "warn", Hint: "brew install optional"},
	})
	if results[0].Severity != SeverityWarn {
		t.Errorf("severity = %v, want Warn", results[0].Severity)
	}
}

func TestRunner_RequiredFalse(t *testing.T) {
	boolFalse := false
	runner := NewRunnerWith(opts(nil, nil))
	results := runner.Run(config.Profile{
		{Type: config.TypeTool, Value: "opt", Required: &boolFalse},
	})
	if results[0].Severity != SeverityWarn {
		t.Errorf("required:false should map to Warn, got %v", results[0].Severity)
	}
}

func TestRunner_Group(t *testing.T) {
	runner := NewRunnerWith(opts([]string{"git", "go"}, nil))
	results := runner.Run(config.Profile{
		{
			Type:  config.TypeGroup,
			Value: "Toolchain",
			Items: []config.Check{
				{Type: config.TypeTool, Value: "git"},
				{Type: config.TypeTool, Value: "go"},
			},
		},
	})
	// [header, git, go]
	if len(results) != 3 {
		t.Fatalf("want 3 results (header + 2 leaves), got %d", len(results))
	}
	if results[0].Kind != KindHeader {
		t.Errorf("results[0] should be KindHeader")
	}
	if results[0].Label != "Toolchain" {
		t.Errorf("header label = %q", results[0].Label)
	}
	if !results[0].Pass {
		t.Errorf("group header should pass when all children pass")
	}
	if results[1].Depth != 1 {
		t.Errorf("child depth = %d, want 1", results[1].Depth)
	}
}

func TestRunner_GroupFailsWhenChildFails(t *testing.T) {
	runner := NewRunnerWith(opts([]string{"git"}, nil)) // go is missing
	results := runner.Run(config.Profile{
		{
			Type:  config.TypeGroup,
			Value: "Toolchain",
			Items: []config.Check{
				{Type: config.TypeTool, Value: "git"},
				{Type: config.TypeTool, Value: "go"},
			},
		},
	})
	if results[0].Pass {
		t.Errorf("group header should fail when a child fails")
	}
	if results[0].Severity != SeverityError {
		t.Errorf("header severity should be Error when child fails")
	}
}

func TestRunner_InfoSkipForUnimplementedType(t *testing.T) {
	runner := NewRunnerWith(opts(nil, nil))
	results := runner.Run(config.Profile{
		{Type: config.TypeEnv, Value: "HOME"},
	})
	if len(results) != 1 {
		t.Fatalf("want 1 result, got %d", len(results))
	}
	if results[0].Severity != SeverityInfo {
		t.Errorf("unimplemented type should emit SeverityInfo, got %v", results[0].Severity)
	}
	if !results[0].Pass {
		t.Errorf("unimplemented type should be non-failing")
	}
}
