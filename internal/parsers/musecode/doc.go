// Package musecode reads Muse Code's session logs, prompt history and
// trusted projects.
//
// It reads, relative to the home (the same layout on macOS and Windows):
//
//   - .local/share/muse/sessions/YYYY/MM/DD/<session id>/session.jsonl and
//     its subagent/<child id>/session.jsonl logs: one JSON object per line,
//     an envelope (stream.id, recorded_at in microseconds, payload_type,
//     payload), a retained_frame (children[].child_index and
//     children[].record_json, a whole envelope as a JSON string) or a
//     skipped retained_marker. A file whose first record is neither is not
//     Muse.
//   - runtime.session records: payload.kind "run" (run_id and an event) or
//     "approval"; and fact records, read from payload.record then payload:
//     the session metadata (workspace_root, model_id), route facts (cwd),
//     the permission declarations, the opened session, the workspace branch,
//     the session name, model configuration and reconfiguration, the accepted
//     user intent, user shell commands and results, and subagent control.
//     Everything else is skipped.
//   - .local/share/muse/tui-history.jsonl (secondary): a JSON string, then
//     {project, session}.
//   - .config/muse/trust.json: projects.<path>.decision.
//
// A subagent log records no workspace: its project and branch come from the
// parent log, read up to parentScanLines lines. Model and permission mode
// rows are tracked per log. See research/muse-code.md.
package musecode
