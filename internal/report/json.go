package report

import (
	"encoding/json"
	"io"

	"github.com/lolay/triage/internal/engine"
)

// JSONReport is the top-level structure for --json output (spec §7.3).
type JSONReport struct {
	Profile string       `json:"profile"`
	Results []JSONResult `json:"results"`
	Summary JSONSummary  `json:"summary"`
}

// JSONResult represents one check outcome in the machine-readable report.
// Shape per spec §7.3: group, name, severity, status, detail, command_log_path,
// plus depth/kind so consumers can reconstruct the delegate tree deterministically.
//
// Field order is the JSON output contract: encoding/json emits keys in
// declaration order, so this layout must stay stable (locked by
// TestJSON_KeyOrder). It is therefore exempt from govet's fieldalignment.
//
//nolint:govet // output key order is part of the JSON contract (spec §7.3)
type JSONResult struct {
	Name           string `json:"name"`
	Kind           string `json:"kind"` // "check" | "group" | "delegate"
	Depth          int    `json:"depth"`
	Severity       string `json:"severity"`
	Status         string `json:"status"` // "pass" | "fail" | "warn" | "info"
	Detail         string `json:"detail,omitempty"`
	CommandLogPath string `json:"command_log_path,omitempty"`
}

// JSONSummary holds aggregate counts.
type JSONSummary struct {
	OK       int `json:"ok"`
	Warnings int `json:"warnings"`
	Errors   int `json:"errors"`
}

// JSON writes the machine-readable report to w. commandLogPath is the path of
// the active --command-log file (empty when not set); it is included in failing
// results that have subprocess output so callers can inspect the full log.
func JSON(w io.Writer, results []engine.Result, profile, commandLogPath string) error {
	report := buildJSONReport(profile, commandLogPath, results)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func buildJSONReport(profile, commandLogPath string, results []engine.Result) JSONReport {
	report := JSONReport{
		Profile: profile,
		Results: make([]JSONResult, 0, len(results)),
	}
	for _, r := range results {
		jr := JSONResult{
			Name:     r.Label,
			Kind:     jsonKind(r),
			Depth:    r.Depth,
			Severity: severityString(r.Severity),
			Status:   statusString(r),
			Detail:   r.Message,
		}
		// Enrich failing results that have subprocess output with the log path.
		if !r.Pass && r.Output != "" && commandLogPath != "" {
			jr.CommandLogPath = commandLogPath
		}
		report.Results = append(report.Results, jr)
		if r.Kind == engine.KindHeader || r.Kind == engine.KindDelegate {
			continue // aggregate lines; children are counted
		}
		if r.Pass {
			report.Summary.OK++
		} else {
			switch r.Severity {
			case engine.SeverityError:
				report.Summary.Errors++
			case engine.SeverityWarn:
				report.Summary.Warnings++
			}
		}
	}
	return report
}

func statusString(r engine.Result) string {
	if r.Pass {
		if r.Severity == engine.SeverityInfo {
			return "info"
		}
		return "pass"
	}
	switch r.Severity {
	case engine.SeverityWarn:
		return "warn"
	case engine.SeverityInfo:
		return "info"
	default:
		return "fail"
	}
}

func jsonKind(r engine.Result) string {
	switch r.Kind {
	case engine.KindHeader:
		return "group"
	case engine.KindDelegate:
		return "delegate"
	default:
		return "check"
	}
}

func severityString(s engine.Severity) string {
	switch s {
	case engine.SeverityError:
		return "error"
	case engine.SeverityWarn:
		return "warn"
	case engine.SeverityInfo:
		return "info"
	default:
		return ""
	}
}
