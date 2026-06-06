package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: write a file in dir and return the absolute path.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("writeFile %s: %v", p, err)
	}
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
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	want := "tool:git tool:go"
	if got := join(ids(cfg.Profiles["default"])); got != want {
		t.Errorf("default ids = %q, want %q", got, want)
	}
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
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Warnings) != 1 || !strings.Contains(cfg.Warnings[0], `"tool:git"`) {
		t.Errorf("want collision warning, got %v", cfg.Warnings)
	}
	// Later definition wins.
	if got := cfg.Profiles["default"][0].Hint; got != "from-top" {
		t.Errorf("later hint should win, got %q", got)
	}
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
	if err != nil {
		t.Fatalf("diamond load: %v", err)
	}
	// git appears once (diamond-safe), go and make added.
	want := "tool:git tool:go tool:make"
	if got := join(ids(cfg.Profiles["default"])); got != want {
		t.Errorf("default = %q, want %q", got, want)
	}
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
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	want := "tool:git tool:go tool:gh"
	if got := join(ids(cfg.Profiles["release"])); got != want {
		t.Errorf("release = %q, want %q", got, want)
	}
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
	if _, err := load(filepath.Join(dir, "triage.yaml")); err == nil {
		t.Error("expected cycle error, got nil")
	}
}

func TestLoad_ExtendsUnknownProfile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `release:
  extends: [nonexistent]
  add: []
`)
	if _, err := load(filepath.Join(dir, "triage.yaml")); err == nil {
		t.Error("expected unknown-profile error, got nil")
	}
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
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.Profiles["default"][0].Constraint; got != ">=1.26.3" {
		t.Errorf("constraint = %q, want >=1.26.3", got)
	}
}

func TestLoad_VersionFrom_BareVersionWithV(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".nvmrc", "v20.11.0\n")
	writeFile(t, dir, "triage.yaml", `default:
  - tool: node
    version_from: .nvmrc
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.Profiles["default"][0].Constraint; got != ">=20.11.0" {
		t.Errorf("constraint = %q, want >=20.11.0", got)
	}
}

func TestLoad_VersionFrom_ExistingRange(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "ver.txt", ">=1.2 <2\n")
	writeFile(t, dir, "triage.yaml", `default:
  - tool: foo
    version_from: ver.txt
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := cfg.Profiles["default"][0].Constraint; got != ">=1.2 <2" {
		t.Errorf("constraint = %q, want >=1.2 <2", got)
	}
}

func TestLoad_VersionFrom_Missing_Warns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `default:
  - tool: go
    version_from: .nonexistent
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Warnings) == 0 {
		t.Error("expected version_from warning, got none")
	}
}

// ── validation errors ─────────────────────────────────────────────────────────

func TestLoad_RootMustBeMapping(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", "- tool: git\n")
	if _, err := load(filepath.Join(dir, "triage.yaml")); err == nil {
		t.Error("expected root-mapping error")
	}
}

func TestLoad_BareCheckAtRoot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", "tool: git\n")
	if _, err := load(filepath.Join(dir, "triage.yaml")); err == nil {
		t.Error("expected bare-check-at-root error")
	}
}

func TestLoad_MultipleTypeKeys(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", "default:\n  - tool: git\n    env: FOO\n")
	if _, err := load(filepath.Join(dir, "triage.yaml")); err == nil {
		t.Error("expected multiple-type-keys error")
	}
}

func TestLoad_NoTypeKey(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", "default:\n  - hint: install something\n")
	if _, err := load(filepath.Join(dir, "triage.yaml")); err == nil {
		t.Error("expected no-type-key error")
	}
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
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Warnings) == 0 {
		t.Error("expected contradiction warning")
	}
	// version wins over version_from.
	if got := cfg.Profiles["default"][0].Constraint; got != ">=1.2" {
		t.Errorf("version should win: constraint = %q", got)
	}
}

func TestLoad_UnsetAndMatches_Warns(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `default:
  - env: FOO
    unset: true
    matches: "x"
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	hasContradiction := false
	for _, w := range cfg.Warnings {
		if strings.Contains(w, "unset") && strings.Contains(w, "matches") {
			hasContradiction = true
		}
	}
	if !hasContradiction {
		t.Errorf("expected unset+matches warning, got %v", cfg.Warnings)
	}
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
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	types := make(map[string]bool)
	for _, c := range cfg.Profiles["default"] {
		types[c.Type] = true
	}
	for _, want := range []string{TypeTool, TypeEnv, TypePath, TypeOneOf, TypeCommand, TypeGroup, TypeDelegate} {
		if !types[want] {
			t.Errorf("missing type %q", want)
		}
	}
}

func TestLoad_GroupLegacyString(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", `default:
  - tool: git
    group: Core
`)
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	c := cfg.Profiles["default"][0]
	if c.Type != TypeTool {
		t.Errorf("type = %q, want tool", c.Type)
	}
	if c.Group != "Core" {
		t.Errorf("group = %q, want Core", c.Group)
	}
}

func TestLoad_EmptyDocument(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", "")
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	if err != nil {
		t.Fatalf("load empty doc: %v", err)
	}
	if len(cfg.Profiles) != 0 {
		t.Errorf("want 0 profiles, got %d", len(cfg.Profiles))
	}
}

func TestLoad_EmptyProfile(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "triage.yaml", "default: []\n")
	cfg, err := load(filepath.Join(dir, "triage.yaml"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(cfg.Profiles["default"]) != 0 {
		t.Errorf("want empty profile")
	}
}
