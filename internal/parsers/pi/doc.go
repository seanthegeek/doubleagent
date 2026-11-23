// Package pi reads the pi coding agent's sessions, trust settings, debug
// snapshot, crash records and session exports, and holds the readers of
// pi's session format that the letta, little-coder and openclaw parsers
// share.
//
// Session files are .pi/agent/sessions/--<cwd>--/<time>_<id>.jsonl and the
// legacy .pi/agent/*.jsonl: a "session" header (version, id, timestamp,
// cwd, parentSession), then entries with type, id and timestamp; message
// entries carry a role and its fields. Rows follow file order, not the
// active branch, and a fork's entries copied from a parent file in the same
// home are dropped. Other files read: trust.json and settings.json
// defaultProjectTrust (permission rows), crashes.json, pi-tui-crash.log and
// the 0.83.0 pi-crash.log, experimental/sessions/<id>/meta.json, and the
// debug log pi-debug.log, a secondary source.
//
// In a project directory, three exports are secondary sources: the HTML
// export pi-session-<name>.html, whose session-data element holds the
// session as base64 JSON; the JSONL export session-<time>.jsonl; and the
// bug report pi-bug-report-<id>.zip, read in memory and never extracted,
// at most maxInflated bytes decompressed. Copies give no `model:` rows. See
// research/pi.md.
package pi
