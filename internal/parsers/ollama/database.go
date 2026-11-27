package ollama

import (
	"cmp"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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

// readCalls returns the tool calls by message, each message's in id order.
func readCalls(database *sqlitefile.Database) (map[string][]toolCall, error) {
	calls := map[string][]toolCall{}
	table, err := database.Table("tool_calls")
	if err != nil {
		return nil, err
	}
	err = table.Select("id", "message_id", "function_name", "function_arguments").
		OrderBy("message_id", "id").Each(func(row sqlitefile.Row) error {
		key := parserkit.ColumnKey(row.Get("message_id"))
		calls[key] = append(calls[key], toolCall{callID: row.Get("id"),
			name: parserkit.NonEmptyColumn(row.Get("function_name")), arguments: row.Get("function_arguments")})
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
		key := parserkit.ColumnKey(row.Get("message_id"))
		files[key] = append(files[key], attachment{filename: parserkit.NonEmptyColumn(row.Get("filename")),
			size: row.Get("n").Int})
		return nil
	})
	return files, err
}

// messageKey returns the key of a message's id, an empty id read as 0.
func messageKey(message sqlitefile.Row) string {
	messageID := message.Get("id")
	if messageID.IsEmpty() {
		messageID = sqlitefile.IntValue(0)
	}
	return parserkit.ColumnKey(messageID)
}

// chatState is what the walk over the messages carries: the current chat,
// the models seen per chat, the calls of the last assistant message with
// calls with how many tool messages have answered them, and the model of a
// message of the chat that the calls of a repeated id may move to (moved).
type chatState struct {
	chat    string
	started bool
	models  parserkit.Tracker
	pending []toolCall
	used    int
	moved   *movedModel
}

// movedModel is the model of a message of a repeated id that has no
// content or thinking of its own, named by a `model:` row shown only when
// the id's calls are shown with it (timeline.WhenChosen): until the chat's
// next model row, the chat's model is that one when they are and the
// tracked one when they are not.
type movedModel struct {
	parent *timeline.Parent
	model  string
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
		switch parserkit.NonEmptyColumn(message.Get("role")) {
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
	options  parserkit.Options
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
	cuts *parserkit.Carried
	emit parserkit.Emit
}

// callIDRoom is the room a tool_use_id leaves after its chat id: the colon
// and the call's id, an INTEGER PRIMARY KEY, whose text is at most the 20
// bytes of the lowest 64-bit integer.
const callIDRoom = len(":") + len("-9223372036854775808")

// attachmentsPointer is what a row writes for the attachments of a
// repeated id another row holds in full; only one message of the id shows
// them, so no row writes it but a copy of that message's row.
func attachmentsPointer(holder *timeline.Row, _ bool) string {
	return "[attachments shown in full above, at source line " + strconv.Itoa(holder.SourceLine) + "]"
}

// user emits a user message's row: its content, then, with the message
// that shows them, its attachments. Each message of a repeated id is a
// parent of the id's group, its row holding the attachments only when the
// group goes with it, as one shared text so that each message does not
// hold its own copy of them.
func (m *messageReader) user(message sqlitefile.Row, row timeline.Row, key string) {
	content := parserkit.NonEmptyColumn(message.Get("content"))
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

// toolUseID returns the tool_use_id of a call of the current chat,
// "<chat id>:<call id>": a chat id too long for the field with the call's
// id after it is cut (parserkit.Carried.CutAhead) once per chat, so the
// calls of a long chat keep distinct ids within the field's bound.
func (m *messageReader) toolUseID(call toolCall) string {
	return m.cuts.CutAhead("tool_use_id", m.state.chat, callIDRoom) + ":" + call.callID.String()
}

// modelRow emits the `model:` row of a message that names model, if the
// chat's model changes. After a moved model of the chat, the row reads
// from that model when the calls are shown with its message, and from the
// tracked one when they are not; a row of another group keeps the second.
// Only the last such moved model is kept: after two or more of one id, the
// row reads from the tracked one when the calls go with an earlier one,
// since a row shown only when its group goes with none of several parents
// is a form timeline.Group lacks (research/ollama.md says so).
func (m *messageReader) modelRow(build func(turnType, text string) timeline.Row, model string) {
	text := m.state.models.Change("model", m.state.chat, model)
	moved := m.state.moved
	if moved == nil || model == "" {
		if text != "" {
			m.emit(build(timeline.TurnSystem, text))
		}
		return
	}
	m.state.moved = nil
	chosen := parserkit.Tracker{m.state.chat: moved.model}.Change("model", m.state.chat, model)
	row := build(timeline.TurnSystem, cmp.Or(text, chosen))
	switch {
	case row.Group != nil:
		if text == "" {
			return
		}
	case text == "":
		moved.parent.Row(&row, timeline.WhenChosen)
	case chosen == "":
		moved.parent.Row(&row, timeline.WhenNotChosen)
	default:
		moved.parent.Texts(&row, timeline.RowText{Text: chosen}, timeline.RowText{Text: text})
	}
	m.emit(row)
}

// movedModelRow emits, for a message of a repeated id with calls that has
// no content or thinking of its own, the `model:` row its model would give
// were the id's calls shown with it, shown only when they are: the chat's
// tracked model is left as it is, and the chat's next model row reads from
// either (modelRow).
func (m *messageReader) movedModelRow(build func(turnType, text string) timeline.Row,
	parent *timeline.Parent, model string) {
	current, seen := m.state.models[m.state.chat]
	text := m.state.models.Change("model", m.state.chat, model)
	if seen {
		m.state.models[m.state.chat] = current
	} else {
		delete(m.state.models, m.state.chat)
	}
	if text == "" {
		return
	}
	row := build(timeline.TurnSystem, text)
	parent.Row(&row, timeline.WhenChosen)
	m.state.moved = &movedModel{parent: parent, model: model}
	m.emit(row)
}

// callText returns the url or query argument of a desktop tool call, else
// the arguments as stored: text that is not JSON or not an object as it
// is, an object as compact JSON, and a value that is not text as JSON.
func callText(artifact *input.Artifact, arguments sqlitefile.Value) string {
	if arguments.Type != sqlitefile.Text {
		if arguments.IsNull() {
			return "null"
		}
		return parserkit.ColumnText(arguments)
	}
	text := string(arguments.Bytes)
	decoded, err := parserkit.DecodeJSON(artifact, text)
	if err != nil || !decoded.IsObject() {
		return text
	}
	for _, key := range []string{"url", "query"} {
		if value := decoded.Get(key); value.IsString() && value.StringValue() != "" {
			return value.StringValue()
		}
	}
	return parserkit.JSONText(decoded)
}

// assistant emits an assistant message's model change, thinking, text and
// tool calls; base is the row the others start from.
func (m *messageReader) assistant(message sqlitefile.Row, base timeline.Row, key string) {
	content, thinking := parserkit.NonEmptyColumn(message.Get("content")), parserkit.NonEmptyColumn(message.Get("thinking"))
	model := parserkit.NonEmptyColumn(message.Get("model_name"))
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
	switch {
	case content != "" || thinking != "" || len(calls) > 0:
		m.modelRow(build, model)
	case parent != nil:
		m.movedModelRow(build, parent, model)
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
		row := build(timeline.TurnToolUse, timeline.Compact(callText(m.artifact, call.arguments)))
		row.SourceLine = parserkit.ColumnLine(call.callID)
		row.ToolName = call.name
		row.ToolUseID = m.toolUseID(call)
		if group != nil {
			group.Member(&row, timeline.TakeTime|timeline.TakeSession|timeline.TakeModel)
		}
		m.emit(row)
	}
}

// resultFallback returns a tool message's tool_result JSON as compact text,
// "" when it is empty or JSON null, and a value that is not JSON text as
// it is.
func resultFallback(artifact *input.Artifact, value sqlitefile.Value) string {
	if value.IsNull() || value.Type == sqlitefile.Text &&
		(len(value.Bytes) == 0 || string(value.Bytes) == "null") {
		return ""
	}
	if value.Type != sqlitefile.Text {
		return value.String()
	}
	decoded, err := parserkit.DecodeJSON(artifact, string(value.Bytes))
	if err != nil {
		return string(value.Bytes)
	}
	return parserkit.JSONText(decoded)
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
	text := parserkit.NonEmptyColumn(message.Get("content"))
	if text == "" {
		text = resultFallback(m.artifact, message.Get("tool_result"))
	}
	row.Text = timeline.Compact(text)
	m.emit(row)
}

// message emits the rows of one message.
func (m *messageReader) message(message sqlitefile.Row) {
	messageID := message.Get("id")
	if messageID.IsEmpty() {
		messageID = sqlitefile.IntValue(0)
	}
	sessionID := parserkit.NonEmptyColumn(message.Get("chat_id"))
	if !m.state.started || sessionID != m.state.chat {
		m.state.chat, m.state.started, m.state.pending, m.state.used = sessionID, true, nil, 0
		m.state.moved = nil
	}
	key := parserkit.ColumnKey(messageID)
	timestamp := parserkit.ColumnTime(message.Get("created_at"))
	row := m.parser.row(m.artifact, parserkit.ColumnLine(messageID), sessionID, timestamp)
	switch role := parserkit.NonEmptyColumn(message.Get("role")); role {
	case "user":
		m.user(message, row, key)
	case "assistant":
		m.assistant(message, row, key)
	case "tool":
		m.toolResult(message, row)
	default:
		row.TurnType = timeline.TurnSystem
		content := parserkit.NonEmptyColumn(message.Get("content"))
		if role != "system" {
			content = role + ": " + content
		}
		row.Text = timeline.Compact(content)
		m.emit(row)
	}
}

// databaseRows emits the rows of the messages, ordered by chat_id then id,
// once every table is read, and emits nothing when a read fails. The
// rows are emitted as they are made rather than held to the end.
func (p *Parser) databaseRows(database *sqlitefile.Database, artifact *input.Artifact,
	options parserkit.Options, emit parserkit.Emit) error {
	tables, err := parserkit.TablesOf(database)
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
	state := &chatState{models: parserkit.Tracker{}}
	messages, err := table.Select().OrderBy("chat_id", "id").All()
	if err != nil {
		return err
	}
	reader := &messageReader{parser: p, artifact: artifact, options: options, calls: calls,
		files: files, shown: map[string]bool{}, groups: repeatedGroups(messages, calls, files),
		attachments: map[string]*timeline.SharedText{}, state: state, cuts: parserkit.NewCarried(artifact), emit: emit}
	for _, row := range messages {
		reader.message(row)
	}
	if reader.repeats > 0 {
		rows := "1 row repeating"
		if reader.repeats > 1 {
			rows = strconv.Itoa(reader.repeats) + " rows repeating"
		}
		emit(p.SystemRow(artifact, parserkit.ProblemPrefix+"messages holds "+rows+
			" an earlier row's id; its tool calls and attachments are shown once"))
	}
	var problems parserkit.Problems
	reader.cuts.Report(0, &problems)
	for _, problem := range problems {
		emit(p.SystemRow(artifact, parserkit.ProblemPrefix+problem.Message))
	}
	return nil
}

// parseDatabase emits the rows of db.sqlite, or one `parser:` row when the
// database cannot be read; a failure part way gives only that row, since
// every table is read before the first row is emitted.
func (p *Parser) parseDatabase(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) {
	database, err := parserkit.OpenDatabase(artifact, artifact.DiskPath)
	if err != nil {
		emit(p.SystemRow(artifact, parserkit.ProblemPrefix+parserkit.DatabaseError(err)))
		return
	}
	defer database.Close()
	if err := p.databaseRows(database, artifact, options, emit); err != nil {
		emit(p.SystemRow(artifact, parserkit.ProblemPrefix+parserkit.DatabaseError(err)))
	}
}
