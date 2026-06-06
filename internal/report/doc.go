// Package report renders check results for humans and machines.
//
// m2 ships the full rendering layer (spec §7.2–§7.3):
//   - Board: static human board — structural group headers with the worst-status
//     glyph, indented children, non-pass lines with hints inline, trailing
//     summary line, and the read-only "To fix, run:" remediation block.
//     ANSI color is gated on opts.IsTTY && !opts.NoColor.
//   - Sink: output interface with StaticSink (non-TTY, buffers nothing —
//     delegates to Board at End) and TTYSink (streaming, prints each line as
//     it arrives). The CLI chooses based on term.IsTerminal.
//   - JSON: machine-readable report aligned to spec §7.3 shape
//     (group, name, severity, status, detail, command_log_path). The
//     command_log_path field is populated on failing results that have
//     subprocess output when --command-log is active.
//   - Glyphs: [✓] pass · [✗] error · [!] warn · [ℹ] info.
//   - Legacy `group:` string field on a check synthesises flat section headers
//     at render time; structural `group` containers (KindHeader results) are
//     preferred.
//
// m3 additions: JSON enriched with command_log_path; board is unchanged since
// platform-skipped checks never reach the report layer.
//
// Delegate-tree rendering (m5) and async TTY pending/back-update are deferred.
package report
