package engine

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	r, _ := checkTool(t.Context(), "git", "", "", "https://git-scm.com",
		opts([]string{"git"}, nil))
	assert.True(t, r.Pass, "want pass, got fail: %s", r.Message)
	assert.Equal(t, "git", r.Label)
}

func TestCheckTool_Missing(t *testing.T) {
	r, _ := checkTool(t.Context(), "notfound", "", "", "install it",
		opts(nil, nil))
	assert.False(t, r.Pass, "want fail, got pass")
	assert.Contains(t, r.Message, "not found")
	assert.Contains(t, r.Message, "install it", "hint missing from message")
	assert.Equal(t, "install it", r.Hint)
}

func TestCheckTool_Missing_NoHint(t *testing.T) {
	r, _ := checkTool(t.Context(), "notfound", "", "", "",
		opts(nil, nil))
	assert.False(t, r.Pass, "want fail")
	assert.NotContains(t, r.Message, " — ", "no hint: message should not contain ' — '")
	assert.Empty(t, r.Hint)
}

func TestCheckTool_VersionPass(t *testing.T) {
	r, _ := checkTool(t.Context(), "mytool", ">=1.2.0", "", "upgrade mytool",
		opts([]string{"mytool"}, map[string]string{"mytool": "mytool version 1.3.0"}))
	assert.True(t, r.Pass, "want pass, got: %s", r.Message)
	assert.Contains(t, r.Message, "1.3.0", "message should contain found version")
}

func TestCheckTool_VersionFail(t *testing.T) {
	r, _ := checkTool(t.Context(), "mytool", ">=2.0.0", "", "upgrade mytool",
		opts([]string{"mytool"}, map[string]string{"mytool": "mytool version 1.9.0"}))
	assert.False(t, r.Pass, "want fail (version too old)")
	assert.Contains(t, r.Message, "1.9.0", "should mention found version")
	assert.Contains(t, r.Message, ">=2.0.0", "should mention required constraint")
}

func TestCheckTool_GoVersionOverride(t *testing.T) {
	r, _ := checkTool(t.Context(), "go", ">=1.21.0", "", "",
		opts([]string{"go"}, map[string]string{"go": "go version go1.26.3 linux/amd64"}))
	assert.True(t, r.Pass, "go version override: want pass, got: %s", r.Message)
	assert.Contains(t, r.Message, "1.26.3", "message should contain 1.26.3")
}

func TestCheckTool_GoVersionFail(t *testing.T) {
	r, _ := checkTool(t.Context(), "go", ">=1.30.0", "", "install newer go",
		opts([]string{"go"}, map[string]string{"go": "go version go1.26.3 linux/amd64"}))
	assert.False(t, r.Pass, "want fail: 1.26.3 < 1.30.0")
}

func TestCheckTool_UnparseableVersion(t *testing.T) {
	r, _ := checkTool(t.Context(), "mytool", ">=1.0.0", "", "",
		opts([]string{"mytool"}, map[string]string{"mytool": "no version info here"}))
	assert.False(t, r.Pass, "want fail when version cannot be parsed")
}

func TestCheckTool_NodeLeadingV(t *testing.T) {
	r, _ := checkTool(t.Context(), "node", ">=18.0.0", "", "",
		opts([]string{"node"}, map[string]string{"node": "v20.11.0"}))
	assert.True(t, r.Pass, "node v20.11.0 should satisfy >=18.0.0: %s", r.Message)
}

// ── recommended version ──────────────────────────────────────────────────────

func TestCheckTool_Recommended_BelowMinimum(t *testing.T) {
	r, below := checkTool(t.Context(), "node", ">=22", ">=26.2.0", "nvm install",
		opts([]string{"node"}, map[string]string{"node": "v21.0.0"}))
	assert.False(t, r.Pass, "want fail: 21.0.0 < 22")
	assert.False(t, below, "a failed minimum is not a recommended shortfall")
	assert.Contains(t, r.Message, "need >=22")
	assert.Equal(t, "nvm install", r.Hint)
}

func TestCheckTool_BelowMinimum_NoHint(t *testing.T) {
	r, _ := checkTool(t.Context(), "node", ">=22", "", "",
		opts([]string{"node"}, map[string]string{"node": "v21.0.0"}))
	assert.False(t, r.Pass)
	assert.Equal(t, "node 21.0.0 — need >=22", r.Message)
	assert.Empty(t, r.Hint, "the 'need' clause is a diagnosis, not a remediation")
}

