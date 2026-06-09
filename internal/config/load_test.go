package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: write a file in dir and return the absolute path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644), "writeFile %s", p)
	return p
}

// ids returns the identity strings for a profile (for easy comparison).
func ids(p Profile) []string {
	out := make([]string, len(p))
	for i, c := range p {
		out[i] = c.identity()
	}
	return out
}

func join(ss []string) string { return strings.Join(ss, " ") }

// ── config format version ─────────────────────────────────────────────────────

func TestLoad_VersionAbsentDefaultsToOne(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `default:
  - tool: git
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load")
	assert.Equal(t, 1, cfg.Version, "Version")
}

func TestLoad_VersionExplicitOne(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `version: 1
default:
  - tool: git
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load")
	assert.Equal(t, 1, cfg.Version, "Version")
}

func TestLoad_VersionUnsupported(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `version: 2
default:
  - tool: git
`)
	_, err := load(filepath.Join(dir, "triage.yaml"))
	require.Error(t, err, "load")
	assert.ErrorContains(t, err, "unsupported config version 2")
}

func TestLoad_VersionInIncludedFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "base.yaml", `version: 1
default:
  - tool: git
`)
	writeFile(t, dir, "triage.yaml", `include:
  - base.yaml
default:
  - tool: go
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load")
	assert.Equal(t, 1, cfg.Version, "Version")
}

func TestLoad_VersionUnsupportedInIncludedFile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "base.yaml", `version: 2
default:
  - tool: git
`)
	writeFile(t, dir, "triage.yaml", `include:
  - base.yaml
default:
  - tool: go
`)
	_, err := load(filepath.Join(dir, "triage.yaml"))
	require.Error(t, err, "load")
	assert.ErrorContains(t, err, "unsupported config version 2")
}

// ── include merge ───────────────────────────────────────────────────────────

func TestLoad_IncludeMerge(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "base.yaml", `default:
  - tool: git
    hint: https://git-scm.com
`)
	writeFile(t, dir, "triage.yaml", `include:
  - base.yaml
default:
  - tool: go
    hint: https://go.dev/dl
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load")
	assert.Equal(t, "tool:git tool:go", join(ids(cfg.Profiles["default"])), "default ids")
}

func TestLoad_IncludeCollisionWarns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "base.yaml", `default:
  - tool: git
    hint: from-base
`)
	writeFile(t, dir, "triage.yaml", `include:
  - base.yaml
default:
  - tool: git
    hint: from-top
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load")
	require.Len(t, cfg.Warnings, 1, "want collision warning, got %v", cfg.Warnings)
	assert.Contains(t, cfg.Warnings[0], `"tool:git"`)
	// Later definition wins.
	require.NotEmpty(t, cfg.Profiles["default"])
	assert.Equal(t, "from-top", cfg.Profiles["default"][0].Hint, "later hint should win")
}

func TestLoad_IncludeDiamondSafe(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "base.yaml", `default:
  - tool: git
`)
	writeFile(t, dir, "left.yaml", `include:
  - base.yaml
default:
  - tool: go
`)
	writeFile(t, dir, "right.yaml", `include:
  - base.yaml
default:
  - tool: make
`)
	writeFile(t, dir, "triage.yaml", `include:
  - left.yaml
  - right.yaml
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "diamond load")
	// git appears once (diamond-safe), go and make added.
	assert.Equal(t, "tool:git tool:go tool:make", join(ids(cfg.Profiles["default"])), "default")
}

// ── extends / add ────────────────────────────────────────────────────────────

