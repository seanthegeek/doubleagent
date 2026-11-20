package hermes

import (
	"errors"
	"slices"
	"strconv"
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

// rowObject returns a row as parser.RowObject does, with the rowid, when
// it is the first column selected, keyed _rowid.
func rowObject(row sqlitefile.Row) jsonvalue.Value {
	record := parser.RowObject(row)
	members := record.Members()
	if len(members) == 0 || members[0].Key != "rowid" {
		return record
	}
	renamed := slices.Clone(members)
	renamed[0].Key = "_rowid"
	return jsonvalue.NewObject(renamed...)
}

// recordError is a problem of one stored record, reported after the file's
// other rows under its session and project; member says the project is
// the session's, not the record's own.
type recordError struct {
	sessionID, project string
	problem            parser.Problem
	member             bool
}

// stateReader reads one state.db.
type stateReader struct {
	parser   *Parser
	artifact *input.Artifact
	handle   *sqlitefile.Database
	tables   parser.Tables
	sessions map[string]jsonvalue.Value
	// groups holds the group of each session id the sessions table
	// repeats (timeline.Group).
	groups map[string]*timeline.Group
	usage  map[string][]jsonvalue.Value
	errors []recordError
	emit   parser.Emit
}

// parseDatabase emits the rows of state.db or a snapshot of it: sessions,
// messages, deletion fences, relay peers and room events, then the records
// that do not decode. A database error ends the rows with one `parser:`
// row; the rows before it stay.
func (p *Parser) parseDatabase(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	handle, err := parser.OpenDatabase(artifact.DiskPath)
	if err != nil {
		emit(p.system(artifact, 0, noSession(), parser.ProblemPrefix+parser.DatabaseError(err)))
		return
	}
	defer handle.Close()
	reader := &stateReader{parser: p, artifact: artifact, handle: handle,
		sessions: map[string]jsonvalue.Value{}, groups: map[string]*timeline.Group{}, emit: emit}
	if err := reader.rows(options); err != nil {
		emit(p.system(artifact, 0, noSession(), parser.ProblemPrefix+parser.DatabaseError(err)))
	}
}

// each runs visit over the rows of table as objects, every stored column
// with the rowid first as _rowid when withRowid is set, filtered by keep
// when not nil, in the order. A virtual generated column, computed when
// read, is left out, as selecting every column leaves it out.
func (s *stateReader) each(table string, withRowid bool, keep func(sqlitefile.Row) bool,
	order []string, visit func(jsonvalue.Value) error) error {
	handle, err := s.handle.Table(table)
	if err != nil {
		return err
	}
	if withRowid && handle.WithoutRowid() {
		return errors.New("table " + handle.Name() + ": no such column: rowid")
	}
	query := handle.Select()
	if keep != nil {
		query = query.Where(keep)
	}
	return query.OrderBy(order...).Each(func(row sqlitefile.Row) error {
		record := rowObject(row)
		if withRowid {
			rowid := jsonvalue.Member{Key: "_rowid", Value: parser.ColumnValue(sqlitefile.IntValue(row.Rowid))}
			record = jsonvalue.NewObject(append([]jsonvalue.Member{rowid}, record.Members()...)...)
		}
		return visit(record)
	})
}

// rows emits the database's rows, stopping at the first error.
func (s *stateReader) rows(options parser.Options) error {
	var err error
	if s.tables, err = parser.TablesOf(s.handle); err != nil || !s.tables.Has("messages") {
		return err
	}
	if err := s.readUsage(); err != nil {
		return err
	}
	if s.tables.Has("sessions") {
		if err := s.readSessions(); err != nil {
			return err
		}
	}
	reader := s.parser.newMessageReader(s.artifact, options, s.usage, s.emit)
	reader.groups = s.groups
	err = s.each("messages", false, nil, []string{"timestamp", "id"}, func(message jsonvalue.Value) error {
		sessionID := parser.Text(message.Get("session_id"))
		session, ok := s.sessions[sessionID]
		if !ok {
			session = sessionOf(sessionID)
		}
		reader.message(message, session, message.Get("id"))
		return nil
	})
	if err != nil {
		return err
	}
	if err := s.tableRows(); err != nil {
		return err
	}
	for _, entry := range s.errors {
		for _, row := range s.parser.ErrorRows(s.artifact, parser.Problems{entry.problem}, entry.sessionID,
			entry.project) {
			if group := s.groups[entry.sessionID]; entry.member && group != nil {
				group.Member(&row, timeline.TakeProject)
			}
			s.emit(row)
		}
	}
	return nil
}

// readSessions emits the rows of the sessions table by started_at. The id
// is the table's key, so a repeat is damage: each row of a repeated id is a
// parent of the id's group (timeline.Group), and the id's usage rows,
// messages and deletion fences its members, shown once with the project,
// branch and model of the row the group goes with. The usage rows give
// their model rows after the id's first row, each later row its model from
// its own model column, and the table one problem row.
func (s *stateReader) readSessions() error {
	var sessions []jsonvalue.Value
	counts := map[string]int{}
	err := s.each("sessions", true, nil, []string{"started_at", "rowid"}, func(session jsonvalue.Value) error {
		sessions = append(sessions, session)
		counts[parser.Text(session.Get("id"))]++
		return nil
	})
	seen, walked, repeats := parser.Tracker{}, map[string]bool{}, 0
	for _, session := range sessions {
		identifier := parser.Text(session.Get("id"))
		if !walked[identifier] {
			s.sessions[identifier] = session
			if counts[identifier] > 1 {
				s.groups[identifier] = timeline.NewGroup()
			}
		}
		repeated := walked[identifier] && len(s.usage[identifier]) > 0
		if repeated {
			repeats++
		}
		walked[identifier] = true
		s.sessionRows(session, seen, repeated)
	}
	if repeats > 0 {
		rows := "1 row repeating"
		if repeats > 1 {
			rows = strconv.Itoa(repeats) + " rows repeating"
		}
		s.errors = append(s.errors, recordError{problem: parser.Problem{Message: "sessions holds " + rows +
			" an earlier row's id; its usage rows are shown once"}})
	}
	return err
}

// tableRows emits the rows of the deletion fences, the relay peers and the
// room events, for the tables the database has.
func (s *stateReader) tableRows() error {
	if s.tables.Has("state_meta") {
		if err := s.retiredRows(); err != nil {
			return err
		}
	}
	if s.tables.Has("conversation_generations") {
		if err := s.generationRows(); err != nil {
			return err
		}
	}
	if s.tables.Has("hosted_room_events") {
		return s.roomRows()
	}
	return nil
}

// readUsage reads the main-loop session_model_usage rows per session, by
// first_seen; a row with a task is a side task's and is skipped.
func (s *stateReader) readUsage() error {
	s.usage = map[string][]jsonvalue.Value{}
	if !s.tables.Has("session_model_usage") {
		return nil
	}
	return s.each("session_model_usage", true, nil, []string{"first_seen", "rowid"}, func(usage jsonvalue.Value) error {
		if usage.Get("task").IsEmpty() {
			sessionID := parser.Text(usage.Get("session_id"))
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
	if !parser.Present(raw) {
		return jsonvalue.NewObject()
	}
	sessionID, project := parser.Text(session.Get("id")), projectOf(session)
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

// archivedValue reports whether sessions.archived is 1 (by value, so 1.0
// too), "1" or true.
func archivedValue(value jsonvalue.Value) bool {
	flag, _ := value.Bool()
	return flag || jsonvalue.Equal(value, jsonvalue.NewInt(1)) || parser.IsText(value, "1")
}

// sessionRows emits a session's start row, its model rows (from its own
// model column alone when repeated, as for a session with no usage rows),
// and its relay, yolo-mode and worktree rows.
func (s *stateReader) sessionRows(session jsonvalue.Value, seen parser.Tracker, repeated bool) {
	origin, config := s.columnJSON(session, "origin_json"), s.columnJSON(session, "model_config")
	imported := ""
	if from := origin.Get("imported_from"); from.IsObject() {
		imported = "imported from " + parser.SettingOrDash(from.Get("tool")) + " " + parser.SettingOrDash(from.Get("path")) +
			" session " + parser.SettingOrDash(from.Get("foreign_session_id"))
	}
	archived := ""
	if archivedValue(session.Get("archived")) {
		archived = "archived=1"
	}
	start := s.parser.system(s.artifact, lineNumber(session.Get("_rowid")), session,
		timeline.Compact(parser.JoinNonEmpty(" | ", "session start", parser.Text(session.Get("title")),
			parser.Labelled("source=", session.Get("source")),
			parser.Labelled("parent=", session.Get("parent_session_id")),
			parser.Labelled("end=", session.Get("end_reason")), archived, imported)))
	start.TimestampUTC = timeutil.FromValue(session.Get("started_at"))
	if group := s.groups[start.SessionID]; group != nil {
		// The rows copied from the start row are this parent's rows too.
		group.Parent(&start)
	}
	s.emit(start)
	s.modelRows(session, start, seen, repeated)
	source := parser.Text(session.Get("source"))
	if slices.Contains(messagingSources(), source) ||
		!session.Get("chat_id").IsEmpty() && source != "local" && source != "api_server" {
		s.emit(copyRow(start, "session relayed: "+source+" chat "+parser.SettingOrDash(session.Get("chat_id"))+
			" type "+parser.SettingOrDash(session.Get("chat_type"))+" thread "+parser.SettingOrDash(session.Get("thread_id"))+
			" user "+parser.SettingOrDash(session.Get("user_id"))+" name "+parser.SettingOrDash(origin.Get("user_name"))+
			" display "+parser.SettingOrDash(displayName(session, origin))+
			" (from state.db sessions, started_at)", start.Model))
	}
	if yolo, ok := config.Lookup("yolo_mode"); ok {
		s.emit(copyRow(start, "permission setting: yolo_mode="+parser.SettingValue(yolo)+
			" (from state.db sessions.model_config, started_at)", start.Model))
	}
	cwd, branch := parser.Text(session.Get("cwd")), parser.Text(session.Get("git_branch"))
	if tree := s.parser.worktree.FindStringSubmatch(cwd); tree != nil &&
		(strings.HasPrefix(branch, "hermes/") || strings.HasPrefix(branch, "hermes-subagent/")) {
		s.emit(copyRow(start, "scratch directory: "+cwd+" (git worktree the agent created in "+
			tree[1]+", branch "+branch+")", start.Model))
	}
}

// modelRows emits a session's model rows: one per main-loop usage model
// that changes it, at the usage's first_seen, else (or when ownOnly) one
// for sessions.model.
func (s *stateReader) modelRows(session jsonvalue.Value, start timeline.Row, seen parser.Tracker, ownOnly bool) {
	uses := s.usage[start.SessionID]
	if len(uses) == 0 || ownOnly {
		model := parser.Text(session.Get("model"))
		if text := seen.Change("model", start.SessionID, model); text != "" {
			s.emit(copyRow(start, text, model))
		}
		return
	}
	for _, usage := range uses {
		model := parser.Text(usage.Get("model"))
		text := seen.Change("model", start.SessionID, model)
		if text == "" {
			continue
		}
		if provider := usage.Get("billing_provider"); !provider.IsEmpty() {
			text += " provider=" + provider.Text()
		}
		row := copyRow(start, text, model)
		row.SourceLine = lineNumber(usage.Get("_rowid"))
		row.TimestampUTC = timeutil.FromValue(usage.Get("first_seen"))
		takes := timeline.TakeProject | timeline.TakeBranch
		if row.TimestampUTC == "" {
			row.TimestampUTC, takes = start.TimestampUTC, takes|timeline.TakeTime
		}
		if group := s.groups[row.SessionID]; group != nil {
			group.Member(&row, takes)
		}
		s.emit(row)
	}
}

// jsonObject decodes a column that stores a JSON object. An empty value
// reads as empty text, which does not decode; a number is "not JSON
// text", and a document of another kind "not a JSON object".
func jsonObject(value jsonvalue.Value) (jsonvalue.Value, error) {
	if value.IsEmpty() {
		value = jsonvalue.NewString("")
	}
	if !value.IsString() {
		return jsonvalue.Value{}, errors.New("not JSON text")
	}
	decoded, err := jsonvalue.DecodeString(value.StringValue())
	switch {
	case err != nil:
		return jsonvalue.Value{}, err
	case !decoded.IsObject():
		return jsonvalue.Value{}, errors.New("not a JSON object")
	}
	return decoded, nil
}

// retiredRows emits one "session deleted:" row per state_meta deletion
// fence (key gateway.retired_session.v1.<session id>, matched with SQL's
// LIKE, so the prefix's letters match in either case), at its retired_at; " sweep" follows the id
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
		value, err := jsonObject(record.Get("value"))
		if err != nil {
			s.addError(sessionID, projectOf(session), record.Get("_rowid"), "unparseable record "+
				record.Get("_rowid").Text()+" (state_meta.value: "+err.Error()+")", parser.Text(record.Get("value")))
			s.errors[len(s.errors)-1].member = true
			return nil
		}
		sweep := ""
		if !value.Get("expires").IsEmpty() {
			sweep = " sweep"
		}
		deleted := s.parser.system(s.artifact, int(row.Rowid), session, "session deleted: "+sessionID+
			sweep+" (from state.db state_meta gateway.retired_session.v1, retired_at)")
		deleted.TimestampUTC = timeutil.FromValue(value.Get("retired_at"))
		if group := s.groups[sessionID]; group != nil {
			group.Member(&deleted, timeline.TakeProject|timeline.TakeBranch|timeline.TakeModel)
		}
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
				parser.SettingOrDash(record.Get("source"))+" "+parser.SettingOrDash(record.Get("session_key"))+" generation "+
				parser.SettingOrDash(record.Get("generation"))+" (from state.db conversation_generations)"))
			return nil
		})
}

