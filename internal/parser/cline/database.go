package cline

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/reason"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// openDatabase opens a database the parser reads (parser.OpenDatabase), or
// returns an error whose text is the row that says why it cannot, with the
// database's name where the kit's words have none: "<name> unreadable
// (<reason>)" for one that failed as it was opened, "<name> is not a SQLite
// database", and the unreadable message of the file or a sidecar.
func openDatabase(path, name string) (*sqlitefile.Database, error) {
	database, err := parser.OpenDatabase(path)
	var openErr *parser.OpenError
	switch {
	case err == nil:
		return database, nil
	case errors.As(err, &openErr):
		return nil, unreadable(name, openErr.Err)
	case errors.Is(err, sqlitefile.ErrNotDatabase):
		return nil, errors.New(name + " is not a SQLite database")
	}
	return nil, reason.Wrap(parser.DatabaseError(err), err)
}

// unreadable returns the error of a database that failed while it was
// read.
func unreadable(name string, err error) error {
	return fmt.Errorf("%s unreadable (%w)", name, err)
}

// readSessions returns every row of the sessions table in rowid order, an
// empty list when there is no such table, or an error whose text is the
// row that replaces them.
func readSessions(path string) ([]sqlitefile.Row, error) {
	database, err := openDatabase(path, "sessions.db")
	if err != nil {
		return nil, err
	}
	defer database.Close()
	names, err := parser.TablesOf(database)
	if err != nil {
		return nil, unreadable("sessions.db", err)
	}
	if !names.Has("sessions") {
		return nil, nil
	}
	table, err := database.Table("sessions")
	if err == nil {
		var rows []sqlitefile.Row
		rows, err = table.Select().OrderBy("rowid").All()
		if err == nil {
			return rows, nil
		}
	}
	return nil, unreadable("sessions.db", err)
}

// sessionsDatabase emits a session index row, with its model and blob
// upload rows, per row of db/sessions.db whose manifest is gone.
func (p *Parser) sessionsDatabase(artifact *input.Artifact, emit parser.Emit) {
	rows, err := readSessions(artifact.DiskPath)
	if err != nil {
		emit(p.ProblemRow(artifact, err.Error()))
		return
	}
	keys := append(startKeys(), "ended_at", "exit_code")
	for _, record := range rows {
		session := parser.RowObject(record)
		if manifestPresent(artifact, parser.Text(session.Get("session_id"))) {
			continue
		}
		meta := metadataJSON(record.Get("metadata_json"))
		addMetadata(session, meta)
		start := p.sessionStart(artifact, session, int(record.Rowid), "started_at", "session index", keys)
		p.startRows(artifact, start, session, meta, "sessions.db sessions.metadata_json", emit)
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

// ordinal returns a search document's ordinal: an integer, a float cut
// to its whole part, or text holding a decimal integer with spaces around
// it; -1 when it is none of those.
func ordinal(value sqlitefile.Value) int {
	switch value.Type {
	case sqlitefile.Integer:
		return int(value.Int)
	case sqlitefile.Float:
		return int(value.Float)
	case sqlitefile.Text, sqlitefile.Blob:
		if number, err := strconv.Atoi(strings.TrimSpace(string(value.Bytes))); err == nil {
			return number
		}
	}
	return -1
}
