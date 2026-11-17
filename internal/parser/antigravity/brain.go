package antigravity

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The two forms of a brain transcript: complete, and shortened.
const (
	fullForm  = "transcript_full"
	shortForm = "transcript"
)

// brainLogs locates the files of one brain transcript's logs directory.
type brainLogs struct {
	artifact *input.Artifact
	levels   int // how far above the artifact's directory the logs directory is
}

// path returns the path of names under the logs directory.
func (l brainLogs) path(names ...string) string {
	found, ok := parser.Above(l.artifact, l.levels, names...)
	if !ok {
		return ""
	}
	return found
}

// regular reports whether names under the logs directory is a regular
// file.
func (l brainLogs) regular(names ...string) bool {
	found := l.path(names...)
	return found != "" && parser.RegularFile(found)
}

// chunks returns the chunk files of a form under logs/chunks, in chunk
// order: regular files named <digits>.jsonl, ties (01 and 1) in name order.
func (l brainLogs) chunks(form string) []string {
	directory := l.path("chunks", form)
	if directory == "" {
		return nil
	}
	entries, err := parser.ReadDir(directory)
	if err != nil {
		return nil
	}
	var names []string
	for _, entry := range entries {
		stem, ok := strings.CutSuffix(entry.Name(), ".jsonl")
		if ok && stem != "" && strings.Trim(stem, "0123456789") == "" &&
			parser.RegularFile(filepath.Join(directory, entry.Name())) {
			names = append(names, entry.Name())
		}
	}
	slices.SortStableFunc(names, func(left, right string) int {
		return compareDigits(strings.TrimSuffix(left, ".jsonl"), strings.TrimSuffix(right, ".jsonl"))
	})
	return names
}

// compareDigits compares two strings of ASCII digits by the numbers they
// write, however long.
func compareDigits(left, right string) int {
	left, right = strings.TrimLeft(left, "0"), strings.TrimLeft(right, "0")
	if len(left) != len(right) {
		return len(left) - len(right)
	}
	return strings.Compare(left, right)
}

// brainCopy returns the bytes of the brain transcript the artifact stands
// for, or false when another copy of the same transcript is the one read
// or the file cannot be read (which adds its problem). The copies, in order
// of preference: transcript_full.jsonl; its chunks, concatenated, so a line
// cut at a chunk boundary is whole again; transcript.jsonl; its chunks. A
// chunked form is read through its first chunk present.
func brainCopy(artifact *input.Artifact, form, chunkForm string, problems *parser.Problems) ([]byte, bool) {
	logs := brainLogs{artifact: artifact}
	switch form {
	case fullForm:
		return parser.FileBytes(artifact.DiskPath, problems)
	case shortForm:
		if logs.regular(fullForm+".jsonl") || len(logs.chunks(fullForm)) > 0 {
			return nil, false
		}
		return parser.FileBytes(artifact.DiskPath, problems)
	}
	logs.levels = 2
	chunks := logs.chunks(chunkForm)
	if len(chunks) == 0 || chunks[0] != filepath.Base(artifact.DiskPath) || logs.regular(fullForm+".jsonl") {
		return nil, false
	}
	if chunkForm == shortForm && (len(logs.chunks(fullForm)) > 0 || logs.regular(shortForm+".jsonl")) {
		return nil, false
	}
	var joined []byte
	for _, chunk := range chunks {
		data, _ := parser.FileBytes(logs.path("chunks", chunkForm, chunk), problems)
		joined = append(joined, data...)
	}
	return joined, true
}

// brain emits the rows of one brain transcript, read once per
// conversation from the copy brainCopy picks.
func (p *Parser) brain(artifact *input.Artifact, match []string, options parser.Options, emit parser.Emit) {
	sessionID := match[2]
	var problems parser.Problems
	blob, ok := brainCopy(artifact, match[3], match[4], &problems)
	if ok {
		lines := &brainLines{owner: p, artifact: artifact, sessionID: sessionID, options: options, emit: emit}
		for i, raw := range bytes.Split(blob, []byte("\n")) {
			stripped := bytes.Trim(raw, " \t\n\r\v\f")
			if len(stripped) == 0 {
				continue
			}
			record, err := jsonvalue.DecodeString(textlines.ValidUTF8(stripped))
			if err != nil {
				problems.AddRaw(i+1, parser.UnparseableLine(err), parser.LineText(raw))
				continue
			}
			if record.IsObject() {
				lines.record(i+1, record)
			}
		}
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, ""))
}

// brainLines renders the lines of one brain transcript as the database
// reader renders the same step kinds.
type brainLines struct {
	owner     *Parser
	artifact  *input.Artifact
	sessionID string
	options   parser.Options
	emit      parser.Emit
	calls     []jsonvalue.Value // the last reply's tool calls not yet matched to a result
}

