package opencode

import (
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// sessionMessageIDs returns the ids of the sessions with a session_message
// row.
func (s *store) sessionMessageIDs() (map[string]bool, error) {
	ids := map[string]bool{}
	if !s.tables.Has("session_message") {
		return ids, nil
	}
	table, err := s.database.Table("session_message")
	if err != nil {
		return nil, err
	}
	rows, err := table.Select("session_id").All()
	for _, row := range rows {
		ids[parser.NonEmptyColumn(row.Get("session_id"))] = true
	}
	return ids, err
}

// v1Messages emits the rows of the 1.x message and part tables for every
// session not in skip, marking every row but a system row recovered when
// asked, and returns the ids of the sessions with a message row. A message
// whose data does not decode is a problem of the database; a part's is a
// `parser:` row after its message's rows.
func (s *store) v1Messages(skip map[string]bool, recovered bool) (map[string]bool, error) {
	covered := map[string]bool{}
	if !s.tables.Has("message") {
		return covered, nil
	}
	table, err := s.database.Table("message")
	if err != nil {
		return nil, err
	}
	rows, err := table.Select("id", "session_id", "time_created", "data").
		OrderBy("session_id, time_created, id").All()
	if err != nil {
		return nil, err
	}
	emit := s.emit
	if recovered {
		emit = markRecovered(s.emit)
	}
	var parts *sqlitefile.Lookup
	for _, row := range rows {
		identifier := parser.NonEmptyColumn(row.Get("session_id"))
		covered[identifier] = true
		if skip[identifier] {
			continue
		}
		message, ok := s.decodeColumn(row, "data", "message.data", identifier)
		if !ok {
			continue
		}
		if parts == nil && s.tables.Has("part") {
			if parts, err = s.partLookup(); err != nil {
				return nil, err
			}
		}
		found, failures := messageParts(parts, row)
		where := origin{line: int(row.Rowid), time: parser.ColumnTime(row.Get("time_created"))}
		s.owner.messageRows(s.artifact, s.scopeFor(identifier), message, where, found, s.options, failures, emit)
	}
	return covered, nil
}

// markRecovered returns an emit that marks every row but a system row
// recovered before passing it to emit.
func markRecovered(emit parser.Emit) parser.Emit {
	return func(row timeline.Row) {
		if row.TurnType != timeline.TurnSystem {
			row.Recovered = "yes"
		}
		emit(row)
	}
}

// partLookup returns the part table's rows ordered by id, by message id.
// It pairs a part with its message as a join of the two id columns
// compares them, which is how a bound message id compares too while both
// columns hold text, as OpenCode's schema declares them.
func (s *store) partLookup() (*sqlitefile.Lookup, error) {
	table, err := s.database.Table("part")
	if err != nil {
		return nil, err
	}
	return table.Select("time_created", "data").OrderBy("id").Lookup("message_id")
}

// messageParts returns the decoded parts of a message row, and a failure
// per part whose data does not decode.
func messageParts(parts *sqlitefile.Lookup, message sqlitefile.Row) ([]part, []failure) {
	if parts == nil {
		return nil, nil
	}
	var found []part
	var failures []failure
	for _, row := range parts.Find(message, "id") {
		data, raw, err := decodeRecord(row.Get("data"))
		if err != nil {
			failures = append(failures, failure{line: int(row.Rowid),
				message: unparseable(row.Rowid, "part.data", err), raw: raw})
			continue
		}
		found = append(found, part{data: data,
			where: origin{line: int(row.Rowid), time: parser.ColumnTime(row.Get("time_created"))}})
	}
	return found, failures
}

// v2Messages emits the rows of session_message for every session not in
// skip, an assistant message's content replaced by its session_part items
// when it has any. A record whose data does not decode is a problem.
func (s *store) v2Messages(skip map[string]bool) error {
	if !s.tables.Has("session_message") {
		return nil
	}
	items, err := s.sessionParts(skip)
	if err != nil {
		return err
	}
	table, err := s.database.Table("session_message")
	if err != nil {
		return err
	}
	rows, err := table.Select("id", "session_id", "type", "seq", "time_created", "data").
		OrderBy("session_id, seq, time_created").All()
	if err != nil {
		return err
	}
	for _, row := range rows {
		identifier := parser.NonEmptyColumn(row.Get("session_id"))
		if skip[identifier] {
			continue
		}
		data, ok := s.decodeColumn(row, "data", "session_message.data", identifier)
		if !ok {
			continue
		}
		kind := row.Get("type")
		found := items[parser.NonEmptyColumn(row.Get("id"))]
		if len(found) > 0 && kind.Type == sqlitefile.Text && kind.Text() == "assistant" &&
			data.Has("content") {
			data.Set("content", jsonvalue.NewArray(found...))
		}
		name := parser.NonEmptyColumn(kind)
		if name == "" {
			name = parser.Text(data.Get("type"))
		}
		s.owner.recordRows(s.artifact, s.scopeFor(identifier), name, data, int(row.Rowid),
			parser.ColumnTime(row.Get("time_created")), s.options, s.emit)
	}
	return nil
}

// sessionParts returns the session_part items of each message id, in
// ordinal order, for every session not in skip. A row's type column
// overrides the item's own. A row whose data does not decode is a problem.
func (s *store) sessionParts(skip map[string]bool) (map[string][]jsonvalue.Value, error) {
	items := map[string][]jsonvalue.Value{}
	if !s.tables.Has("session_part") {
		return items, nil
	}
	table, err := s.database.Table("session_part")
	if err != nil {
		return nil, err
	}
	rows, err := table.Select("message_id", "session_id", "type", "data").
		OrderBy("message_id, ordinal").All()
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		identifier := parser.NonEmptyColumn(row.Get("session_id"))
		if skip[identifier] {
			continue
		}
		data, ok := s.decodeColumn(row, "data", "session_part.data", identifier)
		if !ok {
			continue
		}
		if kind := row.Get("type"); !kind.IsEmpty() {
			data.Set("type", columnValue(kind))
		}
		message := parser.NonEmptyColumn(row.Get("message_id"))
		items[message] = append(items[message], data)
	}
	return items, nil
}

// columnValue returns a column value as the JSON value of the same type:
// NULL as null, a number as a number, and text as a string; a blob, which
// no store writes there, is held as its text.
func columnValue(value sqlitefile.Value) jsonvalue.Value {
	switch value.Type {
	case sqlitefile.Integer:
		return jsonvalue.NewInt(value.Int)
	case sqlitefile.Float:
		return jsonvalue.NewFloat(value.Float)
	case sqlitefile.Null:
		return jsonvalue.NewNull()
	}
	return jsonvalue.NewString(value.Text())
}
