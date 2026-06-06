package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	yaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
)

// reservedTypeKeys are check type keys; finding one at the document root means a
// bare check was written where a profile key belongs (spec §4).
var reservedTypeKeys = map[string]bool{
	TypeTool: true, TypeEnv: true, TypePath: true, TypeOneOf: true,
	TypeCommand: true, TypeDelegate: true, TypeGroup: true,
}

// load parses, composes, and resolves the config rooted at the entry file path.
//
// Pipeline (spec §4): parse → process include: files in order (merging profiles,
// flagging collisions, cycle/diamond-safe) → resolve each profile's extends/add
// chain → produce a Config whose Profiles are flat, ordered check lists. Pin
// files referenced by version_from are read relative to the declaring file and
// resolved into npm-style constraints. Non-fatal issues accumulate in Warnings.
func load(path string) (*Config, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	l := &loader{
		composite: map[string]*compositeProfile{},
		memo:      map[string]Profile{},
	}
	if err := l.loadFile(abs, map[string]bool{}); err != nil {
		return nil, err
	}

	// Resolve in sorted profile order so Warnings are deterministic.
	names := make([]string, 0, len(l.composite))
	for name := range l.composite {
		names = append(names, name)
	}
	sort.Strings(names)

	resolved := make(map[string]Profile, len(names))
	for _, name := range names {
		p, err := l.resolveProfile(name, map[string]bool{})
		if err != nil {
			return nil, err
		}
		resolved[name] = p
	}

	return &Config{Path: abs, Profiles: resolved, Warnings: l.warnings}, nil
}

// compositeProfile is a profile after include-merge but before extends/add
// resolution: accumulated parents plus this-and-included direct checks.
type compositeProfile struct {
	extends []string
	checks  []Check
}

type loader struct {
	composite map[string]*compositeProfile
	memo      map[string]Profile
	warnings  []string
}

func (l *loader) warnf(format string, args ...interface{}) {
	l.warnings = append(l.warnings, fmt.Sprintf(format, args...))
}

// loadFile parses one file and merges its include: chain (processed first, in
// order) and then its own profiles into the composite. visited makes include
// cycle/diamond-safe: each file merges at most once per load.
func (l *loader) loadFile(absPath string, visited map[string]bool) error {
	if visited[absPath] {
		return nil
	}
	visited[absPath] = true

	data, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("reading config %s: %w", absPath, err)
	}
	file, err := parser.ParseBytes(data, 0)
	if err != nil {
		return fmt.Errorf("parsing config %s: %w", absPath, err)
	}

	body := documentBody(file)
	if body == nil || body.Type() == ast.NullType {
		return nil // empty document: no profiles
	}
	values, ok := mappingValues(body)
	if !ok {
		return fmt.Errorf("%s: root must be a mapping of profile names, got %s",
			absPath, body.Type())
	}

	dir := filepath.Dir(absPath)

	var includes []string
	type profileNode struct {
		name string
		node ast.Node
	}
	var profiles []profileNode

	for _, mv := range values {
		key, err := nodeString(mv.Key)
		if err != nil {
			return fmt.Errorf("%s: reading a root key: %w", absPath, err)
		}
		if key == "include" {
			if err := yaml.NodeToValue(mv.Value, &includes); err != nil {
				return fmt.Errorf("%s: include: %w", absPath, err)
			}
			continue
		}
		if reservedTypeKeys[key] {
			return fmt.Errorf("%s: %q at the document root looks like a check; "+
				"checks must live under a profile key, not at the root", absPath, key)
		}
		profiles = append(profiles, profileNode{name: key, node: mv.Value})
	}

	// include: files first, in declared order.
	for _, inc := range includes {
		incPath := inc
		if !filepath.IsAbs(incPath) {
			incPath = filepath.Join(dir, incPath)
		}
		incAbs, err := filepath.Abs(incPath)
		if err != nil {
			incAbs = incPath
		}
		if err := l.loadFile(incAbs, visited); err != nil {
			return err
		}
	}

	// This file's own profiles merge on top of whatever the includes provided.
	for _, p := range profiles {
		checks, extends, err := l.parseProfileNode(p.name, p.node, dir, absPath)
		if err != nil {
			return err
		}
		l.mergeProfile(p.name, extends, checks, absPath)
	}
	return nil
}

