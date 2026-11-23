// Package openhands reads OpenHands conversations: Agent Canvas, the Agent
// Server and the openhands CLI.
//
// Files read, under .openhands/ (see research/openhands.md):
//
//   - [agent-canvas/[dev_]]conversations/<hex id>/events/event-<index>-<id>.json,
//     one event object per file. The conversation's meta.json (title,
//     created_at, parent and fork ids, workspace.working_dir) and
//     base_state.json (agent.llm.model, confirmation_policy,
//     security_analyzer, blocked_actions, token usage) are read once per
//     conversation; their secret fields are never read.
//   - base_state.json as its own file: the standing "permission setting:" rows
//     and a "model:" row for a switch made over REST, at the file's time.
//   - projects/<sha256 of cwd>/prompt_history.json, the CLI's prompts, a
//     secondary source (a file repeating the conversations' prompts).
//   - Legacy 0.x sessions/<sid>/events/<n>.json: one row per session, not
//     parsed.
//
// Event times are naive host local time. When meta.json has a UTC
// created_at, the host's offset is the first event's time less created_at,
// floored to the quarter hour after a minute of skew; without it (the CLI
// writes none) the times are emitted as if UTC, and the header row says
// which. The session id is the directory name; the project path is
// workspace.working_dir; no git branch is recorded; the source line is 1.
package openhands
