package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/lolay/triage/internal/engine"
)

func TestExitCode(t *testing.T) {
	t.Parallel()

	pass := engine.Result{Pass: true, Severity: engine.SeverityError}
	failErr := engine.Result{Pass: false, Severity: engine.SeverityError}
	failWarn := engine.Result{Pass: false, Severity: engine.SeverityWarn}

	tests := []struct {
		name         string
		results      []engine.Result
		want         int
		strict       bool
		severityMode bool
	}{
		{name: "all-pass", results: []engine.Result{pass, pass}, want: ExitOK},
		{name: "default-fail-on-error", results: []engine.Result{failErr}, want: ExitFail},
		{name: "default-warn-is-ok", results: []engine.Result{failWarn}, want: ExitOK},
		{name: "strict-escalates-warn", results: []engine.Result{failWarn}, strict: true, want: ExitFail},
		{name: "severity-error", results: []engine.Result{failErr}, severityMode: true, want: ExitError},
		{name: "severity-warn-only", results: []engine.Result{failWarn}, severityMode: true, want: ExitFail},
		{name: "severity-clean", results: []engine.Result{pass}, severityMode: true, want: ExitOK},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := ExitCode(tc.results, tc.strict, tc.severityMode)
			assert.Equal(t, tc.want, got)
		})
	}
}
