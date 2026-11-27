package nanobot

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/reason"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parseSession emits the rows of a session JSONL file, then the problems
// met in it and in its namespace's .workspace marker.
func (p *Parser) parseSession(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) {
	match := p.files.session.FindStringSubmatch(artifact.Rel)
	var problems parserkit.Problems
	project := ""
	if marker, ok := p.namespaceFile(artifact, ".workspace"); ok {
		project = p.marker(marker, &problems)
	}
	var records []numbered
	for line, record := range parserkit.JSONLines(artifact.DiskPath, &problems) {
		records = append(records, numbered{line, record})
	}
	sessionID := p.sessionRows(artifact, options, DecodeStem(group(p.files.session, match, "stem")),
		project, records, nil, emit, &problems)
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, project))
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

// head is one session of a session database: its key and its sessions
// row, nil when the row is gone.
type head struct {
	key     string
	session *sqlitefile.Row
}

// sessionHeads returns the sessions of a store: each sessions row in
// order, then each key with messages but no sessions row, in key order.
func sessionHeads(store *sessionStore) []head {
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
	return heads
}

// parseDatabase emits the rows of every session of a sessions.sqlite3,
// messages whose session row is gone included, after the problems of the
// .workspace marker beside it.
func (p *Parser) parseDatabase(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) {
	var markerProblems parserkit.Problems
	project := ""
	if marker, ok := parserkit.Beside(artifact, ".workspace"); ok {
		project = p.marker(marker, &markerProblems)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, markerProblems, "", project))
	store, err := readSessionStore(artifact, artifact.DiskPath)
	if err != nil {
		emit(p.fileRow(artifact, project, err.Error()))
		return
	}
	heads := sessionHeads(store)
	// A key is the sessions table's key, so only damage repeats one: its
	// messages are shown once, after its first row, as members of the key's
	// group (timeline.Group), whose parents are the key's rows; each later
	// row gives its own rows, counted for the problem row only when the key
	// has messages to leave out.
	counts := map[string]int{}
	for _, each := range heads {
		counts[each.key]++
	}
	groups, shown, repeats := map[string]*timeline.Group{}, map[string]bool{}, 0
	for _, each := range heads {
		var problems parserkit.Problems
		messages := store.messages[each.key]
		if counts[each.key] > 1 && len(messages) > 0 && groups[each.key] == nil {
			groups[each.key] = timeline.NewGroup()
		}
		if shown[each.key] && len(messages) > 0 {
			messages = nil
			repeats++
		}
		shown[each.key] = true
		records := databaseRecords(artifact, each.session, messages, &problems)
		sessionID := p.sessionRows(artifact, options, each.key, project, records, groups[each.key], emit,
			&problems)
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, project))
	}
	if repeats > 0 {
		var problems parserkit.Problems
		problems.Add(0, repeatedKeys(repeats))
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
	}
}

// repeatedKeys returns the problem text of n sessions rows that repeat an
// earlier row's key and so give their rows without the key's messages.
func repeatedKeys(n int) string {
	rows := "1 row repeating"
	if n > 1 {
		rows = strconv.Itoa(n) + " rows repeating"
	}
	return "sessions holds " + rows + " an earlier row's key; its messages are shown once"
}

// readSessionStore reads the sessions and messages of a sessions.sqlite3,
// or an error whose text is the message of the one `parser:` row that
// replaces them.
func readSessionStore(artifact *input.Artifact, path string) (*sessionStore, error) {
	database, err := parserkit.OpenDatabase(artifact, path)
	if err != nil {
		return nil, reason.Wrap(parserkit.DatabaseError(err), err)
	}
	defer database.Close()
	names, err := parserkit.TablesOf(database)
	if err != nil {
		return nil, reason.Wrap(parserkit.DatabaseError(err), err)
	}
	var missing []string
	for _, name := range []string{"messages", "sessions"} {
		if !names.Has(name) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return nil, errors.New("no " + strings.Join(missing, " or ") + " table in the session database")
	}
	store, err := querySessionStore(database)
	if err != nil {
		return nil, reason.Wrap(parserkit.DatabaseError(err), err)
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
func databaseRecords(artifact *input.Artifact, head *sqlitefile.Row, messages []sqlitefile.Row,
	problems *parserkit.Problems) []numbered {
	var records []numbered
	if head != nil {
		meta, ok := jsonObject(artifact, head.Get("metadata"), head.Rowid, "unparseable session metadata", problems)
		if !ok || meta.Len() == 0 {
			meta = jsonvalue.NewObject()
		}
		record := jsonvalue.NewObject(
			jsonvalue.Member{Key: "_type", Value: jsonvalue.NewString("metadata")},
			jsonvalue.Member{Key: "key", Value: parserkit.ColumnValue(head.Get("key"))},
			jsonvalue.Member{Key: "created_at", Value: parserkit.ColumnValue(head.Get("created_at"))},
			jsonvalue.Member{Key: "updated_at", Value: parserkit.ColumnValue(head.Get("updated_at"))},
			jsonvalue.Member{Key: "metadata", Value: meta})
		records = append(records, numbered{int(head.Rowid), record})
	}
	for _, message := range messages {
		label := "unparseable record " + message.Get("position").String()
		if record, ok := jsonObject(artifact, message.Get("record"), message.Rowid, label, problems); ok {
			records = append(records, numbered{int(message.Rowid), record})
		}
	}
	return records
}

// jsonObject returns a stored JSON text decoded to an object, or false
// after adding "<label> (<reason>)" with the value as text to problems.
func jsonObject(artifact *input.Artifact, raw sqlitefile.Value, rowid int64, label string,
	problems *parserkit.Problems) (jsonvalue.Value, bool) {
	text := raw.String()
	if raw.Type == sqlitefile.Blob {
		text = textlines.ValidUTF8(raw.Bytes)
	}
	value, err := parserkit.DecodeJSON(artifact, text)
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
