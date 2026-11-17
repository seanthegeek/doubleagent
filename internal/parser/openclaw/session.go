package openclaw

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/pi"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// attachedRow is a row of a table read before the transcripts, waiting for
// its generation's session-start row.
type attachedRow struct {
	line                   int
	timestamp, text, model string
}

// databaseState is what the passes over one agent database share: the rows
// to attach after a generation's session-start row, the transcript entry
// ids already given (for the exact joins of the full-text index and the
// pending inputs), and each generation's working directory.
type databaseState struct {
	attached map[string][]attachedRow
	// order lists the generations with attached rows, first attach first.
	order []string
	ids   map[[2]string]bool
	cwds  map[string]string
}

// newDatabaseState returns an empty state.
func newDatabaseState() *databaseState {
	return &databaseState{attached: map[string][]attachedRow{}, ids: map[[2]string]bool{},
		cwds: map[string]string{}}
}

// attach adds a row to a generation's waiting rows.
func (d *databaseState) attach(sessionID string, line int, timestamp, text, model string) {
	if _, ok := d.attached[sessionID]; !ok {
		d.order = append(d.order, sessionID)
	}
	d.attached[sessionID] = append(d.attached[sessionID], attachedRow{line, timestamp, text, model})
}

// pop returns and forgets a generation's waiting rows.
func (d *databaseState) pop(sessionID string) []attachedRow {
	rows := d.attached[sessionID]
	delete(d.attached, sessionID)
	return rows
}

// session is the state carried across one transcript's entries.
type session struct {
	sessionID string
	key       string
	extra     string
	model     string
	provider  string
	cwd       string
	// headerID is set when the header's id replaces the session id.
	headerID bool
	started  bool
	// seen is the model in use, for the `model:` rows.
	seen parser.Tracker
	// store is the agent database's shared state, nil for a file; the
	// session-start row takes the rows attached to its generation.
	store *databaseState
}

// newSession returns the state of a transcript of the given session id.
func newSession(sessionID string) *session {
	return &session{sessionID: sessionID, seen: parser.Tracker{}}
}

// writer emits the rows of one artifact's transcripts.
type writer struct {
	*Parser
	artifact *input.Artifact
	options  parser.Options
	emit     parser.Emit
}

// row returns a row of the session with the text compacted.
func (w *writer) row(current *session, line int, timestamp, turnType, text string) timeline.Row {
	row := w.BaseRow(w.artifact)
	row.SourceLine = line
	row.SessionID = current.sessionID
	row.ProjectPath = current.cwd
	row.TimestampUTC = timestamp
	row.TurnType = turnType
	row.Text = timeline.Compact(text)
	return row
}

// system emits a system row of the session with the given model.
func (w *writer) system(current *session, line int, timestamp, text, model string) {
	row := w.row(current, line, timestamp, timeline.TurnSystem, text)
	row.Model = model
	w.emit(row)
}

// bad emits the `parser:` row of a problem inside a transcript.
func (w *writer) bad(current *session, line int, text, timestamp string) {
	w.emit(w.row(current, line, timestamp, timeline.TurnSystem, parser.ProblemPrefix+text))
}

// start emits the session-start row, then a `model:` row when the window
// names the model, then the rows attached to the generation.
func (w *writer) start(current *session, line int, timestamp string, header pyjson.Value) {
	current.started = true
	if identifier := header.Get("id"); identifier.Truthy() && (current.headerID || current.sessionID == "") {
		current.sessionID = identifier.Str()
	}
	if cwd := header.Get("cwd"); cwd.IsString() {
		current.cwd = cwd.Text()
	}
	parts := []string{"session start:"}
	if current.key != "" {
		parts = append(parts, "key="+current.key)
	}
	if current.extra != "" {
		parts = append(parts, current.extra)
	}
	if version, ok := header.Lookup("version"); ok && !version.IsNull() {
		parts = append(parts, "version="+version.Str())
	}
	if parent := header.Get("parentSession"); parent.Truthy() {
		parts = append(parts, "parent_session="+parent.Str())
	}
	timestamp = timeutil.FirstNonEmpty(timestamp, timeutil.FromValue(header.Get("timestamp")))
	w.system(current, line, timestamp, strings.Join(parts, " "), current.model)
	if text := pi.ModelText(current.seen, current.sessionID, current.model, current.provider, ""); text != "" {
		w.system(current, line, timestamp, text, current.model)
	}
	if current.store == nil {
		return
	}
	if _, ok := current.store.cwds[current.sessionID]; !ok {
		current.store.cwds[current.sessionID] = current.cwd
	}
	for _, attached := range current.store.pop(current.sessionID) {
		w.system(current, attached.line, attached.timestamp, attached.text, attached.model)
	}
}

