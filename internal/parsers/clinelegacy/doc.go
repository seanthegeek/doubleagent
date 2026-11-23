// Package clinelegacy reads the per-task layout Cline introduced and Roo
// Code and PearAI's agent inherited, for the parsers of those agents. It
// registers no parser of its own.
//
// Under a storage root (an editor's User/globalStorage/<extension id>,
// Cline's ~/.cline/data, Roo's ~/.vscode-mock/global-storage) each task is
// tasks/<taskId>/ holding:
//
//   - ui_messages.json, the preferred transcript: a JSON array of messages
//     {ts, type, ask, say, text, reasoning, images, partial, modelInfo},
//     whose text is itself JSON for several kinds;
//   - api_conversation_history.json (Roo's older claude_messages.json), read
//     only when no ui_messages.json sits beside it: model API messages
//     {role, content} with text, tool_use, tool_result, thinking, image and
//     file blocks, plus Roo's ts, isSummary and reasoning records. A record
//     without ts takes the task's start: a millisecond task id, else the
//     earliest task_metadata.json time, else the task index entry's ts;
//   - task_metadata.json: files_in_context[], model_usage[] and
//     environment_history[], one system row per dated event and model: and
//     permission mode: rows on a change.
//
// Each agent's parser embeds a TaskParser built with the storage root's
// pattern and the index key of the project path, and gives it a Family that
// finds a task's index entry. A record that does not decode stops the array
// walk and becomes one `parser: unparseable record <N> (<reason>): <raw>`
// row holding the text from that record to the end of the file. See
// research/cline.md and research/roo-code.md.
package clinelegacy
