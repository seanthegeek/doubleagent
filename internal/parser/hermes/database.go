package hermes

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// retiredPrefix starts the state_meta key of a session's deletion fence.
const retiredPrefix = "gateway.retired_session.v1."

// messagingSources are the session sources that are messaging platforms:
// every gateway platform but local (the CLI) and api_server (the
// OpenAI-compatible HTTP API).
func messagingSources() []string {
	return []string{"telegram", "discord", "whatsapp", "whatsapp_cloud", "slack", "signal",
		"mattermost", "matrix", "email", "sms", "dingtalk", "webhook", "msgraph_webhook", "feishu",
		"wecom", "wecom_callback", "weixin", "bluebubbles", "qqbot", "yuanbao", "relay"}
}

// columnValue returns a column value as the decoded value of the same
// type: a blob as the text Python's str() gives it.
func columnValue(value sqlitefile.Value) jsonvalue.Value {
	switch value.Type {
	case sqlitefile.Integer:
		return jsonvalue.NewInt(value.Int)
	case sqlitefile.Float:
		return jsonvalue.NewFloat(value.Float)
	case sqlitefile.Text, sqlitefile.Blob:
		return jsonvalue.NewString(value.Str())
	}
	return jsonvalue.NewNull()
}

// rowObject returns a row as Python's dict of it: the rowid as _rowid when
// selected, then every column by name.
func rowObject(row sqlitefile.Row) jsonvalue.Value {
	var members []jsonvalue.Member
	for i, column := range row.Columns() {
		key := column
		if i == 0 && column == "rowid" {
			key = "_rowid"
		}
		members = append(members, jsonvalue.Member{Key: key, Value: columnValue(row.Values()[i])})
	}
	return jsonvalue.NewObject(members...)
}

// recordError is a problem of one stored record, reported after the file's
// other rows under its session and project.
type recordError struct {
	sessionID, project string
	problem            parser.Problem
}

// stateReader reads one state.db.
type stateReader struct {
	parser   *Parser
	artifact *input.Artifact
	handle   *sqlitefile.Database
	tables   map[string]bool
	sessions map[string]jsonvalue.Value
	usage    map[string][]jsonvalue.Value
	errors   []recordError
	emit     parser.Emit
}

// parseDatabase emits the rows of state.db or a snapshot of it: sessions,
// messages, deletion fences, relay peers and room events, then the records
// that do not decode. A database error ends the rows with one `parser:`
// row; the rows before it stay.
func (p *Parser) parseDatabase(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	handle, message := parser.OpenDatabase(artifact.DiskPath)
	if message != "" {
		emit(p.system(artifact, 0, noSession(), parser.ProblemPrefix+message))
		return
	}
	defer handle.Close()
	reader := &stateReader{parser: p, artifact: artifact, handle: handle,
		sessions: map[string]jsonvalue.Value{}, emit: emit}
	if err := reader.rows(options); err != nil {
		emit(p.system(artifact, 0, noSession(), parser.ProblemPrefix+parser.DatabaseError(err)))
	}
}

// each runs visit over the rows of table as objects, with the rowid first
// when withRowid is set, filtered by keep when not nil, in the order.
func (s *stateReader) each(table string, withRowid bool, keep func(sqlitefile.Row) bool,
	order []string, visit func(jsonvalue.Value) error) error {
	handle, err := s.handle.Table(table)
	if err != nil {
		return err
	}
	var columns []string
	if withRowid {
		columns = append(columns, "rowid")
	}
	for _, column := range handle.Columns() {
		columns = append(columns, column.Name)
	}
	query := handle.Select(columns...)
	if keep != nil {
		query = query.Where(keep)
	}
	return query.OrderBy(order...).Each(func(row sqlitefile.Row) error { return visit(rowObject(row)) })
}

