// Package nanobot reads nanobot's session stores, their copies, the memory
// and CLI histories, the WebUI mirror, settings and run records, under a
// state directory .nanobot or .nanobot-<instance>:
//
//   - sessions/<workspace id>/sessions.sqlite3, the store of record from
//     0.3.6: tables sessions (key, created_at, updated_at, metadata) and
//     messages (session_key, position, record: one JSONL message line).
//   - sessions/<workspace id>/<base64url key>.jsonl, its jsonl-backup/ and
//     .migration-conflicts/ copies, and legacy sessions/<key>.jsonl: a
//     metadata line, then OpenAI chat messages. The copies a database
//     imported are secondary. .workspace beside them is the project path.
//   - memory/history.jsonl, primary; webui/<key>.jsonl and its .segments/,
//     history/cli_history, cron/runs/*.json and triggers/runs/*.json under a
//     workspace, and .webui_session_index.json, secondary.
//   - config.json: the keys that grant or restrict the agent, as
//     "permission setting:" rows, and the provider keys, MCP server env
//     values and channel secrets it holds, as one "stored credentials:"
//     row; and pairing.json.
//
// The session files hold naive host local time, emitted as if UTC; the
// WebUI and run records carry true UTC, hence their wider matching window.
// A messaging session key gives relay rows. See research/nanobot.md.
package nanobot
