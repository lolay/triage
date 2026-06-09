package engine

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lolay/triage/internal/config"
)

func TestExpand_HitAndWhitespace(t *testing.T) {
	vars := map[string]string{"region": "us-west-2", "x": "y"}
	got, err := expand("prefix {{ region }} suffix", vars)
	require.NoError(t, err)
	assert.Equal(t, "prefix us-west-2 suffix", got)
}

func TestExpand_UnknownVar(t *testing.T) {
	_, err := expand("{{ missing }}", map[string]string{})
	require.Error(t, err, "want unknown var error")
	assert.ErrorContains(t, err, "missing")
}

func TestExpand_EmptyNoOp(t *testing.T) {
	got, err := expand("", nil)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestExpand_MalformedTokensFail(t *testing.T) {
	vars := map[string]string{"ok": "v"}
	cases := []struct {
		name string
		in   string
	}{
		{"digit-led", "{{ 1bad }}"},
		{"hyphen", "{{ foo-bar }}"},
		{"empty", "{{}}"},
		{"unclosed", "echo {{ foo"},
		{"dotted", "{{ .Foo }}"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := expand(tc.in, vars)
			require.Errorf(t, err, "expand(%q) should fail with a malformed-template error", tc.in)
			assert.ErrorContains(t, err, "malformed template")
		})
	}
}

func TestExpand_ValidAmongLiteralBraces(t *testing.T) {
	// A well-formed token next to non-template single braces still resolves.
	got, err := expand("a{b {{ ok }} c}", map[string]string{"ok": "v"})
	require.NoError(t, err)
	assert.Equal(t, "a{b v c}", got)
}

func TestExpandCheck_AllFields(t *testing.T) {
	vars := map[string]string{"v": "expanded"}
	c := config.Check{
		Type:       config.TypeCommand,
		Value:      "{{ v }}",
		Version:    "{{ v }}",
		Constraint: "{{ v }}",
		Hint:       "{{ v }}",
		Dir:        "{{ v }}",
		Label:      "{{ v }}",
		Matches:    "{{ v }}",
		Contains:   "{{ v }}",
		Config:     "{{ v }}",
		WithEnv:    map[string]string{"K": "{{ v }}"},
	}
	out, err := expandCheck(c, vars)
	require.NoError(t, err)
	assert.Equal(t, "expanded", out.Value, "expandCheck incomplete: %+v", out)
	assert.Equal(t, "expanded", out.WithEnv["K"], "expandCheck incomplete: %+v", out)
}

func TestRunner_VarInToolName(t *testing.T) {
	r := NewRunnerWith(RunnerOpts{
		ConfigVars: map[string]string{"tool_name": "git"},
		LookPath:   fakeLookPath("git"),
	})
	results := r.Run(config.Profile{{Type: config.TypeTool, Value: "{{ tool_name }}"}})
	require.Len(t, results, 1, "want 1 result: %+v", results)
	assert.True(t, results[0].Pass, "want pass for expanded tool name: %+v", results)
}

func TestRunner_VarInPath(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "present.txt")
	require.NoError(t, os.WriteFile(f, []byte("x"), 0o644))
	r := NewRunnerWith(RunnerOpts{
		ConfigVars: map[string]string{"fname": "present.txt"},
		BaseDir:    dir,
	})
	results := r.Run(config.Profile{{Type: config.TypePath, Value: "{{ fname }}"}})
	require.Len(t, results, 1, "want 1 result: %+v", results)
	assert.True(t, results[0].Pass, "want pass for expanded path: %+v", results)
}

func TestRunner_VarInCommand(t *testing.T) {
	r := NewRunnerWith(RunnerOpts{
		ConfigVars: map[string]string{"region": "us-west-2"},
		RunCommand: func(_ context.Context, _, script, _ string, _ map[string]string) (string, int, error) {
			assert.Equal(t, "echo us-west-2", script)
			return "us-west-2\n", 0, nil
		},
	})
	results := r.Run(config.Profile{{
		Type:     config.TypeCommand,
		Value:    "echo {{ region }}",
		Label:    "region",
		Contains: "us-west-2",
	}})
	require.Len(t, results, 1, "want 1 result: %+v", results)
	assert.True(t, results[0].Pass, "want pass: %+v", results)
}

func TestRunner_ProfileBuiltinInPath(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "release")
	require.NoError(t, os.Mkdir(sub, 0o755))
	r := NewRunnerWith(RunnerOpts{
		Profile: "release",
		BaseDir: dir,
	})
	results := r.Run(config.Profile{{Type: config.TypePath, Value: "{{ profile }}"}})
	require.Len(t, results, 1, "want 1 result: %+v", results)
	assert.True(t, results[0].Pass, "want pass for {{ profile }} path dir: %+v", results)
}

func TestRunner_UnknownVarFails(t *testing.T) {
	r := NewRunner()
	results := r.Run(config.Profile{{Type: config.TypeTool, Value: "{{ undefined_var }}"}})
	require.Len(t, results, 1, "want failing result for unknown var: %+v", results)
	assert.False(t, results[0].Pass, "want failing result for unknown var: %+v", results)
	assert.Equal(t, SeverityError, results[0].Severity)
	assert.Contains(t, results[0].Message, "undefined_var")
}

func TestEffectiveVars_BuiltinsWin(t *testing.T) {
	r := NewRunnerWith(RunnerOpts{
		ConfigVars: map[string]string{"profile": "wrong", "os": "wrong"},
		Profile:    "release",
		GOOS:       "linux",
	})
	ev := r.effectiveVars()
	assert.Equal(t, "release", ev["profile"], "built-ins should win: %#v", ev)
	assert.Equal(t, "linux", ev["os"], "built-ins should win: %#v", ev)
}
