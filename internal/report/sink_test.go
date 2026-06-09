package report

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lolay/triage/internal/engine"
)

func TestTTYSinkBegin(t *testing.T) {
	t.Parallel()

	opts := BoardOpts{NoColor: true}

	t.Run("default profile", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		sink := NewTTYSink(&buf, opts)
		sink.Begin("")
		assert.Equal(t, "triage (profile: default)\n\n", buf.String())
	})

	t.Run("custom profile", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		sink := NewTTYSink(&buf, opts)
		sink.Begin("ci")
		assert.Equal(t, "triage (profile: ci)\n\n", buf.String())
	})
}

func TestTTYSinkEmit(t *testing.T) {
	t.Parallel()

	opts := BoardOpts{NoColor: true}

	tests := []struct {
		name string
		want string
		r    engine.Result
	}{
		{
			name: "header kind",
			r:    engine.Result{Label: "Core", Kind: engine.KindHeader, Pass: true, Depth: 0},
			want: "[✓] Core\n",
		},
		{
			name: "delegate kind",
			r:    engine.Result{Label: "child", Kind: engine.KindDelegate, Pass: true, Depth: 1},
			want: "    [✓] child\n",
		},
		{
			name: "passing leaf",
			r:    engine.Result{Label: "git", Kind: engine.KindLeaf, Pass: true, Depth: 0},
			want: "[✓] git\n",
		},
		{
			name: "failing leaf with message",
			r: engine.Result{
				Label:    "go",
				Kind:     engine.KindLeaf,
				Pass:     false,
				Severity: engine.SeverityError,
				Message:  "go not found — brew install go",
				Depth:    1,
			},
			want: "    [✗] go — go not found — brew install go\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			s := NewTTYSink(&buf, opts)
			s.Emit(tc.r)
			assert.Equal(t, tc.want, buf.String())
		})
	}

	t.Run("quiet suppresses passing leaf", func(t *testing.T) {
		t.Parallel()
		var buf bytes.Buffer
		s := NewTTYSink(&buf, BoardOpts{NoColor: true, Quiet: true})
		s.Emit(engine.Result{Label: "git", Kind: engine.KindLeaf, Pass: true})
		assert.Empty(t, buf.String())
	})
}

func TestTTYSinkEnd(t *testing.T) {
	t.Parallel()

	results := []engine.Result{
		{Label: "git", Kind: engine.KindLeaf, Pass: true},
		{Label: "go", Kind: engine.KindLeaf, Pass: false, Severity: engine.SeverityError, Message: "go not found — brew install go"},
	}

	var buf bytes.Buffer
	sink := NewTTYSink(&buf, BoardOpts{NoColor: true})
	sink.End(results)

	got := buf.String()
	assert.Contains(t, got, "✗ 1 error, 0 warnings, 1 ok — fix the [✗] items above\n")
	assert.Contains(t, got, "To fix, run:\n    brew install go\n")
}
