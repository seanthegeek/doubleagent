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
// `parser:` row after its message's rows. An id's parts are gathered for
// its first row only: an id held in more than one row is a repeat, its
// rows the parents of one group and its parts' rows members.
func (s *store) v1Messages(skip map[string]bool, recovered bool) (map[string]bool, error) {
	covered := map[string]bool{}
	if !s.tables.Has("message") {
		return covered, nil
	}
	stored, err := s.v1Rows(skip, covered)
	if err != nil {
		return nil, err
	}
	emit := s.emit
	if recovered {
		emit = markRecovered(s.emit)
	}
	counts := partCounts(stored)
	repeats := map[int64]*repeat{}
	for _, each := range stored {
		var found []part
		var failures []failure
		var repeated *repeat
		if each.hasParts {
			repeated = repeats[each.firstPart]
			if repeated == nil {
				found, failures = messageParts(s.parts, each.row)
				if counts[each.firstPart] > 1 {
					repeated = newRepeat("message", counts[each.firstPart])
					repeats[each.firstPart] = repeated
				}
			}
		}
		where := origin{line: int(each.row.Rowid), time: parser.ColumnTime(each.row.Get("time_created"))}
		scoped := s.scopeFor(each.session)
		s.owner.messageRows(s.artifact, scoped, each.message, where, found, s.options, failures, repeated,
			s.memberEmit(each.session, sessionTakes(scoped, each.message), emit))
	}
	return covered, nil
}

// sessionTakes returns what the rows of a 1.x message take from the stored
// row of a repeated session id their group goes with: its project, unless
// the session's first row names none and the message gives its own
// path.cwd, which the rows then keep.
func sessionTakes(scoped *scope, message jsonvalue.Value) timeline.Takes {
	if scoped.projectPath == "" && ownDirectory(message) != "" {
		return 0
	}
	return timeline.TakeProject
}

// partCounts returns the number of message rows of each id with parts, by
// the rowid of its first part.
func partCounts(stored []storedMessage) map[int64]int {
	counts := map[int64]int{}
	for _, each := range stored {
		if each.hasParts {
			counts[each.firstPart]++
		}
	}
	return counts
}

// storedMessage is one decoded 1.x message row: its session id, and
// whether its id has parts, with the rowid of the first part row, which
// names the id's parts: each part row belongs to one id.
type storedMessage struct {
	row       sqlitefile.Row
	message   jsonvalue.Value
	session   string
	hasParts  bool
	firstPart int64
}

// v1Rows returns the decoded message rows of the sessions not in skip, in
// reading order, adding every session with a message row to covered.
func (s *store) v1Rows(skip, covered map[string]bool) ([]storedMessage, error) {
	table, err := s.database.Table("message")
	if err != nil {
		return nil, err
	}
	rows, err := table.Select("id", "session_id", "time_created", "data").
		OrderBy("session_id, time_created, id").All()
	if err != nil {
		return nil, err
	}
	var stored []storedMessage
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
		if s.parts == nil && s.tables.Has("part") {
			if s.parts, err = s.partLookup(); err != nil {
				return nil, err
			}
		}
		each := storedMessage{row: row, message: message, session: identifier}
		if s.parts != nil {
			if first, found := s.parts.First(row, "id"); found {
				each.hasParts, each.firstPart = true, first.Rowid
			}
		}
		stored = append(stored, each)
	}
	return stored, nil
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
// when it has any, for the first row of its id only. A record whose data
// does not decode is a problem. An id more than one such record claims
// is a repeat: its records are the parents of one group, its items' rows
// members, and each record's own content rows those the items replace.
func (s *store) v2Messages(skip map[string]bool) error {
	if !s.tables.Has("session_message") {
		return nil
	}
	items, err := s.sessionParts(skip)
	if err != nil {
		return err
	}
	stored, err := s.v2Rows(skip, items)
	if err != nil {
		return err
	}
	counts := claimCounts(stored)
	repeats, shown := map[string]*repeat{}, map[string]bool{}
	for _, each := range stored {
		var content itemRows
		if each.claims {
			first := !shown[each.message]
			shown[each.message] = true
			switch {
			case counts[each.message] > 1:
				if repeats[each.message] == nil {
					repeats[each.message] = newRepeat("session_message", counts[each.message])
				}
				content.repeated = repeats[each.message]
				if first {
					content.items = items[each.message]
				}
			case first:
				each.data.Set("content", jsonvalue.NewArray(items[each.message]...))
			}
		}
		s.owner.recordRows(s.artifact, s.scopeFor(each.session), each.kind, each.data, int(each.row.Rowid),
			parser.ColumnTime(each.row.Get("time_created")), s.options, content,
			s.memberEmit(each.session, timeline.TakeProject, s.emit))
	}
	return nil
}

// claimCounts returns the number of records claiming each message id's
// items.
func claimCounts(stored []storedRecord) map[string]int {
	counts := map[string]int{}
	for _, each := range stored {
		if each.claims {
			counts[each.message]++
		}
	}
	return counts
}

// storedRecord is one decoded session_message row: its session, its type,
// its message id, and whether it claims the id's session_part items (an
// assistant record with content, of an id that has items).
type storedRecord struct {
	row                    sqlitefile.Row
	data                   jsonvalue.Value
	session, kind, message string
	claims                 bool
}

// v2Rows returns the decoded session_message rows of the sessions not in
// skip, in reading order.
func (s *store) v2Rows(skip map[string]bool, items map[string][]jsonvalue.Value) ([]storedRecord, error) {
	table, err := s.database.Table("session_message")
	if err != nil {
		return nil, err
	}
	rows, err := table.Select("id", "session_id", "type", "seq", "time_created", "data").
		OrderBy("session_id, seq, time_created").All()
	if err != nil {
		return nil, err
	}
	var stored []storedRecord
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
		each := storedRecord{row: row, data: data, session: identifier, kind: parser.NonEmptyColumn(kind),
			message: parser.NonEmptyColumn(row.Get("id"))}
		each.claims = len(items[each.message]) > 0 && kind.Type == sqlitefile.Text && kind.Text() == "assistant" &&
			data.Has("content")
		if each.kind == "" {
			each.kind = parser.Text(data.Get("type"))
		}
		stored = append(stored, each)
	}
	return stored, nil
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
