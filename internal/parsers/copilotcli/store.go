package copilotcli

import (
	"cmp"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// checkpointColumns are the checkpoint fields written into a checkpoint row,
// in order.
var checkpointColumns = []string{"overview", "history", "work_done", "technical_details",
	"important_files", "next_steps"}

// store reads the tables of session-store.db, keeping the first error.
type store struct {
	database *sqlitefile.Database
	err      error
}

// rows returns every row of table ordered by the column, then by rowid; a
// table the database lacks gives none. After a failure it gives none.
func (s *store) rows(table, order string) []sqlitefile.Row {
	if s.err != nil || !s.has(table) {
		return nil
	}
	handle, err := s.database.Table(table)
	if err != nil {
		s.err = err
		return nil
	}
	rows, err := handle.Select().OrderBy(order, "rowid").All()
	if err != nil {
		s.err = err
		return nil
	}
	return rows
}

// has reports whether the database has the table.
func (s *store) has(table string) bool {
	if s.err != nil {
		return false
	}
	tables, err := parserkit.TablesOf(s.database)
	s.err = err
	return tables.Has(table)
}

// stored returns a column as row text (parserkit.ColumnText): "" for NULL.
func stored(row sqlitefile.Row, column string) string {
	return parserkit.ColumnText(row.Get(column))
}

// present reports whether a column is neither NULL nor empty text.
func present(row sqlitefile.Row, column string) bool {
	value := row.Get(column)
	return !value.IsNull() && !(value.Type == sqlitefile.Text && len(value.Bytes) == 0)
}

// parseStore emits the rows of session-store.db: every row is read before
// any is emitted, so a database that fails midway gives one problem row.
func (p *Parser) parseStore(artifact *input.Artifact, emit parserkit.Emit) {
	database, err := parserkit.OpenDatabase(artifact.DiskPath)
	if err != nil {
		message := parserkit.DatabaseError(err)
		line := 0
		if strings.HasPrefix(message, "SQLite error: ") {
			line = 1
		}
		emit(p.row(artifact, line, timeline.TurnSystem, "parser: "+message, columns{}))
		return
	}
	defer database.Close()
	reader := &store{database: database}
	var rows []timeline.Row
	p.storeRows(artifact, reader, func(row timeline.Row) { rows = append(rows, row) })
	if reader.err != nil {
		emit(p.problem(artifact, "parser: "+parserkit.DatabaseError(reader.err), ""))
		return
	}
	parserkit.EmitAll(emit, rows)
}

// stamper builds the rows of a store: each stamped with the time, and the
// session's project, branch and id.
type stamper struct {
	parser   *Parser
	artifact *input.Artifact
	sessions map[string]storedSession
	sidebar  map[string]string
	emit     parserkit.Emit
}

// storedSession is the first sessions row of an id and its cwd and branch
// as row text, made once for every row of the session to share, with the
// number of rows of the id and, when that is more than one, their group.
type storedSession struct {
	row         sqlitefile.Row
	cwd, branch string
	count       int
	group       *timeline.Group
}

// stamp emits a row of a session of the store, with the project and branch
// the session record gives; a row of a repeated session id is a member of
// its group (sessionRows), taking the project and branch of the stored row
// the group goes with.
func (s *stamper) stamp(sessionID string, line int64, turn, text string, timestamp sqlitefile.Value,
	toolUseID string) {
	session := s.sessions[sessionID]
	row := s.parser.row(s.artifact, int(line), turn, text, columns{
		timestamp: parserkit.ColumnTime(timestamp), sessionID: sessionID,
		project: cmp.Or(session.cwd, s.sidebar[sessionID]),
		branch:  session.branch, toolUseID: toolUseID})
	if session.group != nil {
		session.group.Member(&row, timeline.TakeProject|timeline.TakeBranch)
	}
	s.emit(row)
}

// readSessions returns the sessions rows in created_at order and keeps the
// first row of each id, with a group for each id held in more than one.
func (s *stamper) readSessions(reader *store) []sqlitefile.Row {
	sessions := reader.rows("sessions", "created_at")
	for _, session := range sessions {
		sessionID := stored(session, "id")
		entry, seen := s.sessions[sessionID]
		if !seen {
			entry = storedSession{row: session, cwd: stored(session, "cwd"), branch: stored(session, "branch")}
		}
		if entry.count++; entry.count == 2 {
			entry.group = timeline.NewGroup()
		}
		s.sessions[sessionID] = entry
	}
	return sessions
}

// sessionRows emits the rows of one sessions row: its "session name:" row
// when it has a summary, and, for a session id the table holds in more
// than one row, which only damage or an edit writes since the id is the
// table's key, a `parser:` row saying so that makes the stored row a
// parent of the id's group (timeline.Group), both with the row's own time,
// project and branch.
func (s *stamper) sessionRows(session sqlitefile.Row) {
	sessionID := stored(session, "id")
	entry := s.sessions[sessionID]
	set := columns{timestamp: parserkit.ColumnTime(session.Get("created_at")), sessionID: sessionID,
		project: cmp.Or(entry.cwd, s.sidebar[sessionID]), branch: entry.branch}
	if entry.group != nil {
		set.project, set.branch = cmp.Or(stored(session, "cwd"), s.sidebar[sessionID]), stored(session, "branch")
	}
	var rows []timeline.Row
	if present(session, "summary") {
		rows = append(rows, s.parser.row(s.artifact, int(session.Rowid), timeline.TurnSystem,
			"session name: "+session.Get("summary").String(), set))
	}
	if entry.group != nil {
		notice := s.parser.row(s.artifact, int(session.Rowid), timeline.TurnSystem, "", set)
		parent := entry.group.Parent(&notice)
		prefix := parserkit.ProblemPrefix + "sessions holds " + strconv.Itoa(entry.count) +
			" rows of this row's id; its turns, files, references, checkpoints and tool events are shown once, with "
		parent.Texts(&notice, timeline.RowText{Text: prefix + "this row"},
			timeline.RowText{Text: prefix + "another of them"})
		for i := range rows {
			parent.Row(&rows[i], timeline.Always)
		}
		rows = append(rows, notice)
	}
	parserkit.EmitAll(s.emit, rows)
}

// storeRows emits the rows of every table of the store the parser reads.
func (p *Parser) storeRows(artifact *input.Artifact, reader *store, emit parserkit.Emit) {
	stamps := &stamper{parser: p, artifact: artifact, sessions: map[string]storedSession{},
		sidebar: p.sidebarJoin(artifact), emit: emit}
	for _, session := range stamps.readSessions(reader) {
		stamps.sessionRows(session)
	}
	for _, turn := range reader.rows("turns", "session_id, turn_index") {
		for _, column := range [][2]string{{"user_message", timeline.TurnUser},
			{"assistant_response", timeline.TurnAssistant}} {
			if present(turn, column[0]) {
				stamps.stamp(stored(turn, "session_id"), turn.Rowid, column[1], stored(turn, column[0]),
					turn.Get("timestamp"), "")
			}
		}
	}
	p.fileRefRows(reader, stamps)
	p.checkpointRows(reader, stamps)
	p.forgeRows(reader, stamps)
}

// fileRefRows emits the rows of the files a session changed and the
// references it made.
func (p *Parser) fileRefRows(reader *store, stamps *stamper) {
	for _, file := range reader.rows("session_files", "first_seen_at") {
		stamps.stamp(stored(file, "session_id"), file.Rowid, timeline.TurnSystem,
			"file changed: "+stored(file, "file_path")+" tool="+cmp.Or(stored(file, "tool_name"), "-"),
			file.Get("first_seen_at"), "")
	}
	for _, ref := range reader.rows("session_refs", "created_at") {
		stamps.stamp(stored(ref, "session_id"), ref.Rowid, timeline.TurnSystem,
			"reference: "+stored(ref, "ref_type")+" "+stored(ref, "ref_value"), ref.Get("created_at"), "")
	}
}

// checkpointRows emits one row per checkpoint, its title and each field it
// has.
func (p *Parser) checkpointRows(reader *store, stamps *stamper) {
	for _, checkpoint := range reader.rows("checkpoints", "created_at") {
		lines := []string{"checkpoint " + stored(checkpoint, "checkpoint_number") + ": " +
			stored(checkpoint, "title")}
		for _, column := range checkpointColumns {
			if present(checkpoint, column) {
				lines = append(lines, column+": "+stored(checkpoint, column))
			}
		}
		stamps.stamp(stored(checkpoint, "session_id"), checkpoint.Rowid, timeline.TurnSystem,
			strings.Join(lines, "\n"), checkpoint.Get("created_at"), "")
	}
}

// forgeRows emits the forge_trajectory_events rows, read from the bundle's
// shape alone: a row with a command is a tool_use, one with an output or an
// exit_code a tool_result, both joined on tool_call_id; every column may be
// NULL.
func (p *Parser) forgeRows(reader *store, stamps *stamper) {
	for _, event := range reader.rows("forge_trajectory_events", "created_at") {
		sessionID, callID := stored(event, "session_id"), stored(event, "tool_call_id")
		if present(event, "command") {
			stamps.stamp(sessionID, event.Rowid, timeline.TurnToolUse, stored(event, "command"),
				event.Get("created_at"), callID)
		}
		code := event.Get("exit_code")
		if event.Get("output").IsNull() && code.IsNull() {
			continue
		}
		body := stored(event, "output")
		if !isZeroColumn(code) {
			body = "[exit " + code.String() + "] " + body
		}
		stamps.stamp(sessionID, event.Rowid, timeline.TurnToolResult, body, event.Get("created_at"), callID)
	}
}

// isZeroColumn reports whether a column is NULL, the number zero or the text
// "0": an exit code that is not a failure.
func isZeroColumn(value sqlitefile.Value) bool {
	switch value.Type {
	case sqlitefile.Null:
		return true
	case sqlitefile.Integer:
		return value.Int == 0
	case sqlitefile.Float:
		return value.Float == 0
	case sqlitefile.Text:
		return string(value.Bytes) == "0"
	}
	return false
}
