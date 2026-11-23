// Package vscode reads VS Code's chat sessions (GitHub Copilot Chat and
// other participants) and chat state.
//
// The store is VS Code's own, so the parser covers every data directory the
// catalog gives to vscode. It reads, under User/:
//
//   - workspaceStorage/<hash>/chatSessions/<id>.jsonl, a mutation log
//     replayed into the session (kind 0 sets the state, 1 sets at path k,
//     2 pushes after truncating to i, 3 deletes), or <id>.json, and the
//     same under globalStorage/emptyWindowChatSessions/ and
//     transferredChatSessions/; the project is workingDirectory, else the
//     sibling workspace.json's folder;
//   - globalStorage/github.copilot-chat/cloudSessions.json and the
//     GitHub.copilot-chat row of state.vscdb: "session synced:", "session
//     origin:" and "source host repository:" rows;
//   - every profile's and workspace's state.vscdb: the standing tool grants
//     and, in a profile, the GitHub account usage keys; secret:// rows are
//     never read;
//   - a profile's settings.json: the auto-approve keys and the default
//     model;
//   - chatEditingSessions/<key>/state.json, a secondary source repeating
//     which files a request edited, which survives the chat's delete.
//
// See research/vscode.md.
package vscode