func TestCheckTool_Recommended_BetweenMinimumAndRecommended(t *testing.T) {
	r, below := checkTool(t.Context(), "node", ">=22", ">=26.2.0", "nvm install",
		opts([]string{"node"}, map[string]string{"node": "v22.11.0"}))
	assert.False(t, r.Pass, "want not-pass below recommended")
	assert.True(t, below, "want belowRecommended")
	assert.Equal(t, "node 22.11.0, recommended >=26.2.0 (minimum >=22 met) — nvm install", r.Message)
}

func TestCheckTool_Recommended_NoHint_NoRemediationSeparator(t *testing.T) {
	r, below := checkTool(t.Context(), "node", ">=22", ">=26.2.0", "",
		opts([]string{"node"}, map[string]string{"node": "v22.11.0"}))
	assert.True(t, below)
	assert.NotContains(t, r.Message, " — ", "board would read the text after ' — ' as a fix command")
	assert.Empty(t, r.Hint)
}

func TestCheckTool_Recommended_Met(t *testing.T) {
	r, below := checkTool(t.Context(), "node", ">=22", ">=26.2.0", "",
		opts([]string{"node"}, map[string]string{"node": "v26.2.0"}))
	assert.True(t, r.Pass, "want pass: %s", r.Message)
	assert.False(t, below)
	assert.Equal(t, "node 26.2.0 (>=22, recommended >=26.2.0)", r.Message)
}

func TestCheckTool_RecommendedOnly(t *testing.T) {
	r, below := checkTool(t.Context(), "terraform", "", ">=1.9", "",
		opts([]string{"terraform"}, map[string]string{"terraform": "Terraform v1.8.5"}))
	assert.False(t, r.Pass)
	assert.True(t, below)
	assert.Equal(t, "terraform 1.8.5, recommended >=1.9", r.Message)

	r, below = checkTool(t.Context(), "terraform", "", ">=1.9", "",
		opts([]string{"terraform"}, map[string]string{"terraform": "Terraform v1.9.0"}))
	assert.True(t, r.Pass)
	assert.False(t, below)
	assert.Equal(t, "terraform 1.9.0 (recommended >=1.9)", r.Message)
}

func TestCheckTool_RecommendedOnly_Missing_IsNotBelowRecommended(t *testing.T) {
	r, below := checkTool(t.Context(), "terraform", "", ">=1.9", "",
		opts(nil, nil))
	assert.False(t, r.Pass)
	assert.False(t, below, "a missing tool keeps the check's severity")
}

func TestCheckTool_RecommendedOnly_UnparseableVersion_IsBelowRecommended(t *testing.T) {
	_, below := checkTool(t.Context(), "mytool", "", ">=1.0", "",
		opts([]string{"mytool"}, map[string]string{"mytool": "no version info here"}))
	assert.True(t, below, "nothing required is unmet, so an unreadable version is only a warning")

	_, below = checkTool(t.Context(), "mytool", ">=1.0", ">=2.0", "",
		opts([]string{"mytool"}, map[string]string{"mytool": "no version info here"}))
	assert.False(t, below, "with a minimum, an unreadable version stays at the check's severity")
}

func TestCheckTool_Recommended_InvalidConstraint(t *testing.T) {
	r, below := checkTool(t.Context(), "node", ">=22", "not a range", "",
		opts([]string{"node"}, map[string]string{"node": "v26.2.0"}))
	assert.False(t, r.Pass)
	assert.False(t, below)
	assert.Contains(t, r.Message, "invalid constraint")
}

func TestRunner_Recommended_Severity(t *testing.T) {
	node := func(severity string) config.Check {
		return config.Check{
			Type: config.TypeTool, Value: "node", Severity: severity,
			Constraint: ">=22", RecommendedConstraint: ">=26.2.0",
		}
	}
	tests := []struct {
		name     string
		severity string
		version  string
		wantPass bool
		wantSev  Severity
	}{
		{"below minimum keeps error", "", "v21.0.0", false, SeverityError},
		{"below recommended is warn", "", "v22.11.0", false, SeverityWarn},
		{"below recommended on warn check stays warn", "warn", "v22.11.0", false, SeverityWarn},
		{"below recommended on info check stays info", "info", "v22.11.0", false, SeverityInfo},
		{"meets recommended passes", "", "v26.2.0", true, SeverityError},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := NewRunnerWith(opts([]string{"node"}, map[string]string{"node": tc.version}))
			results := runner.Run(config.Profile{node(tc.severity)})
			require.Len(t, results, 1)
			assert.Equal(t, tc.wantPass, results[0].Pass, results[0].Message)
			assert.Equal(t, tc.wantSev, results[0].Severity)
		})
	}
}

