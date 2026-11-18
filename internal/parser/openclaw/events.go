package openclaw

import (
	"cmp"
	"fmt"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// columnBytes returns the bytes of a text or blob column, nil for any
// other value.
func columnBytes(value sqlitefile.Value) []byte {
	if value.Type == sqlitefile.Text || value.Type == sqlitefile.Blob {
		return value.Bytes
	}
	return nil
}

// events emits the entries of transcript_events, by session then seq, each
// session described by its window.
func (a *agentDatabase) events() error {
	rows, err := a.table("transcript_events", func(table *sqlitefile.Table) sqlitefile.Query {
		return table.Select("session_id", "seq", "event_json", "event_zstd", "created_at").
			OrderBy("session_id", "seq")
	})
	if err != nil {
		return err
	}
	var current *session
	for _, row := range rows {
		sessionID := parser.NonEmptyColumn(row.Get("session_id"))
		if current == nil || current.sessionID != sessionID {
			current = a.windowSession(sessionID, "", "")
		}
		a.event(current, row)
	}
	return nil
}

// event emits the rows of one transcript_events row: its entry, or the
// `parser:` row of an event that cannot be decompressed or decoded.
func (a *agentDatabase) event(current *session, row sqlitefile.Row) {
	line, createdAt := int(row.Rowid), parser.ColumnTime(row.Get("created_at"))
	seq := row.Get("seq").String()
	raw := row.Get("event_json")
	data := columnBytes(raw)
	if raw.IsNull() && !row.Get("event_zstd").IsNull() {
		decoded, err := maybeZstd(columnBytes(row.Get("event_zstd")), a.budget)
		if err != nil {
			a.startOnce(current, line, createdAt)
			a.bad(current, line, "event seq "+seq+" could not be decompressed ("+err.Error()+")", createdAt)
			return
		}
		data = decoded
	}
	record, err := jsonvalue.Decode(data)
	if err == nil && record.IsObject() {
		a.entry(current, line, record, createdAt)
		return
	}
	reason := "not a JSON object"
	if err != nil {
		reason = err.Error()
	}
	a.startOnce(current, line, createdAt)
	a.bad(current, line, "event seq "+seq+" unreadable: "+reason, createdAt)
}

// startOnce emits the session-start row unless the session has one.
func (a *agentDatabase) startOnce(current *session, line int, timestamp string) {
	if !current.started {
		a.start(current, line, timestamp, jsonvalue.NewObject())
	}
}

// archives emits the transcripts of the reset and deleted generations in
// session_transcript_archives, by created_at, and a `session deleted:` row
// for a deleted one.
func (a *agentDatabase) archives() error {
	rows, err := a.table("session_transcript_archives", func(table *sqlitefile.Table) sqlitefile.Query {
		return table.Select("session_id", "session_key", "reason", "encoding", "archive_name",
			"created_at", "archive_blob").OrderBy("created_at")
	})
	if err != nil {
		return err
	}
	for _, row := range rows {
		sessionID := parser.NonEmptyColumn(row.Get("session_id"))
		name := parser.NonEmptyColumn(row.Get("archive_name"))
		extra := "archive=" + parser.NonEmptyColumn(row.Get("reason")) + " name=" + name
		current := a.windowSession(sessionID, parser.NonEmptyColumn(row.Get("session_key")), extra)
		line, createdAt := int(row.Rowid), parser.ColumnTime(row.Get("created_at"))
		a.blob(current, line, "archive "+cmp.Or(name, sessionID), columnBytes(row.Get("archive_blob")),
			createdAt)
		if parser.TextColumn(row.Get("reason")) == "deleted" {
			a.system(current, line, createdAt, "session deleted: "+cmp.Or(sessionID, "-")+
				" archive "+cmp.Or(name, "-")+
				" (from openclaw-agent.sqlite session_transcript_archives reason deleted, created_at)", "")
		}
	}
	return nil
}

// blob emits the rows of an archive blob, zstd-compressed or not, then the
// problems of its lines, each labelled with the blob's label and line.
// Every row's source line is the rowid of the row that held it.
func (a *agentDatabase) blob(current *session, line int, label string, blob []byte, timestamp string) {
	data, err := maybeZstd(blob, a.budget)
	if err != nil {
		a.bad(current, line, label+" could not be decompressed ("+err.Error()+")", timestamp)
		return
	}
	var problems parser.Problems
	a.records(current, blobLines(data, &problems), line)
	located := make(parser.Problems, len(problems))
	for i, problem := range problems {
		located[i] = parser.Problem{Line: line,
			Message: fmt.Sprintf("%s line %d: %s", label, problem.Line, problem.Message),
			Raw:     problem.Raw}
	}
	parser.EmitAll(a.emit, a.ErrorRows(a.artifact, located, current.sessionID, current.cwd))
}

// cold emits the cold archives stored in the database, by archived_at,
// chunk by chunk when the chunks table exists. Every row of an archive
// carries the archive's rowid, the chunks table having none; an archive or
// chunk whose blob is NULL has lost its bytes and is one `parser:` row.
func (a *agentDatabase) cold() error {
	rows, err := a.table("session_transcript_cold_archives", func(table *sqlitefile.Table) sqlitefile.Query {
		return table.Select("session_id", "archive_name", "archived_at", "archive_blob").
			Where(func(row sqlitefile.Row) bool { return row.Equals("storage", sqlitefile.TextValue("sqlite")) }).
			OrderBy("archived_at")
	})
	if err != nil {
		return err
	}
	for _, row := range rows {
		sessionID := parser.NonEmptyColumn(row.Get("session_id"))
		current := a.windowSession(sessionID, "", "cold_archive="+parser.NonEmptyColumn(row.Get("archive_name")))
		line, archivedAt := int(row.Rowid), parser.ColumnTime(row.Get("archived_at"))
		blobs := []namedBlob{{cmp.Or(parser.NonEmptyColumn(row.Get("archive_name")), sessionID), row.Get("archive_blob")}}
		if a.names["session_transcript_cold_archive_chunks"] {
			chunks, err := a.chunks(sessionID)
			if err != nil {
				return err
			}
			if len(chunks) > 0 {
				blobs = chunks
			}
		}
		for _, each := range blobs {
			if each.blob.IsNull() {
				a.bad(current, line, "cold archive "+each.name+" has no stored bytes", archivedAt)
				continue
			}
			a.blob(current, line, "cold archive "+each.name, columnBytes(each.blob), archivedAt)
		}
	}
	return nil
}

// namedBlob is one stored cold archive or chunk and the name it is
// reported by.
type namedBlob struct {
	name string
	blob sqlitefile.Value
}

// chunks returns the chunks of a session's cold archive, by chunk_index:
// the rows whose session_id is the session id as text, the key the agent
// gives a cold archive and its chunks. The chunks table is read once, on
// first use, and grouped by session id.
func (a *agentDatabase) chunks(sessionID string) ([]namedBlob, error) {
	if a.chunksBySession == nil {
		rows, err := a.table("session_transcript_cold_archive_chunks", func(table *sqlitefile.Table) sqlitefile.Query {
			return table.Select("session_id", "archive_name", "archive_blob").OrderBy("chunk_index")
		})
		if err != nil {
			return nil, err
		}
		a.chunksBySession = map[string][]sqlitefile.Row{}
		for _, row := range rows {
			if key := row.Get("session_id"); key.Type == sqlitefile.Text {
				a.chunksBySession[key.Text()] = append(a.chunksBySession[key.Text()], row)
			}
		}
	}
	rows := a.chunksBySession[sessionID]
	blobs := make([]namedBlob, len(rows))
	for i, row := range rows {
		blobs[i] = namedBlob{cmp.Or(parser.NonEmptyColumn(row.Get("archive_name")), sessionID), row.Get("archive_blob")}
	}
	return blobs, nil
}
