// Package ollama reads Ollama's desktop chats, the REPL prompt history and
// the launcher's auto-mode settings.
//
// It reads, relative to the home:
//
//   - Library/Application Support/Ollama/db.sqlite and
//     AppData/Local/Ollama/db.sqlite, the desktop app's database: messages,
//     tool_calls and attachments (of which only the byte length of data is
//     read). The users and settings tables are never read. created_at is
//     local time with an offset.
//   - .ollama/history, the `ollama run` REPL's line history, one entry per
//     line with no time, session or model, a line starting with / a
//     "slash command:" row; a line that is not UTF-8 is a bad line. The
//     file keeps only the last 100 entries and splits a pasted multi-line
//     prompt across lines, one entry per line.
//   - .ollama/config.json (integrations.<name>.automode) and the Claude
//     Desktop third-party profile Ollama's launcher writes
//     (autoModeEnabled), as "permission setting:" rows at the file's mtime.
//
// There is no call id: the n-th tool message after an assistant message
// pairs with that message's n-th call, and the tool_use_id is
// "<chat_id>:<tool_calls.id>"; an extra tool message takes the last call's
// name and no id, as the app's reload does. The session id is the chat id.
// See research/ollama.md.
package ollama