func TestRunner_Recommended_PassLabel(t *testing.T) {
	runner := NewRunnerWith(opts([]string{"node"}, map[string]string{"node": "v26.2.0"}))
	results := runner.Run(config.Profile{{
		Type: config.TypeTool, Value: "node", RecommendedConstraint: ">=26.2.0",
	}})
	require.Len(t, results, 1)
	assert.Equal(t, "node 26.2.0 (recommended >=26.2.0)", results[0].Label)
}

// ── runner integration ────────────────────────────────────────────────────────

func TestRunner_EmptyProfile(t *testing.T) {
	r := NewRunner().Run(config.Profile{})
	assert.Empty(t, r, "want 0 results")
}

func TestRunner_ToolPresent(t *testing.T) {
	runner := NewRunnerWith(opts([]string{"git"}, nil))
	results := runner.Run(config.Profile{
		{Type: config.TypeTool, Value: "git", Hint: "https://git-scm.com"},
	})
	require.Len(t, results, 1)
	assert.True(t, results[0].Pass, "want pass: %s", results[0].Message)
	assert.Equal(t, 0, results[0].Depth)
}

func TestRunner_ToolMissing(t *testing.T) {
	runner := NewRunnerWith(opts(nil, nil))
	results := runner.Run(config.Profile{
		{Type: config.TypeTool, Value: "missing", Hint: "install missing"},
	})
	require.Len(t, results, 1)
	assert.False(t, results[0].Pass, "want fail")
	assert.Equal(t, SeverityError, results[0].Severity)
}

func TestRunner_SeverityWarn(t *testing.T) {
	runner := NewRunnerWith(opts(nil, nil))
	results := runner.Run(config.Profile{
		{Type: config.TypeTool, Value: "optional", Severity: "warn", Hint: "brew install optional"},
	})
	require.Len(t, results, 1)
	assert.Equal(t, SeverityWarn, results[0].Severity)
}

func TestRunner_RequiredFalse(t *testing.T) {
	boolFalse := false
	runner := NewRunnerWith(opts(nil, nil))
	results := runner.Run(config.Profile{
		{Type: config.TypeTool, Value: "opt", Required: &boolFalse},
	})
	require.Len(t, results, 1)
	assert.Equal(t, SeverityWarn, results[0].Severity, "required:false should map to Warn")
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
	require.Len(t, results, 3, "want 3 results (header + 2 leaves)")
	assert.Equal(t, KindHeader, results[0].Kind, "results[0] should be KindHeader")
	assert.Equal(t, "Toolchain", results[0].Label)
	assert.True(t, results[0].Pass, "group header should pass when all children pass")
	assert.Equal(t, 1, results[1].Depth, "child depth")
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
	require.NotEmpty(t, results)
	assert.False(t, results[0].Pass, "group header should fail when a child fails")
	assert.Equal(t, SeverityError, results[0].Severity, "header severity should be Error when child fails")
}

func TestRunner_DelegateMissingChildFails(t *testing.T) {
	// A delegate whose child config can't be found fails red (exit 1) rather
	// than crashing, and is not a fatal config error (exit 3).
	runner := NewRunnerWith(RunnerOpts{BaseDir: t.TempDir()})
	results := runner.Run(config.Profile{
		{Type: config.TypeDelegate, Value: "child", Dir: "does-not-exist"},
	})
	require.Len(t, results, 1)
	assert.Equal(t, KindLeaf, results[0].Kind, "childless delegate failure should be a counted KindLeaf")
	assert.False(t, results[0].Pass, "missing child should fail")
	assert.Equal(t, SeverityError, results[0].Severity, "missing child should fail with SeverityError")
	assert.NoError(t, runner.Fatal(), "missing child is not a fatal config error")
}
