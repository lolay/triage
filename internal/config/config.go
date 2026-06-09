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
	// Version is the config format version (spec §4). Absent in YAML defaults to 1.
	Version int
}

// Profile is the ordered list of checks for a named profile.
type Profile []Check

// Check type keys (spec §5). Exactly one is the discriminator for any check.
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
//
// Fields are ordered for struct alignment, not by topic:
//   - common: Type (discriminator), Value (primary value), Severity, Required
//     (sugar resolved to Severity by the engine), Platform (os guard), Hint,
//     Dir, Label (required on command; identity key for command/one_of), Serial
//     (run as a barrier under the global serial lock, §7.2).
//   - tool: Version (range as written), VersionFrom (pin-file path), Constraint
//     (resolved npm-style constraint from Version or VersionFrom).
//   - group/one_of: Items (nested children), OneOf (alternatives; bare tool
//     names are normalized to {tool: <name>}).
//   - env: Unset, Matches. command: Contains, Exit, Interp, WithEnv (per-command
//     env injection layered over inherited env). delegate: Config (child config
//     path; defaults to the discovered config under Dir).
//   - source: the file that declared the check (for collision diagnostics).
type Check struct {
	Required    *bool
	WithEnv     map[string]string
	Exit        *int
	Label       string
	Version     string
	Type        string
	Hint        string
	Dir         string
	Severity    string
	source      string
	VersionFrom string
	Constraint  string
	Config      string
	Value       string
	Interp      string
	Matches     string
	Contains    string
	Platform    []string
	OneOf       []Check
	Items       []Check
	Unset       bool
	Serial      bool
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
// Pointer/zero-aware fields (Tool/Env/Path/Command/Delegate/Group/Required/Exit)
// let UnmarshalYAML detect which keys were present; the rest are plain scalars.
// Fields are ordered for struct alignment, not by topic.
type checkFields struct {
	Required    *bool             `yaml:"required"`
	Env         *string           `yaml:"env"`
	Path        *string           `yaml:"path"`
	WithEnv     map[string]string `yaml:"with_env"`
	Command     *string           `yaml:"command"`
	Delegate    *string           `yaml:"delegate"`
	Group       *string           `yaml:"group"`
	Exit        *int              `yaml:"exit"`
	Tool        *string           `yaml:"tool"`
	Dir         string            `yaml:"dir"`
	VersionFrom string            `yaml:"version_from"`
	Hint        string            `yaml:"hint"`
	Severity    string            `yaml:"severity"`
	Label       string            `yaml:"label"`
	Config      string            `yaml:"config"`
	Version     string            `yaml:"version"`
	Interp      string            `yaml:"interp"`
	Contains    string            `yaml:"contains"`
	Matches     string            `yaml:"matches"`
	Items       []Check           `yaml:"items"`
	Platform    stringList        `yaml:"platform"`
	OneOf       []oneOfElem       `yaml:"one_of"`
	Unset       bool              `yaml:"unset"`
	Serial      bool              `yaml:"serial"`
}

// UnmarshalYAML resolves the single reserved type key for a check, erroring on
// zero or multiple primary type keys.
func (c *Check) UnmarshalYAML(unmarshal func(any) error) error {
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
	if f.Group != nil {
		primary = append(primary, typeKey{TypeGroup, *f.Group})
	}

	switch len(primary) {
	case 0:
		return fmt.Errorf("check has no type key: expected one of " +
			"tool/env/path/one_of/command/delegate/group")
	case 1:
		c.Type = primary[0].name
		c.Value = primary[0].value
	default:
		names := make([]string, len(primary))
		for i, p := range primary {
			names[i] = p.name
		}
		msg := fmt.Sprintf("check has multiple type keys (%s): exactly one required",
			strings.Join(names, ", "))
		if f.Group != nil {
			msg += "; nest checks under a structural group via group: and items:"
		}
		return fmt.Errorf("%s", msg)
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
func (e *oneOfElem) UnmarshalYAML(unmarshal func(any) error) error {
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
func (s *stringList) UnmarshalYAML(unmarshal func(any) error) error {
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
