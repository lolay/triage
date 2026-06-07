package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrNotFound is returned when no config file can be located. The CLI maps
// this to exit code 3 (usage/config error).
var ErrNotFound = errors.New("config not found")

// configNames lists the config file names Discover looks for, in precedence
// order. triage.yaml is the canonical/documented name; the .yml spelling and
// the hidden dotfile variants are accepted for convenience (e.g. for users
// coming from GitHub Actions, which conventionally uses .yml).
var configNames = []string{"triage.yaml", "triage.yml", ".triage.yaml", ".triage.yml"}

// Discover resolves the [config] positional argument to a loaded Config.
//
// Resolution rules (spec §4):
//   - arg == "": look for the configNames in precedence order in the current
//     directory.
//   - arg is a directory: look for the configNames in precedence order inside it.
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
	for _, name := range configNames {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	names := strings.Join(configNames, ", ")
	if dir == "." {
		return "", fmt.Errorf("%w: no %s in the current directory", ErrNotFound, names)
	}
	return "", fmt.Errorf("%w: no %s in %s", ErrNotFound, names, dir)
}