// rows emits the database's rows, stopping at the first error.
func (s *stateReader) rows(options parser.Options) error {
	var err error
	if s.tables, err = s.handle.TableNames(); err != nil || !s.tables["messages"] {
		return err
	}
	if err := s.readUsage(); err != nil {
		return err
	}
	if s.tables["sessions"] {
		seen := parser.Tracker{}
		err := s.each("sessions", true, nil, []string{"started_at", "rowid"}, func(session jsonvalue.Value) error {
			s.sessions[parser.Str(session.Get("id"))] = session
			s.sessionRows(session, seen)
			return nil
		})
		if err != nil {
			return err
		}
	}
	reader := s.parser.newMessageReader(s.artifact, options, s.usage, s.emit)
	err = s.each("messages", false, nil, []string{"timestamp", "id"}, func(message jsonvalue.Value) error {
		sessionID := parser.Str(message.Get("session_id"))
		session, ok := s.sessions[sessionID]
		if !ok {
			session = sessionOf(sessionID)
		}
		reader.message(message, session, either(message.Get("id"), jsonvalue.NewInt(0)))
		return nil
	})
	if err != nil {
		return err
	}
	if err := s.tableRows(); err != nil {
		return err
	}
	for _, entry := range s.errors {
		parser.EmitAll(s.emit, s.parser.ErrorRows(s.artifact, parser.Problems{entry.problem},
			entry.sessionID, entry.project))
	}
	return nil
}

// tableRows emits the rows of the deletion fences, the relay peers and the
// room events, for the tables the database has.
func (s *stateReader) tableRows() error {
	if s.tables["state_meta"] {
		if err := s.retiredRows(); err != nil {
			return err
		}
	}
	if s.tables["conversation_generations"] {
		if err := s.generationRows(); err != nil {
			return err
		}
	}
	if s.tables["hosted_room_events"] {
		return s.roomRows()
	}
	return nil
}

// readUsage reads the main-loop session_model_usage rows per session, by
// first_seen; a row with a task is a side task's and is skipped.
func (s *stateReader) readUsage() error {
	s.usage = map[string][]jsonvalue.Value{}
	if !s.tables["session_model_usage"] {
		return nil
	}
	return s.each("session_model_usage", true, nil, []string{"first_seen", "rowid"}, func(usage jsonvalue.Value) error {
		if usage.Get("task").IsEmpty() {
			sessionID := parser.Str(usage.Get("session_id"))
			s.usage[sessionID] = append(s.usage[sessionID], usage)
		}
		return nil
	})
}

// addError files a record problem under a session and project.
func (s *stateReader) addError(sessionID, project string, line jsonvalue.Value, message, raw string) {
	s.errors = append(s.errors, recordError{sessionID: sessionID, project: project,
		problem: parser.Problem{Line: lineNumber(line), Message: message, Raw: raw}})
}

// columnJSON returns sessions.<column> decoded as a JSON object, an empty
// object when the column is empty. A value that does not decode to an
// object files a record problem with the value as stored and gives an
// empty object.
func (s *stateReader) columnJSON(session jsonvalue.Value, column string) jsonvalue.Value {
	raw := session.Get(column)
	if raw.IsNull() || raw.IsString() && raw.StringValue() == "" {
		return jsonvalue.NewObject()
	}
	sessionID, project := parser.Str(session.Get("id")), projectOf(session)
	rowid := session.Get("_rowid")
	label := "unparseable record " + rowid.Text() + " (sessions." + column
	value := raw
	if raw.IsString() {
		decoded, err := jsonvalue.DecodeString(raw.StringValue())
		if err != nil {
			s.addError(sessionID, project, rowid, label+": "+err.Error()+")", raw.StringValue())
			return jsonvalue.NewObject()
		}
		value = decoded
	}
	if !value.IsObject() {
		s.addError(sessionID, project, rowid, label+": not a JSON object)", raw.Text())
		return jsonvalue.NewObject()
	}
	return value
}

// archivedValue reports whether sessions.archived is 1, "1" or true.
func archivedValue(value jsonvalue.Value) bool {
	if number, ok := numberValue(value); ok {
		return number == 1
	}
	return isText(value, "1")
}

