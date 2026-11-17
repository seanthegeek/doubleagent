package hermes

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The sentinel that starts a content value holding JSON parts, and the TUI
// gateway's model-switch marker: <prefix><model>[ via provider
// <provider>]<end> ...
const (
	jsonPrefix     = "\x00json:"
	switchPrefix   = "[System: The active model for this chat has changed to "
	switchEnd      = ". From this point forward,"
	switchProvider = " via provider "
)

// roleTypes are the turn types of the message roles; any other role is a
// system row.
func roleTypes() map[string]string {
	return map[string]string{"user": timeline.TurnUser, "assistant": timeline.TurnAssistant,
		"tool": timeline.TurnToolResult, "system": timeline.TurnSystem}
}

// messageReader turns messages into rows, remembering each call's tool name
// and each session's last switched model across one file. usage holds the
// main-loop model usage rows per session, for state.db only.
type messageReader struct {
	parser    *Parser
	artifact  *input.Artifact
	options   parser.Options
	toolNames map[string]string
	switches  map[string]string
	usage     map[string][]pyjson.Value
	emit      parser.Emit
}

// newMessageReader returns a reader of one file's messages.
func (p *Parser) newMessageReader(artifact *input.Artifact, options parser.Options,
	usage map[string][]pyjson.Value, emit parser.Emit) *messageReader {
	return &messageReader{parser: p, artifact: artifact, options: options,
		toolNames: map[string]string{}, switches: map[string]string{}, usage: usage, emit: emit}
}

// decodeContent returns a messages.content value as stored: text, or the
// parts behind the JSON sentinel (the text after it when it is not JSON).
func decodeContent(value pyjson.Value) pyjson.Value {
	if !value.IsString() || !strings.HasPrefix(value.Text(), jsonPrefix) {
		return value
	}
	rest := strings.TrimPrefix(value.Text(), jsonPrefix)
	decoded, err := pyjson.DecodeString(rest)
	if err != nil {
		return pyjson.NewString(rest)
	}
	return decoded
}

// contentText returns the text of a messages.content value, image parts as
// "[image]".
func contentText(value pyjson.Value) string {
	content := decodeContent(value)
	if content.IsArray() {
		blocks := make([]pyjson.Value, 0, content.Len())
		for _, block := range content.Items() {
			if kind := block.Get("type"); block.IsObject() && kind.IsString() &&
				(kind.Text() == "image_url" || kind.Text() == "input_image") {
				block = pyjson.NewObject(pyjson.Member{Key: "type", Value: pyjson.NewString("image")})
			}
			blocks = append(blocks, block)
		}
		content = pyjson.NewArray(blocks...)
	}
	return parser.TextOf(content)
}

// jsonOr decodes a JSON text, returning fallback for null, empty text or
// text that is not JSON; a value that is not text is itself.
func jsonOr(value, fallback pyjson.Value) pyjson.Value {
	switch {
	case value.IsNull(), value.IsString() && value.Text() == "":
		return fallback
	case !value.IsString():
		return value
	}
	decoded, err := pyjson.DecodeString(value.Text())
	if err != nil {
		return fallback
	}
	return decoded
}

// modelSwitch returns the model and provider a model-switch marker names,
// the provider "" when it names none, and false when the text is not one.
func modelSwitch(content pyjson.Value) (string, string, bool) {
	if !content.IsString() || !strings.HasPrefix(content.Text(), switchPrefix) {
		return "", "", false
	}
	head, _, found := strings.Cut(strings.TrimPrefix(content.Text(), switchPrefix), switchEnd)
	if !found {
		return "", "", false
	}
	model, provider, _ := strings.Cut(head, switchProvider)
	return model, provider, model != ""
}

