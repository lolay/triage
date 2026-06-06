package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ErrNotFound is returned when no config file can be located. The CLI maps
// this to exit code 3 (usage/config error).
var ErrNotFound = errors.New("config not found")

// Discover resolves the [config] positional argument to a loaded Config.
//
// Resolution rules (spec §4, lines 228-240):
//   - arg == "": look for triage.yaml then .triage.yaml in the current directory.
//   - arg is a directory: look for triage.yaml then .triage.yaml inside it.
//   - arg is a file: use it directly.
//   - not found or >1 positionals: return a wrapped ErrNotFound (exit 3 in CLI).
func Discover(arg string) (*Config, error) {
	path, err := resolvePath(arg)
	if err != nil {
		return nil, err
	}
	return load(path)
}

func resolvePath(arg string) (string, error) {
	if arg == "" {
		return findInDir(".")
	}
	info, err := os.Stat(arg)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("%w: %s", ErrNotFound, arg)
		}
		return "", err
	}
	if info.IsDir() {
		return findInDir(arg)
	}
	return arg, nil
}

func findInDir(dir string) (string, error) {
	for _, name := range []string{"triage.yaml", ".triage.yaml"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if dir == "." {
		return "", fmt.Errorf("%w: no triage.yaml or .triage.yaml in the current directory", ErrNotFound)
	}
	return "", fmt.Errorf("%w: no triage.yaml or .triage.yaml in %s", ErrNotFound, dir)
}

// load parses the config file at path.
//
// The m1 stub confirms the root is a YAML mapping and returns an empty check
// list for every profile key it discovers. Real schema validation lands in m2.
func load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	// Unmarshal into a raw map to confirm the root is a YAML mapping and to
	// enumerate the declared profile names. Values are intentionally ignored.
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", path, err)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}

	profiles := make(map[string]Profile, len(raw))
	for k := range raw {
		profiles[k] = Profile{} // always empty in m1; real loader in m2
	}

	return &Config{Path: abs, Profiles: profiles}, nil
}
