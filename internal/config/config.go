package config

import (
	"fmt"
	"strings"
)

// Config is the fully-resolved root of a parsed triage.yaml.
//
// Profiles is the composite, inheritance-resolved set: include files are merged
// in order and every profile's extends/add chain is expanded into a flat,
// ordered check list. Warnings holds non-fatal load-time diagnostics (collisions,
// contradictory fields, unreadable version_from pins) for the CLI to print to
// stderr before the board.
type Config struct {
	// Path is the resolved absolute path of the entry config file.
	Path string
	// Profiles maps profile names to their resolved, ordered check lists.
	Profiles map[string]Profile
	// Vars holds merged top-level vars: values (referenced via {{ name }} in checks).
	Vars map[string]string
	// Warnings are non-fatal load-time diagnostics, in deterministic order.
	Warnings []string
}

// Profile is the ordered list of checks for a named profile.
type Profile []Check

// Check type keys (spec §5). Exactly one is the discriminator for any check;
// group doubles as a legacy string field when another type key is present.
const (
	TypeTool     = "tool"
	TypeEnv      = "env"
	TypePath     = "path"
	TypeOneOf    = "one_of"
	TypeCommand  = "command"
	TypeDelegate = "delegate"
	TypeGroup    = "group"
)

// Check is one declarative check (spec §5). Type is the discriminator and Value
// carries the primary value for that type (tool name, env var, path glob,
// group/delegate display name; empty for one_of). The remaining fields are the
// optional common fields plus the type-specific fields; only those relevant to
// Type are populated.
type Check struct {
	Type  string // one of the Type* constants
	Value string // primary value for the type key

	// Common optional fields (valid on any check).
	Severity string   // "error" | "warn" | "info"; "" means default (error)
	Required *bool    // required: sugar; resolved to Severity by the engine (m2 s3)
	Group    string   // legacy `group:` string field (flat section label)
	Platform []string // os guard(s): macos | linux | windows
	Hint     string
	Dir      string
	Label    string // required on command; identity key for command/one_of
	Serial   bool   // opt out of concurrency: run as a barrier under a global serial lock (§7.2)

	// tool fields.
	Version     string // npm-style range as written
	VersionFrom string // pin-file path as written
	Constraint  string // resolved npm-style constraint (from Version or VersionFrom)

	// group: nested children.
	Items []Check

	// one_of: alternatives (bare tool names are normalized to {tool: <name>}).
	OneOf []Check

	// env fields.
	Unset   bool
	Matches string

	// command fields.
	Contains string
	Exit     *int
	Interp   string
	WithEnv  map[string]string // per-command env injection (layered over inherited env)

	// delegate fields.
	Config string // child config path (defaults to the discovered config under Dir)

	// source is the file that declared the check (for collision diagnostics).
	source string
}

// identity returns the collision key for a check within a profile: the type key
// plus its primary value. command/one_of have no scalar primary value, so they
// key off the label (or, for an unlabeled one_of, its normalized alternatives).
func (c Check) identity() string {
	switch c.Type {
	case TypeCommand:
		return TypeCommand + ":" + c.Label
	case TypeOneOf:
		if c.Label != "" {
			return TypeOneOf + ":" + c.Label
		}
		alts := make([]string, 0, len(c.OneOf))
		for _, o := range c.OneOf {
			alts = append(alts, o.Type+":"+o.Value)
		}
		return TypeOneOf + ":[" + strings.Join(alts, ",") + "]"
	default:
		return c.Type + ":" + c.Value
	}
}