// toolCalls returns the tool_calls of a message and the problems met: a
// value that is not JSON, not a list (a lone object is a one-call list) or
// holds an entry that is not an object is a problem carrying the value as
// stored; the entries that are objects are still returned.
func toolCalls(message pyjson.Value, line pyjson.Value) ([]pyjson.Value, parser.Problems) {
	value := message.Get("tool_calls")
	if value.IsNull() || value.IsString() && value.Text() == "" {
		return nil, nil
	}
	raw := parser.CompactJSON(value)
	if value.IsString() {
		raw = value.Text()
	}
	number := lineNumber(line)
	label := "unparseable record " + line.Str()
	var problems parser.Problems
	if value.IsString() {
		decoded, err := pyjson.DecodeString(value.Text())
		if err != nil {
			problems.AddRaw(number, label+" (tool_calls is not valid JSON: "+err.Error()+")", raw)
			return nil, problems
		}
		value = decoded
	}
	if value.IsObject() {
		value = pyjson.NewArray(value)
	}
	if !value.IsArray() {
		problems.AddRaw(number, label+" (tool_calls is not a list)", raw)
		return nil, problems
	}
	var calls []pyjson.Value
	for _, call := range value.Items() {
		if call.IsObject() {
			calls = append(calls, call)
		}
	}
	if len(calls) != value.Len() {
		problems.AddRaw(number, label+" (tool_calls entry is not an object)", raw)
	}
	return calls, problems
}

// lineNumber returns a line value as a row's source line, 0 when it is not
// an integer.
func lineNumber(line pyjson.Value) int {
	if number, ok := line.Int(); ok && line.IsInt() {
		return int(number)
	}
	return 0
}

// callSummary returns the command, path, file_path, url or query argument
// of a call, else the arguments as compact JSON, or as stored text.
func callSummary(arguments pyjson.Value) string {
	decoded := jsonOr(arguments, arguments)
	if decoded.IsObject() {
		for _, key := range []string{"command", "path", "file_path", "url", "query"} {
			if value := decoded.Get(key); !value.IsNull() && !(value.IsString() && value.Text() == "") {
				return value.Str()
			}
		}
		return parser.CompactJSON(decoded)
	}
	if decoded.IsString() {
		return decoded.Text()
	}
	return parser.CompactJSON(decoded)
}

// terminalDecision returns the "permission decision:" text, less its label,
// of a terminal tool result: "approved (<note>)", "blocked <error>" with
// " user_summary=<summary>" when a person was asked, or "pending
// pattern=<key> smart_denied=<bool>"; "" for any other result.
func terminalDecision(content pyjson.Value) string {
	value := decodeContent(content)
	if value.IsString() {
		decoded, err := pyjson.DecodeString(value.Text())
		if err != nil {
			return ""
		}
		value = decoded
	}
	if !value.IsObject() {
		return ""
	}
	if approval := value.Get("approval"); approval.IsString() && approval.Text() != "" {
		return "approved (" + approval.Text() + ")"
	}
	switch status := value.Get("status"); {
	case isText(status, "blocked"):
		text := "blocked " + dash(value.Get("error"))
		if summary := value.Get("user_summary"); summary.Truthy() {
			text += " user_summary=" + parser.SettingValue(summary)
		}
		return text
	case isText(status, "pending_approval"):
		return "pending pattern=" + dash(value.Get("pattern_key")) + " smart_denied=" +
			parser.SettingValue(pyjson.NewBool(value.Get("smart_denied").Truthy()))
	}
	return ""
}

// isText reports whether value is the string text.
func isText(value pyjson.Value, text string) bool {
	return value.IsString() && value.Text() == text
}

// rewound reports whether a message's active flag marks it rewound: 0,
// "0" or false; a message without the flag is active.
func rewound(message pyjson.Value) bool {
	active, ok := message.Lookup("active")
	if !ok {
		return false
	}
	if number, isNumber := active.Float(); active.IsNumber() && isNumber {
		return number == 0
	}
	flag, isBool := active.Bool()
	return isBool && !flag || isText(active, "0")
}

