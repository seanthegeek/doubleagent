package nanobot

import (
	"errors"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parseSession emits the rows of a session JSONL file, then the problems
// met in it and in its namespace's .workspace marker.
func (p *Parser) parseSession(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	match := p.files.session.FindStringSubmatch(artifact.Rel)
	var problems parser.Problems
	project := ""
	if marker, ok := p.namespaceFile(artifact, ".workspace"); ok {
		project = readMarker(marker, &problems)
	}
	var records []numbered
	for line, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		records = append(records, numbered{line, record})
	}
	sessionID := p.sessionRows(artifact, options, DecodeStem(group(p.files.session, match, "stem")),
		project, records, emit)
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, project))
}

// fileRow returns a whole-file `parser:` row with the given project path.
func (p *Parser) fileRow(artifact *input.Artifact, project, message string) timeline.Row {
	row := p.ProblemRow(artifact, message)
	row.ProjectPath = project
	return row
}

// sessionStore holds the rows of a sessions.sqlite3: its sessions by
// created_at then key, and its messages by session key, each key's by
// position.
type sessionStore struct {
	sessions []sqlitefile.Row
	messages map[string][]sqlitefile.Row
}

// parseDatabase emits the rows of every session of a sessions.sqlite3,
// messages whose session row is gone included, after the problems of the
// .workspace marker beside it.
func (p *Parser) parseDatabase(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	var markerProblems parser.Problems
	project := ""
	if marker, ok := parser.Beside(artifact, ".workspace"); ok {
		project = readMarker(marker, &markerProblems)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, markerProblems, "", project))
	store, err := readSessionStore(artifact.DiskPath)
	if err != nil {
		emit(p.fileRow(artifact, project, err.Error()))
		return
	}
	type head struct {
		key     string
		session *sqlitefile.Row
	}
	var heads []head
	known := map[string]bool{}
	for i := range store.sessions {
		key := store.sessions[i].Get("key").String()
		known[key] = true
		heads = append(heads, head{key, &store.sessions[i]})
	}
	// Messages whose session row is gone (nanobot enforces the foreign key
	// only on its own connections) still give their rows.
	var orphans []string
	for key := range store.messages {
		if !known[key] {
			orphans = append(orphans, key)
		}
	}
	slices.Sort(orphans)
	for _, key := range orphans {
		heads = append(heads, head{key, nil})
	}
	for _, each := range heads {
		var problems parser.Problems
		records := databaseRecords(each.session, store.messages[each.key], &problems)
		sessionID := p.sessionRows(artifact, options, each.key, project, records, emit)
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, project))
	}
}

// readSessionStore reads the sessions and messages of a sessions.sqlite3,
// or an error whose text is the message of the one `parser:` row that
// replaces them.
func readSessionStore(path string) (*sessionStore, error) {
	database, message := parser.OpenDatabase(path)
	if message != "" {
		return nil, errors.New(message)
	}
	defer database.Close()
	names, err := database.TableNames()
	if err != nil {
		return nil, errors.New(parser.DatabaseError(err))
	}
	var missing []string
	for _, name := range []string{"messages", "sessions"} {
		if !names[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, errors.New("no " + strings.Join(missing, " or ") + " table in the session database")
	}
	store, err := querySessionStore(database)
	if err != nil {
		return nil, errors.New(parser.DatabaseError(err))
	}
	return store, nil
}

// querySessionStore runs the two queries of a session database.
func querySessionStore(database *sqlitefile.Database) (*sessionStore, error) {
	sessions, err := database.Table("sessions")
	if err != nil {
		return nil, err
	}
	store := &sessionStore{messages: map[string][]sqlitefile.Row{}}
	store.sessions, err = sessions.Select("rowid", "key", "created_at", "updated_at", "metadata").
		OrderBy("created_at", "key").All()
	if err != nil {
		return nil, err
	}
	messages, err := database.Table("messages")
	if err != nil {
		return nil, err
	}
	rows, err := messages.Select("rowid", "session_key", "position", "record").
		OrderBy("session_key", "position").All()
	for _, row := range rows {
		key := row.Get("session_key").String()
		store.messages[key] = append(store.messages[key], row)
	}
	return store, err
}

// databaseRecords returns the records of one session of a session
// database: its sessions row as the JSONL metadata record, when the row
// exists, then each message's record in position order, each numbered by
// its rowid. A stored value that is not a JSON object adds a problem and is
// skipped.
func databaseRecords(head *sqlitefile.Row, messages []sqlitefile.Row,
	problems *parser.Problems) []numbered {
	var records []numbered
	if head != nil {
		meta, ok := jsonObject(head.Get("metadata"), head.Rowid, "unparseable session metadata", problems)
		if !ok || meta.Len() == 0 {
			meta = jsonvalue.NewObject()
		}
		record := jsonvalue.NewObject(
			jsonvalue.Member{Key: "_type", Value: jsonvalue.NewString("metadata")},
			jsonvalue.Member{Key: "key", Value: columnValue(head.Get("key"))},
			jsonvalue.Member{Key: "created_at", Value: columnValue(head.Get("created_at"))},
			jsonvalue.Member{Key: "updated_at", Value: columnValue(head.Get("updated_at"))},
			jsonvalue.Member{Key: "metadata", Value: meta})
		records = append(records, numbered{int(head.Rowid), record})
	}
	for _, message := range messages {
		label := "unparseable record " + message.Get("position").String()
		if record, ok := jsonObject(message.Get("record"), message.Rowid, label, problems); ok {
			records = append(records, numbered{int(message.Rowid), record})
		}
	}
	return records
}

// jsonObject returns a stored JSON text decoded to an object, or false
// after adding "<label> (<reason>)" with the value as text to problems.
func jsonObject(raw sqlitefile.Value, rowid int64, label string,
	problems *parser.Problems) (jsonvalue.Value, bool) {
	text := raw.String()
	if raw.Type == sqlitefile.Blob {
		text = textlines.ValidUTF8(raw.Bytes)
	}
	value, err := jsonvalue.DecodeString(text)
	switch {
	case err != nil:
		problems.AddRaw(int(rowid), label+" ("+err.Error()+")", text)
		return jsonvalue.Value{}, false
	case !value.IsObject():
		problems.AddRaw(int(rowid), label+" (not a JSON object)", text)
		return jsonvalue.Value{}, false
	}
	return value, true
}
