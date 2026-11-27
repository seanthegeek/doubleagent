package opencode

import (
	"cmp"
	"errors"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// shareSecret returns " secret=<secret>" of a share's secret column through
// parserkit.SecretText, "" when it is NULL or empty.
func shareSecret(artifact *input.Artifact, value sqlitefile.Value) string {
	if secret := parserkit.NonEmptyColumn(value); secret != "" {
		return " secret=" + parserkit.SecretText(artifact, secret)
	}
	return ""
}

// shareRows emits a "session synced:" row per session_share row, ordered
// by its time then session: the share's URL and,
// in a 1.x database, the secret that authorises changes to the public
// copy through parserkit.SecretText.
func (s *store) shareRows() error {
	if !s.tables.Has("session_share") {
		return nil
	}
	table, err := s.database.Table("session_share")
	if err != nil {
		return err
	}
	columns := []string{"session_id", "id", "url", "time_created"}
	if table.Has("secret") {
		columns = append(columns, "secret")
	}
	rows, err := table.Select(columns...).OrderBy("time_created, session_id").All()
	if err != nil {
		return err
	}
	for _, row := range rows {
		reference := s.sessionRef(row.Get("session_id"))
		s.shares[reference.key] = true
		emit := s.memberEmit(reference, timeline.TakeProject, s.emit)
		emit(s.systemAt(s.scopeFor(reference), row.Rowid, row.Get("time_created"), timeline.SyncedPrefix+
			" share "+cmp.Or(parserkit.NonEmptyColumn(row.Get("id")), "-")+" url "+
			cmp.Or(parserkit.NonEmptyColumn(row.Get("url")), "-")+
			shareSecret(s.artifact, row.Get("secret"))+" (from "+s.label+" session_share, time_created)"))
	}
	return nil
}

// decodeText returns text decoded to a JSON object, or an error saying why
// it is not one.
func decodeText(artifact *input.Artifact, text string) (jsonvalue.Value, error) {
	value, err := parserkit.DecodeJSON(artifact, text)
	if err != nil {
		return jsonvalue.NewNull(), err
	}
	if !value.IsObject() {
		return jsonvalue.NewNull(), errors.New("not a JSON object")
	}
	return value, nil
}

// decodeRecord returns a stored JSON value decoded to an object and the
// text it was read from, with an error saying why it is not an object.
func decodeRecord(artifact *input.Artifact, stored sqlitefile.Value) (jsonvalue.Value, string, error) {
	text, err := columnJSON(stored)
	if err != nil {
		return jsonvalue.NewNull(), text, err
	}
	value, err := decodeText(artifact, text)
	return value, text, err
}

// decodeColumn returns a row's JSON object column decoded, or false after
// adding the problem "unparseable record <rowid> (<label>: <reason>)",
// with the column's text as raw, for the session the row names.
func (s *store) decodeColumn(row sqlitefile.Row, column, label string, reference sessionRef) (jsonvalue.Value, bool) {
	value, raw, err := decodeRecord(s.artifact, row.Get(column))
	if err != nil {
		s.failures = append(s.failures, failure{line: int(row.Rowid),
			message: unparseable(row.Rowid, label, err), raw: raw, session: reference})
		return jsonvalue.NewNull(), false
	}
	return value, true
}

// eventShareRows emits, for a session with no session_share row, the share
// and the unshare its session.updated.1 events record, ordered by session
// then sequence. An event whose data does not decode is a problem.
func (s *store) eventShareRows() error {
	if !s.tables.Has("event") {
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
		reference := s.sessionRef(row.Get("aggregate_id"))
		identifier := reference.key
		data, ok := s.decodeColumn(row, "data", "event.data", reference)
		if !ok || (identifier != "" && s.shares[identifier]) || state[identifier] == "unshared" {
			continue
		}
		info := parserkit.ObjectOrEmpty(data.Get("info"))
		url := parserkit.ObjectOrEmpty(info.Get("share")).Get("url")
		var text string
		switch _, known := state[identifier]; {
		case !url.IsEmpty() && !known:
			state[identifier], text = "shared", timeline.SyncedPrefix+" share url "+url.Text()
		case url.IsEmpty() && state[identifier] == "shared":
			state[identifier], text = "unshared", "session unshared"
		default:
			continue
		}
		emit := s.memberEmit(reference, timeline.TakeProject, s.emit)
		emit(s.owner.systemRow(s.artifact, s.scopeFor(reference), int(row.Rowid),
			timeutil.FromValue(parserkit.ObjectOrEmpty(info.Get("time")).Get("updated")),
			text+" (from "+s.label+" event session.updated.1, info.time.updated)"))
	}
	return nil
}

// permissionRules returns the rules of a permission column holding text or
// a blob and the number of its entries that are not rules; any other value
// is not a JSON array.
func permissionRules(artifact *input.Artifact, stored sqlitefile.Value) ([]rule, int, bool) {
	switch stored.Type {
	case sqlitefile.Text:
		return sessionRules(artifact, stored.Text())
	case sqlitefile.Blob:
		return sessionRules(artifact, textlines.ValidUTF8(stored.Bytes))
	}
	return nil, 0, false
}

// stateRows emits, per stored session row, a "session archived" row at
// its time_archived and a "permission setting: session ..." row per rule
// of its permission column at its time_updated, the row's own. A
// permission value that is not a JSON array is a problem, and so, counted
// once per value, are its entries that are not rules.
func (s *store) stateRows() error {
	for i := range s.sessions {
		each := &s.sessions[i]
		scoped, rowid := s.rowScope(each), each.row.Rowid
		if archived := each.row.Get("time_archived"); !archived.IsEmpty() {
			s.ownRow(each, s.systemAt(scoped, rowid, archived, "session archived (from "+s.label+" "+
				each.table+", time_archived)"))
		}
		stored := each.row.Get("permission")
		if stored.IsNull() || (stored.Type == sqlitefile.Text && len(stored.Bytes) == 0) {
			continue
		}
		rules, skipped, ok := permissionRules(s.artifact, stored)
		if !ok {
			raw := stored.String()
			if stored.Type == sqlitefile.Blob {
				raw = textlines.ValidUTF8(stored.Bytes)
			}
			s.failures = append(s.failures, failure{line: int(rowid), raw: raw, session: sessionRef{each.key, each.identifier}, owner: each,
				message: unparseable(rowid, each.table+".permission", errors.New("not a JSON array"))})
			continue
		}
		for _, rule := range rules {
			s.ownRow(each, s.systemAt(scoped, rowid, each.row.Get("time_updated"), "permission setting: session "+
				rule.permission+" "+rule.pattern+"="+rule.effect+" (from "+s.label+
				" "+each.table+".permission, time_updated)"))
		}
		if skipped > 0 {
			s.failures = append(s.failures, failure{line: int(rowid), session: sessionRef{each.key, each.identifier}, owner: each,
				message: notRules(each.table+".permission", skipped)})
		}
	}
	return nil
}

// cutRows emits the problem rows of the values carried cut.
func (s *store) cutRows(carried *parserkit.Carried) {
	var problems parserkit.Problems
	carried.Report(0, &problems)
	parserkit.EmitAll(s.emit, s.owner.ErrorRows(s.artifact, problems, "", ""))
}

// projectNamed returns how a worktree's row names its project: the
// project's worktree, cut by carried, "-" when there is none, or, for a
// project id the table repeats, "project <id>": the worktree is text
// inside a row of another id, which no stored row of the project lends.
func (s *store) projectNamed(carried *parserkit.Carried, projectID sqlitefile.Value) string {
	if s.projectRepeats[s.projectKey(projectID)] != nil {
		return "project " + carried.Cut("project id", parserkit.ColumnText(projectID))
	}
	return cmp.Or(carried.Cut("project worktree", s.worktreeOf(projectID)), "-")
}

// worktreeRows emits a "scratch directory:" row per worktree OpenCode
// made: the worktree table's rows, then project_directory's of type
// git_worktree, each table ordered by time and directory, a directory
// named once.
func (s *store) worktreeRows() error {
	seen := map[string]bool{}
	// Each worktree's row repeats its project's worktree.
	carried := parserkit.NewCarried(s.artifact)
	defer s.cutRows(carried)
	for _, name := range []string{"worktree", "project_directory"} {
		if !s.tables.Has(name) {
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
			directory := parserkit.NonEmptyColumn(row.Get("directory"))
			kind := row.Get("type")
			if directory == "" || seen[directory] || (name == "project_directory" &&
				(kind.Type != sqlitefile.Text || kind.Text() != "git_worktree")) {
				continue
			}
			seen[directory] = true
			made := s.systemAt(newScope("", ""), row.Rowid, row.Get("time_created"), timeline.ScratchPrefix+
				" "+directory+" (git worktree of "+
				s.projectNamed(carried, row.Get("project_id"))+" strategy="+
				cmp.Or(parserkit.NonEmptyColumn(row.Get("strategy")), "-")+") (from "+s.label+" "+name+
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
	if !s.tables.Has("permission") {
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
			cmp.Or(parserkit.NonEmptyColumn(row.Get("action")), "-")+" "+cmp.Or(parserkit.NonEmptyColumn(row.Get("resource")), "-")+
			"=allow (from "+s.label+" permission, time_created)")
		made.ProjectPath = s.worktreeOf(row.Get("project_id"))
		s.projectRepeats[s.projectKey(row.Get("project_id"))].member(&made, timeline.TakeProject)
		s.emit(made)
	}
	return nil
}
