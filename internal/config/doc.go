// Package config resolves and loads the triage configuration.
//
// m1 ships config discovery (resolve the [config] positional: cwd lookup of
// triage.yaml then .triage.yaml, a directory argument, or an explicit file;
// not-found is a usage error, exit 3 — see spec §4) plus minimal types
// (Config, Profile, Check). The real triage.yaml schema and loader
// (include/extends/add, validation, JSON Schema) are deferred to m2; until
// then the loader is a thin stub that returns the selected profile as an
// empty check list.
package config
