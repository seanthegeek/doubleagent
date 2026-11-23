// Package tabby reads Tabby's server stores: chat threads, pages,
// completion events, the model config and the editors' inline-edit command
// histories.
//
// Tabby is self-hosted: its stores sit under the server account's home and
// each thread's system row names who chatted. Paths start at .tabby/, or at
// the server root of a container volume's _data. Files read:
//
//   - ee/db.sqlite or ee/dev-db.sqlite, and the ee/*db.backup-<date>.sqlite
//     copies made before each migration, a secondary source that can hold
//     threads since deleted, which gives a "backup database" row. Only the
//     columns of tableColumns are read, each when the table has it; of the
//     secret-bearing columns, only a user's auth_token, as a "tabby user:"
//     row, and an integration's access_token, through parserkit.SecretText.
//   - events/<date>.json, one log entry per line (user, ts, event), a
//     secondary source: the server also writes each event to user_events.
//   - config.toml: one "model configured:" row per [model.<group>.<http|local>]
//     table at the file's time, api_key through parserkit.SecretText.
//   - The IntelliJ plugin's options/intellij-tabby-command-history.xml and
//     the VS Code extension's edit.recentlyCommand in the editor's
//     state.vscdb, secondary sources with no times ("inline edit command
//     history:" then a row per command), and that state's
//     server.serverRecords, a "tabby server:" row per endpoint with a token.
//
// The session id is the thread id, page:<id> or the completion_id; the
// project path is the first attached code[].git_url (Tabby records no local
// directory) or the event's segments.git_url. Integrations give "source
// host:" rows and their cached repositories "source host repository:" rows.
// An events line without an event object gives an "unknown log entry:" row,
// and a problem in any store a "parser:" row. See research/tabby.md.
package tabby
