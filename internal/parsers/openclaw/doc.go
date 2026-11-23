// Package openclaw reads OpenClaw (formerly Clawdbot and Moltbot): its
// agent databases, its Gateway state database and its JSONL transcripts,
// under the state directory .openclaw, .openclaw-<profile>, .clawdbot or
// .moltbot. Files read:
//
//   - agents/<id>/agent/openclaw-agent.sqlite: the entries of
//     transcript_events (event_json, or a zstd frame in event_zstd), the
//     generations of session_windows, the reset and deleted generations of
//     session_transcript_archives, the cold archives stored in the database
//     (or their chunks), session_nodes.entry_json, session_conversations
//     with conversations, session_pending_inputs, the full-text index's
//     content table (joined on the entry id, keeping what the transcript
//     lost), conversation_deliveries and memory_session_tombstones.
//   - state/openclaw.sqlite, the Gateway's state (stateQueries).
//   - [agents/<id>/]sessions/<id>.jsonl legacy transcripts, with the session
//     key from the sibling sessions.json, and sessions/cold/<sha256>.jsonl.zst.
//   - Secondary sources: the published .jsonl.reset.* and .jsonl.deleted.*
//     archives, .checkpoint.<uuid>.jsonl snapshots and
//     .pre-doctor-*-repair-*.bak copies.
//
// Entries share pi's format and model rows (pi.ModelText, pi.SwitchText).
// A PostgreSQL anchor database is one `parser:` row; credential tables are
// never read. ToolArgsText is shared with the nanobot parser. See
// research/openclaw.md.
package openclaw
