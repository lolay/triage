package cli

import (
	"fmt"
	"strings"
)

// parseCLIVars parses repeatable --var name=value entries.
func parseCLIVars(entries []string) (map[string]string, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(entries))
	for _, e := range entries {
		name, val, ok := strings.Cut(e, "=")
		if !ok || name == "" {
			return nil, fmt.Errorf("invalid --var %q: expected name=value", e)
		}
		out[name] = val
	}
	return out, nil
}

// mergeVarMaps overlays cli on top of config (CLI wins on duplicate keys).
func mergeVarMaps(configVars, cliVars map[string]string) map[string]string {
	if len(configVars) == 0 && len(cliVars) == 0 {
		return nil
	}
	out := make(map[string]string, len(configVars)+len(cliVars))
	for k, v := range configVars {
		out[k] = v
	}
	for k, v := range cliVars {
		out[k] = v
	}
	return out
}
