package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/lolay/triage/internal/engine"
)

// TestJSON_KeyOrder locks the key order of --json result objects. encoding/json
// emits struct keys in declaration order, so a field reorder (e.g. govet's
// fieldalignment) would silently change the output shape (spec §7.3). This test
// fails if that happens. JSONResult carries a //nolint:govet for the same reason.
func TestJSON_KeyOrder(t *testing.T) {
	var buf bytes.Buffer
	results := []engine.Result{
		{
			Label:    "fake-tool",
			Kind:     engine.KindLeaf,
			Severity: engine.SeverityError,
			Pass:     false,
			Message:  "fake-tool not found — install it",
			Depth:    1,
		},
	}
	require.NoError(t, JSON(&buf, results, "default", ""), "JSON")
	got := buf.String()

	// Among the always-present + populated keys, this is the documented order.
	want := []string{"name", "kind", "depth", "severity", "status", "detail"}
	lastIdx := -1
	for _, key := range want {
		idx := strings.Index(got, `"`+key+`"`)
		require.GreaterOrEqualf(t, idx, 0, "key %q missing from JSON output:\n%s", key, got)
		assert.GreaterOrEqualf(t, idx, lastIdx, "key %q is out of declaration order in JSON output:\n%s", key, got)
		lastIdx = idx
	}
}

func decodeJSONReport(t *testing.T, results []engine.Result, profile, commandLogPath string) JSONReport {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, JSON(&buf, results, profile, commandLogPath))
	var report JSONReport
	require.NoError(t, json.Unmarshal(buf.Bytes(), &report))
	return report
}

func TestJSON_WarnResult(t *testing.T) {
	t.Parallel()

	results := []engine.Result{
		{
			Label:    "lint",
			Kind:     engine.KindLeaf,
			Severity: engine.SeverityWarn,
			Pass:     false,
			Message:  "lint missing",
			Depth:    0,
		},
	}
	report := decodeJSONReport(t, results, "default", "")
	require.Len(t, report.Results, 1)
	assert.Equal(t, "warn", report.Results[0].Status)
	assert.Equal(t, 0, report.Summary.OK)
	assert.Equal(t, 1, report.Summary.Warnings)
	assert.Equal(t, 0, report.Summary.Errors)
}

func TestJSON_PassResult(t *testing.T) {
	t.Parallel()

	results := []engine.Result{
		{
			Label:    "git",
			Kind:     engine.KindLeaf,
			Severity: engine.SeverityError,
			Pass:     true,
			Depth:    0,
		},
	}
	report := decodeJSONReport(t, results, "default", "")
	require.Len(t, report.Results, 1)
	assert.Equal(t, "pass", report.Results[0].Status)
	assert.Equal(t, 1, report.Summary.OK)
	assert.Equal(t, 0, report.Summary.Warnings)
	assert.Equal(t, 0, report.Summary.Errors)
}

func TestJSON_InfoStatus(t *testing.T) {
	t.Parallel()

	t.Run("pass info", func(t *testing.T) {
		t.Parallel()
		results := []engine.Result{
			{Label: "skip", Kind: engine.KindLeaf, Severity: engine.SeverityInfo, Pass: true},
		}
		report := decodeJSONReport(t, results, "default", "")
		assert.Equal(t, "info", report.Results[0].Status)
		assert.Equal(t, 1, report.Summary.OK)
	})

	t.Run("fail info", func(t *testing.T) {
		t.Parallel()
		results := []engine.Result{
			{Label: "skip", Kind: engine.KindLeaf, Severity: engine.SeverityInfo, Pass: false, Message: "skipped"},
		}
		report := decodeJSONReport(t, results, "default", "")
		assert.Equal(t, "info", report.Results[0].Status)
		assert.Equal(t, 0, report.Summary.OK)
		assert.Equal(t, 0, report.Summary.Warnings)
		assert.Equal(t, 0, report.Summary.Errors)
	})
}

