package cli

import "github.com/lolay/triage/internal/engine"

// Exit code constants (spec §7.3).
//
// Default (pass-fail) mode: 0 = all pass, 1 = any failure, 3 = config/usage error.
// Severity mode (--severity flag): 0 = clean, 1 = warnings only, 2 = errors, 3 = config/usage error.
const (
	ExitOK          = 0   // all checks pass
	ExitFail        = 1   // any check fails (default pass-fail mode)
	ExitError       = 2   // --severity: ≥1 error-severity failure
	ExitUsageError  = 3   // config not found, flag parse error, or usage error
	ExitInterrupted = 130 // run cancelled by SIGINT/SIGTERM (128 + signal)
)

// ExitCode computes the process exit code from check results and active flags.
//
// --strict escalates SeverityWarn results to SeverityError before the
// exit-code calculation is applied. --severity enables the 4-level ladder;
// without it, any failure returns ExitFail (1).
func ExitCode(results []engine.Result, strict, severityMode bool) int {
	var hasError, hasWarn bool
	for _, r := range results {
		if r.Pass {
			continue
		}
		sev := r.Severity
		if strict && sev == engine.SeverityWarn {
			sev = engine.SeverityError
		}
		switch sev {
		case engine.SeverityError:
			hasError = true
		case engine.SeverityWarn:
			hasWarn = true
		}
	}

	if severityMode {
		switch {
		case hasError:
			return ExitError
		case hasWarn:
			return ExitFail // code 1 = warnings only in severity mode
		default:
			return ExitOK
		}
	}

	if hasError {
		return ExitFail
	}
	return ExitOK
}
