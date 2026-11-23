// Package cursor reads the Cursor IDE's agent conversations and standing
// settings.
//
// Cursor's names: a composer is one conversation, a bubble one message,
// reasoning block or tool call in it, toolFormerData a tool call's record,
// yolo auto-run, and a cloud agent (bc-<uuid>) a conversation run on
// Cursor's servers. <ide> is Library/Application Support/Cursor,
// .config/Cursor or AppData/Roaming/Cursor. It reads:
//
//   - <ide>/User/globalStorage/state.vscdb, the primary store: the
//     cursorDiskKV composerData, composerConvHeaders, bubbleId,
//     composerDraft and (for deleted composers only) JSON agentKv:blob:
//     rows; the composerHeaders table; and ItemTable keys: the settings
//     blob, the sandbox switch, github-<login> and its -usages,
//     remoteAgentHost., the cloud agent lists and MCP authorisations, the
//     repository list, the key names of mcpOAuth. and the installed
//     plugins. No secret:// value is read;
//   - .cursor/projects/<cwd>/agent-transcripts/<id>/<id>.jsonl, a
//     secondary source whose user blocks carry a minute-precision time tag;
//   - .cursor/ai-tracking/ai-code-tracking.db, conversation-search.db (its
//     full-text content table read as a plain table) and
//     mcp-oauth-attempts/*.json under <ide>/User/globalStorage/,
//     .cursor/cli-config.json and .cursor/permissions.json.
//
// The project, session and model of a side database's or transcript's row
// come from the home's state.vscdb, read once per state of that file. See
// research/cursor.md.
package cursor
