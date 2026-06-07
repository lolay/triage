package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCLIVars_Valid(t *testing.T) {
	m, err := parseCLIVars([]string{"region=eu", "mode=release"})
	require.NoError(t, err, "parseCLIVars")
	assert.Equal(t, "eu", m["region"], "got %#v", m)
	assert.Equal(t, "release", m["mode"], "got %#v", m)
}

func TestParseCLIVars_ValueWithEquals(t *testing.T) {
	m, err := parseCLIVars([]string{"url=https://x=y"})
	require.NoError(t, err, "parseCLIVars")
	assert.Equal(t, "https://x=y", m["url"])
}

func TestParseCLIVars_Invalid(t *testing.T) {
	_, err := parseCLIVars([]string{"nope"})
	require.Error(t, err, "want invalid error")
	assert.ErrorContains(t, err, "name=value")
}
