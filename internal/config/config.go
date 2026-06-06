package config

// Config is the root of a parsed triage.yaml.
// The full schema (include/extends/add, validation, JSON Schema) lands in m2;
// this stub is sufficient for m1.
type Config struct {
	// Path is the resolved absolute path of the config file.
	Path string
	// Profiles maps profile names to their ordered check lists.
	Profiles map[string]Profile
}

// Profile is the ordered list of checks for a named profile.
// In m1 all profiles are empty; real check types land in m2.
type Profile []Check

// Check is a placeholder type. The real schema and execution land in m2.
type Check struct{}
