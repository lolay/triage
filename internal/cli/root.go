// Package cli wires the triage command-line interface using cobra.
//
// Entry point: Execute (called by cmd/triage/main.go). The testable variant
// ExecuteWith accepts io.Writers so tests can capture output without touching
// os.Stdout/os.Stderr. The full flag surface, config discovery, and exit-code
// policy (§7.3) are all implemented here; check execution lives in
// internal/engine and rendering in internal/report.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/lolay/triage/internal/buildinfo"
	"github.com/lolay/triage/internal/config"
	"github.com/lolay/triage/internal/engine"
	"github.com/lolay/triage/internal/report"
)

// Execute runs the CLI with the given arguments and returns the process exit
// code. It is the only symbol imported by cmd/triage/main.go.
func Execute(args []string) int {
	return ExecuteWith(args, os.Stdout, os.Stderr)
}

// ExecuteWith is the testable entry point: callers inject io.Writers so tests
// can capture output without touching os.Stdout / os.Stderr.
func ExecuteWith(args []string, stdout, stderr io.Writer) int {
	exitCode := ExitOK
	cmd := newRootCmd(stdout, stderr, &exitCode)
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		// cobra parse error (unknown flag, too many args, …) → usage error.
		if exitCode == ExitOK {
			exitCode = ExitUsageError
		}
		fmt.Fprintln(stderr, "triage:", err)
	}
	return exitCode
}

// newRootCmd constructs the cobra root command. exitCode is written by RunE so
// that ExecuteWith can return it after cobra returns.
func newRootCmd(stdout, stderr io.Writer, exitCode *int) *cobra.Command {
	var f Flags

	cmd := &cobra.Command{
		Use:   "triage [config]",
		Short: "The environment doctor — checks prerequisites for your project",
		Long: `triage reads a declarative triage.yaml in your repo and reports what's
present, what's missing, and exactly how to fix it.

Run from the directory that contains the config (or pass the path).
triage does not search parent directories.`,
		Args:          cobra.MaximumNArgs(1),
		SilenceErrors: true,
		SilenceUsage:  true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(cmd, args, &f, stdout, stderr, exitCode)
		},
	}

	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	// --version uses cobra's built-in version flag; we customise the template
	// so it outputs exactly buildinfo.String() followed by a newline.
	cmd.Version = buildinfo.String()
	cmd.SetVersionTemplate("{{.Version}}\n")

	bindFlags(cmd, &f)
	return cmd
}

// bindFlags attaches all flags to cmd and populates f via pointer binding.
func bindFlags(cmd *cobra.Command, f *Flags) {
	fl := cmd.Flags()

	fl.StringVar(&f.Profile, "profile", "default", "Profile to run")
	fl.BoolVar(&f.JSON, "json", false, "Emit machine-readable JSON instead of the board")
	fl.BoolVar(&f.Quiet, "quiet", false, "Suppress passing checks; show only failures")
	fl.BoolVar(&f.Strict, "strict", false, "Escalate warnings to errors")
	fl.BoolVar(&f.Severity, "severity", false, "Enable graded exit-code ladder (0 clean / 1 warn / 2 error / 3 config)")
	fl.BoolVar(&f.NoColor, "no-color", false, "Disable ANSI color output")
	fl.BoolVar(&f.Verbose, "verbose", false, "Replay probe subprocess output for failures on stderr")
	fl.BoolVar(&f.NoUpdateCheck, "no-update-check", false, "Disable the update-availability banner")
	fl.BoolVar(&f.Migrate, "migrate", false, "Migrate config to the current schema (not yet implemented)")

	// --command-log has an optional value: present with no path uses the
	// default; present with a path uses that path; absent = disabled.
	fl.StringVar(&f.CommandLog, "command-log", "", "Stream probe subprocess output to `path` (default: .triage/commands.log)")
	if lookup := fl.Lookup("command-log"); lookup != nil {
		lookup.NoOptDefVal = ".triage/commands.log"
	}
}

// run is the core logic invoked by cobra's RunE.
func run(_ *cobra.Command, args []string, f *Flags, stdout, stderr io.Writer, exitCode *int) error {
	// --migrate stub: not implemented until m2.
	if f.Migrate {
		fmt.Fprintln(stdout, "triage: --migrate is not yet implemented (coming in m2)")
		*exitCode = ExitUsageError
		return nil
	}

	// Resolve the optional [config] positional argument.
	configArg := ""
	if len(args) > 0 {
		configArg = args[0]
	}

	cfg, err := config.Discover(configArg)
	if err != nil {
		*exitCode = ExitUsageError
		fmt.Fprintf(stderr, "triage: %v\n", err)
		if errors.Is(err, config.ErrNotFound) {
			fmt.Fprintln(stderr, "\nRun 'triage --help' for usage.")
		}
		return nil
	}

	// Select the active profile (always empty in m1).
	profile := cfg.Profiles[f.Profile]
	if profile == nil {
		profile = config.Profile{}
	}

	// Run the check engine (no-op in m1; real execution in m2).
	results := engine.NewRunner().Run(profile)

	// Render output.
	if f.JSON {
		if err := report.JSON(stdout, results, f.Profile); err != nil {
			fmt.Fprintf(stderr, "triage: json output: %v\n", err)
			*exitCode = ExitUsageError
			return nil
		}
	} else {
		report.Board(stdout, results, report.BoardOpts{
			Profile: f.Profile,
			NoColor: f.NoColor,
			Quiet:   f.Quiet,
		})
	}

	*exitCode = ExitCode(results, f.Strict, f.Severity)
	return nil
}
