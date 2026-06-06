// Package cli wires the triage command-line interface.
//
// m1 s1 ships a deliberately thin shell so the binary builds and runs end to
// end. The full cobra command, flag surface, config discovery, and exit-code
// policy land in m1 s2 (root.go, flags.go, version.go, exit.go); the human
// board and --json reporters land in m1 s4. Until then Execute recognizes only
// --version/-v and otherwise prints a placeholder notice.
package cli

import (
	"fmt"

	"github.com/lolay/triage/internal/buildinfo"
)

// Execute runs the CLI with the given arguments (os.Args[1:]) and returns the
// process exit code. The exit-code policy (§7.3) is implemented in m1 s2.
func Execute(args []string) int {
	for _, arg := range args {
		if arg == "--version" || arg == "-v" {
			fmt.Println(buildinfo.String())
			return 0
		}
	}

	fmt.Println("triage: m1 skeleton — the check engine and board land in m2")
	return 0
}
