// Package pearai reads PearAI's chat sessions (a fork of Continue) and
// agent tasks (a fork of Roo Code 3.15).
//
// Both forks are older snapshots of their upstreams, so the shapes differ
// from packages continuedev and roocode. It reads:
//
//   - .pearai/sessions/<sessionId>.json: history[] and perplexityHistory[]
//     items' message, contextItems[], citations[] and
//     promptLogs[].completionOptions.model; every row takes the index's
//     dateCreated, and source_line is the item's position across both lists;
//   - .pearai/sessions/sessions.json, the index, read as Continue's; it
//     yields rows only for what it loses;
//   - User/globalStorage/pearai.pearai-roo-cline/tasks/<taskId>/ task files,
//     read through package clinelegacy; tool calls are XML tags in the
//     assistant text and have no id, so tool_use_id is the message's
//     position, and results that open with one of Roo's permission texts
//     give "permission decision:" rows;
//   - User/globalStorage/state.vscdb, row pearai.pearai-roo-cline:
//     taskHistory[] (one row per task) and the standing permission keys;
//   - the PearAI output channel log and .pearai/dev_data/chat.jsonl,
//     secondary sources repeating the sessions' prompts and replies, with no
//     time, and .pearai/dev_data/tokens_generated.jsonl, one "tokens
//     generated:" row per request.
//
// See research/pearai.md.
package pearai
