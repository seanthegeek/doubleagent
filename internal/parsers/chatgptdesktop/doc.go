// Package chatgptdesktop reads the ChatGPT desktop app's list of account
// conversations.
//
// The app keeps .codex/sqlite/codex-dev.db (SQLite in WAL mode, its recent
// rows in the -wal sidecar), whose table local_thread_catalog lists the
// account's conversations. A ChatGPT-mode conversation leaves no transcript
// on disk, only the row the app writes at sign-in, whose source_kind is
// "chatgpt"; the other rows are the app's Codex threads, whose rollouts the
// codex-cli parser reads, and are skipped. The catalog attributes .codex to
// codex-cli, so this parser reads that agent's artifacts too.
//
// Columns read, each possibly NULL: thread_id, display_title,
// source_created_at, source_updated_at (REAL epoch seconds),
// conversation_origin, host_id and source_kind. Each conversation gives three
// system rows at source_updated_at, else source_created_at: the "account
// conversation:" row, a "session origin: remote" row and a "session synced:"
// row. A file that is not SQLite, a SQLite error, or a database without the
// table is one `parser:` row. See research/chatgpt-desktop.md.
package chatgptdesktop
