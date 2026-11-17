package ollama

import (
	"math"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// toolCall is one row of tool_calls.
type toolCall struct {
	callID    sqlitefile.Value
	name      string
	arguments sqlitefile.Value
}

// attachment is a user message's attached file: its name and byte length.
type attachment struct {
	filename string
	size     int64
}

// parseDatabase emits the rows of db.sqlite, or one `parser:` row when the
// database cannot be read; a failure part way gives only that row.
func (p *Parser) parseDatabase(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	database, message := parser.OpenDatabase(artifact.DiskPath)
	if message != "" {
		emit(p.SystemRow(artifact, parser.ProblemPrefix+message))
		return
	}
	defer database.Close()
	var rows []timeline.Row
	err := p.databaseRows(database, artifact, options, func(row timeline.Row) { rows = append(rows, row) })
	if err != nil {
		emit(p.SystemRow(artifact, parser.ProblemPrefix+parser.DatabaseError(err)))
		return
	}
	parser.EmitAll(emit, rows)
}

// valueKey returns the key a column value has in a Python dict: numbers
// equal in value share one key whatever their type, and text and blobs
// never equal a number.
func valueKey(value sqlitefile.Value) string {
	switch value.Type {
	case sqlitefile.Integer:
		return "n" + strconv.FormatInt(value.Int, 10)
	case sqlitefile.Float:
		if value.Float == math.Trunc(value.Float) && math.Abs(value.Float) < 1<<63 {
			return "n" + strconv.FormatInt(int64(value.Float), 10)
		}
		return "f" + strconv.FormatFloat(value.Float, 'g', -1, 64)
	case sqlitefile.Text:
		return "t" + string(value.Bytes)
	case sqlitefile.Blob:
		return "b" + string(value.Bytes)
	}
	return "null"
}

// orText returns a column as Python's str(value or "") writes it.
func orText(value sqlitefile.Value) string {
	if value.IsEmpty() {
		return ""
	}
	return value.String()
}

// readCalls returns the tool calls by message, each message's in id order.
func readCalls(database *sqlitefile.Database) (map[string][]toolCall, error) {
	calls := map[string][]toolCall{}
	table, err := database.Table("tool_calls")
	if err != nil {
		return nil, err
	}
	err = table.Select("id", "message_id", "function_name", "function_arguments").
		OrderBy("message_id", "id").Each(func(row sqlitefile.Row) error {
		key := valueKey(row.Get("message_id"))
		calls[key] = append(calls[key], toolCall{callID: row.Get("id"),
			name: orText(row.Get("function_name")), arguments: row.Get("function_arguments")})
		return nil
	})
	return calls, err
}

// readAttachments returns the attachments by message, in id order, with
// the length of data computed as SQL's length() does, never its bytes.
func readAttachments(database *sqlitefile.Database) (map[string][]attachment, error) {
	files := map[string][]attachment{}
	table, err := database.Table("attachments")
	if err != nil {
		return nil, err
	}
	err = table.Select("message_id", "filename").
		Compute("n", func(row sqlitefile.Row) sqlitefile.Value { return sqlitefile.Length(row.Get("data")) }).
		OrderBy("message_id", "id").Each(func(row sqlitefile.Row) error {
		key := valueKey(row.Get("message_id"))
		files[key] = append(files[key], attachment{filename: orText(row.Get("filename")),
			size: row.Get("n").Int})
		return nil
	})
	return files, err
}

// chatState is what the walk over the messages carries: the current chat,
// the models seen per chat, and the calls of the last assistant message
// with calls with how many tool messages have answered them.
type chatState struct {
	chat    string
	started bool
	models  parser.Tracker
	pending []toolCall
	used    int
}

// databaseRows emits the rows of the messages, ordered by chat_id then id.
func (p *Parser) databaseRows(database *sqlitefile.Database, artifact *input.Artifact,
	options parser.Options, emit parser.Emit) error {
	tables, err := database.TableNames()
	if err != nil || !tables["messages"] {
		return err
	}
	calls, files := map[string][]toolCall{}, map[string][]attachment{}
	if tables["tool_calls"] {
		if calls, err = readCalls(database); err != nil {
			return err
		}
	}
	if tables["attachments"] {
		if files, err = readAttachments(database); err != nil {
			return err
		}
	}
	table, err := database.Table("messages")
	if err != nil {
		return err
	}
	state := &chatState{models: parser.Tracker{}}
	reader := &messageReader{parser: p, artifact: artifact, options: options, calls: calls,
		files: files, state: state, emit: emit}
	return table.Select().OrderBy("chat_id", "id").Each(func(row sqlitefile.Row) error {
		reader.message(row)
		return nil
	})
}

// messageReader turns message rows into timeline rows.
type messageReader struct {
	parser   *Parser
	artifact *input.Artifact
	options  parser.Options
	calls    map[string][]toolCall
	files    map[string][]attachment
	state    *chatState
	emit     parser.Emit
}

// sourceLine returns an id column as a row's source line, 0 when it is not
// an integer.
func sourceLine(value sqlitefile.Value) int {
	if value.Type == sqlitefile.Integer {
		return int(value.Int)
	}
	return 0
}

// message emits the rows of one message.
func (m *messageReader) message(message sqlitefile.Row) {
	messageID := message.Get("id")
	if messageID.IsEmpty() {
		messageID = sqlitefile.IntValue(0)
	}
	sessionID := orText(message.Get("chat_id"))
	if !m.state.started || sessionID != m.state.chat {
		m.state.chat, m.state.started, m.state.pending, m.state.used = sessionID, true, nil, 0
	}
	key := valueKey(messageID)
	timestamp := parser.ColumnTime(message.Get("created_at"))
	row := m.parser.row(m.artifact, sourceLine(messageID), sessionID, timestamp)
	switch role := orText(message.Get("role")); role {
	case "user":
		parts := []string{}
		if content := orText(message.Get("content")); content != "" {
			parts = append(parts, content)
		}
		for _, file := range m.files[key] {
			parts = append(parts, "[attachment: "+file.filename+", "+
				strconv.FormatInt(file.size, 10)+" bytes]")
		}
		row.TurnType = timeline.TurnUser
		row.Text = timeline.Compact(strings.Join(parts, "\n"))
		m.emit(row)
	case "assistant":
		m.assistant(message, row, key)
	case "tool":
		m.toolResult(message, row)
	default:
		row.TurnType = timeline.TurnSystem
		content := orText(message.Get("content"))
		if role != "system" {
			content = role + ": " + content
		}
		row.Text = timeline.Compact(content)
		m.emit(row)
	}
}

// assistant emits an assistant message's model change, thinking, text and
// tool calls; base is the row the others start from.
func (m *messageReader) assistant(message sqlitefile.Row, base timeline.Row, key string) {
	content, thinking := orText(message.Get("content")), orText(message.Get("thinking"))
	model := orText(message.Get("model_name"))
	calls := m.calls[key]
	build := func(turnType, text string) timeline.Row {
		row := base
		row.TurnType, row.Model, row.Text = turnType, model, text
		return row
	}
	if content != "" || thinking != "" || len(calls) > 0 {
		if text := m.state.models.Change("model", m.state.chat, model); text != "" {
			m.emit(build(timeline.TurnSystem, text))
		}
	}
	if thinking != "" && m.options.IncludeThinking {
		m.emit(build(timeline.TurnThinking, timeline.Compact(thinking)))
	}
	if content != "" {
		m.emit(build(timeline.TurnAssistant, timeline.Compact(content)))
	}
	if len(calls) > 0 {
		m.state.pending, m.state.used = calls, 0
	}
	for _, call := range calls {
		row := build(timeline.TurnToolUse, timeline.Compact(callText(call.arguments)))
		row.SourceLine = sourceLine(call.callID)
		row.ToolName = call.name
		row.ToolUseID = m.state.chat + ":" + call.callID.String()
		m.emit(row)
	}
}

// toolResult emits a tool message, paired by position with the pending
// calls.
func (m *messageReader) toolResult(message sqlitefile.Row, row timeline.Row) {
	row.TurnType = timeline.TurnToolResult
	switch pending := m.state.pending; {
	case m.state.used < len(pending):
		call := pending[m.state.used]
		row.ToolName = call.name
		row.ToolUseID = m.state.chat + ":" + call.callID.String()
	case len(pending) > 0:
		row.ToolName = pending[len(pending)-1].name
	}
	m.state.used++
	text := orText(message.Get("content"))
	if text == "" {
		text = resultFallback(message.Get("tool_result"))
	}
	row.Text = timeline.Compact(text)
	m.emit(row)
}

// callText returns the url or query argument of a desktop tool call, else
// the arguments as stored: text that is not JSON or not an object as it
// is, an object as compact JSON, and a value that is not text as JSON.
func callText(arguments sqlitefile.Value) string {
	if arguments.Type != sqlitefile.Text {
		if arguments.IsNull() {
			return "null"
		}
		return parser.ColumnText(arguments)
	}
	text := string(arguments.Bytes)
	decoded, err := jsonvalue.DecodeString(text)
	if err != nil || !decoded.IsObject() {
		return text
	}
	for _, key := range []string{"url", "query"} {
		if value := decoded.Get(key); value.IsString() && value.StringValue() != "" {
			return value.StringValue()
		}
	}
	return parser.CompactJSON(decoded)
}

// resultFallback returns a tool message's tool_result JSON as compact text,
// "" when it is empty or JSON null, and a value that is not JSON text as
// it is.
func resultFallback(value sqlitefile.Value) string {
	if value.IsNull() || value.Type == sqlitefile.Text &&
		(len(value.Bytes) == 0 || string(value.Bytes) == "null") {
		return ""
	}
	if value.Type != sqlitefile.Text {
		return value.String()
	}
	decoded, err := jsonvalue.DecodeString(string(value.Bytes))
	if err != nil {
		return string(value.Bytes)
	}
	return parser.CompactJSON(decoded)
}
