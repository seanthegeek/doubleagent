package goose

import (
	"cmp"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// storedJSON is a decoded JSON column of sessions.db: the value (JSON null
// for a NULL or empty column), and for one that does not decode, or
// decodes to another kind than Goose writes there, the reason and the
// stored text in full.
type storedJSON struct {
	value       jsonvalue.Value
	reason, raw string
}

// readStored decodes a JSON column that should hold an array when array is
// set, else an object; a blob is read as UTF-8 with replacement, and JSON
// null is accepted.
func readStored(column sqlitefile.Value, array bool) storedJSON {
	var text string
	switch column.Type {
	case sqlitefile.Null:
		return storedJSON{}
	case sqlitefile.Integer, sqlitefile.Float:
		return storedJSON{reason: "not JSON text", raw: column.String()}
	case sqlitefile.Blob:
		text = textlines.ValidUTF8(column.Bytes)
	default:
		text = column.Text()
	}
	if len(column.Bytes) == 0 {
		return storedJSON{}
	}
	decoded, err := jsonvalue.DecodeString(text)
	switch {
	case err != nil:
		return storedJSON{reason: err.Error(), raw: text}
	case decoded.IsNull():
	case array && !decoded.IsArray():
		return storedJSON{reason: "not a JSON array", raw: text}
	case !array && !decoded.IsObject():
		return storedJSON{reason: "not a JSON object", raw: text}
	}
	return storedJSON{value: decoded, raw: text}
}

// epochColumn returns a created_timestamp column as epochNumber reads a
// number, text as timeutil reads it.
func epochColumn(value sqlitefile.Value) string {
	switch value.Type {
	case sqlitefile.Integer:
		if value.Int > 10_000_000_000 {
			return timeutil.FromEpoch(float64(value.Int) / 1000.0)
		}
	case sqlitefile.Float:
		return epochNumber(value.Float)
	}
	return parser.ColumnTime(value)
}

// database reads one sessions.db: the first stored row of each session id,
// the model tracked per session across sessions and messages, each
// session's model, and the group of each session id stored more than once.
type database struct {
	parser   *Parser
	artifact *input.Artifact
	options  parser.Options
	handle   *sqlitefile.Database
	sessions map[string]sqlitefile.Row
	models   parser.Tracker
	modes    parser.Tracker
	byModel  map[string]string
	repeats  map[string]*sessionRepeat
	emit     parser.Emit
}

// sessionRepeat is a session id the sessions table holds in more than one
// row, which only damage or an edit writes, since the id is the table's
// key. Its rows are a group (timeline.Group): each stored row is a parent,
// giving its own rows and a `parser:` row saying so with its own project
// and model; the session's messages are members, shown once with the
// project, and the model where they name none, of the row the group goes
// with. The model rows of the stored rows and of the messages are not
// chained: each stored row's reads from its own model, and the messages'
// first from none, since which stored row the messages follow is settled
// only once the output's rows are known.
type sessionRepeat struct {
	group *timeline.Group
	count int
}

// parseDatabase emits the rows of sessions.db: each session's rows, then
// each message's. A database error ends the rows with one `parser:` row;
// the rows before it stay.
func (p *Parser) parseDatabase(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	problem := func(message string) {
		row := p.row(artifact, 0, "", "", "", "")
		row.TurnType, row.Text = timeline.TurnSystem, parser.ProblemPrefix+message
		emit(row)
	}
	handle, err := parser.OpenDatabase(artifact.DiskPath)
	if err != nil {
		problem(parser.DatabaseError(err))
		return
	}
	defer handle.Close()
	reader := &database{parser: p, artifact: artifact, options: options, handle: handle,
		sessions: map[string]sqlitefile.Row{}, models: parser.Tracker{}, modes: parser.Tracker{},
		byModel: map[string]string{}, repeats: map[string]*sessionRepeat{}, emit: emit}
	if err := reader.rows(); err != nil {
		problem(parser.DatabaseError(err))
	}
}

// rows emits the database's rows, stopping at the first error.
func (d *database) rows() error {
	tables, err := parser.TablesOf(d.handle)
	if err != nil || !tables.Has("messages") {
		return err
	}
	if tables.Has("sessions") {
		if err := d.readSessions(); err != nil {
			return err
		}
	}
	answered, err := d.answered()
	if err != nil {
		return err
	}
	table, err := d.handle.Table("messages")
	if err != nil {
		return err
	}
	reader := &blockReader{parser: d.parser, artifact: d.artifact, options: d.options,
		toolNames: map[string]string{}}
	return table.Select().OrderBy("session_id", "created_timestamp", "id").
		Each(func(message sqlitefile.Row) error {
			d.message(message, reader, answered)
			return nil
		})
}

// readSessions emits the rows of the sessions table by created_at, each
// session id stored more than once a group (sessionRepeat) whose first
// parent is its first row.
func (d *database) readSessions() error {
	table, err := d.handle.Table("sessions")
	if err != nil {
		return err
	}
	sessions, err := table.Select().OrderBy("created_at", "rowid").All()
	if err != nil {
		return err
	}
	counts := map[string]int{}
	for _, session := range sessions {
		counts[parser.NonEmptyColumn(session.Get("id"))]++
	}
	for _, session := range sessions {
		sessionID := parser.NonEmptyColumn(session.Get("id"))
		if counts[sessionID] > 1 && d.repeats[sessionID] == nil {
			d.repeats[sessionID] = &sessionRepeat{group: timeline.NewGroup(), count: counts[sessionID]}
		}
		d.session(session, d.repeats[sessionID])
	}
	return nil
}

// session emits a session's start row, its bad-config row, and its relay,
// model and permission mode rows. A stored row of a repeated id (repeat
// not nil) is a parent of the id's group, with a `parser:` row saying so,
// and its model and mode rows read from its own values alone.
func (d *database) session(session sqlitefile.Row, repeat *sessionRepeat) {
	rowid := int(session.Rowid)
	config := readStored(session.Get("model_config_json"), false)
	var problems parser.Problems
	if config.reason != "" {
		problems.AddRaw(rowid, "unparseable record "+strconv.Itoa(rowid)+" (sessions.model_config_json: "+
			config.reason+")", config.raw)
	}
	modelValue := ""
	if config.value.IsObject() {
		modelValue = parser.Text(config.value.Get("model_name"))
	}
	sessionID := parser.NonEmptyColumn(session.Get("id"))
	provider := parser.NonEmptyColumn(session.Get("provider_name"))
	if _, met := d.sessions[sessionID]; !met {
		d.byModel[sessionID] = modelName(provider, modelValue)
		d.sessions[sessionID] = session
	}
	workingDir := parser.NonEmptyColumn(session.Get("working_dir"))
	row := d.parser.row(d.artifact, rowid, sessionID, workingDir, modelName(provider, modelValue),
		parser.ColumnTime(session.Get("created_at")))
	row.TurnType = timeline.TurnSystem
	row.Text = timeline.Compact(parser.JoinNonEmpty(" | ", "session start", parser.NonEmptyColumn(session.Get("name")),
		parser.NonEmptyColumn(session.Get("description")), labelled("type=", session.Get("session_type")),
		labelled("parent=", session.Get("parent_session_id")),
		labelled("schedule=", session.Get("schedule_id")),
		labelled("archived=", session.Get("archived_at"))))
	models, modes := d.models, d.modes
	if repeat != nil {
		models, modes = parser.Tracker{}, parser.Tracker{}
		repeat.start(&row, d.emit)
	} else {
		d.emit(row)
	}
	parser.EmitAll(d.emit, d.parser.ErrorRows(d.artifact, problems, sessionID, workingDir))
	if text := d.parser.gatewayText(session); text != "" {
		d.emit(systemCopy(row, text))
	}
	if text := models.Change("model", sessionID, modelValue); text != "" {
		if provider != "" {
			text += " provider=" + provider
		}
		d.emit(systemCopy(row, text))
	}
	if text := modes.Change("permission mode", sessionID, parser.NonEmptyColumn(session.Get("goose_mode"))); text != "" {
		d.emit(systemCopy(row, text))
	}
}

// start makes start, the session start row of one stored row of the id,
// a new parent of the group and emits it, then a copy of it, the parent's
// `parser:` row.
func (r *sessionRepeat) start(start *timeline.Row, emit parser.Emit) {
	parent := r.group.Parent(start)
	emit(*start)
	notice := systemCopy(*start, "")
	prefix := parser.ProblemPrefix + "sessions holds " + strconv.Itoa(r.count) +
		" rows of this row's id; its messages are shown once, with "
	parent.Texts(&notice, timeline.RowText{Text: prefix + "this row"},
		timeline.RowText{Text: prefix + "another of them"})
	emit(notice)
}

// labelled returns label and the column's text when the column is not
// empty, else "".
func labelled(label string, value sqlitefile.Value) string {
	if value.IsEmpty() {
		return ""
	}
	return label + value.String()
}

// gatewayText returns the "session relayed:" text of a gateway session, ""
// for any other. The platform and user id come from working_dir
// (<config>/gateway/<platform>/<user_id>), else the platform from name; the
// display name comes from name (<platform>/<display name or user id>), "-"
// when it is the user id or absent.
func (p *Parser) gatewayText(session sqlitefile.Row) string {
	if kind := session.Get("session_type"); kind.Type != sqlitefile.Text || kind.Text() != "gateway" {
		return ""
	}
	platform, userID := "", ""
	if match := p.gateway.FindStringSubmatch(parser.NonEmptyColumn(session.Get("working_dir"))); match != nil {
		platform, userID = match[1], match[2]
	}
	namePlatform, display, _ := strings.Cut(parser.NonEmptyColumn(session.Get("name")), "/")
	if platform == "" {
		platform = namePlatform
	}
	if display == userID {
		display = ""
	}
	return "session relayed: gateway " + cmp.Or(platform, "-") + " user " + cmp.Or(userID, "-") + " display " +
		cmp.Or(display, "-") + " (from sessions.db session_type gateway, created_at)"
}

// answered maps each session id to the tool request ids its
// toolConfirmationResponse blocks answer. A content_json that does not
// decode is skipped here; the full read of the same row reports it.
func (d *database) answered() (map[string]map[string]bool, error) {
	answered := map[string]map[string]bool{}
	table, err := d.handle.Table("messages")
	if err != nil {
		return nil, err
	}
	err = table.Select("session_id", "content_json").Where(func(row sqlitefile.Row) bool {
		return sqlitefile.Like(row.Get("content_json"), "%toolConfirmationResponse%")
	}).Each(func(row sqlitefile.Row) error {
		content := readStored(row.Get("content_json"), true).value
		for _, block := range parser.Elements(content) {
			data := block.Get("data")
			if data.IsObject() && parser.IsText(data.Get("actionType"), "toolConfirmationResponse") {
				sessionID := parser.NonEmptyColumn(row.Get("session_id"))
				if answered[sessionID] == nil {
					answered[sessionID] = map[string]bool{}
				}
				answered[sessionID][parser.Text(data.Get("id"))] = true
			}
		}
		return nil
	})
	return answered, err
}

// message emits a message's model change, its bad-value rows and the rows
// of its content blocks.
func (d *database) message(message sqlitefile.Row, reader *blockReader,
	answered map[string]map[string]bool) {
	sessionID := parser.NonEmptyColumn(message.Get("session_id"))
	projectPath := parser.NonEmptyColumn(d.sessions[sessionID].Get("working_dir"))
	record := message.Get("id")
	if record.IsEmpty() {
		record = sqlitefile.IntValue(0)
	}
	line := parser.ColumnLine(record)
	var problems parser.Problems
	stored := readStored(message.Get("metadata_json"), false)
	if stored.reason != "" {
		problems.AddRaw(line, "unparseable record "+record.String()+" (messages.metadata_json: "+
			stored.reason+")", stored.raw)
	}
	meta := stored.value
	if !meta.IsObject() {
		meta = jsonvalue.NewObject()
	}
	base := d.parser.row(d.artifact, line, sessionID, projectPath, "",
		timeutil.FirstNonEmpty(epochColumn(message.Get("created_timestamp")),
			parser.ColumnTime(message.Get("timestamp"))))
	inference := orObject(meta.Get("inference"))
	repeat := d.repeats[sessionID]
	if repeat != nil {
		takes := timeline.TakeProject
		if inferredModel(inference) == "" {
			takes |= timeline.TakeModel
		}
		repeat.group.Member(&base, takes)
	}
	base.Model = d.messageModel(base, inference)
	d.emitProblems(problems, sessionID, projectPath)
	content := readStored(message.Get("content_json"), true)
	if content.value.IsNull() && content.reason == "" {
		content.reason = "empty"
		if content.raw != "" {
			content.reason = "not a JSON array"
		}
	}
	if content.reason != "" {
		var bad parser.Problems
		bad.AddRaw(line, "unparseable record "+record.String()+" (messages.content_json: "+
			content.reason+")", content.raw)
		d.emitProblems(bad, sessionID, projectPath)
		return
	}
	var wrong parser.Problems
	reader.blocks(base, parser.NonEmptyColumn(message.Get("role")), content.value, userVisible(meta),
		answeredOf(answered, sessionID), &wrong, d.emit)
	d.emitProblems(wrong, sessionID, projectPath)
}

// emitProblems emits the problem rows of a message; those of a repeated
// session's message are members of its group, taking the project of the
// stored row the group goes with.
func (d *database) emitProblems(problems parser.Problems, sessionID, projectPath string) {
	repeat := d.repeats[sessionID]
	for _, row := range d.parser.ErrorRows(d.artifact, problems, sessionID, projectPath) {
		if repeat != nil {
			repeat.group.Member(&row, timeline.TakeProject)
		}
		d.emit(row)
	}
}

// inferredModel returns the model a message's inference metadata names,
// the resolved one else the requested one, after its provider.
func inferredModel(inference jsonvalue.Value) string {
	resolved := parser.Text(inference.Get("resolvedModel"))
	if resolved == "" {
		resolved = parser.Text(inference.Get("requestedModel"))
	}
	return modelName(parser.Text(inference.Get("provider")), resolved)
}

// messageModel returns the model of a message from its inference
// metadata, else its session's, and emits the model row of a change.
func (d *database) messageModel(base timeline.Row, inference jsonvalue.Value) string {
	resolved := parser.Text(inference.Get("resolvedModel"))
	if resolved == "" {
		resolved = parser.Text(inference.Get("requestedModel"))
	}
	provider := parser.Text(inference.Get("provider"))
	base.Model = modelName(provider, resolved)
	if base.Model == "" {
		base.Model = d.byModel[base.SessionID]
	}
	suffix := ""
	if provider != "" {
		suffix = " provider=" + provider
	}
	if requested := parser.Text(inference.Get("requestedModel")); requested != "" && requested != resolved {
		suffix += " requested=" + requested
	}
	if text := d.models.Change("model", base.SessionID, resolved); text != "" {
		d.emit(systemCopy(base, text+suffix))
	}
	return base.Model
}

// answeredOf returns the answered ids of a session, an empty set when it
// has none.
func answeredOf(answered map[string]map[string]bool, sessionID string) map[string]bool {
	if ids := answered[sessionID]; ids != nil {
		return ids
	}
	return map[string]bool{}
}
