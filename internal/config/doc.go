// Package config resolves and loads the triage configuration.
//
// Discovery (m1) resolves the [config] positional: a cwd lookup of triage.yaml,
// triage.yml, .triage.yaml, then .triage.yml (in that precedence order), a
// directory argument, or an explicit file; not-found is a usage error, exit 3
// (spec §4).
//
// Loading (m2 s1) parses the full triage.yaml schema with github.com/goccy/go-yaml
// and produces a composite, inheritance-resolved Config:
//   - version: optional format version (default 1); unsupported values are a load error;
//   - type-as-key checks: each Check carries a single reserved type key
//     (tool/env/path/one_of/command/delegate/group) plus the common and
//     type-specific fields (spec §5);
//   - include: top-level files merged in order, collisions flagged (later wins),
//     cycle/diamond-safe;
//   - vars: top-level reusable values merged across includes (later wins);
//     referenced via {{ name }} in check fields at runtime;
//   - extends/add: per-profile inheritance resolved within the merged config,
//     cycle/diamond-safe;
//   - version_from: pin files read relative to the declaring file and resolved
//     into npm-style constraints the tool check consumes;
//   - validation + non-fatal Warnings surfaced for the CLI to print to stderr.
//
// A JSON Schema (schema/triage.schema.json) ships for editor autocomplete and
// validation. Executing the parsed checks lives in internal/engine; the tool
// check and grouped board land across the rest of m2.
package config
