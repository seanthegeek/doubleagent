// Package shellgpt reads ShellGPT's chat files, settings, functions and
// response cache.
//
// Files read, from research/shellgpt.md:
//
//   - chat_cache/<chat id>: one JSON array of OpenAI chat messages (role,
//     content, tool_calls, tool_call_id, and the older function_call answered
//     by a "function" message). A function call has no id, so it gets
//     "function_call_<array index>", shared with the "function" message after
//     it.
//   - .config/shell_gpt/.sgptrc: KEY=value lines. Only the permission keys and
//     the default model reach a row, since the file holds the API key.
//   - .config/shell_gpt/functions/*.py: one "permission setting:" row per
//     file, at its modification time; the code is not read.
//   - shell_gpt/cache/<md5>: one response string per file, a secondary source
//     that repeats the chat's answers, whose rows have no time or session.
//
// A chat file records no time, model, working directory or branch; the
// session id is the file name and the source line is 1, since the file is one
// line.
package shellgpt

import (
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// permissionKeys are the .sgptrc keys read besides the model; every other key
// (the API key above all) is skipped.
var permissionKeys = []string{"OPENAI_USE_FUNCTIONS", "DEFAULT_EXECUTE_SHELL_CMD",
	"SHELL_INTERACTION"}

// modelKey is the .sgptrc key that names the default model.
const modelKey = "DEFAULT_MODEL"

// Parser reads ShellGPT's files.
type Parser struct {
	parser.Base
	chat, settings, function, cache *regexp.Regexp
}

// New returns the parser.
func New() *Parser {
	return &Parser{
		Base:     parser.NewBase("shellgpt", "shellgpt"),
		chat:     regexp.MustCompile(`(^|/)chat_cache/[^/]+\z`),
		settings: regexp.MustCompile(`(^|/)\.config/shell_gpt/\.sgptrc\z`),
		function: regexp.MustCompile(`(^|/)\.config/shell_gpt/functions/[^/]+\.py\z`),
		cache:    regexp.MustCompile(`(^|/)shell_gpt/cache/[0-9a-f]{32}\z`),
	}
}

// Wants reports whether the artifact is a chat, settings, function or
// response cache file.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	rel := artifact.Rel
	return p.chat.MatchString(rel) || p.settings.MatchString(rel) ||
		p.function.MatchString(rel) || p.cache.MatchString(rel)
}

// Secondary reports whether the artifact is a response cache file, which
// repeats chat answers.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return p.cache.MatchString(artifact.Rel)
}

// Parse emits the rows of one ShellGPT file.
func (p *Parser) Parse(artifact *input.Artifact, _ parser.Options, emit parser.Emit) error {
	rel := artifact.Rel
	switch {
	case p.settings.MatchString(rel):
		p.parseSettings(artifact, emit)
	case p.function.MatchString(rel):
		directory := rel
		if i := strings.LastIndex(rel, "/"); i >= 0 {
			directory = rel[:i]
		}
		text := "permission setting: function " + filepath.Base(artifact.DiskPath) +
			" (from " + directory + ", file mtime)"
		emit(p.systemRow(artifact, parser.FileMtime(artifact), text))
	case p.cache.MatchString(rel):
		p.parseCache(artifact, emit)
	default:
		p.parseChat(artifact, emit)
	}
	return nil
}

// systemRow returns a system row at the given time and source line 1.
func (p *Parser) systemRow(artifact *input.Artifact, timestamp, text string) timeline.Row {
	row := p.SystemRow(artifact, text)
	row.TimestampUTC = timestamp
	row.SourceLine = 1
	return row
}

