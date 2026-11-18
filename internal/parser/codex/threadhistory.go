package codex

import (
	"cmp"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// errNotObject is the reason a stored JSON value other than an object is
// not read.
var errNotObject = errors.New("not a JSON object")

// turnColumns and itemColumns are the columns read from
// thread_history_1.sqlite, each when the table has it.
var (
	turnColumns = []string{"thread_id", "turn_id", "rollout_ordinal", "rollout_end_ordinal",
		"status", "started_at", "completed_at", "duration_ms"}
	itemColumns = []string{"thread_id", "turn_id", "item_id", "rollout_ordinal", "created_at_ms",
		"started_at_ms", "completed_at_ms", "item_json"}
)

// turnItemTypes and turnItemKeys rename a camelCase ThreadItem's type and
// keys to the rollout's turn-item form that itemRows reads.
var (
	turnItemTypes = map[string]string{"commandExecution": "CommandExecution",
		"mcpToolCall": "McpToolCall", "dynamicToolCall": "DynamicToolCall",
		"fileChange": "FileChange", "functionCallOutput": "FunctionCallOutput",
		"webSearch": "WebSearch", "plan": "Plan", "subAgentActivity": "SubAgentActivity",
		"imageGeneration": "ImageGeneration"}
	turnItemKeys = map[string]string{"aggregatedOutput": "aggregated_output",
		"exitCode": "exit_code", "contentItems": "content_items", "agentPath": "agent_path",
		"agentThreadId": "agent_thread_id"}
)

// The kinds of event of the thread history, in the order they sort at one
// ordinal: a turn's start, an item, a turn's end.
const (
	turnStart = iota
	turnItem
	turnEnd
)

// historyEvent is one turn start, item or turn end, with its sort key.
type historyEvent struct {
	threadID string
	ordinal  sqlitefile.Value
	kind     int
	row      sqlitefile.Row
}

// itemProblem is a thread_items row whose item_json is not a JSON object,
// with its thread, reported under that thread after the other rows.
type itemProblem struct {
	sessionID string
	thread    *sqlitefile.Row
	problem   parser.Problem
}

// readThreadHistory returns the turns and items of thread_history_1.sqlite,
// or an error whose text is the problem row that replaces them.
func readThreadHistory(path string) ([]sqlitefile.Row, []sqlitefile.Row, error) {
	database, err := openDatabase(path)
	if err != nil {
		return nil, nil, err
	}
	defer database.Close()
	turns, err := selectRows(database, "thread_turns", turnColumns)
	if err != nil {
		return nil, nil, databaseError(err)
	}
	items, err := selectRows(database, "thread_items", itemColumns)
	if err != nil {
		return nil, nil, databaseError(err)
	}
	return turns, items, nil
}

// parseThreadHistory emits the rows of thread_history_1.sqlite: the turns
// and their items in rollout order, thread by thread, then a problem row
// per item that does not decode.
func (p *Parser) parseThreadHistory(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	turns, items, err := readThreadHistory(artifact.DiskPath)
	if err != nil {
		emit(p.ProblemRow(artifact, err.Error()))
		return
	}
	threads := siblingThreads(artifact)
	var problems []itemProblem
	tasked := map[string]bool{} // subagent threads whose first prompt was seen
	for _, event := range historyEvents(turns, items) {
		thread := threads[event.threadID]
		if event.kind != turnItem {
			p.turnRow(artifact, event, thread, emit)
			continue
		}
		item, problem, ok := decodeItem(event.row)
		if !ok {
			problems = append(problems, itemProblem{event.threadID, thread, problem})
			continue
		}
		p.itemEventRows(artifact, event, thread, item, options, tasked, emit)
	}
	for _, entry := range problems {
		project := ""
		if entry.thread != nil {
			project = columnText(entry.thread.Get("cwd"))
		}
		parser.EmitAll(emit, p.ErrorRows(artifact, parser.Problems{entry.problem}, entry.sessionID,
			project))
	}
}

// itemEventRows emits the rows of one thread item. A subagent thread's
// first user message is its task; tasked holds the threads whose first
// user message has been seen.
func (p *Parser) itemEventRows(artifact *input.Artifact, event historyEvent, thread *sqlitefile.Row,
	item jsonvalue.Value, options parser.Options, tasked map[string]bool, emit parser.Emit) {
	for _, part := range threadItemRows(item, event.row) {
		if part.turnType == timeline.TurnThinking && !options.IncludeThinking {
			continue
		}
		row := p.stamp(artifact, thread, event.threadID)
		row.SourceLine = int(event.row.Rowid)
		row.TurnType = part.turnType
		row.TimestampUTC = parser.ColumnTime(part.when)
		row.ToolName, row.ToolUseID = part.toolName, part.toolUseID
		text := part.text
		if part.turnType == timeline.TurnUser {
			if thread != nil && isSubagentThread(*thread) && !tasked[event.threadID] {
				row.TurnType, text = timeline.TurnSystem, "subagent task: "+text
			}
			tasked[event.threadID] = true
		}
		row.Text = timeline.Compact(text)
		emit(row)
	}
}

// turnRow emits the row of a turn's start, or of the end of a turn that
// completed or failed.
func (p *Parser) turnRow(artifact *input.Artifact, event historyEvent, thread *sqlitefile.Row,
	emit parser.Emit) {
	turn := event.row
	turnID := columnText(turn.Get("turn_id"))
	when, text := turn.Get("started_at"), "task_started turn="+turnID
	if event.kind == turnEnd {
		status := turn.Get("status")
		if !isColumnText(status, "completed") && !isColumnText(status, "failed") {
			return
		}
		when = turn.Get("completed_at")
		text = "task_complete turn=" + turnID + " duration_ms=" + columnText(turn.Get("duration_ms"))
	}
	row := p.stamp(artifact, thread, event.threadID)
	row.SourceLine = int(turn.Rowid)
	row.TurnType = timeline.TurnSystem
	row.TimestampUTC = parser.ColumnTime(when)
	row.Text = timeline.Compact(text)
	emit(row)
}

// ordinalOf returns a row's rollout_ordinal, 0 when it has none.
func ordinalOf(row sqlitefile.Row) sqlitefile.Value {
	if ordinal := row.Get("rollout_ordinal"); !ordinal.IsEmpty() {
		return ordinal
	}
	return sqlitefile.IntValue(0)
}

// greater returns the greater of two values in SQL's order, the first on a
// tie.
func greater(first, second sqlitefile.Value) sqlitefile.Value {
	if sqlitefile.Compare(second, first) > 0 {
		return second
	}
	return first
}

// historyEvents returns the turn starts, items and turn ends sorted by
// thread, ordinal, kind and rowid. A turn without an end ordinal ends at
// its last item's ordinal, or its start.
func historyEvents(turns, items []sqlitefile.Row) []historyEvent {
	lastItem := map[string]sqlitefile.Value{}
	for _, item := range items {
		key := valueKey(item.Get("thread_id")) + "\x00" + valueKey(item.Get("turn_id"))
		last, ok := lastItem[key]
		if !ok {
			last = sqlitefile.IntValue(-1)
		}
		lastItem[key] = greater(last, ordinalOf(item))
	}
	events := make([]historyEvent, 0, 2*len(turns)+len(items))
	for _, turn := range turns {
		threadID, start := columnText(turn.Get("thread_id")), ordinalOf(turn)
		end := turn.Get("rollout_end_ordinal")
		if end.IsNull() {
			key := valueKey(turn.Get("thread_id")) + "\x00" + valueKey(turn.Get("turn_id"))
			last, ok := lastItem[key]
			if !ok {
				last = start
			}
			end = greater(start, last)
		}
		events = append(events, historyEvent{threadID, start, turnStart, turn},
			historyEvent{threadID, end, turnEnd, turn})
	}
	for _, item := range items {
		events = append(events, historyEvent{columnText(item.Get("thread_id")), ordinalOf(item),
			turnItem, item})
	}
	slices.SortStableFunc(events, compareEvents)
	return events
}

// compareEvents orders two events by thread, ordinal, kind and rowid.
func compareEvents(left, right historyEvent) int {
	if order := strings.Compare(left.threadID, right.threadID); order != 0 {
		return order
	}
	if order := sqlitefile.Compare(left.ordinal, right.ordinal); order != 0 {
		return order
	}
	if left.kind != right.kind {
		return left.kind - right.kind
	}
	return cmp.Compare(left.row.Rowid, right.row.Rowid)
}

// valueKey returns a column value as a map key that tells the storage
// classes apart: NULL, an integer, a float, text and a blob holding the
// same text are five keys.
func valueKey(value sqlitefile.Value) string {
	return strconv.Itoa(int(value.Type)) + ":" + value.String()
}

// decodeItem returns the ThreadItem object a thread_items row's item_json
// holds, or false with the problem "unparseable record <rowid> (<reason>)"
// and the stored value.
func decodeItem(row sqlitefile.Row) (jsonvalue.Value, parser.Problem, bool) {
	rowid := int(row.Rowid)
	value := row.Get("item_json")
	if value.IsNull() {
		message := "unparseable record " + strconv.Itoa(rowid) + " (no item_json)"
		return jsonvalue.NewNull(), parser.Problem{Line: rowid, Message: message}, false
	}
	var problems parser.Problems
	item, ok := decodeColumnObject(value, rowid, &problems)
	if !ok {
		return jsonvalue.NewNull(), problems[0], false
	}
	return item, parser.Problem{}, true
}

// threadRow is one row of a thread_items row: an itemRow with the time it
// takes.
type threadRow struct {
	itemRow
	when sqlitefile.Value
}

// threadItemRows returns the rows of one thread_items row whose decoded
// item_json is item: a user message, an agent message or reasoning at the
// item's completion, nothing for a hook prompt or a compaction, and any
// other item renamed to the turn-item form and read as itemRows reads it.
// A time the row does not record is its creation time.
func threadItemRows(item jsonvalue.Value, row sqlitefile.Row) []threadRow {
	created := row.Get("created_at_ms")
	started, completed := row.Get("started_at_ms"), row.Get("completed_at_ms")
	if started.IsNull() {
		started = created
	}
	if completed.IsNull() {
		completed = created
	}
	one := func(turnType, text string) []threadRow {
		if text == "" {
			return nil
		}
		return []threadRow{{itemRow{turnType: turnType, text: text}, completed}}
	}
	switch itemType := parser.Text(item.Get("type")); itemType {
	case "userMessage":
		return one(timeline.TurnUser, joinedTexts(item.Get("content"), "text"))
	case "agentMessage":
		return one(timeline.TurnAssistant, parser.Text(item.Get("text")))
	case "reasoning":
		text := joinedTexts(item.Get("summary"), "")
		if text == "" {
			text = joinedTexts(item.Get("content"), "")
		}
		return one(timeline.TurnThinking, text)
	case "hookPrompt", "contextCompaction":
		return nil
	}
	var rows []threadRow
	for _, part := range itemRows(toTurnItem(item)) {
		when := completed
		if part.when == atStart {
			when = started
		}
		rows = append(rows, threadRow{part, when})
	}
	return rows
}

// joinedTexts returns the strings of a list joined by LF, taking key of
// each member that is a text block when key is given; "" for a value that
// is not a list.
func joinedTexts(values jsonvalue.Value, key string) string {
	var parts []string
	for _, value := range values.Items() {
		if key != "" && parser.IsText(value.Get("type"), "text") {
			value = value.Get(key)
		}
		if value.IsString() && value.StringValue() != "" {
			parts = append(parts, value.StringValue())
		}
	}
	return strings.Join(parts, "\n")
}

// toTurnItem returns a camelCase ThreadItem in the rollout's turn-item
// form: a sleep as the clock.sleep Extension, any other item with its type
// and keys renamed and a file change's list of changes keyed by path.
func toTurnItem(item jsonvalue.Value) jsonvalue.Value {
	itemType := parser.Text(item.Get("type"))
	if itemType == "sleep" {
		return jsonvalue.NewObject(jsonvalue.Member{Key: "type", Value: jsonvalue.NewString("Extension")},
			jsonvalue.Member{Key: "kind", Value: jsonvalue.NewString("clock.sleep")},
			jsonvalue.Member{Key: "durationMs", Value: item.Get("durationMs")})
	}
	out := jsonvalue.NewObject()
	for _, member := range item.Members() {
		key := member.Key
		if renamed, ok := turnItemKeys[key]; ok {
			key = renamed
		}
		out.Set(key, member.Value)
	}
	renamed, ok := turnItemTypes[itemType]
	if !ok {
		renamed = itemType
	}
	out.Set("type", jsonvalue.NewString(renamed))
	if changes := item.Get("changes"); itemType == "fileChange" && changes.IsArray() {
		byPath := jsonvalue.NewObject()
		for _, change := range changes.Items() {
			if path := change.Get("path"); change.IsObject() && !path.IsEmpty() {
				byPath.Set(path.Text(), change)
			}
		}
		out.Set("changes", byPath)
	}
	return out
}
