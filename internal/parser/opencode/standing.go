package opencode

import (
	"errors"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// shareRows emits a "session synced:" row per session_share row, ordered
// by its time then session. The share's secret is never read.
func (s *store) shareRows() error {
	if !s.tables["session_share"] {
		return nil
	}
	table, err := s.database.Table("session_share")
	if err != nil {
		return err
	}
	rows, err := table.Select("session_id", "id", "url", "time_created").
		OrderBy("time_created, session_id").All()
	if err != nil {
		return err
	}
	for _, row := range rows {
		identifier := columnText(row.Get("session_id"))
		s.shares[identifier] = true
		s.emit(s.systemAt(s.scopeFor(identifier), row.Rowid, row.Get("time_created"), timeline.SyncedPrefix+
			" share "+orDash(columnText(row.Get("id")))+" url "+
			orDash(columnText(row.Get("url")))+" (from "+s.label+
			" session_share, time_created)"))
	}
	return nil
}

// eventShareRows emits, for a session with no session_share row, the share
// and the unshare its session.updated.1 events record, ordered by session
// then sequence. An event whose data does not decode is a problem.
func (s *store) eventShareRows() error {
	if !s.tables["event"] {
		return nil
	}
	table, err := s.database.Table("event")
	if err != nil {
		return err
	}
	updated := sqlitefile.TextValue("session.updated.1")
	rows, err := table.Select("aggregate_id", "data").
		Where(func(row sqlitefile.Row) bool { return row.Equals("type", updated) }).
		OrderBy("aggregate_id, seq").All()
	if err != nil {
		return err
	}
	state := map[string]string{}
	for _, row := range rows {
		identifier := columnText(row.Get("aggregate_id"))
		data, ok := s.decodeColumn(row, "data", "event.data", identifier)
		if !ok || s.shares[identifier] || state[identifier] == "unshared" {
			continue
		}
		info := objectOf(data.Get("info"))
		url := objectOf(info.Get("share")).Get("url")
		var text string
		switch _, known := state[identifier]; {
		case !url.IsEmpty() && !known:
			state[identifier], text = "shared", timeline.SyncedPrefix+" share url "+url.Text()
		case url.IsEmpty() && state[identifier] == "shared":
			state[identifier], text = "unshared", "session unshared"
		default:
			continue
		}
		s.emit(s.owner.systemRow(s.artifact, s.scopeFor(identifier), int(row.Rowid),
			timeutil.FromValue(objectOf(info.Get("time")).Get("updated")),
			text+" (from "+s.label+" event session.updated.1, info.time.updated)"))
	}
	return nil
}

// decodeColumn returns a row's JSON object column decoded, or false after
// adding the problem "unparseable record <rowid> (<label>: <reason>)",
// with the column's text as raw, for the session id.
func (s *store) decodeColumn(row sqlitefile.Row, column, label, sessionID string) (jsonvalue.Value, bool) {
	value, raw, err := decodeRecord(row.Get(column))
	if err != nil {
		s.failures = append(s.failures, failure{line: int(row.Rowid),
			message: unparseable(row.Rowid, label, err), raw: raw, sessionID: sessionID})
		return jsonvalue.NewNull(), false
	}
	return value, true
}

// decodeRecord returns a stored JSON value decoded to an object and the
// text it was read from, with an error saying why it is not an object.
func decodeRecord(stored sqlitefile.Value) (jsonvalue.Value, string, error) {
	text, err := columnJSON(stored)
	if err != nil {
		return jsonvalue.NewNull(), text, err
	}
	value, err := decodeText(text)
	return value, text, err
}

// decodeText returns text decoded to a JSON object, or an error saying why
// it is not one.
func decodeText(text string) (jsonvalue.Value, error) {
	value, err := jsonvalue.DecodeString(text)
	if err != nil {
		return jsonvalue.NewNull(), err
	}
	if !value.IsObject() {
		return jsonvalue.NewNull(), errors.New("not a JSON object")
	}
	return value, nil
}

// stateRows emits, per session, a "session archived" row at its
// time_archived and a "permission setting: session ..." row per rule of
// its permission column at its time_updated. A permission value that is
// not a JSON array is a problem.
func (s *store) stateRows() error {
	for _, each := range s.sessions {
		scoped, rowid := s.scopeFor(each.identifier), each.row.Rowid
		if archived := each.row.Get("time_archived"); !archived.IsEmpty() {
			s.emit(s.systemAt(scoped, rowid, archived, "session archived (from "+s.label+" "+
				each.table+", time_archived)"))
		}
		stored := each.row.Get("permission")
		if stored.IsNull() || (stored.Type == sqlitefile.Text && len(stored.Bytes) == 0) {
			continue
		}
		rules, ok := permissionRules(stored)
		if !ok {
			raw := stored.String()
			if stored.Type == sqlitefile.Blob {
				raw = textlines.ValidUTF8(stored.Bytes)
			}
			s.failures = append(s.failures, failure{line: int(rowid), raw: raw, sessionID: each.identifier,
				message: unparseable(rowid, each.table+".permission", errors.New("not a JSON array"))})
			continue
		}
		for _, rule := range rules {
			s.emit(s.systemAt(scoped, rowid, each.row.Get("time_updated"), "permission setting: session "+
				rule.permission+" "+rule.pattern+"="+rule.effect+" (from "+s.label+
				" "+each.table+".permission, time_updated)"))
		}
	}
	return nil
}

// permissionRules returns the rules of a permission column holding text or
// a blob; any other value is not a JSON array.
func permissionRules(stored sqlitefile.Value) ([]rule, bool) {
	switch stored.Type {
	case sqlitefile.Text:
		return sessionRules(stored.Text())
	case sqlitefile.Blob:
		return sessionRules(textlines.ValidUTF8(stored.Bytes))
	}
	return nil, false
}

// worktreeRows emits a "scratch directory:" row per worktree OpenCode
// made: the worktree table's rows, then project_directory's of type
// git_worktree, each table ordered by time and directory, a directory
// named once.
func (s *store) worktreeRows() error {
	seen := map[string]bool{}
	for _, name := range []string{"worktree", "project_directory"} {
		if !s.tables[name] {
			continue
		}
		table, err := s.database.Table(name)
		if err != nil {
			return err
		}
		rows, err := table.Select().OrderBy("time_created, directory").All()
		if err != nil {
			return err
		}
		for _, row := range rows {
			directory := columnText(row.Get("directory"))
			kind := row.Get("type")
			if directory == "" || seen[directory] || (name == "project_directory" &&
				(kind.Type != sqlitefile.Text || kind.Text() != "git_worktree")) {
				continue
			}
			seen[directory] = true
			made := s.systemAt(newScope("", ""), row.Rowid, row.Get("time_created"), timeline.ScratchPrefix+
				" "+directory+" (git worktree of "+
				orDash(s.worktreeOf(row.Get("project_id")))+" strategy="+
				orDash(columnText(row.Get("strategy")))+") (from "+s.label+" "+name+
				", time_created)")
			made.ProjectPath = directory
			s.emit(made)
		}
	}
	return nil
}

// grantRows emits a "permission setting:" row per saved grant of the
// permission table, ordered by time and id. The first layout's
// permission(project_id, data), without an action or an id, is not read,
// and the rest of the database is read as without the table.
func (s *store) grantRows() error {
	if !s.tables["permission"] {
		return nil
	}
	table, err := s.database.Table("permission")
	if err != nil {
		return err
	}
	if !table.Has("action") {
		return nil
	}
	rows, err := table.Select().OrderBy("time_created, id").All()
	if err != nil {
		return err
	}
	for _, row := range rows {
		made := s.systemAt(newScope("", ""), row.Rowid, row.Get("time_created"), "permission setting: "+
			orDash(columnText(row.Get("action")))+" "+orDash(columnText(row.Get("resource")))+
			"=allow (from "+s.label+" permission, time_created)")
		made.ProjectPath = s.worktreeOf(row.Get("project_id"))
		s.emit(made)
	}
	return nil
}

// orDash returns text, or "-" when it is empty.
func orDash(text string) string {
	if text == "" {
		return "-"
	}
	return text
}
