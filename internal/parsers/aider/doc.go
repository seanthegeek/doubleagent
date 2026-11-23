// Package aider reads Aider's chat, input and LLM history files.
//
// It reads .aider.chat.history.md (the primary store) and
// .aider.input.history and .aider.llm.history (secondary sources), selected
// by file name wherever they sit: Aider writes them into the git root or
// working directory, so they usually arrive through the project catalog with
// the project as the home. Rows are labelled aider, and the project path is
// the directory holding the file.
//
// Formats read: the chat history's "# aider chat started at <local time>"
// headers, "#### " user lines, "> " tool and system output lines and
// assistant markdown, where a "> " run's announcement lines (Model:, Main
// model:, Editor model:, Weak model:) give the model rows and its
// confirm_ask lines the permission rows; the input history's prompt_toolkit
// FileHistory entries ("# <local time>" then "+<line>" lines); and the LLM
// history's "TO LLM <time>" and "LLM RESPONSE <time>" blocks of
// role-prefixed lines.
//
// Every timestamp is host local time with no zone and is emitted as if it
// were UTC. The session id is "<path of .aider.chat.history.md>#<header
// timestamp>"; input and LLM rows take the last header at or before their
// own time, and a chat user row takes the time of the matching input-history
// entry. No git branch or tool call id is recorded. See research/aider.md.
package aider
