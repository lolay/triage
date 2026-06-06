package engine

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// CommandLog streams per-probe output blocks to a log file (spec §5
// --command-log). The file is truncated at the start of each run; within a run
// each probe writes one labeled block (label, cwd, run line, ---, raw output).
// A nil *CommandLog is safe: all methods are no-ops.
type CommandLog struct {
	path string
	mu   sync.Mutex
	w    io.WriteCloser
}

// OpenCommandLog opens (or creates, with parents) path, truncating any existing
// content, and returns a CommandLog ready to receive probe blocks.
func OpenCommandLog(path string) (*CommandLog, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("command-log: mkdir %s: %w", dir, err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("command-log: open %s: %w", path, err)
	}
	return &CommandLog{path: path, w: f}, nil
}

// WriteBlock writes one labeled probe block to the log.
// Safe for concurrent use.
func (cl *CommandLog) WriteBlock(label, cwd, run string, output []byte) {
	if cl == nil {
		return
	}
	cl.mu.Lock()
	defer cl.mu.Unlock()
	fmt.Fprintf(cl.w, "=== %s ===\ncwd: %s\nrun: %s\n---\n", label, cwd, run)
	if len(output) > 0 {
		cl.w.Write(output)
		if output[len(output)-1] != '\n' {
			fmt.Fprintln(cl.w)
		}
	}
	fmt.Fprintln(cl.w)
}

// Path returns the log file path (empty when cl is nil).
func (cl *CommandLog) Path() string {
	if cl == nil {
		return ""
	}
	return cl.path
}

// Close flushes and closes the log file.
func (cl *CommandLog) Close() error {
	if cl == nil {
		return nil
	}
	cl.mu.Lock()
	defer cl.mu.Unlock()
	return cl.w.Close()
}
