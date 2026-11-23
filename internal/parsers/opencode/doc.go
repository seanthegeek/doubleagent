// Package opencode reads OpenCode's sessions: its SQLite databases, its
// legacy JSON trees and its terminal UI state.
//
// The databases are .local/share/opencode/opencode[-<channel>].db. A 2.x
// database keeps its transcript in session_message (with the session_part
// items of an assistant message) and its sessions in session_v2, and holds
// the 1.x session, message and part tables a migration left behind, which
// are read, marked recovered, only for a session the 2.x transcript lacks;
// a 1.x database is read the other way round. Beside the transcript: the
// session_share rows and the shares of session.updated.1 events (sync
// rows), archived sessions, per-session permission rules, saved permission
// grants and the worktrees OpenCode made (scratch directories).
// opencode-next.db, a pre-release 2.0 database, is a secondary source. The
// legacy storage/ and project/<slug>/storage/session/ trees hold one JSON
// file per session, message and part. The terminal UI's state directory
// holds prompt-history.jsonl (secondary), prompt-stash.jsonl and
// session.json (pinned sessions, secondary).
//
// The model and the agent (an OpenCode agent is a mode with its own
// permission rules) are tracked per session as "model:" and
// "permission mode:" rows. A row's project path is the session's
// directory, else its project's worktree unless "/", else the message's own
// working directory. Kilo Code's CLI is a fork that keeps the same stores
// under its own directories: Config describes where a build keeps them,
// and the kilocode parser holds a Parser made for Kilo's. See
// research/opencode.md.
package opencode
