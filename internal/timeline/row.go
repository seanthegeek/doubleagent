// Package timeline defines the rows of timeline.jsonl and sessions.jsonl:
// the Row every parser emits, the per-session summary built from the rows,
// the marker texts a parser starts a system row with, the spellings of a
// Windows project path, the stable sorts and the JSONL writers.
//
// Both files are interfaces analysts build on: fields are added at the end,
// never renamed or removed. The writers put each record on one line, keys
// in column order, non-ASCII text as UTF-8, U+2028 and U+2029 escaped since
// some readers split lines at them, and each byte of invalid UTF-8 as
// U+FFFD.
package timeline

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/secrets"
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
	TimestampUTC string `json:"timestamp_utc"`
	Host         string `json:"host"`
	User         string `json:"user"`
	Agent        string `json:"agent"`
	SessionID    string `json:"session_id"`
	ProjectPath  string `json:"project_path"`
	GitBranch    string `json:"git_branch"`
	TurnType     string `json:"turn_type"`
	Model        string `json:"model"`
	ToolName     string `json:"tool_name"`
	ToolUseID    string `json:"tool_use_id"`
	Text         string `json:"text"`
	SourceFile   string `json:"source_file"`
	SourceLine   int    `json:"source_line"`
	// Recovered is "yes" for a row of a secondary source that survived
	// deduplication, so the primary store does not hold the event.
	Recovered string `json:"recovered"`
	// Secrets is what the parse that gave the row identified, which the
	// output replaces in it (Withheld); nil in a run that shows secrets. It
	// is no column.
	Secrets *secrets.Found `json:"-"`
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
// as strings.TrimSpace removes it (Unicode spaces included). Inner line
// breaks and indentation stay: the writer escapes them, so each record
// still takes one line.
func Compact(text string) string {
	return strings.TrimSpace(text)
}

// CompactValue returns Compact of a decoded value's row text (Text): a
// string as it is, a number as written, true or false, "" for null, and an
// array or object as compact JSON.
func CompactValue(value jsonvalue.Value) string {
	if value.IsNull() {
		return ""
	}
	return strings.TrimSpace(value.Text())
}
