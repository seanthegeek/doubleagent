// Package agentzero reads Agent Zero's chats, scheduler tasks, plugin
// settings and connector state.
//
// It reads <a0>/usr/chats/<ctxid>/chat.json with its messages/<n>.txt tool
// outputs (and the older <a0>/usr/chats/<ctxid>.json and <a0>/tmp/chats/
// copies), where <a0> is [Desktop/]agent-zero[/<instance>], or the same
// files under chats/ at the base of a collected /a0/usr Docker or Podman
// volume. Beside them it reads usr/scheduler/tasks.json and the Telegram
// plugin's state.json (both secondary sources), the _tool_access and
// _orchestrator plugin config.json and .toggle-0/.toggle-1 files, and the A0
// CLI connector's ~/.agent-zero/.env. It yields the chat log as user,
// assistant, thinking, tool_use, tool_result and system rows, and model,
// permission, sync, relay, origin and scheduled-task rows.
//
// Fields read are those listed in research/agent-zero.md; a task's token and
// system prompt are never read. A Telegram bot's token and webhook secret,
// a model configuration's API key and the connector .env's restore token,
// the one credential that file holds, are shown through parserkit.SecretText;
// the .env's other values are shown as written.
package agentzero
