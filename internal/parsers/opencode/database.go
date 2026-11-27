package opencode

import (
	"errors"
	"fmt"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// legacyNote ends the session-start row of a session that a 2.x database
// holds only in the 1.x session table: deleted in 2.0, which leaves the
// 1.x rows behind.
const legacyNote = "(only in the 1.x session table)"

// session is one row of a session table, by its id's text, with its
// project path: its directory, else its project's worktree unless "/".
// parent is its place in the group of an id its table holds in more than
// one row, nil for any other row.
type session struct {
	identifier string
	// key is the id as the session table's id column compares it ("" for
	// NULL, which no reference names).
	key     string
	row     sqlitefile.Row
	table   string
	project string
	parent  *timeline.Parent
	// projectRepeat is the group of the id of the project the row's
	// project path comes from, when that id is repeated, nil otherwise.
	projectRepeat *projectRepeat
}

// store is the reading of one database: its tables, the worktree of each
// project by id, its sessions, each session's scope, and the problems
// found so far.
type store struct {
	owner    *Parser
	artifact *input.Artifact
	options  parserkit.Options
	emit     parserkit.Emit
	database *sqlitefile.Database
	tables   parserkit.Tables
	label    string
	projects map[string]string
	// projectCollation and sessionCollation are the collations the
	// project and session tables declare for their ids, under which a
	// reference to either is compared (sessionRef, projectKey).
	projectCollation string
	sessionCollation string
	// messageCollation is the one session_message.id declares, under
	// which session_part.message_id is compared with it.
	messageCollation string
	// projectRepeats holds the project ids the project table holds in more
	// than one row, which only damage or an edit writes, and
	// repeatedProjects those rows, in reading order.
	projectRepeats   map[string]*projectRepeat
	repeatedProjects []sqlitefile.Row
	sessions         []session
	sessionAt        map[string]int
	// repeats holds the session ids a session table holds in more than
	// one row, which only damage or an edit writes.
	repeats map[string]*sessionRepeat
	scopes  map[string]*scope
	// shares holds the ids of the sessions with a session_share row.
	shares map[string]bool
	// parts holds the part table's rows by message id, read on first use,
	// and messages the message table's by id, read on first use too.
	parts    *sqlitefile.Lookup
	messages *sqlitefile.Lookup
	failures []failure
}

// database emits the rows of one database. A file that is not SQLite or
// cannot be read is one `parser:` row; an error met while opening or
// reading ends the rows with a "database could not be read" row.
func (p *Parser) database(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) {
	database, err := parserkit.OpenDatabase(artifact, artifact.DiskPath)
	var openErr *parserkit.OpenError
	if err != nil && !errors.As(err, &openErr) {
		emit(p.ProblemRow(artifact, parserkit.DatabaseError(err)))
		return
	}
	if err == nil {
		reading := &store{owner: p, artifact: artifact, options: options, emit: emit,
			database: database, label: databaseLabel(artifact), projects: map[string]string{},
			projectRepeats: map[string]*projectRepeat{},
			sessionAt:      map[string]int{}, repeats: map[string]*sessionRepeat{}, scopes: map[string]*scope{},
			shares: map[string]bool{}}
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
	tables, err := parserkit.TablesOf(s.database)
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
	s.projectNotices()
	for i := range s.sessions {
		s.sessionStart(&s.sessions[i])
	}
	for _, step := range []func() error{s.shareRows, s.eventShareRows, s.stateRows,
		s.worktreeRows, s.grantRows, s.transcript} {
		if err := step(); err != nil {
			return err
		}
	}
	for i, row := range s.owner.failureRows(s.artifact, s.failures, "", "") {
		failed := s.failures[i]
		switch {
		case failed.owner != nil:
			row.SessionID, row.ProjectPath = failed.session.text, failed.owner.project
			s.ownRow(failed.owner, row)
		case failed.session.key != "":
			row.SessionID, row.ProjectPath = failed.session.text, s.scopeFor(failed.session).projectPath
			s.memberEmit(failed.session, timeline.TakeProject, s.emit)(row)
		default:
			s.emit(row)
		}
	}
	return nil
}

// sessionStart emits the "session start:" row of one stored session row,
// with the row's own project; a session a 2.x database holds only in the
// 1.x session table ends with legacyNote. A row of an id its table repeats
// is a parent of the id's group, its start row followed by a `parser:`
// row saying so.
func (s *store) sessionStart(each *session) {
	note := ""
	if s.tables.Has("session_v2") && each.table == "session" {
		note = legacyNote
	}
	start := s.owner.sessionRow(s.artifact, s.rowScope(each), rowLookup(each.row),
		parserkit.ColumnTime(each.row.Get("time_created")), int(each.row.Rowid), note)
	repeated := s.repeats[each.key]
	if repeated == nil {
		each.projectRepeat.member(&start, timeline.TakeProject)
		s.emit(start)
		return
	}
	notice := start
	each.parent = repeated.start(&start, &notice)
	s.emit(start)
	s.emit(notice)
}

// rowScope returns the scope of a stored session row's own rows: the
// session's scope for its first row, else one with the row's own project.
func (s *store) rowScope(each *session) *scope {
	if where, ok := s.sessionAt[each.key]; ok && &s.sessions[where] == each {
		return s.scopeFor(sessionRef{key: each.key, text: each.identifier})
	}
	return newScope(each.identifier, each.project)
}

// ownRow emits row, a row of one stored session row alone, as one of its
// parent's rows when its id is repeated, else as a member of the group of
// its project's id when that is repeated.
func (s *store) ownRow(each *session, row timeline.Row) {
	if each.parent != nil {
		each.parent.Row(&row, timeline.Always)
	} else {
		each.projectRepeat.member(&row, timeline.TakeProject)
	}
	s.emit(row)
}

// memberEmit returns an emit of the rows of a session, which makes each
// row it is given that has no place in a group yet a member, taking takes,
// of the group of the session's id when its table repeats the id, else of
// the group of the id of the project its path comes from when that is
// repeated; emit itself otherwise or when takes is empty. A row holds one
// place in one group, so a row already placed keeps that place.
func (s *store) memberEmit(reference sessionRef, takes timeline.Takes, emit parserkit.Emit) parserkit.Emit {
	var group *timeline.Group
	if repeated := s.repeats[reference.key]; repeated != nil {
		group = repeated.group
	} else if where, ok := s.sessionAt[reference.key]; ok && s.sessions[where].projectRepeat != nil {
		group = s.sessions[where].projectRepeat.group
	}
	if group == nil || takes == 0 {
		return emit
	}
	return func(row timeline.Row) {
		if row.Group == nil {
			group.Member(&row, takes)
		}
		emit(row)
	}
}

// transcript emits the messages. In a 2.x database session_message is the
// transcript and the 1.x tables are read, as recovered rows, only for a
// session it does not hold (one deleted in 2.0). In a 1.x database message
// and part are the transcript, and session_message, which a migration
// once wiped, is read only for a session with no 1.x message.
func (s *store) transcript() error {
	if s.tables.Has("session_v2") {
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

// readProjects indexes the project table's rows by their id, an id by its
// first row in reading order (storedOrder); an id held in more than one
// row is a repeat (projectRepeat), each of its rows kept.
func (s *store) readProjects() error {
	if !s.tables.Has("project") {
		return nil
	}
	table, err := s.database.Table("project")
	if err != nil {
		return err
	}
	rows, err := table.Select().OrderBy(storedOrder(table)...).All()
	s.projectCollation = parserkit.KeyCollation(table, "id")
	counts := map[string]int{}
	for _, row := range rows {
		if key := s.projectKey(row.Get("id")); key != "" {
			counts[key]++
		}
	}
	for _, row := range rows {
		s.addProject(row, counts)
	}
	return err
}

// addProject indexes one project row's worktree by its id, made once
// however many sessions name the project: "" when it is empty or "/". A
// row whose id is NULL is left out: NULL equals nothing in SQL, so no
// session's project_id finds it. A row of an id counts gives more than
// one row is kept for its `parser:` row, and the id's first row is the
// one indexed.
func (s *store) addProject(row sqlitefile.Row, counts map[string]int) {
	key := s.projectKey(row.Get("id"))
	if key == "" {
		return
	}
	if counts[key] > 1 {
		s.repeatedProjects = append(s.repeatedProjects, row)
		if s.projectRepeats[key] == nil {
			s.projectRepeats[key] = newProjectRepeat(counts[key])
		}
	}
	if _, met := s.projects[key]; met {
		return
	}
	worktree := parserkit.NonEmptyColumn(row.Get("worktree"))
	if worktree == "/" {
		worktree = ""
	}
	s.projects[key] = worktree
}

// readSessions reads session_v2 then session, each ordered by its time
// and id. An id session_v2 holds is left out of session, where a
// migration leaves it; an id one table holds in more than one row is a
// repeat (sessionRepeat), each of its rows kept.
func (s *store) readSessions() error {
	for _, name := range []string{"session_v2", "session"} {
		if !s.tables.Has(name) {
			continue
		}
		table, err := s.database.Table(name)
		if err != nil {
			return err
		}
		rows, err := table.Select().OrderBy(storedOrder(table)...).All()
		if err != nil {
			return err
		}
		if s.sessionCollation == "" {
			s.sessionCollation = parserkit.KeyCollation(table, "id")
		}
		counts := map[string]int{}
		for _, row := range rows {
			key := s.sessionRef(row.Get("id")).key
			if where, seen := s.sessionAt[key]; !seen && key != "" {
				s.sessionAt[key] = len(s.sessions)
			} else if seen && s.sessions[where].table != name {
				continue
			}
			if key != "" {
				counts[key]++
			}
			s.sessions = append(s.sessions, session{identifier: row.Get("id").String(), key: key, row: row,
				table: name, project: s.projectOf(row), projectRepeat: s.projectRepeatOf(row)})
		}
		for key, count := range counts {
			if count > 1 {
				s.repeats[key] = newSessionRepeat(name, count)
			}
		}
	}
	return nil
}

// sessionRef is a reference to a session by a column's value: key is the
// value as the session table's id column compares it, under that
// column's collation ("" for NULL, which names no session), and text the
// id a row shows, "" for an empty value.
type sessionRef struct {
	key, text string
}

// sessionRef returns the reference a column's value makes to a session.
func (s *store) sessionRef(value sqlitefile.Value) sessionRef {
	return sessionRef{key: parserkit.CollatedKey(value, s.sessionCollation), text: parserkit.NonEmptyColumn(value)}
}

// projectKey returns a project id's value as the project table's id
// column compares it, "" for NULL.
func (s *store) projectKey(value sqlitefile.Value) string {
	return parserkit.CollatedKey(value, s.projectCollation)
}

// storedOrder returns the order the stored rows of a project or session
// table are read in, which decides the first stored row of a repeated id:
// oldest first by time_created, ties by id, then by rowid, the order the
// table holds them; a column the table lacks is left out. A message table
// is read session by session, each session's messages in this order
// (session_message by seq first), so the first stored row of a message id
// is its first in the first session that names it.
func storedOrder(table *sqlitefile.Table) []string {
	var keys []string
	for _, column := range []string{"time_created", "id"} {
		if table.Has(column) {
			keys = append(keys, column)
		}
	}
	return append(keys, "rowid")
}

// scopeFor returns the scope of a session, made on first use: its project
// path is the session's directory, else its project's worktree unless "/".
// The rows naming no session (a NULL reference) share one scope.
func (s *store) scopeFor(reference sessionRef) *scope {
	if found, ok := s.scopes[reference.key]; ok {
		return found
	}
	path := ""
	if where, ok := s.sessionAt[reference.key]; ok && reference.key != "" {
		path = s.sessions[where].project
	}
	made := newScope(reference.text, path)
	s.scopes[reference.key] = made
	return made
}

// projectOf returns the project path of a session row: its directory, else
// its project's worktree unless "/".
func (s *store) projectOf(record sqlitefile.Row) string {
	if path := parserkit.NonEmptyColumn(record.Get("directory")); path != "" {
		return path
	}
	return s.worktreeOf(record.Get("project_id"))
}

// projectRepeatOf returns the group of the project id a session row's
// project path comes from, nil when the row names its own directory or
// the id is not repeated.
func (s *store) projectRepeatOf(record sqlitefile.Row) *projectRepeat {
	if parserkit.NonEmptyColumn(record.Get("directory")) != "" {
		return nil
	}
	return s.projectRepeats[s.projectKey(record.Get("project_id"))]
}

// worktreeOf returns the worktree of the project with the given id, "" when
// there is none or it is "/"; a NULL id has no project.
func (s *store) worktreeOf(identifier sqlitefile.Value) string {
	key := s.projectKey(identifier)
	if key == "" {
		return ""
	}
	return s.projects[key]
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
	return s.owner.systemRow(s.artifact, scoped, int(line), parserkit.ColumnTime(when), text)
}
