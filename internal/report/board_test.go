package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lolay/triage/internal/engine"
)

func TestExtractHint(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		msg  string
		want string
	}{
		{name: "no separator", msg: "tool not found", want: ""},
		{name: "single separator", msg: "tool not found — brew install foo", want: "brew install foo"},
		{name: "multiple separators uses last", msg: "a — b — c", want: "c"},
		{name: "trims whitespace", msg: "fail —   brew install foo  ", want: "brew install foo"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, extractHint(tc.msg))
		})
	}
}

func TestPlural(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "1 error", plural(1, "error"))
	assert.Equal(t, "0 errors", plural(0, "error"))
	assert.Equal(t, "2 warnings", plural(2, "warning"))
}

func TestColor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		opts BoardOpts
	}{
		{name: "no color", opts: BoardOpts{NoColor: true, IsTTY: true}, want: "[✓]"},
		{name: "non tty", opts: BoardOpts{IsTTY: false}, want: "[✓]"},
		{name: "tty with color", opts: BoardOpts{IsTTY: true}, want: colorGreen + "[✓]" + colorReset},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, color("[✓]", colorGreen, tc.opts))
		})
	}
}

func TestGlyphFor(t *testing.T) {
	t.Parallel()

	opts := BoardOpts{NoColor: true}

	tests := []struct {
		name string
		want string
		r    engine.Result
	}{
		{
			name: "pass error severity",
			r:    engine.Result{Pass: true, Severity: engine.SeverityError},
			want: "[✓]",
		},
		{
			name: "pass info",
			r:    engine.Result{Pass: true, Severity: engine.SeverityInfo},
			want: "[ℹ]",
		},
		{
			name: "fail error",
			r:    engine.Result{Pass: false, Severity: engine.SeverityError},
			want: "[✗]",
		},
		{
			name: "fail warn",
			r:    engine.Result{Pass: false, Severity: engine.SeverityWarn},
			want: "[!]",
		},
		{
			name: "fail info",
			r:    engine.Result{Pass: false, Severity: engine.SeverityInfo},
			want: "[ℹ]",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, glyphFor(tc.r, opts))
		})
	}
}

func TestPrintSummary(t *testing.T) {
	t.Parallel()

	opts := BoardOpts{NoColor: true}

	tests := []struct {
		name    string
		want    string
		results []engine.Result
	}{
		{
			name: "all ok",
			results: []engine.Result{
				{Label: "git", Kind: engine.KindLeaf, Pass: true},
				{Label: "go", Kind: engine.KindLeaf, Pass: true},
			},
			want: "✓ 2 ok\n",
		},
		{
			name: "warnings only",
			results: []engine.Result{
				{Label: "git", Kind: engine.KindLeaf, Pass: true},
				{Label: "lint", Kind: engine.KindLeaf, Pass: false, Severity: engine.SeverityWarn},
			},
			want: "! 1 warning, 1 ok — see [!] items above\n",
		},
		{
			name: "errors and warnings",
			results: []engine.Result{
				{Label: "Core", Kind: engine.KindHeader, Pass: false},
				{Label: "child", Kind: engine.KindDelegate, Pass: false},
				{Label: "git", Kind: engine.KindLeaf, Pass: true},
				{Label: "lint", Kind: engine.KindLeaf, Pass: false, Severity: engine.SeverityWarn},
				{Label: "go", Kind: engine.KindLeaf, Pass: false, Severity: engine.SeverityError},
			},
			want: "✗ 1 error, 1 warning, 1 ok — fix the [✗] items above\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			printSummary(&buf, tc.results, opts)
			assert.Equal(t, tc.want, buf.String())
		})
	}
}

