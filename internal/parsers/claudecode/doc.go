// Package claudecode reads Claude Code's transcripts and the files beside
// them, under ~/.claude or a CLAUDE_CONFIG_DIR home ~/.claude-<name>.
//
// The transcripts are projects/<slug>/<session>.jsonl and
// <session>/subagents/[<dir>/]agent-<id>.jsonl, with the full text of
// persisted tool outputs in <session>/tool-results/<name>.txt. Beside them:
// ccr-tip.json and bridge-pointer.json under projects/ (Remote Control),
// history.jsonl (a secondary source, the prompts as typed) with its
// paste-cache/, the process registry sessions/<pid>.json, the user and
// project settings[.local].json and ~/.claude.json. The rows are user,
// assistant, thinking, tool_use, tool_result and system rows, the system
// rows carrying surface, origin, sync, permission, model, prompt trigger,
// title, scratch directory and setting changes. Only the settings keys
// named here are read: several others can hold credentials.
//
// A history line and its transcript record differ in form for slash
// commands and pastes, and a command's record can come long after its line:
// the parser declares deduplication hooks (MatchForm, MatchWindow) for that.
//
// Reader holds the readers of these formats under any agent name, since the
// Claude Desktop app keeps the same transcripts; ToolSummary and ResultText
// are shared with the parsers of other agents that write Claude's tool
// blocks. See research/claude-code.md.
package claudecode
