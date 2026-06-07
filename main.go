// Command triage is the environment doctor CLI: it reads a declarative
// triage.yaml and reports what's present, missing, and how to fix it.
//
// This is the m1 skeleton entry point; it delegates all behavior to
// internal/cli and exits with the code that package returns.
package main

import (
	"os"

	"github.com/lolay/triage/internal/cli"
)

func main() {
	os.Exit(cli.Execute(os.Args[1:]))
}