// row emits a row of a brain line; text whose field the line marks as
// truncated says so.
func (b *brainLines) row(number int, record jsonvalue.Value, turnType, text, field string) timeline.Row {
	row := b.owner.BaseRow(b.artifact)
	row.SourceLine, row.SessionID, row.TurnType = number, b.sessionID, turnType
	row.TimestampUTC = timeutil.FromValue(record.Get("created_at"))
	for _, truncated := range record.Get("truncated_fields").Items() {
		if truncated.Text() == field {
			text += " [truncated: " + field + "]"
			break
		}
	}
	row.Text = timeline.Compact(text)
	return row
}

// isText reports whether value is the string text.
func isText(value jsonvalue.Value, text string) bool {
	return value.IsString() && value.StringValue() == text
}

// record emits the rows of one brain line.
func (b *brainLines) record(number int, record jsonvalue.Value) {
	kind := parser.Str(record.Get("type"))
	content := parser.TextOf(record.Get("content"))
	switch kind {
	case "USER_INPUT":
		turnType := timeline.TurnUser
		if isText(record.Get("source"), "USER_IMPLICIT") {
			turnType = timeline.TurnSystem
		}
		b.calls = nil
		b.emit(b.row(number, record, turnType, content, "content"))
		return
	case "PLANNER_RESPONSE":
		b.reply(number, record, content)
		return
	}
	text := brainEventText(kind, record, content)
	if status := strings.ToLower(parser.Str(record.Get("status"))); status != "" && status != "done" {
		text = "[" + status + "] " + text
	}
	if kind == "SYSTEM_MESSAGE" || kind == "ERROR_MESSAGE" || len(b.calls) == 0 {
		b.emit(b.row(number, record, timeline.TurnSystem, text, "content"))
		return
	}
	call := b.calls[0]
	b.calls = b.calls[1:]
	row := b.row(number, record, timeline.TurnToolResult, text, "content")
	row.ToolName, row.ToolUseID = parser.Str(call.Get("name")), parser.Str(call.Get("id"))
	b.emit(row)
}

// brainEventText returns the text of a brain line that is neither a prompt
// nor a reply.
func brainEventText(kind string, record jsonvalue.Value, content string) string {
	switch kind {
	case "SYSTEM_MESSAGE":
		return "system_message: " + content
	case "ERROR_MESSAGE":
		failure := record.Get("error")
		if failure.IsObject() {
			failure = firstTruthy(failure.Get("short_error"), failure.Get("user_error_message"),
				failure.Get("full_error"))
		}
		if failure.IsString() && failure.StringValue() != "" {
			return "error: " + failure.StringValue()
		}
		return "error: " + content
	}
	if content != "" {
		return content
	}
	return "step " + strings.ToLower(kind)
}

// firstTruthy returns the first value Python counts as true, else the
// last, as `a or b or c` does.
func firstTruthy(values ...jsonvalue.Value) jsonvalue.Value {
	for _, value := range values {
		if !value.IsEmpty() {
			return value
		}
	}
	return values[len(values)-1]
}

// reply emits the rows of a reply: its reasoning when asked for, its text
// and its tool calls, which the next lines' results answer in order.
func (b *brainLines) reply(number int, record jsonvalue.Value, content string) {
	if thinking := parser.TextOf(record.Get("thinking")); thinking != "" && b.options.IncludeThinking {
		b.emit(b.row(number, record, timeline.TurnThinking, thinking, "thinking"))
	}
	if content != "" {
		b.emit(b.row(number, record, timeline.TurnAssistant, content, "content"))
	}
	b.calls = nil
	for _, raw := range record.Get("tool_calls").Items() {
		call := raw
		if !call.IsObject() {
			call = jsonvalue.NewObject()
		}
		b.calls = append(b.calls, call)
		row := b.row(number, record, timeline.TurnToolUse, toolArgsSummary(brainArgs(call)), "tool_calls")
		row.ToolName, row.ToolUseID = parser.Str(call.Get("name")), parser.Str(call.Get("id"))
		b.emit(row)
	}
}

// brainArgs returns a brain tool call's arguments as JSON text:
// arguments_json as the database records it, else args or arguments, an
// object or a string.
func brainArgs(call jsonvalue.Value) string {
	for _, key := range []string{"arguments_json", "args", "arguments"} {
		value := call.Get(key)
		if value.IsString() && value.StringValue() != "" {
			return value.StringValue()
		}
		if value.IsObject() || value.IsArray() {
			return value.JSON()
		}
	}
	return ""
}
