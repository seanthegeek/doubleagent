package ollama

import (
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
// database cannot be read; a failure part way gives only that row, since
// every table is read before the first row is emitted.
func (p *Parser) parseDatabase(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	database, err := parser.OpenDatabase(artifact.DiskPath)
	if err != nil {
		emit(p.SystemRow(artifact, parser.ProblemPrefix+parser.DatabaseError(err)))
		return
	}
	defer database.Close()
	if err := p.databaseRows(database, artifact, options, emit); err != nil {
		emit(p.SystemRow(artifact, parser.ProblemPrefix+parser.DatabaseError(err)))
	}
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
		key := parser.ColumnKey(row.Get("message_id"))
		calls[key] = append(calls[key], toolCall{callID: row.Get("id"),
			name: parser.NonEmptyColumn(row.Get("function_name")), arguments: row.Get("function_arguments")})
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
		key := parser.ColumnKey(row.Get("message_id"))
		files[key] = append(files[key], attachment{filename: parser.NonEmptyColumn(row.Get("filename")),
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

// databaseRows emits the rows of the messages, ordered by chat_id then id,
// once every table is read, and emits nothing when a read fails. The
// rows are emitted as they are made rather than held to the end.
func (p *Parser) databaseRows(database *sqlitefile.Database, artifact *input.Artifact,
	options parser.Options, emit parser.Emit) error {
	tables, err := parser.TablesOf(database)
	if err != nil || !tables.Has("messages") {
		return err
	}
	calls, files := map[string][]toolCall{}, map[string][]attachment{}
	if tables.Has("tool_calls") {
		if calls, err = readCalls(database); err != nil {
			return err
		}
	}
	if tables.Has("attachments") {
		if files, err = readAttachments(database); err != nil {
			return err
		}
	}
	table, err := database.Table("messages")
	if err != nil {
		return err
	}
	state := &chatState{models: parser.Tracker{}}
	messages, err := table.Select().OrderBy("chat_id", "id").All()
	if err != nil {
		return err
	}
	reader := &messageReader{parser: p, artifact: artifact, options: options, calls: calls,
		files: files, shown: map[string]bool{}, groups: repeatedGroups(messages, calls, files),
		attachments: map[string]*timeline.SharedText{}, state: state, cuts: parser.NewCarried(artifact), emit: emit}
	for _, row := range messages {
		reader.message(row)
	}
	if reader.repeats > 0 {
		rows := "1 row repeating"
		if reader.repeats > 1 {
			rows = strconv.Itoa(reader.repeats) + " rows repeating"
		}
		emit(p.SystemRow(artifact, parser.ProblemPrefix+"messages holds "+rows+
			" an earlier row's id; its tool calls and attachments are shown once"))
	}
	var problems parser.Problems
	reader.cuts.Report(0, &problems)
	for _, problem := range problems {
		emit(p.SystemRow(artifact, parser.ProblemPrefix+problem.Message))
	}
	return nil
}

// messageKey returns the key of a message's id, an empty id read as 0.
func messageKey(message sqlitefile.Row) string {
	messageID := message.Get("id")
	if messageID.IsEmpty() {
		messageID = sqlitefile.IntValue(0)
	}
	return parser.ColumnKey(messageID)
}

// repeatedGroups returns a group (timeline.Group) for each id that more
// than one assistant message with tool calls, or more than one user
// message with attachments, repeats, by kind and key as messageReader's
// shown holds them: "c" or "a" and the id's key.
func repeatedGroups(messages []sqlitefile.Row, calls map[string][]toolCall,
	files map[string][]attachment) map[string]*timeline.Group {
	counts := map[string]int{}
	for _, message := range messages {
		key := messageKey(message)
		switch parser.NonEmptyColumn(message.Get("role")) {
		case "assistant":
			if len(calls[key]) > 0 {
				counts["c"+key]++
			}
		case "user":
			if len(files[key]) > 0 {
				counts["a"+key]++
			}
		}
	}
	groups := map[string]*timeline.Group{}
	for kindKey, count := range counts {
		if count > 1 {
			groups[kindKey] = timeline.NewGroup()
		}
	}
	return groups
}

// messageReader turns message rows into timeline rows. A message id is the
// key of the messages table, so its tool calls and attachments are shown
// once, with the first message of the id in the order the rows are read:
// shown holds the kinds already given, by kind and key, and repeats counts
// the later messages that met one. groups holds the group of each such
// repeated id, by kind and key, whose parents are the messages of the id
// and which settles which of them shows the calls or the attachments.
type messageReader struct {
	parser   *Parser
	artifact *input.Artifact
	options  parser.Options
	calls    map[string][]toolCall
	files    map[string][]attachment
	shown    map[string]bool
	groups   map[string]*timeline.Group
	// attachments holds the shared text of the attachments of each id
	// that groups holds, by key.
	attachments map[string]*timeline.SharedText
	repeats     int
	state       *chatState
	// cuts cuts the chat id each call's tool_use_id starts with.
	cuts *parser.Carried
	emit parser.Emit
}

// callIDRoom is the room a tool_use_id leaves after its chat id: the colon
// and the call's id, an INTEGER PRIMARY KEY, whose text is at most the 20
// bytes of the lowest 64-bit integer.
const callIDRoom = len(":") + len("-9223372036854775808")

// toolUseID returns the tool_use_id of a call of the current chat,
// "<chat id>:<call id>": a chat id too long for the field with the call's
// id after it is cut (parser.Carried.CutAhead) once per chat, so the
// calls of a long chat keep distinct ids within the field's bound.
func (m *messageReader) toolUseID(call toolCall) string {
	return m.cuts.CutAhead("tool_use_id", m.state.chat, callIDRoom) + ":" + call.callID.String()
}

// firstShowing reports whether the group of kind under key, which has
// count members, is to be shown with this message: true the first time,
// false (counting a repeat) after that, and false for an empty group.
func (m *messageReader) firstShowing(kind, key string, count int) bool {
	if count == 0 {
		return false
	}
	if m.shown[kind+key] {
		m.repeats++
		return false
	}
	m.shown[kind+key] = true
	return true
}

// message emits the rows of one message.
func (m *messageReader) message(message sqlitefile.Row) {
	messageID := message.Get("id")
	if messageID.IsEmpty() {
		messageID = sqlitefile.IntValue(0)
	}
	sessionID := parser.NonEmptyColumn(message.Get("chat_id"))
	if !m.state.started || sessionID != m.state.chat {
		m.state.chat, m.state.started, m.state.pending, m.state.used = sessionID, true, nil, 0
	}
	key := parser.ColumnKey(messageID)
	timestamp := parser.ColumnTime(message.Get("created_at"))
	row := m.parser.row(m.artifact, parser.ColumnLine(messageID), sessionID, timestamp)
	switch role := parser.NonEmptyColumn(message.Get("role")); role {
	case "user":
		m.user(message, row, key)
	case "assistant":
		m.assistant(message, row, key)
	case "tool":
		m.toolResult(message, row)
	default:
		row.TurnType = timeline.TurnSystem
		content := parser.NonEmptyColumn(message.Get("content"))
		if role != "system" {
			content = role + ": " + content
		}
		row.Text = timeline.Compact(content)
		m.emit(row)
	}
}

// user emits a user message's row: its content, then, with the message
// that shows them, its attachments. Each message of a repeated id is a
// parent of the id's group, its row holding the attachments only when the
// group goes with it, as one shared text so that each message does not
// hold its own copy of them.
func (m *messageReader) user(message sqlitefile.Row, row timeline.Row, key string) {
	content := parser.NonEmptyColumn(message.Get("content"))
	files := m.files[key]
	row.TurnType = timeline.TurnUser
	showing := m.firstShowing("a", key, len(files))
	if group := m.groups["a"+key]; group != nil {
		if m.attachments[key] == nil {
			m.attachments[key] = timeline.NewSharedText(attachmentLines("", files))
		}
		var chosen timeline.TextBuilder
		chosen.Write(content)
		chosen.Write("\n")
		chosen.Share(m.attachments[key], "[attachments]", attachmentsPointer)
		group.Parent(&row).Texts(&row, chosen.Text(), timeline.RowText{Text: timeline.Compact(content)})
		m.emit(row)
		return
	}
	if !showing {
		files = nil
	}
	row.Text = timeline.Compact(attachmentLines(content, files))
	m.emit(row)
}

// attachmentLines returns content, when not empty, and a line per file.
func attachmentLines(content string, files []attachment) string {
	parts := []string{}
	if content != "" {
		parts = append(parts, content)
	}
	for _, file := range files {
		parts = append(parts, "[attachment: "+file.filename+", "+strconv.FormatInt(file.size, 10)+" bytes]")
	}
	return strings.Join(parts, "\n")
}

// attachmentsPointer is what a row writes for the attachments of a
// repeated id another row holds in full; only one message of the id shows
// them, so no row writes it but a copy of that message's row.
func attachmentsPointer(holder *timeline.Row, _ bool) string {
	return "[attachments shown in full above, at source line " + strconv.Itoa(holder.SourceLine) + "]"
}

// assistant emits an assistant message's model change, thinking, text and
// tool calls; base is the row the others start from.
func (m *messageReader) assistant(message sqlitefile.Row, base timeline.Row, key string) {
	content, thinking := parser.NonEmptyColumn(message.Get("content")), parser.NonEmptyColumn(message.Get("thinking"))
	model := parser.NonEmptyColumn(message.Get("model_name"))
	calls := m.calls[key]
	if !m.firstShowing("c", key, len(calls)) {
		calls = nil
	}
	// Each message of a repeated id is a parent of the id's group, and the
	// calls, shown once, its members, at the time and with the chat and
	// model of the message the group goes with.
	group := m.groups["c"+key]
	var parent *timeline.Parent
	if group != nil {
		holder := base
		holder.Model = model
		parent = group.Parent(&holder)
	}
	build := func(turnType, text string) timeline.Row {
		row := base
		row.TurnType, row.Model, row.Text = turnType, model, text
		if parent != nil {
			parent.Row(&row, timeline.Always)
		}
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
		row.SourceLine = parser.ColumnLine(call.callID)
		row.ToolName = call.name
		row.ToolUseID = m.toolUseID(call)
		if group != nil {
			group.Member(&row, timeline.TakeTime|timeline.TakeSession|timeline.TakeModel)
		}
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
		row.ToolUseID = m.toolUseID(call)
	case len(pending) > 0:
		row.ToolName = pending[len(pending)-1].name
	}
	m.state.used++
	text := parser.NonEmptyColumn(message.Get("content"))
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
	return parser.JSONText(decoded)
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
	return parser.JSONText(decoded)
}
