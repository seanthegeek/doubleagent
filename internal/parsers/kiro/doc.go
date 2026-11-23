// Package kiro reads the conversations of the Amazon Q Developer CLI, the
// open-source predecessor of the Kiro CLI.
//
// It reads, relative to the home: data.sqlite3 under .local/share,
// Library/Application Support or AppData/Local, in amazon-q/ or kiro-cli/
// (the conversations table, one serialised ConversationState per working
// directory, and the legacy Fig shell history table; auth_kv and state hold
// tokens and are never read); /save exports, a .json whose top-level object
// has conversation_id and history; .aws/amazonq/.cli_bash_history, the
// prompt's line-editor history (a secondary source); the agent
// configurations {.aws/amazonq,.amazonq}/cli-agents/<name>.json; and the
// delegate tool's .amazonq/.subagents/<agent>.json. Kiro CLI's
// .kiro/sessions/ files are not read.
//
// A ConversationState cut mid-write is salvaged entry by entry. Entries
// without a time of their own take the previous row's; the entries only a
// checkpoint snapshot or the tangent-mode copy still holds follow the
// history's rows, marked recovered. See research/kiro.md.
package kiro
