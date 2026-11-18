package openclaw

import (
	"cmp"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// sourceLine returns a column value as a row's source line: an integer as
// it is, anything else as 0.
func sourceLine(value sqlitefile.Value) int {
	if value.Type == sqlitefile.Integer {
		return int(value.Int)
	}
	return 0
}

// pending emits the prompts of session_pending_inputs the transcript
// lacks. One never consumed never reached the transcript and gives its user
// row and a `pending input:` row; a consumed one gives its user row, marked
// recovered, only when the transcript entry that consumed it is not in the
// database. A message_json that does not decode adds a problem.
func (a *agentDatabase) pending() error {
	rows, err := a.table("session_pending_inputs", func(table *sqlitefile.Table) sqlitefile.Query {
		return table.Select("seq", "input_id", "session_id", "run_id", "message_json", "state",
			"accepted_at", "consumed_event_id").OrderBy("seq")
	})
	if err != nil {
		return err
	}
	for _, record := range rows {
		sessionID := parser.NonEmptyColumn(record.Get("session_id"))
		consumed := record.Get("consumed_event_id")
		if !consumed.IsEmpty() && a.store.ids[[2]string{sessionID, consumed.String()}] {
			continue
		}
		line := sourceLine(record.Get("seq"))
		raw := parser.TextColumn(record.Get("message_json"))
		message, err := jsonvalue.DecodeString(raw)
		if err != nil {
			a.problems.AddRaw(line, "session_pending_inputs message_json unparseable ("+err.Error()+")", raw)
			continue
		}
		timestamp := parser.ColumnTime(record.Get("accepted_at"))
		row := a.contextRow(sessionID, line, timestamp, timeline.TurnUser,
			parser.TextOf(message.Get("content")))
		if !consumed.IsEmpty() {
			row.Recovered = "yes"
			a.emit(row)
			continue
		}
		a.emit(row)
		a.emit(a.contextRow(sessionID, line, timestamp, timeline.TurnSystem, "pending input: "+
			cmp.Or(parser.NonEmptyColumn(record.Get("state")), "-")+", not consumed by the transcript (input "+
			cmp.Or(parser.NonEmptyColumn(record.Get("input_id")), "-")+" run "+
			cmp.Or(parser.NonEmptyColumn(record.Get("run_id")), "-")+
			"; from openclaw-agent.sqlite session_pending_inputs, accepted_at)"))
	}
	return nil
}

// fullText emits the entries of the full-text index the transcript no
// longer holds, marked recovered. The index session_transcript_fts is read
// from its content table (c0 text, c1 session_id, c2 message_id, c3 role,
// c4 timestamp) as a plain table, matched on (session_id, message_id).
func (a *agentDatabase) fullText() error {
	table, err := a.database.Table("session_transcript_fts_content")
	if err != nil {
		return err
	}
	for _, column := range []string{"id", "c0", "c1", "c2", "c3", "c4"} {
		if !table.Has(column) {
			return nil
		}
	}
	rows, err := table.Select("id", "c0", "c1", "c2", "c3", "c4").OrderBy("id").All()
	if err != nil {
		return err
	}
	for _, record := range rows {
		sessionID, messageID := parser.NonEmptyColumn(record.Get("c1")), parser.NonEmptyColumn(record.Get("c2"))
		if messageID != "" && a.store.ids[[2]string{sessionID, messageID}] {
			continue
		}
		role := parser.NonEmptyColumn(record.Get("c3"))
		text := ""
		if value := record.Get("c0"); !value.IsNull() {
			text = timeline.Compact(value.String())
		}
		turnType := timeline.TurnSystem
		if role == "user" || role == "assistant" {
			turnType = role
		} else {
			text = cmp.Or(role, "message") + ": " + text
		}
		row := a.contextRow(sessionID, sourceLine(record.Get("id")),
			parser.ColumnTime(record.Get("c4")), turnType, text)
		row.Recovered = "yes"
		a.emit(row)
	}
	return nil
}

// deliveries emits one `channel delivery:` row per send or turn to a
// platform conversation, by created_at, then a `channel delivery reply:`
// row for the platform message that replied to it. The session is the
// current generation of source_session_key in session_nodes.
func (a *agentDatabase) deliveries(nodes []node) error {
	if !a.names["conversations"] {
		return nil
	}
	current := map[string]string{}
	for _, each := range nodes {
		current[each.sessionKey.String()] = each.currentSessionID.String()
	}
	conversations, err := a.database.Table("conversations")
	if err != nil {
		return err
	}
	byID, err := conversations.Select("channel").Lookup("conversation_id")
	if err != nil {
		return err
	}
	rows, err := a.table("conversation_deliveries", func(table *sqlitefile.Table) sqlitefile.Query {
		return table.Select().OrderBy("created_at", "rowid")
	})
	if err != nil {
		return err
	}
	for _, row := range rows {
		for _, conversation := range byID.Find(row, "conversation_id") {
			a.delivery(row, conversation.Get("channel"), current[parser.NonEmptyColumn(row.Get("source_session_key"))])
		}
	}
	return nil
}

// delivery emits the rows of one delivery to a conversation of channel.
func (a *agentDatabase) delivery(row sqlitefile.Row, channel sqlitefile.Value, sessionID string) {
	field := func(name string) string { return cmp.Or(parser.NonEmptyColumn(row.Get(name)), "-") }
	rejection := ""
	if value := row.Get("rejection_error"); !value.IsEmpty() {
		rejection = " error=" + value.String()
	}
	line := int(row.Rowid)
	a.emit(a.contextRow(sessionID, line, parser.ColumnTime(row.Get("created_at")), timeline.TurnSystem,
		"channel delivery: "+field("operation_kind")+" "+field("status")+" channel "+
			cmp.Or(parser.NonEmptyColumn(channel), "-")+" conversation "+field("conversation_id")+
			" platform message "+field("platform_message_id")+rejection+
			" (from openclaw-agent.sqlite conversation_deliveries, created_at)"))
	if reply := row.Get("reply_text"); !reply.IsNull() {
		a.emit(a.contextRow(sessionID, line, parser.ColumnTime(row.Get("reply_timestamp")),
			timeline.TurnSystem, "channel delivery reply: "+reply.String()+" (reply message "+
				field("reply_message_id")+
				"; from openclaw-agent.sqlite conversation_deliveries, reply_timestamp)"))
	}
}

// tombstones emits one row per session `openclaw memory forget` removed
// from the memory index, by created_at.
func (a *agentDatabase) tombstones() error {
	rows, err := a.table("memory_session_tombstones", func(table *sqlitefile.Table) sqlitefile.Query {
		return table.Select("session_id", "agent_id", "reason", "created_at").OrderBy("created_at", "rowid")
	})
	if err != nil {
		return err
	}
	for _, row := range rows {
		sessionID := parser.NonEmptyColumn(row.Get("session_id"))
		a.emit(a.contextRow(sessionID, int(row.Rowid), parser.ColumnTime(row.Get("created_at")),
			timeline.TurnSystem, "session forgotten from memory: "+cmp.Or(sessionID, "-")+" agent "+
				cmp.Or(parser.NonEmptyColumn(row.Get("agent_id")), "-")+" reason="+
				cmp.Or(parser.NonEmptyColumn(row.Get("reason")), "-")+
				" (from openclaw-agent.sqlite memory_session_tombstones, created_at)"))
	}
	return nil
}
