package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
			require.NoError(t, err, "Discover(dir) with %s", name)
			require.NotNil(t, cfg, "Discover(dir) returned nil config for %s", name)
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
		require.NoErrorf(t, err, "Discover after removing %d", i)
		assert.Equalf(t, name, cfg.Vars["picked"], "with %v present", configNames[i:])
		require.NoErrorf(t, os.Remove(filepath.Join(dir, name)), "remove %s", name)
	}
}

// An explicit .yml file argument is loaded directly.
func TestDiscover_ExplicitYmlFile(t *testing.T) {
	dir := t.TempDir()
	p := writeFile(t, dir, "custom.yml", minimalConfig)
	_, err := Discover(p)
	require.NoError(t, err, "Discover(explicit .yml)")
}

// A directory with none of the accepted names yields ErrNotFound, and the
// message enumerates every accepted name.
func TestDiscover_NotFoundListsAllNames(t *testing.T) {
	dir := t.TempDir()
	_, err := Discover(dir)
	require.ErrorIs(t, err, ErrNotFound)
	for _, name := range configNames {
		assert.Containsf(t, err.Error(), name, "not-found message missing %q", name)
	}
}
