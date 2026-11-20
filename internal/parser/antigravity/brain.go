package antigravity

import (
	"bytes"
	"errors"
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

// errNotObject is the reason a brain line that decodes to something other
// than an object is reported.
var errNotObject = errors.New("not a JSON object")

// The two forms of a brain transcript: complete, and shortened.
const (
	fullForm  = "transcript_full"
	shortForm = "transcript"
)

// brainLogs locates the files of one brain transcript's logs directory.
type brainLogs struct {
	artifact *input.Artifact
	levels   int // how far above the artifact's directory the logs directory is
	listed   *chunkListings
	work     parser.Work
}

// chunkListings keeps the chunk names of each chunks directory listed, so
// the many chunk files of one transcript list their directory once; it
// holds each chunk's name once, as the run's list of files does.
type chunkListings struct {
	names map[string][]string
}

// newChunkListings returns an empty set of listings.
func newChunkListings() *chunkListings {
	return &chunkListings{names: map[string][]string{}}
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
	if held, ok := l.listed.names[directory]; ok {
		return held
	}
	l.work.Add("chunk listings", 1)
	listed, ok := parser.DirectoryAt(directory)
	entries, err := parser.ReadDir(directory)
	if !ok || err != nil {
		l.listed.names[directory] = nil
		return nil
	}
	var names []string
	for _, entry := range entries {
		stem, ok := strings.CutSuffix(entry.Name(), ".jsonl")
		if !ok || stem == "" || strings.Trim(stem, "0123456789") != "" {
			continue
		}
		if path, named := listed.Child(entry.Name()); named && parser.RegularFile(path) {
			names = append(names, entry.Name())
		}
	}
	slices.SortStableFunc(names, func(left, right string) int {
		return parser.CompareDigits(strings.TrimSuffix(left, ".jsonl"), strings.TrimSuffix(right, ".jsonl"))
	})
	l.listed.names[directory] = names
	return names
}

// brainCopy returns the bytes of the brain transcript the artifact stands
// for, or false when another copy of the same transcript is the one read
// or the file cannot be read (which adds its problem). The copies, in order
// of preference: transcript_full.jsonl; its chunks, concatenated, so a line
// cut at a chunk boundary is whole again; transcript.jsonl; its chunks. A
// chunked form is read through its first chunk present.
func brainCopy(artifact *input.Artifact, form, chunkForm string, listed *chunkListings, work parser.Work,
	problems *parser.Problems) ([]byte, bool) {
	logs := brainLogs{artifact: artifact, listed: listed, work: work}
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
	blob, ok := brainCopy(artifact, match[3], match[4], p.chunks, p.Work, &problems)
	if ok {
		lines := &brainLines{owner: p, artifact: artifact, sessionID: sessionID, options: options, emit: emit,
			problems: &problems}
		for i, raw := range bytes.Split(blob, []byte("\n")) {
			stripped := bytes.Trim(raw, " \t\n\r\v\f")
			if len(stripped) == 0 {
				continue
			}
			record, err := jsonvalue.DecodeString(textlines.ValidUTF8(stripped))
			if err == nil && !record.IsObject() {
				err = errNotObject
			}
			if err != nil {
				problems.AddRaw(i+1, parser.UnparseableLine(err), parser.LineText(raw))
				continue
			}
			lines.record(i+1, record)
		}
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, ""))
}

// brainLines renders the lines of one brain transcript as the database
// reader renders the same step kinds, adding to problems the members of
// the wrong kind it meets.
type brainLines struct {
	owner     *Parser
	artifact  *input.Artifact
	sessionID string
	options   parser.Options
	emit      parser.Emit
	calls     []jsonvalue.Value // the last reply's tool calls not yet matched to a result
	problems  *parser.Problems
	// truncated holds the field names the current line's truncated_fields
	// lists, read once per line, since a line gives a row per tool call.
	truncated map[string]bool
}

// row emits a row of a brain line; text whose field the line marks as
// truncated says so.
func (b *brainLines) row(number int, record jsonvalue.Value, turnType, text, field string) timeline.Row {
	row := b.owner.BaseRow(b.artifact)
	row.SourceLine, row.SessionID, row.TurnType = number, b.sessionID, turnType
	row.TimestampUTC = timeutil.FromValue(record.Get("created_at"))
	if b.truncated[field] {
		text += " [truncated: " + field + "]"
	}
	row.Text = timeline.Compact(text)
	return row
}

// record emits the rows of one brain line.
func (b *brainLines) record(number int, record jsonvalue.Value) {
	b.truncated = truncatedFields(record, b.owner.Work)
	kind := parser.Text(record.Get("type"))
	content := parser.TextOf(record.Get("content"))
	switch kind {
	case "USER_INPUT":
		turnType := timeline.TurnUser
		if parser.IsText(record.Get("source"), "USER_IMPLICIT") {
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
	if status := strings.ToLower(parser.Text(record.Get("status"))); status != "" && status != "done" {
		text = "[" + status + "] " + text
	}
	if kind == "SYSTEM_MESSAGE" || kind == "ERROR_MESSAGE" || len(b.calls) == 0 {
		b.emit(b.row(number, record, timeline.TurnSystem, text, "content"))
		return
	}
	call := b.calls[0]
	b.calls = b.calls[1:]
	row := b.row(number, record, timeline.TurnToolResult, text, "content")
	row.ToolName, row.ToolUseID = parser.Text(call.Get("name")), parser.Text(call.Get("id"))
	b.emit(row)
}

// truncatedFields returns the set of names a brain line's truncated_fields
// lists, each entry read once and counted as a "truncated field reads"
// unit of work.
func truncatedFields(record jsonvalue.Value, work parser.Work) map[string]bool {
	entries := record.Get("truncated_fields").Items()
	work.Add("truncated field reads", len(entries))
	names := make(map[string]bool, len(entries))
	for _, entry := range entries {
		names[entry.Text()] = true
	}
	return names
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
			failure = parser.FirstNonEmpty(failure.Get("short_error"), failure.Get("user_error_message"),
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

// reply emits the rows of a reply: its reasoning when asked for, its text
// and its tool calls, which the next lines' results answer in order. A
// tool_calls that is not an array adds a problem.
func (b *brainLines) reply(number int, record jsonvalue.Value, content string) {
	if thinking := parser.TextOf(record.Get("thinking")); thinking != "" && b.options.IncludeThinking {
		b.emit(b.row(number, record, timeline.TurnThinking, thinking, "thinking"))
	}
	if content != "" {
		b.emit(b.row(number, record, timeline.TurnAssistant, content, "content"))
	}
	b.calls = nil
	for _, raw := range parser.ArrayMember(record.Get("tool_calls"), "tool_calls", number, b.problems) {
		call := raw
		if !call.IsObject() {
			call = jsonvalue.NewObject()
		}
		b.calls = append(b.calls, call)
		row := b.row(number, record, timeline.TurnToolUse, toolArgsSummary(brainArgs(call)), "tool_calls")
		row.ToolName, row.ToolUseID = parser.Text(call.Get("name")), parser.Text(call.Get("id"))
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
