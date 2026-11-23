// Package claudedesktop reads the Claude Desktop app's local agent
// sessions: Cowork, the Code tab and scheduled tasks.
//
// Under the app-data directory (Claude or Claude-3p under Library/
// Application Support, .config, AppData/Roaming or AppData/Local,
// AppData/Local/Claude-Data, or the MSIX build's AppData/Local/Packages/
// <package>/LocalCache/Roaming/Claude) it reads: the Claude Code
// transcripts under local-agent-mode-sessions/<account>/<org>/[agent/]
// <session dir>/.claude/projects/ and claude-code-sessions/<account>/<org>/
// (imported-staging/ included), in Claude Code's own format; each session
// dir's audit.jsonl and the session records local_<uuid>.json, both
// secondary sources; and scheduled-tasks.json, git-worktrees.json and
// remote-session-spaces.json.
//
// A transcript's rows take the session record's folder as project path.
// The audit log gives user, assistant, tool, model, permission, result and
// init rows; the records, scheduled tasks, worktrees and remote folder
// grants give system rows. Every createdAt, errorAt and fireAt is epoch
// milliseconds. The app is closed source: the shapes come from its bundle.
// See research/claude-desktop.md.
package claudedesktop
