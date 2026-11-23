// Package letta reads the local files of Letta Code, which keeps its
// conversations on the Letta server by default. Under .letta/ unless said:
//
//   - lc-local-backend/conversations/<base64url key>/messages.jsonl, the
//     local backend: pi's session format, read with the pi package's
//     reader; only the last copy of each message id is kept. Schema 1 files
//     (one bare message per line, no header) are timed by the message's
//     metadata.created_at.
//   - transcripts/<agent>/<conversation>/transcript.jsonl, a secondary
//     source stamped once per turn when the turn ends; its project comes
//     from sessions.jsonl by time, a heuristic. A transcript of an agent not
//     named agent-local-... is a server conversation: a `session synced:`
//     row.
//   - sessions.jsonl, settings.json, the legacy .config/letta/settings.json
//     and a project's settings.local.json (sessionsByServer, permissions,
//     agents[], and env and refreshToken, the credentials, through
//     parserkit.SecretText), channels/<channel>/routing.json or routing.yaml (read as
//     JSON), crons.json, runs/<task id>.jsonl and
//     listener-state/<24 hex>/<agent>_<conversation>.json.
//
// See research/letta.md.
package letta