func TestLoad_ExtendsAdd(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `default:
  - tool: git
  - tool: go

release:
  extends: [default]
  add:
    - tool: gh
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load")
	assert.Equal(t, "tool:git tool:go tool:gh", join(ids(cfg.Profiles["release"])), "release")
}

func TestLoad_ExtendsCycleDetected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `a:
  extends: [b]
  add: []
b:
  extends: [a]
  add: []
`)
	_, err := load(filepath.Join(dir, "triage.yaml"))
	assert.Error(t, err, "expected cycle error")
}

func TestLoad_ExtendsUnknownProfile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `release:
  extends: [nonexistent]
  add: []
`)
	_, err := load(filepath.Join(dir, "triage.yaml"))
	assert.Error(t, err, "expected unknown-profile error")
}

// ── version_from ─────────────────────────────────────────────────────────────

func TestLoad_VersionFrom_BareVersion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".go-version", "1.26.3\n")
	writeFile(t, dir, "triage.yaml", `default:
  - tool: go
    version_from: .go-version
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load")
	require.NotEmpty(t, cfg.Profiles["default"])
	assert.Equal(t, ">=1.26.3", cfg.Profiles["default"][0].Constraint, "constraint")
}

func TestLoad_VersionFrom_BareVersionWithV(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".nvmrc", "v20.11.0\n")
	writeFile(t, dir, "triage.yaml", `default:
  - tool: node
    version_from: .nvmrc
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load")
	require.NotEmpty(t, cfg.Profiles["default"])
	assert.Equal(t, ">=20.11.0", cfg.Profiles["default"][0].Constraint, "constraint")
}

func TestLoad_VersionFrom_ExistingRange(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "ver.txt", ">=1.2 <2\n")
	writeFile(t, dir, "triage.yaml", `default:
  - tool: foo
    version_from: ver.txt
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load")
	require.NotEmpty(t, cfg.Profiles["default"])
	assert.Equal(t, ">=1.2 <2", cfg.Profiles["default"][0].Constraint, "constraint")
}

func TestLoad_VersionFrom_Missing_Warns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `default:
  - tool: go
    version_from: .nonexistent
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load")
	assert.NotEmpty(t, cfg.Warnings, "expected version_from warning")
}

// ── validation errors ─────────────────────────────────────────────────────────

func TestLoad_RootMustBeMapping(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", "- tool: git\n")
	_, err := load(filepath.Join(dir, "triage.yaml"))
	assert.Error(t, err, "expected root-mapping error")
}

func TestLoad_BareCheckAtRoot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", "tool: git\n")
	_, err := load(filepath.Join(dir, "triage.yaml"))
	assert.Error(t, err, "expected bare-check-at-root error")
}

func TestLoad_MultipleTypeKeys(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", "default:\n  - tool: git\n    env: FOO\n")
	_, err := load(filepath.Join(dir, "triage.yaml"))
	assert.Error(t, err, "expected multiple-type-keys error")
}

func TestLoad_NoTypeKey(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", "default:\n  - hint: install something\n")
	_, err := load(filepath.Join(dir, "triage.yaml"))
	assert.Error(t, err, "expected no-type-key error")
}

// ── contradiction warnings ────────────────────────────────────────────────────

func TestLoad_VersionAndVersionFrom_Warns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".v", "1.0\n")
	writeFile(t, dir, "triage.yaml", `default:
  - tool: go
    version: ">=1.2"
    version_from: .v
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load")
	assert.NotEmpty(t, cfg.Warnings, "expected contradiction warning")
	// version wins over version_from.
	require.NotEmpty(t, cfg.Profiles["default"])
	assert.Equal(t, ">=1.2", cfg.Profiles["default"][0].Constraint, "version should win")
}

func TestLoad_UnsetAndMatches_Warns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `default:
  - env: FOO
    unset: true
    matches: "x"
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load")
	hasContradiction := false
	for _, w := range cfg.Warnings {
		if strings.Contains(w, "unset") && strings.Contains(w, "matches") {
			hasContradiction = true
		}
	}
	assert.True(t, hasContradiction, "expected unset+matches warning, got %v", cfg.Warnings)
}

// ── type-as-key parsing for all §5 types ──────────────────────────────────────

func TestLoad_AllTypeKeys(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `default:
  - tool: git
  - env: HOME
  - path: /tmp
  - one_of: [pnpm, npm]
  - command: echo hello
    label: echo check
  - group: My section
    items:
      - tool: make
  - delegate: sub
    dir: subdir
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load")
	types := make(map[string]bool)
	for _, c := range cfg.Profiles["default"] {
		types[c.Type] = true
	}
	for _, want := range []string{TypeTool, TypeEnv, TypePath, TypeOneOf, TypeCommand, TypeGroup, TypeDelegate} {
		assert.Contains(t, types, want, "missing type %q", want)
	}
}

