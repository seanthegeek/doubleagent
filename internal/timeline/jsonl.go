package timeline

import (
	"bytes"
	"io"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// The line and paragraph separators, which the writers escape, and their
// escapes.
const (
	lineSeparator            = "\U00002028"
	paragraphSeparator       = "\U00002029"
	lineSeparatorEscape      = `\u2028`
	paragraphSeparatorEscape = `\u2029`
)

// WriteRows writes rows to w as timeline.jsonl lines, one write per row.
func WriteRows(w io.Writer, rows []Row) error {
	var line []byte
	for i := range rows {
		line = AppendRow(line[:0], &rows[i])
		if _, err := w.Write(line); err != nil {
			return err
		}
	}
	return nil
}

// WriteSessions writes summaries to w as sessions.jsonl lines, one write
// per summary.
func WriteSessions(w io.Writer, summaries []SessionSummary) error {
	var line []byte
	for i := range summaries {
		line = AppendSession(line[:0], &summaries[i])
		if _, err := w.Write(line); err != nil {
			return err
		}
	}
	return nil
}

// AppendRow appends row to out as one line of timeline.jsonl, ending in LF.
func AppendRow(out []byte, row *Row) []byte {
	record := newRecord(out, row.TimestampUTC, row.Host, row.User, row.Agent, row.SessionID,
		row.ProjectPath, row.GitBranch, row.TurnType, row.Model, row.ToolName, row.ToolUseID,
		row.Text, row.SourceFile, row.Recovered)
	record.text("timestamp_utc", row.TimestampUTC)
	record.text("host", row.Host)
	record.text("user", row.User)
	record.text("agent", row.Agent)
	record.text("session_id", row.SessionID)
	record.text("project_path", row.ProjectPath)
	record.text("git_branch", row.GitBranch)
	record.text("turn_type", row.TurnType)
	record.text("model", row.Model)
	record.text("tool_name", row.ToolName)
	record.text("tool_use_id", row.ToolUseID)
	record.text("text", row.Text)
	record.text("source_file", row.SourceFile)
	record.integer("source_line", row.SourceLine)
	record.text("recovered", row.Recovered)
	return record.finish()
}

// AppendSession appends summary to out as one line of sessions.jsonl,
// ending in LF.
func AppendSession(out []byte, summary *SessionSummary) []byte {
	record := newRecord(out, summary.Host, summary.User, summary.Agent, summary.SessionID,
		summary.ProjectPath, summary.FirstTimestampUTC, summary.LastTimestampUTC,
		summary.SourceFile, summary.Synced, summary.Relayed, summary.Origin, summary.Recovered)
	for _, model := range summary.Models {
		record.format.ASCII = record.format.ASCII || pytext.HasLoneSurrogate(model)
	}
	record.text("host", summary.Host)
	record.text("user", summary.User)
	record.text("agent", summary.Agent)
	record.text("session_id", summary.SessionID)
	record.text("project_path", summary.ProjectPath)
	record.text("first_timestamp_utc", summary.FirstTimestampUTC)
	record.text("last_timestamp_utc", summary.LastTimestampUTC)
	record.list("models", summary.Models)
	record.integer("user_turns", summary.UserTurns)
	record.integer("assistant_turns", summary.AssistantTurns)
	record.integer("tool_calls", summary.ToolCalls)
	record.text("source_file", summary.SourceFile)
	record.text("synced", summary.Synced)
	record.text("relayed", summary.Relayed)
	record.text("origin", summary.Origin)
	record.text("recovered", summary.Recovered)
	return record.finish()
}

// record writes one JSONL record, which starts at offset start of out.
type record struct {
	out    []byte
	start  int
	format pyjson.Format
	fields int
}

// newRecord starts a record at the end of out. When any of texts, the
// record's strings, holds a lone surrogate, the whole record is written in
// ASCII escapes.
func newRecord(out []byte, texts ...string) record {
	format := pyjson.Format{}
	for _, text := range texts {
		format.ASCII = format.ASCII || pytext.HasLoneSurrogate(text)
	}
	return record{out: append(out, '{'), start: len(out), format: format}
}

// key appends the separator before a field and the field's key.
func (r *record) key(name string) {
	if r.fields > 0 {
		r.out = append(r.out, ',')
	}
	r.fields++
	r.out = append(append(append(r.out, '"'), name...), '"', ':')
}

// text appends a string field.
func (r *record) text(name, value string) {
	r.key(name)
	r.out = r.format.AppendString(r.out, value)
}

// integer appends an integer field.
func (r *record) integer(name string, value int) {
	r.key(name)
	r.out = strconv.AppendInt(r.out, int64(value), 10)
}

// list appends a field holding an array of strings.
func (r *record) list(name string, values []string) {
	r.key(name)
	r.out = append(r.out, '[')
	for i, value := range values {
		if i > 0 {
			r.out = append(r.out, ',')
		}
		r.out = r.format.AppendString(r.out, value)
	}
	r.out = append(r.out, ']')
}

// finish closes the record, escapes the line and paragraph separators that
// UTF-8 output leaves raw, and appends the line's LF.
func (r *record) finish() []byte {
	r.out = append(r.out, '}')
	body := r.out[r.start:]
	if bytes.Contains(body, []byte(lineSeparator)) ||
		bytes.Contains(body, []byte(paragraphSeparator)) {
		body = bytes.ReplaceAll(body, []byte(lineSeparator), []byte(lineSeparatorEscape))
		body = bytes.ReplaceAll(body, []byte(paragraphSeparator),
			[]byte(paragraphSeparatorEscape))
		r.out = append(r.out[:r.start], body...)
	}
	return append(r.out, '\n')
}
