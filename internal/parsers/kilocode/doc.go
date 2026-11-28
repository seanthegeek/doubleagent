// Package kilocode reads Kilo Code's sessions and settings.
//
// Kilo Code's CLI, terminal UI and server are a fork of OpenCode with the
// same SQLite and JSON stores, so the parser holds an OpenCode parser made
// for Kilo's directories: .local/share/kilo/kilo.db, kilo-<channel>.db and
// opencode-<channel>.db, the storage/ and project/<slug>/storage/ JSON
// trees, and .local/state/kilo's prompt-history.jsonl and session.json
// (both secondary) and prompt-stash.jsonl. Every database is primary. It
// also reads Kilo's own files:
//
//   - the pre-fork editor extension's tasks,
//     <editor user directory>/User/globalStorage/kilocode.kilo-code/tasks/
//     <id>/api_conversation_history.json, and their history_item.json and
//     tasks/_index.json, which keep Roo Code's layout and are read through
//     package clinelegacy as Roo Code's are; the session id is the
//     directory name;
//   - storage/session_share/<session>.json, a cloud session marker;
//   - .config/kilo/{kilo.jsonc,kilo.json,opencode.jsonc,opencode.json,
//     config.json}: the permission key, read as JSON with comments;
//   - the sandbox policy and preference files under .local/state;
//   - retention/state.json, the last automatic deletion pass;
//   - memory/<folder>/sessions/<file>.md, project memory session digests,
//     with the folder's manifest.json canonical as the project path;
//   - log/<time>.log: the four remote control lines only.
//
// See research/kilo-code.md.
package kilocode