// entry emits the rows of one transcript entry. createdAt is the time of
// the database row that held it, "" for a file.
func (w *writer) entry(current *session, line int, record pyjson.Value, createdAt string) {
	entryType := record.Get("type")
	message := record.Get("message")
	if !message.IsObject() {
		message = pyjson.NewObject()
	}
	timestamp := timeutil.FirstNonEmpty(timeutil.FromValue(record.Get("timestamp")),
		timeutil.FromValue(message.Get("timestamp")), createdAt)
	if isText(entryType, "session") {
		w.start(current, line, timestamp, record)
		return
	}
	if !current.started {
		w.start(current, line, timestamp, pyjson.NewObject())
	}
	if identifier := record.Get("id"); current.store != nil && !identifier.IsNull() && !isText(identifier, "") {
		current.store.ids[[2]string{current.sessionID, identifier.Str()}] = true
	}
	if isText(entryType, "message") {
		w.message(current, line, timestamp, record, message)
		return
	}
	if isText(entryType, "model_change") {
		current.model = parser.Str(record.Get("modelId"))
		text := pi.SwitchText(current.seen, current.sessionID, current.model, parser.Str(record.Get("provider")))
		if text != "" {
			w.system(current, line, timestamp, text, current.model)
		}
		return
	}
	w.system(current, line, timestamp, entryText(entryType, record), "")
}

// entryText returns the text of the system row of an entry that is neither
// a message nor a model change.
func entryText(entryType, record pyjson.Value) string {
	field := func(key string) string { return parser.Str(record.Get(key)) }
	switch {
	case isText(entryType, "thinking_level_change"):
		return "thinking level: " + field("thinkingLevel")
	case isText(entryType, "compaction"):
		var tokens []string
		for _, pair := range [][2]string{{"tokens_before", "tokensBefore"}, {"tokens_after", "tokensAfter"}} {
			if value, ok := record.Lookup(pair[1]); ok && !value.IsNull() {
				tokens = append(tokens, pair[0]+"="+value.Str())
			}
		}
		label := "compaction"
		if len(tokens) > 0 {
			label += " (" + strings.Join(tokens, " ") + ")"
		}
		return label + ": " + field("summary")
	case isText(entryType, "reset"):
		return "reset: " + field("reason")
	case isText(entryType, "branch_summary"):
		return "branch summary: " + field("summary")
	case isText(entryType, "custom"):
		return "custom " + field("customType") + ": " + parser.CompactJSON(record.Get("data"))
	case isText(entryType, "custom_message"):
		return "custom message " + field("customType") + ": " + parser.TextOf(record.Get("content"))
	case isText(entryType, "label"):
		return "label: " + field("label") + " (entry " + field("targetId") + ")"
	case isText(entryType, "session_info"):
		return "session name: " + field("name")
	}
	var rest []pyjson.Member
	for _, member := range record.Members() {
		switch member.Key {
		case "type", "id", "parentId", "timestamp":
		default:
			rest = append(rest, member)
		}
	}
	return orText(parser.Str(entryType), "entry") + ": " + parser.CompactJSON(pyjson.NewObject(rest...))
}

// isText reports whether value is the string text.
func isText(value pyjson.Value, text string) bool {
	return value.IsString() && value.Text() == text
}

// orText returns text, or fallback when text is empty.
func orText(text, fallback string) string {
	if text == "" {
		return fallback
	}
	return text
}