// cutRows emits the problem rows of the values carried cut.
func (s *stateReader) cutRows(carried *parser.Carried) {
	var problems parser.Problems
	carried.Report(0, &problems)
	for _, problem := range problems {
		s.emit(s.parser.system(s.artifact, 0, noSession(), parser.ProblemPrefix+problem.Message))
	}
}

// roomRows emits one system row per Bot Mode room event, at its
// created_at: the text of a message.user or message.member event, the
// payload as compact JSON for the other kinds. Whether a room message is
// also a messages row is not known, so these are never user rows. A room's
// name, which each of its events' rows repeats, is cut by the kit
// (parser.Carried).
func (s *stateReader) roomRows() error {
	names := map[string]string{}
	carried := parser.NewCarried(s.artifact)
	defer s.cutRows(carried)
	if s.tables.Has("hosted_rooms") {
		table, err := s.handle.Table("hosted_rooms")
		if err != nil {
			return err
		}
		err = table.Select("room_id", "name").Each(func(row sqlitefile.Row) error {
			record := rowObject(row)
			names[record.Get("room_id").Text()] = carried.Cut("room name", parser.Text(record.Get("name")))
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
		kind, room := parser.Text(event.Get("kind")), parser.Text(event.Get("room_id"))
		body := parser.JSONText(payload)
		if text := payload.Get("text"); text.IsString() && (kind == "message.user" || kind == "message.member") {
			body = text.StringValue()
		}
		roomName := jsonvalue.NewNull()
		if name, ok := names[room]; ok {
			roomName = jsonvalue.NewString(name)
		}
		row := s.parser.system(s.artifact, lineNumber(event.Get("_rowid")), noSession(),
			"bot mode room "+parser.SettingOrDash(roomName)+" ("+room+") seq "+parser.SettingOrDash(event.Get("seq"))+" "+kind+
				" actor "+parser.SettingOrDash(actor.Get("kind"))+":"+parser.SettingOrDash(actor.Get("id"))+" ("+
				parser.SettingOrDash(actor.Get("display_name"))+"): "+body)
		row.TimestampUTC = timeutil.FromValue(event.Get("created_at"))
		s.emit(row)
		return nil
	})
}

// roomJSON returns hosted_room_events.<column> of an event as a JSON
// object, or false after filing a record problem when it does not decode
// to one.
func (s *stateReader) roomJSON(event jsonvalue.Value, column string) (jsonvalue.Value, bool) {
	value, err := jsonObject(event.Get(column))
	if err == nil {
		return value, true
	}
	rowid := event.Get("_rowid")
	s.addError("", "", rowid, "unparseable record "+rowid.Text()+" (hosted_room_events."+column+": "+
		err.Error()+")", parser.Text(event.Get(column)))
	return jsonvalue.Value{}, false
}
