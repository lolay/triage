package cli

import (
	"strings"
	"testing"
)

func TestParseCLIVars_Valid(t *testing.T) {
	m, err := parseCLIVars([]string{"region=eu", "mode=release"})
	if err != nil {
		t.Fatalf("parseCLIVars: %v", err)
	}
	if m["region"] != "eu" || m["mode"] != "release" {
		t.Errorf("got %#v", m)
	}
}

func TestParseCLIVars_ValueWithEquals(t *testing.T) {
	m, err := parseCLIVars([]string{"url=https://x=y"})
	if err != nil {
		t.Fatalf("parseCLIVars: %v", err)
	}
	if m["url"] != "https://x=y" {
		t.Errorf("got %q", m["url"])
	}
}

func TestParseCLIVars_Invalid(t *testing.T) {
	_, err := parseCLIVars([]string{"nope"})
	if err == nil || !strings.Contains(err.Error(), "name=value") {
		t.Errorf("want invalid error, got %v", err)
	}
}

func TestMergeVarMaps_CLIWins(t *testing.T) {
	got := mergeVarMaps(map[string]string{"a": "config", "b": "config"}, map[string]string{"a": "cli"})
	if got["a"] != "cli" || got["b"] != "config" {
		t.Errorf("merge = %#v", got)
	}
}