func TestJSON_KindMapping(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		kind engine.ResultKind
	}{
		{name: "header", kind: engine.KindHeader, want: "group"},
		{name: "delegate", kind: engine.KindDelegate, want: "delegate"},
		{name: "leaf", kind: engine.KindLeaf, want: "check"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			results := []engine.Result{{Label: "x", Kind: tc.kind, Pass: true}}
			report := decodeJSONReport(t, results, "default", "")
			assert.Equal(t, tc.want, report.Results[0].Kind)
		})
	}
}

func TestJSON_SeverityString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		want     string
		severity engine.Severity
	}{
		{name: "error", severity: engine.SeverityError, want: "error"},
		{name: "warn", severity: engine.SeverityWarn, want: "warn"},
		{name: "info", severity: engine.SeverityInfo, want: "info"},
		{name: "zero", severity: engine.Severity(99), want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			results := []engine.Result{{Label: "x", Kind: engine.KindLeaf, Severity: tc.severity, Pass: true}}
			report := decodeJSONReport(t, results, "default", "")
			assert.Equal(t, tc.want, report.Results[0].Severity)
		})
	}
}

func TestJSON_CommandLogPath(t *testing.T) {
	t.Parallel()

	failWithOutput := engine.Result{
		Label:    "cmd",
		Kind:     engine.KindLeaf,
		Severity: engine.SeverityError,
		Pass:     false,
		Output:   "stderr output",
		Message:  "command failed",
	}

	t.Run("failing with output and path", func(t *testing.T) {
		t.Parallel()
		report := decodeJSONReport(t, []engine.Result{failWithOutput}, "default", "/tmp/triage-cmd.log")
		assert.Equal(t, "/tmp/triage-cmd.log", report.Results[0].CommandLogPath)
	})

	t.Run("passing omits path", func(t *testing.T) {
		t.Parallel()
		pass := failWithOutput
		pass.Pass = true
		report := decodeJSONReport(t, []engine.Result{pass}, "default", "/tmp/triage-cmd.log")
		assert.Empty(t, report.Results[0].CommandLogPath)
	})

	t.Run("no path omits field", func(t *testing.T) {
		t.Parallel()
		report := decodeJSONReport(t, []engine.Result{failWithOutput}, "default", "")
		assert.Empty(t, report.Results[0].CommandLogPath)
	})
}

func TestJSON_HeaderExcludedFromSummary(t *testing.T) {
	t.Parallel()

	results := []engine.Result{
		{Label: "Core", Kind: engine.KindHeader, Pass: false},
		{Label: "git", Kind: engine.KindLeaf, Pass: true},
	}
	report := decodeJSONReport(t, results, "default", "")
	assert.Equal(t, 1, report.Summary.OK)
	assert.Equal(t, 0, report.Summary.Warnings)
	assert.Equal(t, 0, report.Summary.Errors)
}

func TestStatusString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		want string
		r    engine.Result
	}{
		{name: "pass", r: engine.Result{Pass: true, Severity: engine.SeverityError}, want: "pass"},
		{name: "pass info", r: engine.Result{Pass: true, Severity: engine.SeverityInfo}, want: "info"},
		{name: "fail warn", r: engine.Result{Pass: false, Severity: engine.SeverityWarn}, want: "warn"},
		{name: "fail info", r: engine.Result{Pass: false, Severity: engine.SeverityInfo}, want: "info"},
		{name: "fail error", r: engine.Result{Pass: false, Severity: engine.SeverityError}, want: "fail"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, statusString(tc.r))
		})
	}
}

func TestJsonKind(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "group", jsonKind(engine.Result{Kind: engine.KindHeader}))
	assert.Equal(t, "delegate", jsonKind(engine.Result{Kind: engine.KindDelegate}))
	assert.Equal(t, "check", jsonKind(engine.Result{Kind: engine.KindLeaf}))
}

func TestSeverityString(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "error", severityString(engine.SeverityError))
	assert.Equal(t, "warn", severityString(engine.SeverityWarn))
	assert.Equal(t, "info", severityString(engine.SeverityInfo))
	assert.Equal(t, "", severityString(engine.Severity(99)))
}
