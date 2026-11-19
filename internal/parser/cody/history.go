package cody

import (
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// rejected is the error a refused confirmation step completes with.
const rejected = "Command rejected"

// historyRows emits the rows of every chat of every account of a history.
// A chat key found under more than one account takes the account before
// it in its session id. An account that is not an object, or a chat list,
// interactions, content or processes member of the wrong kind, adds a
// problem to problems, at line.
func (p *Parser) historyRows(artifact *input.Artifact, history jsonvalue.Value, line int,
	options parser.Options, emit parser.Emit, problems *[]string) {
	if !history.IsObject() {
		return
	}
	var found parser.Problems
	seen := map[string]int{}
	for _, account := range history.Members() {
		for _, chatID := range account.Value.Get("chat").Keys() {
			seen[chatID]++
		}
	}
	for _, account := range history.Members() {
		if !account.Value.IsObject() && !account.Value.IsNull() {
			found.Add(line, parser.WrongKind("chat history account", jsonvalue.Object))
		}
		chats := parser.MembersOfKind(account.Value.Get("chat"), "chat", jsonvalue.Object, line, &found)
		for chatKey, chat := range chats {
			sessionID := parser.Text(chat.Get("id"))
			if sessionID == "" {
				sessionID = chatKey
			}
			if seen[chatKey] > 1 {
				sessionID = account.Key + "/" + sessionID
			}
			rows := chatRows{owner: p, artifact: artifact, sessionID: sessionID, line: line, emit: emit,
				names: map[string]string{}, models: parser.Tracker{}, problems: &found}
			rows.chat(account.Key, chat)
		}
	}
	addProblems(problems, found)
}

// addProblems appends the messages of found to problems.
func addProblems(problems *[]string, found parser.Problems) {
	for _, problem := range found {
		*problems = append(*problems, problem.Message)
	}
}

// chatRows builds the rows of one chat: each takes the chat's time,
// session id and source line.
type chatRows struct {
	owner     *Parser
	artifact  *input.Artifact
	timestamp string
	sessionID string
	line      int
	emit      parser.Emit
	names     map[string]string // tool names by call id
	models    parser.Tracker    // per chat, so the key needs no session id
	problems  *parser.Problems  // of the whole history, at the history's line
}

// row emits a row of the chat with its text compacted.
func (c *chatRows) row(turnType, text, model, toolName, toolUseID string) {
	row := c.owner.BaseRow(c.artifact)
	row.TimestampUTC = c.timestamp
	row.SessionID = c.sessionID
	row.SourceLine = c.line
	row.TurnType = turnType
	row.Model = model
	row.ToolName = toolName
	row.ToolUseID = toolUseID
	row.Text = timeline.Compact(text)
	c.emit(row)
}

// chat emits a heading row, its account key (<endpoint>-<username>)
// through parser.URLText, then the rows of each interaction's human and
// assistant messages; an interactions that is not an array adds a problem.
func (c *chatRows) chat(account string, chat jsonvalue.Value) {
	c.timestamp = chatTime(parser.Text(chat.Get("id")), chat.Get("lastInteractionTimestamp"))
	interactions := parser.ListOfKind(chat.Get("interactions"), "interactions", jsonvalue.Object, c.line, c.problems)
	c.row(timeline.TurnSystem, "chat: "+parser.Text(chat.Get("chatTitle"))+" | account="+parser.URLText(c.artifact, account)+
		" | interactions="+strconv.Itoa(len(interactions)), "", "", "")
	for _, interaction := range interactions {
		for _, key := range []string{"humanMessage", "assistantMessage"} {
			message := interaction.Get(key)
			parser.ObjectMember(message, key, c.line, c.problems)
			if message.IsObject() {
				c.message(message)
			}
		}
	}
}

// message emits the rows of one message: a model change, its text, its
// tool calls and results, its process steps and its error; a content or
// processes that is not an array adds a problem.
func (c *chatRows) message(message jsonvalue.Value) {
	speaker := message.Get("speaker")
	model := ""
	if parser.IsText(speaker, "assistant") {
		model = parser.Text(message.Get("model"))
	}
	if change := c.models.Change("model", "model", model); change != "" {
		c.row(timeline.TurnSystem, change, model, "", "")
	}
	parts := parser.ListOfKind(message.Get("content"), "content", jsonvalue.Object, c.line, c.problems)
	text := messageText(message, parts)
	switch {
	case parser.IsText(speaker, "human"):
		var files []string
		for _, entry := range parser.Elements(message.Get("contextFiles")) {
			if file := fsPath(entry.Get("uri")); entry.IsObject() && file != "" {
				files = append(files, file)
			}
		}
		if len(files) > 0 {
			text += "\n[context: " + strings.Join(files, ", ") + "]"
		}
		if text != "" {
			c.row(timeline.TurnUser, text, "", "", "")
		}
	case text != "" && parser.IsText(speaker, "assistant"):
		c.row(timeline.TurnAssistant, text, model, "", "")
	case text != "":
		c.row(timeline.TurnSystem, text, model, "", "")
	}
	callIDs := c.parts(parts, model)
	c.processes(message.Get("processes"), callIDs, model)
	if failure := message.Get("error"); failure.IsObject() && !failure.Get("message").IsEmpty() {
		c.row(timeline.TurnSystem, "error: "+failure.Get("message").Text(), model, "", "")
	}
}

// messageText returns a message's text, else the text of its text parts
// joined by LF.
func messageText(message jsonvalue.Value, parts []jsonvalue.Value) string {
	text := message.Get("text")
	if !text.IsString() {
		text = jsonvalue.NewString(parser.TextOf(text))
	}
	if text.StringValue() != "" {
		return text.StringValue()
	}
	var texts []string
	for _, part := range parts {
		if parser.IsText(part.Get("type"), "text") && !part.Get("text").IsEmpty() {
			texts = append(texts, part.Get("text").Text())
		}
	}
	return strings.Join(texts, "\n")
}

// fsPath returns the file path of a context file's uri: a string as it
// is, else the first non-empty fsPath, path or external of an object.
func fsPath(uri jsonvalue.Value) string {
	if uri.IsString() {
		return uri.StringValue()
	}
	for _, key := range []string{"fsPath", "path", "external"} {
		if value := uri.Get(key); value.IsString() && value.StringValue() != "" {
			return value.StringValue()
		}
	}
	return ""
}

// parts emits the tool call and result rows of a message's content parts
// and returns the ids of its calls.
func (c *chatRows) parts(parts []jsonvalue.Value, model string) map[string]bool {
	callIDs := map[string]bool{}
	for _, part := range parts {
		kind := part.Get("type")
		if call := part.Get("tool_call"); parser.IsText(kind, "tool_call") && call.IsObject() {
			toolID, name := parser.Text(call.Get("id")), parser.Text(call.Get("name"))
			callIDs[toolID] = true
			c.names[toolID] = name
			args := call.Get("arguments")
			text := args.StringValue()
			if !args.IsString() {
				text = parser.JSONText(args)
			}
			c.row(timeline.TurnToolUse, text, model, name, toolID)
		} else if result := part.Get("tool_result"); parser.IsText(kind, "tool_result") && result.IsObject() {
			toolID := parser.Text(result.Get("id"))
			content := result.Get("content")
			text := content.StringValue()
			switch {
			case content.IsArray():
				text = parser.TextOf(content)
			case !content.IsString():
				text = parser.JSONText(content)
			}
			c.row(timeline.TurnToolResult, text, model, c.names[toolID], toolID)
		}
	}
	return callIDs
}

// processes emits the rows of a message's process steps: a permission row
// per confirmation step and a tool row per tool step that no content part
// already called.
func (c *chatRows) processes(processes jsonvalue.Value, callIDs map[string]bool, model string) {
	for _, process := range parser.EntriesOfKind(processes, "processes", jsonvalue.Object, c.line, c.problems) {
		kind := process.Get("type")
		toolID := parser.Text(process.Get("id"))
		if parser.IsText(kind, "confirmation") {
			lines := []string{confirmationHead(process)}
			for _, key := range []string{"description", "content"} {
				if value := process.Get(key); !value.IsEmpty() {
					lines = append(lines, parser.SettingValue(value))
				}
			}
			c.row(timeline.TurnSystem, strings.Join(lines, "\n"), model, c.names[toolID], toolID)
			continue
		}
		if !parser.IsText(kind, "tool") || (toolID != "" && callIDs[toolID]) {
			continue
		}
		name := parser.Text(process.Get("title"))
		c.names[toolID] = name
		content := process.Get("content")
		body := content.StringValue()
		if !content.IsString() && !content.IsNull() {
			body = parser.JSONText(content)
		}
		if state := process.Get("state"); !state.IsEmpty() && !parser.IsText(state, "success") {
			body = "[" + state.Text() + "] " + body
		}
		c.row(timeline.TurnToolUse, body, model, name, toolID)
	}
}

// confirmationHead returns the first line of the permission row of a
// confirmation step. A refusal completes the step with the error "Command
// rejected"; an approval never completes it, so "pending" is an approval
// or an unanswered request.
func confirmationHead(process jsonvalue.Value) string {
	state := process.Get("state")
	failure := process.Get("error")
	message := failure
	if failure.IsObject() {
		message = failure.Get("message")
	}
	shown := ""
	if !message.IsEmpty() {
		shown = parser.SettingValue(message)
	}
	head := "permission decision: confirmation"
	switch {
	case parser.IsText(state, "error") && shown == rejected:
		head = "permission decision: rejected confirmation"
	case parser.IsText(state, "pending"):
		head = "permission decision: approved or unanswered confirmation"
	}
	if !state.IsEmpty() {
		head += " state=" + parser.SettingValue(state)
	}
	if shown != "" {
		head += " error=" + shown
	}
	if title := process.Get("title"); !title.IsEmpty() {
		head += " title=" + parser.SettingValue(title)
	}
	return head
}
