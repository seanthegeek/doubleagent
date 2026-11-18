package claudecode

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// toolKeys returns the input fields that best identify a call of the tool
// name, in the order they are joined, or nil for a tool not listed.
func toolKeys(name string) []string {
	switch name {
	case "Bash":
		return []string{"command"}
	case "Read", "Write", "Edit", "MultiEdit":
		return []string{"file_path"}
	case "NotebookEdit":
		return []string{"notebook_path"}
	case "Glob", "Grep":
		return []string{"pattern", "path"}
	case "WebFetch":
		return []string{"url"}
	case "WebSearch":
		return []string{"query"}
	case "Agent", "Task":
		return []string{"description", "prompt"}
	case "Skill":
		return []string{"skill", "args"}
	case "LS":
		return []string{"path"}
	}
	return nil
}

// ToolSummary returns the text of a tool call's row: the input fields that
// name its action (the command, the file, the pattern and path), joined by
// " | ", else the first of the common fields present, else the input as
// compact JSON with sorted keys. An input that is not an object is written
// as compact JSON. A field counts as present unless it is missing, null or
// an empty string.
func ToolSummary(name string, toolInput jsonvalue.Value) string {
	if !toolInput.IsObject() {
		return parser.JSONText(toolInput)
	}
	var values []string
	for _, key := range toolKeys(name) {
		if value := toolInput.Get(key); parser.Present(value) {
			values = append(values, value.Text())
		}
	}
	if len(values) > 0 {
		return strings.Join(values, " | ")
	}
	for _, key := range []string{"command", "file_path", "path", "pattern", "url", "query",
		"description"} {
		if value := toolInput.Get(key); parser.Present(value) {
			return value.Text()
		}
	}
	return parser.JSONText(toolInput)
}

// ResultText returns the text of a tool_result block's content: for a list
// of blocks, each block's text (a tool_reference block's tool name) joined
// by LF, empty parts left out; anything else as parser.TextOf reads it.
func ResultText(content jsonvalue.Value) string {
	if !content.IsArray() {
		return parser.TextOf(content)
	}
	var parts []string
	for _, block := range content.Items() {
		part := ""
		if block.IsObject() && parser.IsText(block.Get("type"), "tool_reference") {
			part = parser.Text(block.Get("tool_name"))
		} else {
			part = parser.TextOf(jsonvalue.NewArray(block))
		}
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "\n")
}

// forkMark starts the task of a forked subagent.
const forkMark = "<fork-boilerplate>"

// markerLabel returns the system-row label of the text marker at the start
// of text, leading whitespace aside, or "": the tags that start a user
// record's text when the agent, not the person, wrote it.
func markerLabel(text string) string {
	head := strings.TrimLeftFunc(text, unicode.IsSpace)
	markers := [...]struct{ mark, label string }{
		{"<task-notification>", "task notification"},
		{"<command-name>", "slash command"},
		{"<command-message>", "slash command"},
		{"<local-command-stdout>", "command output"},
		{"<local-command-caveat>", "context"},
		{"<system-reminder>", "context"},
		{forkMark, "subagent task"},
	}
	for _, marker := range markers {
		if strings.HasPrefix(head, marker.mark) {
			return marker.label
		}
	}
	return ""
}

// originLabel returns the label of a user record's origin.kind when the
// record is not the person typing, "" for the person or an unknown kind.
func originLabel(kind jsonvalue.Value) string {
	switch kind.Text() {
	case "task-notification":
		return "task notification"
	case "peer":
		return "peer message"
	case "coordinator":
		return "coordinator message"
	case "auto-continuation":
		return "auto-continuation"
	}
	return ""
}

// textOr returns the value's row text, or missing when it is null or an
// empty string.
func textOr(value jsonvalue.Value, missing string) string {
	if !parser.Present(value) {
		return missing
	}
	return value.Text()
}

// Match forms of a slash command in the transcript, and of a pasted text.
var (
	commandNamePattern  = regexp.MustCompile(`(?s)<command-name>(.*?)</command-name>`)
	commandArgsPattern  = regexp.MustCompile(`(?s)<command-args>(.*?)</command-args>`)
	asciiSpacePattern   = regexp.MustCompile(`[ \t\r\n\f\v]+`)
	pasteWrapperPattern = regexp.MustCompile(`</?pasted_content(?:[ \t\r\n][^>]*)?>`)
	// A history line that is a slash command as typed (/model, /rename x).
	historyCommandPattern = regexp.MustCompile(`^/[A-Za-z][A-Za-z0-9:_-]*(?:[ \t]|\z)`)
)

// commandWindowSeconds is how long after its history line a slash command's
// transcript record may be written: a command that opens a picker, an
// overlay or a flow (/model, /help) is recorded when it completes.
const commandWindowSeconds = 600.0

// MatchForm returns the turn type and text a row is deduplicated under. A
// slash command, typed in history.jsonl and in tag form in the transcript
// (a "slash command:" or "local_command:" row), matches as the user text
// "/<name>", plus " <args>" when the arguments are not empty. A prompt
// matches with the transcript's <pasted_content> wrapper removed, since
// the history holds the bare paste. Both collapse each run of ASCII
// whitespace to one space; every other row matches as it is.
func MatchForm(row timeline.Row) (turnType, text string) {
	switch row.TurnType {
	case timeline.TurnSystem:
		if command, ok := commandForm(row.Text); ok {
			return timeline.TurnUser, squash(command)
		}
	case timeline.TurnUser:
		return timeline.TurnUser, squash(pasteWrapperPattern.ReplaceAllLiteralString(row.Text, " "))
	}
	return row.TurnType, row.Text
}

// commandForm returns the command a "slash command:" or "local_command:"
// row's tags name, as typed, and false for any other text.
func commandForm(text string) (string, bool) {
	if !strings.HasPrefix(text, "slash command: ") && !strings.HasPrefix(text, "local_command: ") {
		return "", false
	}
	name := commandNamePattern.FindStringSubmatch(text)
	if name == nil {
		return "", false
	}
	command := strings.Trim(name[1], " \t\r\n")
	if !strings.HasPrefix(command, "/") {
		command = "/" + command
	}
	if arguments := commandArgsPattern.FindStringSubmatch(text); arguments != nil {
		if trimmed := strings.Trim(arguments[1], " \t\r\n"); trimmed != "" {
			command += " " + trimmed
		}
	}
	return command, true
}

// squash returns text with each run of ASCII whitespace collapsed to one
// space and spaces trimmed from both ends.
func squash(text string) string {
	return strings.Trim(asciiSpacePattern.ReplaceAllLiteralString(text, " "), " ")
}

// MatchWindow returns the forward window of a history row that is a slash
// command as typed, and false for any other row, which keeps the source's
// own tolerance.
func MatchWindow(row timeline.Row) (float64, bool) {
	if row.TurnType == timeline.TurnUser && historyCommandPattern.MatchString(row.Text) {
		return commandWindowSeconds, true
	}
	return 0, false
}