// parseSettings emits one row per permission key and default model of the
// settings file, then its problems.
func (p *Parser) parseSettings(artifact *input.Artifact, emit parser.Emit) {
	timestamp := parser.FileMtime(artifact)
	var problems parser.Problems
	raw := ""
	if content, ok := parser.FileBytes(artifact.DiskPath, &problems); ok {
		raw = textlines.ValidUTF8(content)
	}
	for _, setting := range readSettings(raw, &problems) {
		if setting.key == modelKey {
			text := "model: " + setting.value + " (configured default, from " + artifact.Rel +
				", file mtime)"
			row := p.systemRow(artifact, timestamp, text)
			row.Model = setting.value
			emit(row)
			continue
		}
		emit(p.systemRow(artifact, timestamp, "permission setting: "+setting.key+"="+
			setting.value+" (from "+artifact.Rel+", file mtime)"))
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// setting is one key and value of the settings file.
type setting struct {
	key, value string
}

// readSettings returns the permission and model settings in file order. A
// non-blank line without "=" adds a problem; the raw line is never kept,
// since the file holds the API key.
func readSettings(raw string, problems *parser.Problems) []setting {
	var found []setting
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	for i, line := range strings.Split(raw, "\n") {
		number := i + 1
		if strings.TrimSpace(line) == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.Contains(line, "=") {
			problems.Add(number, "unparseable line "+strconv.Itoa(number)+
				` (no "=" separator; not shown, the file holds the API key)`)
			continue
		}
		key, value, _ := strings.Cut(strings.TrimSpace(line), "=")
		if key == modelKey || slices.Contains(permissionKeys, key) {
			found = append(found, setting{key, value})
		}
	}
	return found
}

// parseCache emits the one assistant row of a cached response.
func (p *Parser) parseCache(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	text := textlines.ValidUTF8(content)
	if strings.TrimSpace(text) == "" {
		return
	}
	row := p.BaseRow(artifact)
	row.TurnType = timeline.TurnAssistant
	row.SourceLine = 1
	row.Text = timeline.Compact(text)
	emit(row)
}

// chatState is what one chat file's walk remembers: the function name of each
// tool call id, the synthetic id and name of an older function call
// waiting for its result, and the problems met.
type chatState struct {
	names    map[string]string
	problems parser.Problems
	pending  [2]string
}

// parseChat emits the rows of one chat file, then the problems of its
// elements and members of the wrong kind, then the problem of a damaged
// file.
func (p *Parser) parseChat(artifact *input.Artifact, emit parser.Emit) {
	sessionID := filepath.Base(artifact.DiskPath)
	makeRow := func(turn, text, toolName, toolUseID string) timeline.Row {
		row := p.BaseRow(artifact)
		row.SessionID = sessionID
		row.TurnType = turn
		row.ToolName, row.ToolUseID = toolName, toolUseID
		row.SourceLine = 1
		row.Text = timeline.Compact(text)
		return row
	}
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, ""))
		return
	}
	messages, failure := loadMessages(textlines.ValidUTF8(content), &problems)
	if !slices.ContainsFunc(messages, func(message jsonvalue.Value) bool { return message.Has("role") }) {
		text := "parser: not a ShellGPT chat file"
		if failure != nil {
			text += ": " + failure.Error()
		}
		emit(makeRow(timeline.TurnSystem, text, "", ""))
		return
	}
	state := chatState{names: map[string]string{}}
	for index, message := range messages {
		walkMessage(message, index, &state, makeRow, emit)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, append(problems, state.problems...), sessionID, ""))
	if failure != nil {
		emit(makeRow(timeline.TurnSystem, "parser: "+failure.Error(), "", ""))
	}
}

// rowMaker builds a chat row from its turn type, text, tool name and tool
// call id.
type rowMaker func(turn, text, toolName, toolUseID string) timeline.Row

