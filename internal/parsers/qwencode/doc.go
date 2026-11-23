// Package qwencode reads Qwen Code's chat files, prompt logs, settings,
// channel files and past-use stores, under ~/.qwen and the managed local
// mode's .qwen/managed-local/harness/ and .qwen/managed-local/worker-home/,
// which are read as .qwen/ is.
//
// The transcripts are projects/<cwd>/chats/[archive/]<sessionId>.jsonl, one
// ChatRecord per line, with the sidecar <sessionId>.worktree.json of a
// session in a worktree checkout and <sessionId>.pr.json of its pull
// requests. tmp/<hash>/logs.json, usage_record.jsonl and a channel's
// <channel>-group-history.jsonl are secondary sources. The parser also
// reads the token usage log, the todo files, the Web Shell's session
// organization, PROJECT_SUMMARY.md, trustedFolders.json (through Gemini
// CLI's reader, whose form the fork keeps), settings.json (only the
// permission lists and the approval mode, since the file holds provider
// keys and channel tokens), the channel routes and the GitHub channel's
// audit log. Model and approval mode are tracked per sessionId.
//
// A group history message matches the transcript's quote of it at the time
// the quote cleared it, not at its own: the parser declares deduplication
// hooks (MatchForm, MatchAnchor) for that. See research/qwen-code.md.
package qwencode
