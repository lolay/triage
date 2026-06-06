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
// Shape per spec §7.3: group, name, severity, status, detail.
type JSONResult struct {
	Group    string `json:"group,omitempty"`
	Name     string `json:"name"`
	Severity string `json:"severity"`
	Status   string `json:"status"` // "pass" | "fail" | "warn" | "info"
	Detail   string `json:"detail,omitempty"`
}

// JSONSummary holds aggregate counts.
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
			Group:    r.Group,
			Name:     r.Label,
			Severity: severityString(r.Severity),
			Status:   statusString(r),
			Detail:   r.Message,
		})
		if r.Kind == engine.KindHeader {
			continue // don't count headers in the summary
		}
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
