package opencode

import (
	"errors"
	"fmt"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// legacyNote ends the session-start row of a session that a 2.x database
// holds only in the 1.x session table: deleted in 2.0, which leaves the
// 1.x rows behind.
const legacyNote = "(only in the 1.x session table)"

// session is one row of a session table, by its id's text.
type session struct {
	identifier string
	row        sqlitefile.Row
	table      string
}

// store is the reading of one database: its tables, projects and sessions,
// each session's scope, and the problems found so far.
type store struct {
	owner     *Parser
	artifact  *input.Artifact
	options   parser.Options
	emit      parser.Emit
	database  *sqlitefile.Database
	tables    map[string]bool
	label     string
	projects  map[string]sqlitefile.Row
	sessions  []session
	sessionAt map[string]int
	scopes    map[string]*scope
	// shares holds the ids of the sessions with a session_share row.
	shares   map[string]bool
	failures []failure
}

// database emits the rows of one database. A file that is not SQLite or
// cannot be read is one `parser:` row; an error met while opening or
// reading ends the rows with a "database could not be read" row.
func (p *Parser) database(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	if err := sqlitefile.Problem(artifact.DiskPath); err != nil {
		emit(p.ProblemRow(artifact, parser.DatabaseError(err)))
		return
	}
	database, err := sqlitefile.OpenPath(artifact.DiskPath)
	if err == nil {
		reading := &store{owner: p, artifact: artifact, options: options, emit: emit,
			database: database, label: databaseLabel(artifact), projects: map[string]sqlitefile.Row{},
			sessionAt: map[string]int{}, scopes: map[string]*scope{}, shares: map[string]bool{}}
		err = reading.read()
		if closeErr := database.Close(); err == nil {
			err = closeErr
		}
	}
	if err != nil {
		emit(p.ProblemRow(artifact, "database could not be read: "+err.Error()))
	}
}

// read emits the database's rows: the session starts, the shares, the
// standing state, the transcript, then the problems.
func (s *store) read() error {
	tables, err := s.database.TableNames()
	if err != nil {
		return err
	}
	s.tables = tables
	if err := s.readProjects(); err != nil {
		return err
	}
	if err := s.readSessions(); err != nil {
		return err
	}
	hasV2 := tables["session_v2"]
	for _, each := range s.sessions {
		note := ""
		if hasV2 && each.table == "session" {
			note = legacyNote
		}
		s.emit(s.owner.sessionRow(s.artifact, s.scopeFor(each.identifier), rowLookup(each.row),
			parser.ColumnTime(each.row.Get("time_created")), int(each.row.Rowid), note))
	}
	for _, step := range []func() error{s.shareRows, s.eventShareRows, s.stateRows,
		s.worktreeRows, s.grantRows, s.transcript} {
		if err := step(); err != nil {
			return err
		}
	}
	for i, row := range s.owner.failureRows(s.artifact, s.failures, "", "") {
		if identifier := s.failures[i].sessionID; identifier != "" {
			row.SessionID, row.ProjectPath = identifier, s.scopeFor(identifier).projectPath
		}
		s.emit(row)
	}
	return nil
}

// transcript emits the messages. In a 2.x database session_message is the
// transcript and the 1.x tables are read, as recovered rows, only for a
// session it does not hold (one deleted in 2.0). In a 1.x database message
// and part are the transcript, and session_message, which a migration
// once wiped, is read only for a session with no 1.x message.
func (s *store) transcript() error {
	if s.tables["session_v2"] {
		covered, err := s.sessionMessageIDs()
		if err != nil {
			return err
		}
		if err := s.v2Messages(map[string]bool{}); err != nil {
			return err
		}
		_, err = s.v1Messages(covered, true)
		return err
	}
	covered, err := s.v1Messages(map[string]bool{}, false)
	if err != nil {
		return err
	}
	return s.v2Messages(covered)
}

// readProjects indexes the project table's rows by their id, a later row
// replacing an earlier one with the same id.
func (s *store) readProjects() error {
	if !s.tables["project"] {
		return nil
	}
	table, err := s.database.Table("project")
	if err != nil {
		return err
	}
	rows, err := table.Select().All()
	for _, row := range rows {
		s.projects[parser.ColumnKey(row.Get("id"))] = row
	}
	return err
}

// readSessions reads session_v2 then session, each ordered by its time
// and id; the first row of an id is kept.
func (s *store) readSessions() error {
	for _, name := range []string{"session_v2", "session"} {
		if !s.tables[name] {
			continue
		}
		table, err := s.database.Table(name)
		if err != nil {
			return err
		}
		rows, err := table.Select().OrderBy("time_created, id").All()
		if err != nil {
			return err
		}
		for _, row := range rows {
			identifier := row.Get("id").String()
			if _, seen := s.sessionAt[identifier]; !seen {
				s.sessionAt[identifier] = len(s.sessions)
				s.sessions = append(s.sessions, session{identifier: identifier, row: row, table: name})
			}
		}
	}
	return nil
}

// scopeFor returns the scope of a session, made on first use: its project
// path is the session's directory, else its project's worktree unless "/".
func (s *store) scopeFor(identifier string) *scope {
	if found, ok := s.scopes[identifier]; ok {
		return found
	}
	var record sqlitefile.Row
	if where, ok := s.sessionAt[identifier]; ok {
		record = s.sessions[where].row
	}
	path := parser.NonEmptyColumn(record.Get("directory"))
	if path == "" {
		path = s.worktreeOf(record.Get("project_id"))
	}
	made := newScope(identifier, path)
	s.scopes[identifier] = made
	return made
}

// worktreeOf returns the worktree of the project with the given id, "" when
// there is none or it is "/".
func (s *store) worktreeOf(identifier sqlitefile.Value) string {
	project, ok := s.projects[parser.ColumnKey(identifier)]
	if !ok {
		return ""
	}
	worktree := parser.NonEmptyColumn(project.Get("worktree"))
	if worktree == "/" {
		return ""
	}
	return worktree
}

// rowLookup reads the fields of a database row.
func rowLookup(row sqlitefile.Row) lookup {
	return func(key string) (string, bool) {
		value := row.Get(key)
		return value.String(), !value.IsEmpty()
	}
}

// columnJSON returns the text of a column that stores JSON, with an error
// saying why it is not text: NULL is "no data", a number "not JSON text"
// with the number as the text. A blob is decoded as UTF-8 with
// replacement.
func columnJSON(value sqlitefile.Value) (string, error) {
	switch value.Type {
	case sqlitefile.Null:
		return "", errors.New("no data")
	case sqlitefile.Blob:
		return textlines.ValidUTF8(value.Bytes), nil
	case sqlitefile.Text:
		return value.Text(), nil
	}
	return value.String(), errors.New("not JSON text")
}

// unparseable returns the message of a stored record that does not decode:
// "unparseable record <rowid> (<table.column>: <reason>)".
func unparseable(rowid int64, column string, reason error) string {
	return fmt.Sprintf("unparseable record %d (%s: %v)", rowid, column, reason)
}

// systemAt returns a system row of the database at line and the column
// time when, with text compacted.
func (s *store) systemAt(scoped *scope, line int64, when sqlitefile.Value, text string) timeline.Row {
	return s.owner.systemRow(s.artifact, scoped, int(line), parser.ColumnTime(when), text)
}
