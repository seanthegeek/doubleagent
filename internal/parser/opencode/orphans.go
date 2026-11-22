package opencode

import (
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// orphanPrefix starts the system row of a plain text part of no message
// read here: either side writes text parts, and only the message says
// which, so the row names no speaker.
const orphanPrefix = "part of no message read here: "

// orphanParts emits the rows of each 1.x part row that no message read
// here took (consumed holds the rowids of those taken): one whose
// message_id names no message row, or is NULL, or names a message whose
// data did not decode. A part is evidence of its own, so its rows are
// shown with the attributes it has itself, its session_id, the project of
// that session and its own time, and no model; a plain text part is a
// system row (orphanPrefix), the other types keep the rows their type
// gives. One `parser:` row counts them. A part of a message in a session
// skip leaves out is the 1.x copy of a session read elsewhere.
func (s *store) orphanParts(consumed map[int]bool, skip map[string]bool, emit parser.Emit) error {
	if !s.tables.Has("part") {
		return nil
	}
	table, err := s.database.Table("part")
	if err != nil {
		return err
	}
	rows, err := table.Select().OrderBy("id").All()
	if err != nil {
		return err
	}
	shown := 0
	for _, row := range rows {
		if consumed[int(row.Rowid)] {
			continue
		}
		skipped, err := s.inSkippedMessage(row, skip)
		if err != nil {
			return err
		}
		if skipped {
			continue
		}
		shown++
		reference := s.sessionRef(row.Get("session_id"))
		data, ok := s.decodeColumn(row, "data", "part.data", reference)
		if !ok {
			continue
		}
		builder := &partRows{owner: s.owner, artifact: s.artifact, scoped: s.scopeFor(reference), options: s.options,
			orphan: true}
		builder.parts([]part{{data: data, where: origin{line: int(row.Rowid),
			time: parser.ColumnTime(row.Get("time_created"))}}}, false)
		parser.EmitAll(s.memberEmit(reference, timeline.TakeProject, emit), builder.rows)
	}
	if shown > 0 {
		emit(s.owner.ProblemRow(s.artifact, orphanText("part", shown, "message", "message")))
	}
	return nil
}

// inSkippedMessage reports whether a part names a message of a session
// skip leaves out. The message table is indexed on first use, so a
// database whose parts all have their message builds no index.
func (s *store) inSkippedMessage(row sqlitefile.Row, skip map[string]bool) (bool, error) {
	if len(skip) == 0 {
		return false, nil
	}
	if s.messages == nil {
		table, err := s.database.Table("message")
		if err != nil {
			return false, err
		}
		if s.messages, err = table.Select("session_id").Lookup("id"); err != nil {
			return false, err
		}
	}
	message, found := s.messages.First(row, "message_id")
	return found && skip[s.sessionRef(message.Get("session_id")).key], nil
}

// orphanItems emits the rows of each session_part item that no assistant
// record read here claims (stored holds the records read): one whose
// message_id names no record, or is NULL, or names a record that does not
// take items. An item is evidence of its own, so its rows are shown as an
// assistant record's content, the only records items belong to, with the
// item's session_id and that session's project, no model, and its own
// times, its row's time_created where it has none inside it. One
// `parser:` row counts them.
func (s *store) orphanItems(items sessionItems, stored []storedRecord) {
	claimed := map[string]bool{}
	for _, each := range stored {
		if each.claims {
			claimed[each.message] = true
		}
	}
	shown := 0
	for _, item := range items.rows {
		if item.message != "" && claimed[item.message] {
			continue
		}
		shown++
		builder := &recordRows{owner: s.owner, artifact: s.artifact, scoped: s.scopeFor(item.session),
			line: int(item.rowid), timestamp: item.created, options: s.options,
			emit: s.memberEmit(item.session, timeline.TakeProject, s.emit)}
		builder.items(jsonvalue.NewArray(item.data))
	}
	if shown > 0 {
		s.emit(s.owner.ProblemRow(s.artifact, orphanText("session_part", shown, "assistant record", "record")))
	}
}

// orphanText returns the problem text counting the rows of table that no
// owner read here takes: "<table> holds <n> rows naming no <owner> read
// here; their rows are shown with their own session_id and no <short>".
func orphanText(table string, count int, owner, short string) string {
	rows := strconv.Itoa(count) + " rows"
	if count == 1 {
		rows = "1 row"
	}
	return table + " holds " + rows + " naming no " + owner + " read here; their rows are shown with their own " +
		"session_id and no " + short
}
