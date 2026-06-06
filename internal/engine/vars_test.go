package engine

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lolay/triage/internal/config"
)

func TestExpand_HitAndWhitespace(t *testing.T) {
	vars := map[string]string{"region": "us-west-2", "x": "y"}
	got, err := expand("prefix {{ region }} suffix", vars)
	if err != nil || got != "prefix us-west-2 suffix" {
		t.Errorf("expand = %q, err = %v", got, err)
	}
}

func TestExpand_UnknownVar(t *testing.T) {
	_, err := expand("{{ missing }}", map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "missing") {
		t.Errorf("want unknown var error, got %v", err)
	}
}

func TestExpand_EmptyNoOp(t *testing.T) {
	got, err := expand("", nil)
	if err != nil || got != "" {
		t.Errorf("empty expand = %q, err = %v", got, err)
	}
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
			if _, err := expand(tc.in, vars); err == nil {
				t.Errorf("expand(%q) = nil error, want malformed-template error", tc.in)
			} else if !strings.Contains(err.Error(), "malformed template") {
				t.Errorf("expand(%q) error = %v, want malformed-template error", tc.in, err)
			}
		})
	}
}

func TestExpand_ValidAmongLiteralBraces(t *testing.T) {
	// A well-formed token next to non-template single braces still resolves.
	got, err := expand("a{b {{ ok }} c}", map[string]string{"ok": "v"})
	if err != nil || got != "a{b v c}" {
		t.Errorf("expand = %q, err = %v", got, err)
	}
}

func TestExpandCheck_AllFields(t *testing.T) {
	vars := map[string]string{"v": "expanded"}
	c := config.Check{
		Type:       config.TypeCommand,
		Value:      "{{ v }}",
		Version:    "{{ v }}",
		Constraint: "{{ v }}",
		Group:      "{{ v }}",
		Hint:       "{{ v }}",
		Dir:        "{{ v }}",
		Label:      "{{ v }}",
		Matches:    "{{ v }}",
		Contains:   "{{ v }}",
		Config:     "{{ v }}",
		WithEnv:    map[string]string{"K": "{{ v }}"},
	}
	out, err := expandCheck(c, vars)
	if err != nil {
		t.Fatalf("expandCheck: %v", err)
	}
	if out.Value != "expanded" || out.WithEnv["K"] != "expanded" {
		t.Errorf("expandCheck incomplete: %+v", out)
	}
}

func TestRunner_VarInToolName(t *testing.T) {
	r := NewRunnerWith(RunnerOpts{
		ConfigVars: map[string]string{"tool_name": "git"},
		LookPath:   fakeLookPath("git"),
	})
	results := r.Run(config.Profile{{Type: config.TypeTool, Value: "{{ tool_name }}"}})
	if len(results) != 1 || !results[0].Pass {
		t.Errorf("want pass for expanded tool name: %+v", results)
	}
}

func TestRunner_VarInPath(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "present.txt")
	if err := os.WriteFile(f, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := NewRunnerWith(RunnerOpts{
		ConfigVars: map[string]string{"fname": "present.txt"},
		BaseDir:    dir,
	})
	results := r.Run(config.Profile{{Type: config.TypePath, Value: "{{ fname }}"}})
	if len(results) != 1 || !results[0].Pass {
		t.Errorf("want pass for expanded path: %+v", results)
	}
}

func TestRunner_VarInCommand(t *testing.T) {
	r := NewRunnerWith(RunnerOpts{
		ConfigVars: map[string]string{"region": "us-west-2"},
		RunCommand: func(_ context.Context, _, script, _ string, _ map[string]string) (string, int, error) {
			if script != "echo us-west-2" {
				t.Errorf("script = %q", script)
			}
			return "us-west-2\n", 0, nil
		},
	})
	results := r.Run(config.Profile{{
		Type:     config.TypeCommand,
		Value:    "echo {{ region }}",
		Label:    "region",
		Contains: "us-west-2",
	}})
	if len(results) != 1 || !results[0].Pass {
		t.Errorf("want pass: %+v", results)
	}
}

func TestRunner_ProfileBuiltinInPath(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "release")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	r := NewRunnerWith(RunnerOpts{
		Profile: "release",
		BaseDir: dir,
	})
	results := r.Run(config.Profile{{Type: config.TypePath, Value: "{{ profile }}"}})
	if len(results) != 1 || !results[0].Pass {
		t.Errorf("want pass for {{ profile }} path dir: %+v", results)
	}
}

func TestRunner_UnknownVarFails(t *testing.T) {
	r := NewRunner()
	results := r.Run(config.Profile{{Type: config.TypeTool, Value: "{{ undefined_var }}"}})
	if len(results) != 1 || results[0].Pass {
		t.Errorf("want failing result for unknown var: %+v", results)
	}
	if results[0].Severity != SeverityError {
		t.Errorf("severity = %v, want Error", results[0].Severity)
	}
	if !strings.Contains(results[0].Message, "undefined_var") {
		t.Errorf("message = %q", results[0].Message)
	}
}

func TestEffectiveVars_BuiltinsWin(t *testing.T) {
	r := NewRunnerWith(RunnerOpts{
		ConfigVars: map[string]string{"profile": "wrong", "os": "wrong"},
		Profile:    "release",
		GOOS:       "linux",
	})
	ev := r.effectiveVars()
	if ev["profile"] != "release" || ev["os"] != "linux" {
		t.Errorf("built-ins should win: %#v", ev)
	}
}