// inheritSpec is the {extends, add} mapping form of a profile value.
type inheritSpec struct {
	Extends stringList `yaml:"extends"`
	Add     []Check    `yaml:"add"`
}

// parseProfileNode decodes a profile value, which is either a check list or an
// {extends, add} mapping (spec §4), and post-processes the checks (version_from
// resolution, contradiction warnings) against the declaring file's directory.
func (l *loader) parseProfileNode(name string, node ast.Node, dir, file string) ([]Check, []string, error) {
	switch node.Type() {
	case ast.NullType:
		return nil, nil, nil
	case ast.SequenceType:
		var checks []Check
		if err := yaml.NodeToValue(node, &checks); err != nil {
			return nil, nil, fmt.Errorf("%s: profile %q: %w", file, name, err)
		}
		l.postProcess(checks, dir, file)
		return checks, nil, nil
	case ast.MappingType, ast.MappingValueType:
		var spec inheritSpec
		if err := yaml.NodeToValue(node, &spec); err != nil {
			return nil, nil, fmt.Errorf("%s: profile %q: %w", file, name, err)
		}
		if len(spec.Extends) == 0 && spec.Add == nil {
			l.warnf("%s: profile %q: mapping form should set 'extends' and/or 'add'", file, name)
		}
		l.postProcess(spec.Add, dir, file)
		return spec.Add, []string(spec.Extends), nil
	default:
		return nil, nil, fmt.Errorf("%s: profile %q must be a list of checks or an "+
			"{extends, add} mapping, got %s", file, name, node.Type())
	}
}

// mergeProfile folds a file's contribution to a profile into the composite:
// parents are accumulated; checks are appended in order, with collisions (same
// identity within the profile) flagged and the later definition winning.
func (l *loader) mergeProfile(name string, extends []string, checks []Check, file string) {
	cp := l.composite[name]
	if cp == nil {
		cp = &compositeProfile{}
		l.composite[name] = cp
	}
	cp.extends = append(cp.extends, extends...)

	idx := make(map[string]int, len(cp.checks))
	for i, c := range cp.checks {
		idx[c.identity()] = i
	}
	for _, c := range checks {
		id := c.identity()
		if pos, dup := idx[id]; dup {
			l.warnf("profile %q: check %q in %s overrides an earlier definition in %s (later wins)",
				name, id, file, cp.checks[pos].source)
			cp.checks[pos] = c
			continue
		}
		idx[id] = len(cp.checks)
		cp.checks = append(cp.checks, c)
	}
}

// resolveProfile expands a profile's extends chain (in order, diamond-deduped,
// cycle-detected) and merges its own checks on top, flagging collisions where a
// child check shadows an inherited one. Results are memoized.
func (l *loader) resolveProfile(name string, resolving map[string]bool) (Profile, error) {
	if p, ok := l.memo[name]; ok {
		return p, nil
	}
	cp, ok := l.composite[name]
	if !ok {
		return nil, fmt.Errorf("profile %q is not defined", name)
	}
	if resolving[name] {
		return nil, fmt.Errorf("profile inheritance cycle detected at %q", name)
	}
	resolving[name] = true
	defer delete(resolving, name)

	var out Profile
	idx := map[string]int{}
	merge := func(checks []Check, shadowMsg func(id string, prev Check) string) {
		for _, c := range checks {
			id := c.identity()
			if pos, dup := idx[id]; dup {
				l.warnf("%s", shadowMsg(id, out[pos]))
				out[pos] = c
				continue
			}
			idx[id] = len(out)
			out = append(out, c)
		}
	}

	for _, parent := range dedupStrings(cp.extends) {
		if _, ok := l.composite[parent]; !ok {
			return nil, fmt.Errorf("profile %q extends unknown profile %q", name, parent)
		}
		resolved, err := l.resolveProfile(parent, resolving)
		if err != nil {
			return nil, err
		}
		merge(resolved, func(id string, _ Check) string {
			return fmt.Sprintf("profile %q: inherited check %q overridden while extending %q (later wins)",
				name, id, parent)
		})
	}
	merge(cp.checks, func(id string, _ Check) string {
		return fmt.Sprintf("profile %q: check %q shadows an inherited definition (child wins)", name, id)
	})

	l.memo[name] = out
	return out, nil
}