// walkMessage emits the rows of one chat message.
func walkMessage(message jsonvalue.Value, index int, state *chatState, makeRow rowMaker,
	emit parser.Emit) {
	role := parser.Text(message.Get("role"))
	content := parser.TextOf(message.Get("content"))
	previous := state.pending
	state.pending = [2]string{}
	switch role {
	case "system", "user":
		emit(makeRow(role, content, "", ""))
	case "assistant":
		walkAssistant(message, index, content, state, makeRow, emit)
	case "tool":
		callID := parser.Text(message.Get("tool_call_id"))
		emit(makeRow(timeline.TurnToolResult, content, state.names[callID], callID))
	case "function":
		name := parser.Text(message.Get("name"))
		if name == "" {
			name = previous[1]
		}
		emit(makeRow(timeline.TurnToolResult, content, name, previous[0]))
	default:
		label := role
		if label == "" {
			label = "message"
		}
		emit(makeRow(timeline.TurnSystem, label+": "+content, "", ""))
	}
}

// walkAssistant emits an assistant message's text and its tool calls; a
// tool_calls that is not an array adds a problem.
func walkAssistant(message jsonvalue.Value, index int, content string, state *chatState,
	makeRow rowMaker, emit parser.Emit) {
	if strings.TrimSpace(content) != "" {
		emit(makeRow(timeline.TurnAssistant, content, "", ""))
	}
	for _, call := range parser.ArrayMember(message.Get("tool_calls"), "tool_calls", 1, &state.problems) {
		function := call.Get("function")
		callID := parser.Text(call.Get("id"))
		name := parser.Text(function.Get("name"))
		state.names[callID] = name
		emit(makeRow(timeline.TurnToolUse, callText(function.Get("arguments")), name, callID))
	}
	if functionCall := message.Get("function_call"); functionCall.IsObject() {
		state.pending = [2]string{"function_call_" + strconv.Itoa(index),
			parser.Text(functionCall.Get("name"))}
		emit(makeRow(timeline.TurnToolUse, callText(functionCall.Get("arguments")),
			state.pending[1], state.pending[0]))
	}
}

// errNotArray is the reason a chat file that decodes to something other
// than an array is not read.
var errNotArray = errors.New("not a JSON array")

// errNotObject is the reason a chat file's element that is not an object is
// not read.
var errNotObject = errors.New("not a JSON object")

// loadMessages decodes a chat file into its messages that are objects, and
// the error that kept it from decoding whole (nil when it did). A whole
// array decodes at once, an element that is not an object adding a problem;
// a truncated one is decoded element by element so the messages before the
// cut survive, and the error says how many did.
func loadMessages(raw string, problems *parser.Problems) ([]jsonvalue.Value, error) {
	data, err := jsonvalue.DecodeString(raw)
	if err == nil {
		if !data.IsArray() {
			return nil, errNotArray
		}
		return objects(data.Items(), problems), nil
	}
	text := strings.TrimLeftFunc(raw, unicode.IsSpace)
	if !strings.HasPrefix(text, "[") {
		return nil, err
	}
	found := parser.ObjectsFrom(text, 1)
	return found, fmt.Errorf("truncated after %d message(s): %w", len(found), err)
}

// objects returns the elements of values that are objects; each other
// element adds "unparseable record <position> (not a JSON object)" with the
// element as JSON.
func objects(values []jsonvalue.Value, problems *parser.Problems) []jsonvalue.Value {
	var found []jsonvalue.Value
	for i, value := range values {
		if value.IsObject() {
			found = append(found, value)
			continue
		}
		position := i + 1
		problems.AddRaw(position, parser.UnparseableRecord(position, errNotObject), parser.JSONText(value))
	}
	return found
}

// callText returns a tool call's "shell_command" argument, else its
// arguments as text.
func callText(arguments jsonvalue.Value) string {
	switch {
	case arguments.IsString():
		parsed, err := jsonvalue.DecodeString(arguments.StringValue())
		if err == nil && parsed.IsObject() && parsed.Get("shell_command").IsString() {
			return parsed.Get("shell_command").StringValue()
		}
		return arguments.StringValue()
	case arguments.IsObject():
		if command := arguments.Get("shell_command"); command.IsString() {
			return command.StringValue()
		}
		return arguments.JSON()
	case arguments.IsNull():
		return ""
	}
	return arguments.Text()
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parser.Parser          = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
)
