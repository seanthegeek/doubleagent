// Package cody reads Sourcegraph Cody's chat history, settings and model
// preferences.
//
// It reads the account-keyed chat history,
// {"<endpoint>-<username>": {"chat": {"<chatID>": transcript}}}, and the
// top-level "chat" of a history from before account keys, from the
// sourcegraph.cody-ai row of an editor's state.vscdb (its
// cody-local-chatHistory-v2 member) or the JetBrains plugin's
// Cody-nodejs/[Data/]JetBrains-globalState/cody-local-chatHistory-v2 file;
// the cody-model-preferences value (the row's member or the JetBrains
// file); the editor's User/settings.json (JSON with comments); and a
// project's .idea/cody_history.xml, the older plugin's history, a
// secondary source. The catalog gives the editor files to the editor, so
// the parser reads every editor agent's.
//
// No message has a time: every row takes the chat id, the creation time as
// Date.toUTCString() writes it, else lastInteractionTimestamp. A history
// that does not parse is walked chat by chat, and the XML is read as a
// stream, so the chats before the damage are still read. A MCP server's
// headers, env, url, command and args are never read. See research/cody.md.
package cody