func TestPrintRemediation(t *testing.T) {
	t.Parallel()

	opts := BoardOpts{NoColor: true}

	tests := []struct {
		name    string
		want    string
		results []engine.Result
	}{
		{
			name: "no hints",
			results: []engine.Result{
				{Label: "git", Kind: engine.KindLeaf, Pass: true},
			},
			want: "",
		},
		{
			name: "one hint",
			results: []engine.Result{
				{Label: "go", Kind: engine.KindLeaf, Pass: false, Severity: engine.SeverityError, Message: "go not found — brew install go"},
			},
			want: "To fix, run:\n    brew install go\n",
		},
		{
			name: "dedupe hints",
			results: []engine.Result{
				{Label: "a", Kind: engine.KindLeaf, Pass: false, Severity: engine.SeverityError, Message: "a missing — brew install go"},
				{Label: "b", Kind: engine.KindLeaf, Pass: false, Severity: engine.SeverityError, Message: "b missing — brew install go"},
			},
			want: "To fix, run:\n    brew install go\n",
		},
		{
			name: "skip header pass and info",
			results: []engine.Result{
				{Label: "Core", Kind: engine.KindHeader, Pass: false},
				{Label: "ok", Kind: engine.KindLeaf, Pass: true},
				{Label: "info", Kind: engine.KindLeaf, Pass: false, Severity: engine.SeverityInfo, Message: "skip — hint"},
				{Label: "fail", Kind: engine.KindLeaf, Pass: false, Severity: engine.SeverityError, Message: "missing — brew install gh"},
			},
			want: "To fix, run:\n    brew install gh\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			printRemediation(&buf, tc.results, opts)
			assert.Equal(t, tc.want, buf.String())
		})
	}
}

func TestRenderResults(t *testing.T) {
	t.Parallel()

	opts := BoardOpts{NoColor: true}

	t.Run("header and delegate indent", func(t *testing.T) {
		t.Parallel()
		results := []engine.Result{
			{Label: "Core", Kind: engine.KindHeader, Pass: true, Depth: 0},
			{Label: "child", Kind: engine.KindDelegate, Pass: true, Depth: 1},
		}
		var buf bytes.Buffer
		renderResults(&buf, results, opts)
		got := buf.String()
		assert.Contains(t, got, "[✓] Core\n")
		assert.Contains(t, got, "    [✓] child\n")
	})

	t.Run("failing leaf with message", func(t *testing.T) {
		t.Parallel()
		results := []engine.Result{
			{Label: "go", Kind: engine.KindLeaf, Pass: false, Severity: engine.SeverityError, Message: "go not found — brew install go", Depth: 1},
		}
		var buf bytes.Buffer
		renderResults(&buf, results, opts)
		assert.Equal(t, "    [✗] go — go not found — brew install go\n", buf.String())
	})

	t.Run("quiet skips passing leaf", func(t *testing.T) {
		t.Parallel()
		results := []engine.Result{
			{Label: "git", Kind: engine.KindLeaf, Pass: true, Depth: 0},
			{Label: "go", Kind: engine.KindLeaf, Pass: false, Severity: engine.SeverityError, Message: "missing", Depth: 0},
		}
		var buf bytes.Buffer
		renderResults(&buf, results, BoardOpts{NoColor: true, Quiet: true})
		assert.Equal(t, "[✗] go — missing\n", buf.String())
	})
}

func TestBoard(t *testing.T) {
	t.Parallel()

	opts := BoardOpts{NoColor: true, Profile: "default"}

	t.Run("pass only no remediation", func(t *testing.T) {
		t.Parallel()
		results := []engine.Result{
			{Label: "git", Kind: engine.KindLeaf, Pass: true},
		}
		var buf bytes.Buffer
		Board(&buf, results, opts)
		got := buf.String()
		assert.True(t, strings.HasPrefix(got, "triage (profile: default)\n\n"))
		assert.Contains(t, got, "[✓] git\n")
		assert.Contains(t, got, "✓ 1 ok\n")
		assert.NotContains(t, got, "To fix, run:")
	})

	t.Run("mixed pass fail with remediation", func(t *testing.T) {
		t.Parallel()
		results := []engine.Result{
			{Label: "git", Kind: engine.KindLeaf, Pass: true},
			{Label: "go", Kind: engine.KindLeaf, Pass: false, Severity: engine.SeverityError, Message: "go not found — brew install go"},
		}
		var buf bytes.Buffer
		Board(&buf, results, opts)
		got := buf.String()
		assert.Contains(t, got, "[✗] go — go not found — brew install go\n")
		assert.Contains(t, got, "To fix, run:\n    brew install go\n")
	})

	t.Run("empty results", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		Board(&buf, nil, BoardOpts{NoColor: true})
		got := buf.String()
		assert.Equal(t, "triage (profile: default)\n\n\n✓ 0 ok\n", got)
	})
}