// message emits the rows of one message: its thinking, its text or tool
// result with a terminal's permission decision, its tool calls, and the
// problems of its tool_calls. line is the message's line or record id.
func (m *messageReader) message(message, session, line pyjson.Value) {
	number := lineNumber(line)
	timestamp := timeutil.FromValue(message.Get("timestamp"))
	role := parser.Str(message.Get("role"))
	if isText(message.Get("display_kind"), "model_switch") {
		m.emit(m.switchRow(message, session, number))
		return
	}
	marker := ""
	if rewound(message) {
		marker = "[rewound] "
	}
	makeRow := func(turnType, text string) timeline.Row {
		row := m.parser.row(m.artifact, number, session, timestamp)
		row.TurnType = turnType
		row.Text = timeline.Compact(marker + text)
		return row
	}
	if m.options.IncludeThinking {
		if reasoning := either(message.Get("reasoning"), message.Get("reasoning_content")); reasoning.Truthy() {
			m.emit(makeRow(timeline.TurnThinking, reasoning.Str()))
		}
	}
	text := contentText(message.Get("content"))
	turnType, known := roleTypes()[role]
	if !known {
		turnType = timeline.TurnSystem
	}
	switch {
	case turnType == timeline.TurnToolResult:
		m.toolResult(message, session, makeRow(timeline.TurnToolResult, text), number, timestamp)
	case text != "":
		if turnType == timeline.TurnSystem && role != "" && role != "system" {
			text = role + ": " + text
		}
		m.emit(makeRow(turnType, text))
	}
	m.toolUses(message, session, line, makeRow)
}

// toolUses emits a tool_use row per tool call of a message, remembering
// each call's tool name, then the problems of its tool_calls.
func (m *messageReader) toolUses(message, session, line pyjson.Value,
	makeRow func(turnType, text string) timeline.Row) {
	calls, problems := toolCalls(message, line)
	for _, call := range calls {
		function := call.Get("function")
		if !function.IsObject() {
			function = pyjson.NewObject()
		}
		row := makeRow(timeline.TurnToolUse, callSummary(function.Get("arguments")))
		row.ToolUseID = parser.Str(either(call.Get("id"), call.Get("call_id")))
		row.ToolName = parser.Str(function.Get("name"))
		m.toolNames[row.ToolUseID] = row.ToolName
		m.emit(row)
	}
	parser.EmitAll(m.emit, m.parser.ErrorRows(m.artifact, problems, parser.Str(session.Get("id")),
		projectOf(session)))
}

// toolResult emits a tool result row and, for the terminal tool, the
// permission decision its result records.
func (m *messageReader) toolResult(message, session pyjson.Value, row timeline.Row, number int,
	timestamp string) {
	row.ToolUseID = parser.Str(message.Get("tool_call_id"))
	row.ToolName = parser.Str(message.Get("tool_name"))
	if row.ToolName == "" {
		row.ToolName = m.toolNames[row.ToolUseID]
	}
	m.emit(row)
	if row.ToolName != "terminal" {
		return
	}
	if decision := terminalDecision(message.Get("content")); decision != "" {
		permission := m.parser.row(m.artifact, number, session, timestamp)
		permission.TurnType = timeline.TurnSystem
		permission.Text = timeline.Compact("permission decision: " + decision)
		permission.ToolName, permission.ToolUseID = row.ToolName, row.ToolUseID
		m.emit(permission)
	}
}

// switchRow returns the system row of a TUI gateway model-switch marker.
// The marker names only the new model: the old one is the session's
// previous marker's, else the last main-loop usage model first seen before
// it, else unknown. A marker whose text does not parse is a
// "model_switch: <text>" row.
func (m *messageReader) switchRow(message, session pyjson.Value, number int) timeline.Row {
	row := m.parser.row(m.artifact, number, session, timeutil.FromValue(message.Get("timestamp")))
	row.TurnType = timeline.TurnSystem
	model, provider, ok := modelSwitch(message.Get("content"))
	if !ok {
		row.Text = timeline.Compact("model_switch: " + contentText(message.Get("content")))
		return row
	}
	old := m.switches[row.SessionID]
	switchTime, isNumber := numberValue(message.Get("timestamp"))
	if old == "" && isNumber {
		for _, usage := range m.usage[row.SessionID] {
			if first, ok := numberValue(usage.Get("first_seen")); ok && first < switchTime {
				old = parser.Str(usage.Get("model"))
			}
		}
	}
	m.switches[row.SessionID] = model
	row.Text = "model: " + model
	if old != "" && old != model {
		row.Text = "model: " + old + " -> " + model
	}
	if provider != "" {
		row.Text += " provider=" + provider
	}
	row.Model = model
	return row
}

// numberValue returns a number (or a boolean, which Python counts as an
// integer) as a float.
func numberValue(value pyjson.Value) (float64, bool) {
	if flag, ok := value.Bool(); ok {
		if flag {
			return 1, true
		}
		return 0, true
	}
	if !value.IsNumber() {
		return 0, false
	}
	return value.Float()
}