// checkFields mirrors the YAML surface of a check before type-key resolution.
// Pointer/zero-aware fields let UnmarshalYAML detect which keys were present.
type checkFields struct {
	Tool     *string     `yaml:"tool"`
	Env      *string     `yaml:"env"`
	Path     *string     `yaml:"path"`
	OneOf    []oneOfElem `yaml:"one_of"`
	Command  *string     `yaml:"command"`
	Delegate *string     `yaml:"delegate"`
	Group    *string     `yaml:"group"`
	Items    []Check     `yaml:"items"`

	Severity string     `yaml:"severity"`
	Required *bool      `yaml:"required"`
	Platform stringList `yaml:"platform"`
	Hint     string     `yaml:"hint"`
	Dir      string     `yaml:"dir"`
	Label    string     `yaml:"label"`
	Serial   bool       `yaml:"serial"`

	Version     string `yaml:"version"`
	VersionFrom string `yaml:"version_from"`

	Unset   bool   `yaml:"unset"`
	Matches string `yaml:"matches"`

	Contains string            `yaml:"contains"`
	Exit     *int              `yaml:"exit"`
	Interp   string            `yaml:"interp"`
	WithEnv  map[string]string `yaml:"with_env"`

	Config string `yaml:"config"`
}

// UnmarshalYAML resolves the single reserved type key for a check, erroring on
// zero or multiple primary type keys. `group` is the type only when no other
// primary type key is present; otherwise it is the legacy string field.
func (c *Check) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var f checkFields
	if err := unmarshal(&f); err != nil {
		return err
	}

	type typeKey struct {
		name  string
		value string
	}
	var primary []typeKey
	if f.Tool != nil {
		primary = append(primary, typeKey{TypeTool, *f.Tool})
	}
	if f.Env != nil {
		primary = append(primary, typeKey{TypeEnv, *f.Env})
	}
	if f.Path != nil {
		primary = append(primary, typeKey{TypePath, *f.Path})
	}
	if f.OneOf != nil {
		primary = append(primary, typeKey{TypeOneOf, ""})
	}
	if f.Command != nil {
		primary = append(primary, typeKey{TypeCommand, *f.Command})
	}
	if f.Delegate != nil {
		primary = append(primary, typeKey{TypeDelegate, *f.Delegate})
	}

	switch len(primary) {
	case 0:
		if f.Group == nil {
			return fmt.Errorf("check has no type key: expected one of " +
				"tool/env/path/one_of/command/delegate/group")
		}
		c.Type = TypeGroup
		c.Value = *f.Group
	case 1:
		c.Type = primary[0].name
		c.Value = primary[0].value
		if f.Group != nil { // legacy flat-section label alongside the real type
			c.Group = *f.Group
		}
	default:
		names := make([]string, len(primary))
		for i, p := range primary {
			names[i] = p.name
		}
		return fmt.Errorf("check has multiple type keys (%s): exactly one required",
			strings.Join(names, ", "))
	}

	c.Severity = f.Severity
	c.Required = f.Required
	c.Platform = []string(f.Platform)
	c.Hint = f.Hint
	c.Dir = f.Dir
	c.Label = f.Label
	c.Serial = f.Serial
	c.Version = f.Version
	c.VersionFrom = f.VersionFrom
	c.Unset = f.Unset
	c.Matches = f.Matches
	c.Contains = f.Contains
	c.Exit = f.Exit
	c.Interp = f.Interp
	c.WithEnv = f.WithEnv
	c.Config = f.Config
	c.Items = f.Items
	for _, e := range f.OneOf {
		c.OneOf = append(c.OneOf, e.check)
	}
	return nil
}

// oneOfElem is a single one_of alternative: a bare tool name (string) or a
// nested sub-check (mapping).
type oneOfElem struct {
	check Check
}

// UnmarshalYAML accepts either a bare tool name or a full sub-check.
func (e *oneOfElem) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var name string
	if err := unmarshal(&name); err == nil {
		e.check = Check{Type: TypeTool, Value: name}
		return nil
	}
	var c Check
	if err := unmarshal(&c); err != nil {
		return err
	}
	e.check = c
	return nil
}

// stringList accepts a single scalar or a sequence, yielding a []string.
type stringList []string

// UnmarshalYAML accepts either "x" or [x, y].
func (s *stringList) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var single string
	if err := unmarshal(&single); err == nil {
		*s = []string{single}
		return nil
	}
	var list []string
	if err := unmarshal(&list); err != nil {
		return err
	}
	*s = list
	return nil
}
