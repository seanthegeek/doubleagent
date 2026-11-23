// Package goose reads the Goose CLI's and desktop app's sessions.
//
// It reads, relative to the home (<data> is .local/share/goose or
// AppData/Roaming/Block/goose/data, <state> is .local/state/goose or the
// Windows data directory, <config> is .config/goose or
// AppData/Roaming/Block/goose/config): <data>/sessions/sessions.db, the
// legacy <data>/sessions/<YYYYMMDD_HHMMSS>.jsonl, the request logs
// <state>/logs/llm_request.<N>.jsonl and the prompt histories
// <state>/history.txt and .config/goose/history.txt (both secondary), and
// the standing files <config>/permission.yaml, <config>/roaming_trust.json
// and <data>/schedule.json.
//
// It yields session start rows, one row per content block, model,
// permission, relay and scheduled-task system rows, and `parser:` rows for
// stored values that do not decode. Content blocks are read by type: text,
// image, document, toolRequest, toolResponse, thinking, redactedThinking,
// toolConfirmationRequest, actionRequired, systemNotification and error.
// See research/goose.md.
package goose
