package hermes

import (
	"slices"
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// storedResponse is one responses row whose data decodes: its response's
// created_at (0 when absent), rowid, response id and data.
type storedResponse struct {
	created    float64
	rowid      int64
	responseID string
	data       jsonvalue.Value
}

// parseResponses emits the rows of the API server's Responses store: per
// responses row, by the response's created_at, an "api server response" row,
// then the messages of its conversation_history that no earlier response
// of the session held at the same position, read as state.db messages with
// data.session_id and no time of their own.
func (p *Parser) parseResponses(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) {
	handle, err := parserkit.OpenDatabase(artifact, artifact.DiskPath)
	if err != nil {
		emit(p.system(artifact, 0, noSession(), parserkit.ProblemPrefix+parserkit.DatabaseError(err)))
		return
	}
	defer handle.Close()
	var problems parserkit.Problems
	carried := parserkit.NewCarried(artifact)
	names, stored, message, err := readResponses(artifact, handle, carried, &problems)
	switch {
	case err != nil:
		emit(p.system(artifact, 0, noSession(), parserkit.ProblemPrefix+parserkit.DatabaseError(err)))
		return
	case message != "":
		emit(p.system(artifact, 0, noSession(), parserkit.ProblemPrefix+message))
		return
	}
	seen := map[string]map[string]bool{}
	reader := p.newMessageReader(artifact, options, nil, emit)
	for _, entry := range stored {
		p.responseRows(artifact, entry, names, seen, reader, &problems)
	}
	carried.Report(0, &problems)
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// responseRows emits one response's header row, then the messages of its
// conversation history that no earlier response of the session held at
// the same position. A conversation_history that is not an array adds a
// problem.
func (p *Parser) responseRows(artifact *input.Artifact, entry storedResponse, names map[string]string,
	seen map[string]map[string]bool, reader *messageReader, problems *parserkit.Problems) {
	sessionID := parserkit.Text(entry.data.Get("session_id"))
	response := entry.data.Get("response")
	if !response.IsObject() {
		response = jsonvalue.NewObject()
	}
	name := jsonvalue.NewNull()
	if value, ok := names[entry.responseID]; ok {
		name = jsonvalue.NewString(value)
	}
	header := p.system(artifact, int(entry.rowid), sessionOf(sessionID), "api server response "+
		entry.responseID+" status="+parserkit.SettingOrDash(response.Get("status"))+" model="+parserkit.SettingOrDash(response.Get("model"))+
		" conversation="+parserkit.SettingOrDash(name)+" (from response_store.db responses, created_at)")
	header.TimestampUTC = timeutil.FromValue(response.Get("created_at"))
	reader.emit(header)
	if seen[sessionID] == nil {
		seen[sessionID] = map[string]bool{}
	}
	rowid := strconv.FormatInt(entry.rowid, 10)
	for i, item := range parserkit.ArrayMember(entry.data.Get("conversation_history"),
		"conversation_history", int(entry.rowid), problems) {
		if !item.IsObject() {
			problems.AddRaw(int(entry.rowid), "unparseable record "+rowid+
				" (responses.data conversation_history "+strconv.Itoa(i)+": not an object)",
				parserkit.JSONText(item))
			continue
		}
		key := strconv.Itoa(i) + "\x00" + parserkit.JSONText(item)
		if seen[sessionID][key] {
			continue
		}
		seen[sessionID][key] = true
		reader.message(item, sessionOf(sessionID), jsonvalue.NewInt(entry.rowid))
	}
}

// readResponses returns the conversation names by response id and the
// responses whose data decodes, sorted by created_at then rowid; a row
// whose data does not decode adds a problem. A name is cut as a carried
// value, since each response of its id repeats it. The message is set for
// a store without a responses table.
func readResponses(artifact *input.Artifact, handle *sqlitefile.Database, carried *parserkit.Carried, problems *parserkit.Problems) (map[string]string,
	[]storedResponse, string, error) {
	tables, err := parserkit.TablesOf(handle)
	if err != nil {
		return nil, nil, "", err
	}
	if !tables.Has("responses") {
		return nil, nil, "no responses table", nil
	}
	names := map[string]string{}
	if tables.Has("conversations") {
		table, err := handle.Table("conversations")
		if err != nil {
			return nil, nil, "", err
		}
		err = table.Select("name", "response_id").Each(func(row sqlitefile.Row) error {
			record := rowObject(row)
			names[record.Get("response_id").Text()] = carried.Cut("conversation name", record.Get("name").Text())
			return nil
		})
		if err != nil {
			return nil, nil, "", err
		}
	}
	stored, err := storedResponses(artifact, handle, problems)
	return names, stored, "", err
}

// storedResponses reads the responses rows whose data decodes to an object.
func storedResponses(artifact *input.Artifact, handle *sqlitefile.Database, problems *parserkit.Problems) ([]storedResponse, error) {
	table, err := handle.Table("responses")
	if err != nil {
		return nil, err
	}
	var stored []storedResponse
	err = table.Select("rowid", "response_id", "data").Each(func(row sqlitefile.Row) error {
		record := rowObject(row)
		data, err := jsonObject(artifact, record.Get("data"))
		if err != nil {
			problems.AddRaw(int(row.Rowid), "unparseable record "+strconv.FormatInt(row.Rowid, 10)+
				" (responses.data: "+err.Error()+")", parserkit.Text(record.Get("data")))
			return nil
		}
		stored = append(stored, storedResponse{created: parserkit.NumberOrder(data.Get("response").Get("created_at")), rowid: row.Rowid,
			responseID: parserkit.Text(record.Get("response_id")), data: data})
		return nil
	})
	slices.SortStableFunc(stored, func(left, right storedResponse) int {
		switch {
		case left.created < right.created:
			return -1
		case left.created > right.created:
			return 1
		case left.rowid < right.rowid:
			return -1
		case left.rowid > right.rowid:
			return 1
		}
		return 0
	})
	return stored, err
}
