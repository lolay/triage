package cli

// Flags holds all parsed command-line options for a triage run.
type Flags struct {
	Profile       string
	Vars          []string // raw --var name=value entries
	JSON          bool
	Quiet         bool
	Strict        bool
	Severity      bool // enables the 4-level exit-code ladder (0/1/2/3)
	NoColor       bool
	CommandLog    string // "" = disabled; path = stream probe output there each run
	Verbose       bool
	NoUpdateCheck bool
	Jobs          int // max concurrent checks; 0 = auto (min NumCPU, 8); 1 = sequential
}
