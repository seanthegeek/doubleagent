// Package timeline defines the rows of timeline.jsonl and sessions.jsonl:
// the Row every parser emits, the per-session summary built from the rows,
// the marker texts a parser starts a system row with, the spellings of a
// Windows project path, the stable sorts and the JSONL writers.
//
// Both files are interfaces analysts build on: fields are added at the end,
// never renamed or removed. The writers put each record on one line, keys
// in column order, non-ASCII text as UTF-8, U+2028 and U+2029 escaped since
// some readers split lines at them, and a record holding a lone surrogate
// (which UTF-8 cannot encode) wholly in ASCII escapes.
package timeline

import (
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// The turn types a row's TurnType holds.
const (
	TurnUser       = "user"
	TurnAssistant  = "assistant"
	TurnToolUse    = "tool_use"
	TurnToolResult = "tool_result"
	TurnSystem     = "system"
	TurnThinking   = "thinking"
)

// The texts that start a parser's system row carrying a mark of its
// session, which Summarise and the inventory read.
const (
	// SyncedPrefix starts the row of a session synced to a remote account.
	SyncedPrefix = "session synced:"
	// RelayedPrefix starts the row of a session carried through a
	// messaging platform.
	RelayedPrefix = "session relayed:"
	// OriginPrefix starts the row of a session that began on the vendor's
	// servers.
	OriginPrefix = "session origin: remote"
	// ScratchPrefix starts a row naming a scratch directory, a working
	// directory that belongs to the agent rather than to the user; the
	// inventory lists its ProjectPath as a scratch directory, not a project.
	ScratchPrefix = "scratch directory:"
	// GitRemotePrefix starts a row naming a git remote URL the agent
	// recorded for the session's repository, the URL following as recorded.
	GitRemotePrefix = "git remote: "
)

// Row is one line of timeline.jsonl, its fields in column order.
type Row struct {
	TimestampUTC string
	Host         string
	User         string
	Agent        string
	SessionID    string
	ProjectPath  string
	GitBranch    string
	TurnType     string
	Model        string
	ToolName     string
	ToolUseID    string
	Text         string
	SourceFile   string
	SourceLine   int
	// Recovered is "yes" for a row of a secondary source that survived
	// deduplication, so the primary store does not hold the event.
	Recovered string
}

// Columns returns the field names of timeline.jsonl in order.
func Columns() []string {
	return []string{"timestamp_utc", "host", "user", "agent", "session_id", "project_path",
		"git_branch", "turn_type", "model", "tool_name", "tool_use_id", "text", "source_file",
		"source_line", "recovered"}
}

// TurnTypes returns the turn types in their documented order.
func TurnTypes() []string {
	return []string{TurnUser, TurnAssistant, TurnToolUse, TurnToolResult, TurnSystem,
		TurnThinking}
}

// Compact returns event text with leading and trailing whitespace removed,
// as Python's str.strip() removes it (Unicode spaces included). Inner line
// breaks and indentation stay: the writer escapes them, so each record
// still takes one line.
func Compact(text string) string {
	return pytext.Strip(text)
}

// CompactValue returns Compact of a decoded value's text as Python's str()
// writes it, and "" for null: a string as it is, a number, true or false
// in Python's spelling, an array or object as Python prints a list or dict.
func CompactValue(value pyjson.Value) string {
	if value.IsNull() {
		return ""
	}
	return pytext.Strip(value.Str())
}