func TestLoad_FlatGroupRejected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `default:
  - tool: git
    group: Core
`)
	_, err := load(filepath.Join(dir, "triage.yaml"))
	require.Error(t, err, "load")
	assert.ErrorContains(t, err, "multiple type keys")
	assert.ErrorContains(t, err, "group: and items:")
}

func TestLoad_GrouplessProfile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `default:
  - tool: git
  - tool: go
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load")
	require.Len(t, cfg.Profiles["default"], 2, "checks")
	assert.Equal(t, TypeTool, cfg.Profiles["default"][0].Type)
	assert.Equal(t, "git", cfg.Profiles["default"][0].Value)
}

func TestLoad_MixedGroupsAndTopLevel(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `default:
  - tool: git
  - group: Toolchain
    items:
      - tool: go
  - tool: make
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load")
	require.Len(t, cfg.Profiles["default"], 3, "checks")
	assert.Equal(t, TypeGroup, cfg.Profiles["default"][1].Type)
	assert.Equal(t, "Toolchain", cfg.Profiles["default"][1].Value)
	require.Len(t, cfg.Profiles["default"][1].Items, 1)
}

func TestLoad_EmptyDocument(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", "")
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load empty doc")
	assert.Empty(t, cfg.Profiles, "want 0 profiles")
}

func TestLoad_EmptyProfile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", "default: []\n")
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load")
	assert.Empty(t, cfg.Profiles["default"], "want empty profile")
}

// ── vars ──────────────────────────────────────────────────────────────────────

func TestLoad_VarsParsing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `vars:
  region: us-west-2
  tool_prefix: fake
default:
  - tool: git
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load")
	assert.Equal(t, "us-west-2", cfg.Vars["region"], "vars = %#v", cfg.Vars)
	assert.Equal(t, "fake", cfg.Vars["tool_prefix"], "vars = %#v", cfg.Vars)
}

func TestLoad_VarsIncludeMergeLaterWins(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "base.yaml", `vars:
  region: from-base
`)
	writeFile(t, dir, "triage.yaml", `include:
  - base.yaml
vars:
  region: from-top
default: []
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load")
	assert.Equal(t, "from-top", cfg.Vars["region"], "region")
}

func TestLoad_VarsCollisionWarns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "base.yaml", `vars:
  key: first
`)
	writeFile(t, dir, "triage.yaml", `include:
  - base.yaml
vars:
  key: second
default: []
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load")
	require.Len(t, cfg.Warnings, 1, "want vars collision warning, got %v", cfg.Warnings)
	assert.Contains(t, cfg.Warnings[0], `"key"`)
	assert.Equal(t, "second", cfg.Vars["key"], "later value should win")
}

func TestLoad_VarsReservedNameWarns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `vars:
  profile: oops
  os: bad
  ok: fine
default: []
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	require.NoError(t, err, "load")
	assert.Len(t, cfg.Warnings, 2, "want 2 reserved-name warnings, got %v", cfg.Warnings)
	assert.NotContains(t, cfg.Vars, "profile", "profile should not be in merged vars")
	assert.Equal(t, "fine", cfg.Vars["ok"], "ok var missing: %#v", cfg.Vars)
}

func TestLoad_ProfileScalarRejected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `default: 1
`)
	_, err := load(filepath.Join(dir, "triage.yaml"))
	require.Error(t, err, "load")
	assert.ErrorContains(t, err, "must be a list of checks")
}

func TestLoad_OneOfScalarRejected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `default:
  - one_of: pnpm
`)
	_, err := load(filepath.Join(dir, "triage.yaml"))
	require.Error(t, err, "load")
}

func TestLoad_OneOfInvalidAlternativeRejected(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `default:
  - one_of:
      - foo: bar
`)
	_, err := load(filepath.Join(dir, "triage.yaml"))
	require.Error(t, err, "load")
	assert.ErrorContains(t, err, "no type key")
}
