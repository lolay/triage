package report

import (
	"encoding/json"
	"io"

	"github.com/lolay/triage/internal/engine"
)

// JSONReport is the top-level structure for --json output.
type JSONReport struct {
	Profile string       `json:"profile"`
	Results []JSONResult `json:"results"`
	Summary JSONSummary  `json:"summary"`
}

// JSONResult represents a single check outcome.
type JSONResult struct {
	Label    string `json:"label"`
	Pass     bool   `json:"pass"`
	Severity string `json:"severity,omitempty"`
	Message  string `json:"message,omitempty"`
}

// JSONSummary holds aggregate pass/warn/error counts.
type JSONSummary struct {
	OK       int `json:"ok"`
	Warnings int `json:"warnings"`
	Errors   int `json:"errors"`
}

// JSON writes the machine-readable report to w.
func JSON(w io.Writer, results []engine.Result, profile string) error {
	report := buildJSONReport(profile, results)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func buildJSONReport(profile string, results []engine.Result) JSONReport {
	jr := JSONReport{
		Profile: profile,
		Results: make([]JSONResult, 0, len(results)),
	}
	for _, r := range results {
		jr.Results = append(jr.Results, JSONResult{
			Label:    r.Label,
			Pass:     r.Pass,
			Severity: severityString(r.Severity),
			Message:  r.Message,
		})
		if r.Pass {
			jr.Summary.OK++
		} else {
			switch r.Severity {
			case engine.SeverityError:
				jr.Summary.Errors++
			case engine.SeverityWarn:
				jr.Summary.Warnings++
			}
		}
	}
	return jr
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
