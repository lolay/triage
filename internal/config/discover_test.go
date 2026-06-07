package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const minimalConfig = "default: []\n"

// Each accepted config name resolves on its own when it is the only file present.
func TestDiscover_EachNameResolves(t *testing.T) {
	for _, name := range configNames {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeFile(t, dir, name, minimalConfig)

			// Directory argument.
			cfg, err := Discover(dir)
			if err != nil {
				t.Fatalf("Discover(dir) with %s: %v", name, err)
			}
			if cfg == nil {
				t.Fatalf("Discover(dir) returned nil config for %s", name)
			}
		})
	}
}

// When multiple accepted names coexist, the canonical precedence order wins:
// triage.yaml > triage.yml > .triage.yaml > .triage.yml.
func TestDiscover_PrecedenceOrder(t *testing.T) {
	dir := t.TempDir()
	// Write every variant, each tagged with a distinct var so we can tell which
	// one was loaded.
	for _, name := range configNames {
		writeFile(t, dir, name, "vars:\n  picked: "+name+"\ndefault: []\n")
	}

	// Remove from most-preferred to least, asserting the next-in-line wins.
	for i, name := range configNames {
		cfg, err := Discover(dir)
		if err != nil {
			t.Fatalf("Discover after removing %d: %v", i, err)
		}
		if got := cfg.Vars["picked"]; got != name {
			t.Errorf("with %v present, picked %q, want %q", configNames[i:], got, name)
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			t.Fatalf("remove %s: %v", name, err)
		}
	}
}

// An explicit .yml file argument is loaded directly.
func TestDiscover_ExplicitYmlFile(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "custom.yml", minimalConfig)
	if _, err := Discover(p); err != nil {
		t.Fatalf("Discover(explicit .yml): %v", err)
	}
}

// A directory with none of the accepted names yields ErrNotFound, and the
// message enumerates every accepted name.
func TestDiscover_NotFoundListsAllNames(t *testing.T) {
	dir := t.TempDir()
	_, err := Discover(dir)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
	for _, name := range configNames {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("not-found message %q missing %q", err.Error(), name)
		}
	}
}