// postProcess resolves and validates a freshly-parsed slice of checks (and their
// nested group/one_of children) against the directory of the declaring file.
func (l *loader) postProcess(checks []Check, dir, file string) {
	for i := range checks {
		l.postProcessCheck(&checks[i], dir, file)
	}
}

func (l *loader) postProcessCheck(c *Check, dir, file string) {
	c.source = file

	if c.Version != "" && c.VersionFrom != "" {
		l.warnf("%s: tool %q: both 'version' and 'version_from' set; using 'version'", file, c.Value)
	}
	if c.Unset && c.Matches != "" {
		l.warnf("%s: env %q: 'unset: true' with 'matches' is contradictory; 'matches' ignored", file, c.Value)
	}
	if len(c.WithEnv) > 0 && c.Type != TypeCommand {
		l.warnf("%s: %s %q: 'with_env' is only used by command checks; ignored", file, c.Type, c.Value)
	}

	switch {
	case c.Version != "":
		c.Constraint = strings.TrimSpace(c.Version)
	case c.VersionFrom != "":
		constraint, err := readVersionFrom(dir, c.VersionFrom)
		if err != nil {
			l.warnf("%s: tool %q: cannot read version_from %q: %v", file, c.Value, c.VersionFrom, err)
		} else {
			c.Constraint = constraint
		}
	}

	for i := range c.Items {
		l.postProcessCheck(&c.Items[i], dir, file)
	}
	for i := range c.OneOf {
		l.postProcessCheck(&c.OneOf[i], dir, file)
	}
}

// readVersionFrom reads a pin file (relative to the declaring config's dir) and
// resolves it to an npm-style constraint: a bare version becomes ">=<version>";
// an existing range is used as-is (spec §5).
func readVersionFrom(dir, rel string) (string, error) {
	p := rel
	if !filepath.IsAbs(p) {
		p = filepath.Join(dir, rel)
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return constraintFromPin(string(data)), nil
}

func constraintFromPin(content string) string {
	var line string
	for _, raw := range strings.Split(content, "\n") {
		s := strings.TrimSpace(raw)
		if s == "" || strings.HasPrefix(s, "#") {
			continue
		}
		line = s
		break
	}
	if line == "" {
		return ""
	}
	if isRange(line) {
		return line
	}
	return ">=" + strings.TrimPrefix(line, "v")
}

// isRange reports whether a pin-file value already expresses a semver range
// rather than a single bare version.
func isRange(s string) bool {
	switch s[0] {
	case '^', '~', '>', '<', '=', '*':
		return true
	}
	if strings.ContainsAny(s, " ,|") {
		return true
	}
	return strings.Contains(s, ".x") || strings.Contains(s, ".X")
}

func dedupStrings(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

// documentBody returns the body node of the first document, or nil if empty.
func documentBody(file *ast.File) ast.Node {
	if file == nil || len(file.Docs) == 0 {
		return nil
	}
	return file.Docs[0].Body
}

// mappingValues normalizes a mapping node to its key/value entries. goccy emits
// a single-pair mapping as *ast.MappingValueNode and a multi-pair mapping as
// *ast.MappingNode.
func mappingValues(node ast.Node) ([]*ast.MappingValueNode, bool) {
	switch m := node.(type) {
	case *ast.MappingNode:
		return m.Values, true
	case *ast.MappingValueNode:
		return []*ast.MappingValueNode{m}, true
	default:
		return nil, false
	}
}

func nodeString(node ast.Node) (string, error) {
	var s string
	if err := yaml.NodeToValue(node, &s); err != nil {
		return "", err
	}
	return s, nil
}