// sessionRows emits a session's start row, its model rows, and its relay,
// yolo-mode and worktree rows.
func (s *stateReader) sessionRows(session jsonvalue.Value, seen parser.Tracker) {
	origin, config := s.columnJSON(session, "origin_json"), s.columnJSON(session, "model_config")
	imported := ""
	if from := origin.Get("imported_from"); from.IsObject() {
		imported = "imported from " + dash(from.Get("tool")) + " " + dash(from.Get("path")) +
			" session " + dash(from.Get("foreign_session_id"))
	}
	archived := ""
	if archivedValue(session.Get("archived")) {
		archived = "archived=1"
	}
	start := s.parser.system(s.artifact, lineNumber(session.Get("_rowid")), session,
		timeline.Compact(joinParts("session start", parser.Str(session.Get("title")),
			labelled("source=", session.Get("source")),
			labelled("parent=", session.Get("parent_session_id")),
			labelled("end=", session.Get("end_reason")), archived, imported)))
	start.TimestampUTC = timeutil.FromValue(session.Get("started_at"))
	s.emit(start)
	s.modelRows(session, start, seen)
	source := parser.Str(session.Get("source"))
	if slices.Contains(messagingSources(), source) ||
		!session.Get("chat_id").IsEmpty() && source != "local" && source != "api_server" {
		s.emit(copyRow(start, "session relayed: "+source+" chat "+dash(session.Get("chat_id"))+
			" type "+dash(session.Get("chat_type"))+" thread "+dash(session.Get("thread_id"))+
			" user "+dash(session.Get("user_id"))+" name "+dash(origin.Get("user_name"))+
			" display "+dash(either(session.Get("display_name"), origin.Get("chat_name")))+
			" (from state.db sessions, started_at)", start.Model))
	}
	if yolo, ok := config.Lookup("yolo_mode"); ok {
		s.emit(copyRow(start, "permission setting: yolo_mode="+parser.SettingValue(yolo)+
			" (from state.db sessions.model_config, started_at)", start.Model))
	}
	cwd, branch := parser.Str(session.Get("cwd")), parser.Str(session.Get("git_branch"))
	if tree := s.parser.worktree.FindStringSubmatch(cwd); tree != nil &&
		(strings.HasPrefix(branch, "hermes/") || strings.HasPrefix(branch, "hermes-subagent/")) {
		s.emit(copyRow(start, "scratch directory: "+cwd+" (git worktree the agent created in "+
			tree[1]+", branch "+branch+")", start.Model))
	}
}

// labelled returns label and the value's text when the value is truthy,
// else "".
func labelled(label string, value jsonvalue.Value) string {
	if value.IsEmpty() {
		return ""
	}
	return label + value.Text()
}

// modelRows emits a session's model rows: one per main-loop usage model
// that changes it, at the usage's first_seen, else one for sessions.model.
func (s *stateReader) modelRows(session jsonvalue.Value, start timeline.Row, seen parser.Tracker) {
	uses := s.usage[start.SessionID]
	if len(uses) == 0 {
		model := parser.Str(session.Get("model"))
		if text := seen.Change("model", start.SessionID, model); text != "" {
			s.emit(copyRow(start, text, model))
		}
		return
	}
	for _, usage := range uses {
		model := parser.Str(usage.Get("model"))
		text := seen.Change("model", start.SessionID, model)
		if text == "" {
			continue
		}
		if provider := usage.Get("billing_provider"); !provider.IsEmpty() {
			text += " provider=" + provider.Text()
		}
		row := copyRow(start, text, model)
		row.SourceLine = lineNumber(usage.Get("_rowid"))
		row.TimestampUTC = timeutil.FirstNonEmpty(timeutil.FromValue(usage.Get("first_seen")),
			start.TimestampUTC)
		s.emit(row)
	}
}

// jsonText decodes a column read as Python's json.loads(value or "") reads
// it: the reason is the decoder's error, or "not JSON text" for a number,
// which is not text to decode.
func jsonText(value jsonvalue.Value) (jsonvalue.Value, string) {
	if value.IsEmpty() {
		value = jsonvalue.NewString("")
	}
	if !value.IsString() {
		return jsonvalue.Value{}, "not JSON text"
	}
	decoded, err := jsonvalue.DecodeString(value.StringValue())
	if err != nil {
		return jsonvalue.Value{}, err.Error()
	}
	return decoded, ""
}

// orEmpty returns str(value or "").
func orEmpty(value jsonvalue.Value) string {
	return parser.Str(value)
}

