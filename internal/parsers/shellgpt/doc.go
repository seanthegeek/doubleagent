// Package shellgpt reads ShellGPT's chat files, settings, functions and
// response cache.
//
// Files read, from research/shellgpt.md:
//
//   - chat_cache/<chat id>: one JSON array of OpenAI chat messages (role,
//     content, tool_calls, tool_call_id, and the older function_call answered
//     by a "function" message). A function call has no id, so it gets
//     "function_call_<array index>", shared with the "function" message after
//     it.
//   - .config/shell_gpt/.sgptrc: KEY=value lines. The permission keys, the
//     default model and the API key reach a row, the key through
//     parserkit.SecretText; no other line does.
//   - .config/shell_gpt/functions/*.py: one "permission setting:" row per
//     file, at its modification time; the code is not read.
//   - shell_gpt/cache/<md5>: one response string per file, a secondary source
//     that repeats the chat's answers, whose rows have no time or session.
//
// A chat file records no time, model, working directory or branch; the
// session id is the file name and the source line is 1, since the file is one
// line.
package shellgpt
