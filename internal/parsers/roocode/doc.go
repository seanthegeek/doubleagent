// Package roocode reads Roo Code's tasks, their permission decisions, its
// standing settings and the CLI's prompt history.
//
// Roo Code forked Cline and keeps its per-task layout, read through package
// clinelegacy, under the extension's
// User/globalStorage/rooveterinaryinc.roo-cline/ and the CLI's
// .vscode-mock/global-storage/. Besides the task files it reads:
//
//   - tasks/<taskId>/history_item.json (the task's index entry; workspace is
//     the project path, mode gives a "permission mode:" row) and
//     tasks/_index.json (only entries whose history_item.json is gone);
//   - the global state (the extension's row of the editor's state.vscdb, or
//     the CLI's global-state.json): the standing permission keys and the
//     Roo Code Cloud settings cache as setting rows; the workspace state's
//     roo.retryQueue[], one "cloud upload queued:" row per request with
//     its bearer token through parserkit.SecretText, never its body;
//   - .roo/cli-settings.json, settings/mcp_settings.json and a project's
//     .roo/mcp.json (mcpServers.<name>.alwaysAllow only);
//   - .roo/cli-history.json, a secondary source repeating the tasks'
//     prompts: one user row per entry, with no time, task or project.
//
// A ui_messages.json tool ask marked isAnswered and an API history result
// opening with the JSON Roo writes for a denial or an approval with
// feedback give "permission decision:" rows. See research/roo-code.md.
package roocode
