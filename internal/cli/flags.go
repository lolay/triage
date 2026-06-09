package cli

// Flags holds all parsed command-line options for a triage run.
type Flags struct {
	Profile       string
	CommandLog    string   // "" = disabled; path = stream probe output there each run
	Vars          []string // raw --var name=value entries
	Jobs          int      // max concurrent checks; 0 = auto (min NumCPU, 8); 1 = sequential
	JSON          bool
	Quiet         bool
	Strict        bool
	Severity      bool // enables the 4-level exit-code ladder (0/1/2/3)
	NoColor       bool
	Verbose       bool
	NoUpdateCheck bool
	Init          bool
}
