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
// m4 additions: KindDelegate renders as an indented summary line (worst-child
// glyph) with the child subtree nested beneath it, at the depth the engine
// assigns; delegate summaries and group headers are skipped in the summary
// counts (their children are the counted leaves). JSON gains depth and kind
// (check/group/delegate) so consumers can reconstruct the tree. Because the
// engine materializes results in list order regardless of --jobs, the renderer
// is unchanged by concurrency and golden output stays byte-stable.
//
// Async TTY pending/back-update streaming is still deferred.
package report
