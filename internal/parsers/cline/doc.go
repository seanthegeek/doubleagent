// Package cline reads Cline's legacy task layout, its SDK sessions and the
// stores beside them.
//
// The legacy tasks live under the VS Code extension's
// <editor>/User/globalStorage/saoudrizwan.claude-dev/ or ~/.cline/data/ and
// are read through package clinelegacy, with state/taskHistory.json as the
// task index and the extension's cache/sync-queue.json (only taskId, key,
// timestamp, status and retryCount; data and lastError are never read).
// Under ~/.cline/data/ are the SDK sessions: sessions/<id>/<id>.json
// manifests and *.messages.json transcripts, db/sessions.db for sessions
// whose manifest is gone, and the secondary sources db/session-search.db,
// cache/user_input_history.jsonl and logs/hooks.jsonl. The standing
// settings of globalState.json, settings/global-settings.json,
// settings/cline_mcp_settings.json (only autoApprove and disabled per
// server) and cloud-pending-starts/*.json give permission setting, sync and
// origin rows.
//
// The API history is a fallback, not a secondary source: it words a turn
// differently from ui_messages.json. See research/cline.md.
package cline
