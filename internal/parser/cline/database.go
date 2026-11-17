package cline

import (
	"errors"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// openDatabase opens a database the parser reads, or returns the message
// of the row that says why it cannot: "<name> is not a SQLite database",
// the unreadable message of the file or a sidecar, or "<name> unreadable
// (<reason>)".
func openDatabase(path, name string) (*sqlitefile.Database, string) {
	if err := sqlitefile.Problem(path); err != nil {
		if errors.Is(err, sqlitefile.ErrNotDatabase) {
			return nil, name + " is not a SQLite database"
		}
		return nil, parser.DatabaseError(err)
	}
	database, err := sqlitefile.OpenPath(path)
	if err != nil {
		return nil, unreadable(name, err)
	}
	return database, ""
}

// unreadable returns the message of a database that failed while it was
// read.
func unreadable(name string, err error) string {
	return name + " unreadable (" + err.Error() + ")"
}

// columnValue returns a column value as the JSON value Python's sqlite3
// module would give the parser: a number, text, null, or for a blob the
// text Python's str() writes for bytes.
func columnValue(value sqlitefile.Value) jsonvalue.Value {
	switch value.Type {
	case sqlitefile.Integer:
		return jsonvalue.NewInt(value.Int)
	case sqlitefile.Float:
		return jsonvalue.NewFloat(value.Float)
	case sqlitefile.Text, sqlitefile.Blob:
		return jsonvalue.NewString(value.String())
	}
	return jsonvalue.NewNull()
}

// rowObject returns a database row as an object of its columns.
func rowObject(row sqlitefile.Row) jsonvalue.Value {
	object := jsonvalue.NewObject()
	for _, column := range row.Columns() {
		object.Set(column, columnValue(row.Get(column)))
	}
	return object
}

// readSessions returns every row of the sessions table in rowid order, an
// empty list when there is no such table, or the message of the row that
// replaces them.
func readSessions(path string) ([]sqlitefile.Row, string) {
	database, message := openDatabase(path, "sessions.db")
	if message != "" {
		return nil, message
	}
	defer database.Close()
	names, err := database.TableNames()
	if err != nil {
		return nil, unreadable("sessions.db", err)
	}
	if !names["sessions"] {
		return nil, ""
	}
	table, err := database.Table("sessions")
	if err == nil {
		var rows []sqlitefile.Row
		rows, err = table.Select().OrderBy("rowid").All()
		if err == nil {
			return rows, ""
		}
	}
	return nil, unreadable("sessions.db", err)
}

// sessionsDatabase emits a session index row, with its model and blob
// upload rows, per row of db/sessions.db whose manifest is gone.
func (p *Parser) sessionsDatabase(artifact *input.Artifact, emit parser.Emit) {
	rows, message := readSessions(artifact.DiskPath)
	if message != "" {
		emit(p.ProblemRow(artifact, message))
		return
	}
	keys := append(startKeys(), "ended_at", "exit_code")
	for _, record := range rows {
		session := rowObject(record)
		if manifestPresent(artifact, parser.Str(session.Get("session_id"))) {
			continue
		}
		meta := metadataJSON(record.Get("metadata_json"))
		addMetadata(session, meta)
		start := p.sessionStart(artifact, session, int(record.Rowid), "started_at", "session index", keys)
		p.startRows(start, session, meta, "sessions.db sessions.metadata_json", emit)
	}
}

// manifestPresent reports whether the session's manifest,
// sessions/<id>/<id>.json beside the db directory, is a regular file. An id
// that is not a plain file name has no manifest.
func manifestPresent(artifact *input.Artifact, sessionID string) bool {
	name, named := parser.Component(sessionID)
	manifest, found := parser.Above(artifact, 1, "sessions", name, name+".json")
	return named && found && parser.RegularFile(manifest)
}

// metadataJSON returns a metadata_json column decoded as JSON, or null for
// an empty or absent value, a number, or text that is not JSON.
func metadataJSON(value sqlitefile.Value) jsonvalue.Value {
	if value.IsEmpty() {
		return jsonvalue.NewNull()
	}
	var decoded jsonvalue.Value
	var err error
	switch value.Type {
	case sqlitefile.Text:
		decoded, err = jsonvalue.DecodeString(value.Text())
	case sqlitefile.Blob:
		decoded, err = jsonvalue.Decode(value.Bytes)
	default:
		return jsonvalue.NewNull()
	}
	if err != nil {
		return jsonvalue.NewNull()
	}
	return decoded
}

// ordinal returns a search document's ordinal as Python's int() reads it,
// -1 when it is not a number.
func ordinal(value sqlitefile.Value) int {
	switch value.Type {
	case sqlitefile.Integer:
		return int(value.Int)
	case sqlitefile.Float:
		return int(value.Float)
	case sqlitefile.Text, sqlitefile.Blob:
		text := strings.ReplaceAll(strings.TrimSpace(string(value.Bytes)), "_", "")
		if number, err := strconv.Atoi(text); err == nil {
			return number
		}
	}
	return -1
}