// retiredRows emits one "session deleted:" row per state_meta deletion
// fence (key gateway.retired_session.v1.<session id>, matched with LIKE as
// the analyzer's query does), at its retired_at; " sweep" follows the id
// when a sweep, not a delete, retired it.
func (s *stateReader) retiredRows() error {
	table, err := s.handle.Table("state_meta")
	if err != nil {
		return err
	}
	return table.Select("rowid", "key", "value").Where(func(row sqlitefile.Row) bool {
		return sqlitefile.Like(row.Get("key"), retiredPrefix+"%")
	}).OrderBy("rowid").Each(func(row sqlitefile.Row) error {
		record := rowObject(row)
		key := record.Get("key").Text()
		characters := []rune(key)
		sessionID := string(characters[min(utf8.RuneCountInString(retiredPrefix), len(characters)):])
		session, ok := s.sessions[sessionID]
		if !ok {
			session = sessionOf(sessionID)
		}
		value, reason := jsonText(record.Get("value"))
		if reason == "" && !value.IsObject() {
			reason = "not a JSON object"
		}
		if reason != "" {
			s.addError(sessionID, projectOf(session), record.Get("_rowid"), "unparseable record "+
				record.Get("_rowid").Text()+" (state_meta.value: "+reason+")", orEmpty(record.Get("value")))
			return nil
		}
		sweep := ""
		if !value.Get("expires").IsEmpty() {
			sweep = " sweep"
		}
		deleted := s.parser.system(s.artifact, int(row.Rowid), session, "session deleted: "+sessionID+
			sweep+" (from state.db state_meta gateway.retired_session.v1, retired_at)")
		deleted.TimestampUTC = timeutil.FromValue(value.Get("retired_at"))
		s.emit(deleted)
		return nil
	})
}

// generationRows emits one "relay peer:" row per conversation_generations
// row, with no time: Hermes never deletes these rows, so they name every
// messaging chat that had a conversation.
func (s *stateReader) generationRows() error {
	table, err := s.handle.Table("conversation_generations")
	if err != nil {
		return err
	}
	return table.Select("rowid", "source", "session_key", "generation").OrderBy("rowid").
		Each(func(row sqlitefile.Row) error {
			record := rowObject(row)
			s.emit(s.parser.system(s.artifact, int(row.Rowid), noSession(), "relay peer: "+
				dash(record.Get("source"))+" "+dash(record.Get("session_key"))+" generation "+
				dash(record.Get("generation"))+" (from state.db conversation_generations)"))
			return nil
		})
}

// roomRows emits one system row per Bot Mode room event, at its
// created_at: the text of a message.user or message.member event, the
// payload as compact JSON for the other kinds. Whether a room message is
// also a messages row is not known, so these are never user rows.
func (s *stateReader) roomRows() error {
	names := map[string]string{}
	if s.tables["hosted_rooms"] {
		table, err := s.handle.Table("hosted_rooms")
		if err != nil {
			return err
		}
		err = table.Select("room_id", "name").Each(func(row sqlitefile.Row) error {
			record := rowObject(row)
			names[record.Get("room_id").Text()] = orEmpty(record.Get("name"))
			return nil
		})
		if err != nil {
			return err
		}
	}
	order := []string{"created_at", "room_id", "seq"}
	return s.each("hosted_room_events", true, nil, order, func(event jsonvalue.Value) error {
		actor, actorOK := s.roomJSON(event, "actor_json")
		payload, payloadOK := s.roomJSON(event, "payload_json")
		if !actorOK || !payloadOK {
			return nil
		}
		kind, room := orEmpty(event.Get("kind")), orEmpty(event.Get("room_id"))
		body := parser.CompactJSON(payload)
		if text := payload.Get("text"); text.IsString() && (kind == "message.user" || kind == "message.member") {
			body = text.StringValue()
		}
		roomName := jsonvalue.NewNull()
		if name, ok := names[room]; ok {
			roomName = jsonvalue.NewString(name)
		}
		row := s.parser.system(s.artifact, lineNumber(event.Get("_rowid")), noSession(),
			"bot mode room "+dash(roomName)+" ("+room+") seq "+dash(event.Get("seq"))+" "+kind+
				" actor "+dash(actor.Get("kind"))+":"+dash(actor.Get("id"))+" ("+
				dash(actor.Get("display_name"))+"): "+body)
		row.TimestampUTC = timeutil.FromValue(event.Get("created_at"))
		s.emit(row)
		return nil
	})
}

// roomJSON returns hosted_room_events.<column> of an event as a JSON
// object, or false after filing a record problem when it does not decode
// to one.
func (s *stateReader) roomJSON(event jsonvalue.Value, column string) (jsonvalue.Value, bool) {
	value, reason := jsonText(event.Get(column))
	if reason == "" && value.IsObject() {
		return value, true
	}
	if reason == "" {
		reason = "not a JSON object"
	}
	rowid := event.Get("_rowid")
	s.addError("", "", rowid, "unparseable record "+rowid.Text()+" (hosted_room_events."+column+": "+
		reason+")", orEmpty(event.Get(column)))
	return jsonvalue.Value{}, false
}
