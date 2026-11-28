package cody

import (
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// rejected is the error a refused confirmation step completes with.
const rejected = "Command rejected"

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

// chatRows builds the rows of one chat: each takes the chat's time,
// session id and source line.
type chatRows struct {
	owner     *Parser
	artifact  *input.Artifact
	timestamp string
	sessionID string
	line      int
	emit      parserkit.Emit
	names     map[string]string     // tool names by call id
	models    parserkit.Tracker     // per chat, so the key needs no session id
	tally     *parserkit.EntryTally // of the whole history, reported at the history's line
}

// messageText returns a message's text, else the text of its text parts
// joined by LF.
func messageText(message jsonvalue.Value, parts []jsonvalue.Value) string {
	text := message.Get("text")
	if !text.IsString() {
		text = jsonvalue.NewString(parserkit.TextOf(text))
	}
	if text.StringValue() != "" {
		return text.StringValue()
	}
	var texts []string
	for _, part := range parts {
		if parserkit.IsText(part.Get("type"), "text") && !part.Get("text").IsEmpty() {
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
		if call := part.Get("tool_call"); parserkit.IsText(kind, "tool_call") && call.IsObject() {
			toolID, name := parserkit.Text(call.Get("id")), parserkit.Text(call.Get("name"))
			callIDs[toolID] = true
			c.names[toolID] = name
			args := call.Get("arguments")
			c.row(timeline.TurnToolUse, parserkit.SettingValue(args), model, name, toolID)
		} else if result := part.Get("tool_result"); parserkit.IsText(kind, "tool_result") && result.IsObject() {
			toolID := parserkit.Text(result.Get("id"))
			content := result.Get("content")
			text := content.StringValue()
			switch {
			case content.IsArray():
				text = parserkit.TextOf(content)
			case !content.IsString():
				text = parserkit.JSONText(content)
			}
			c.row(timeline.TurnToolResult, text, model, c.names[toolID], toolID)
		}
	}
	return callIDs
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
		shown = parserkit.SettingValue(message)
	}
	head := "permission decision: confirmation"
	switch {
	case parserkit.IsText(state, "error") && shown == rejected:
		head = "permission decision: rejected confirmation"
	case parserkit.IsText(state, "pending"):
		head = "permission decision: approved or unanswered confirmation"
	}
	if !state.IsEmpty() {
		head += " state=" + parserkit.SettingValue(state)
	}
	if shown != "" {
		head += " error=" + shown
	}
	if title := process.Get("title"); !title.IsEmpty() {
		head += " title=" + parserkit.SettingValue(title)
	}
	return head
}

// processes emits the rows of a message's process steps: a permission row
// per confirmation step and a tool row per tool step that no content part
// already called.
func (c *chatRows) processes(processes jsonvalue.Value, callIDs map[string]bool, model string) {
	for _, process := range c.tally.EntriesOfKind(processes, "processes", jsonvalue.Object) {
		kind := process.Get("type")
		toolID := parserkit.Text(process.Get("id"))
		if parserkit.IsText(kind, "confirmation") {
			lines := []string{confirmationHead(process)}
			for _, key := range []string{"description", "content"} {
				if value := process.Get(key); !value.IsEmpty() {
					lines = append(lines, parserkit.SettingValue(value))
				}
			}
			c.row(timeline.TurnSystem, strings.Join(lines, "\n"), model, c.names[toolID], toolID)
			continue
		}
		if !parserkit.IsText(kind, "tool") || (toolID != "" && callIDs[toolID]) {
			continue
		}
		name := parserkit.Text(process.Get("title"))
		c.names[toolID] = name
		content := process.Get("content")
		body := content.StringValue()
		if !content.IsString() && !content.IsNull() {
			body = parserkit.JSONText(content)
		}
		if state := process.Get("state"); !state.IsEmpty() && !parserkit.IsText(state, "success") {
			body = "[" + state.Text() + "] " + body
		}
		c.row(timeline.TurnToolUse, body, model, name, toolID)
	}
}

// message emits the rows of one message: a model change, its text with the
// files of its contextFiles and of usedContextFiles, its interaction's older
// list, its tool calls and results, its process steps and its error; a
// content or processes that is not an array is counted in the tally.
func (c *chatRows) message(message, usedContextFiles jsonvalue.Value) {
	speaker := message.Get("speaker")
	model := ""
	if parserkit.IsText(speaker, "assistant") {
		model = parserkit.Text(message.Get("model"))
	}
	if change := c.models.Change("model", "model", model); change != "" {
		c.row(timeline.TurnSystem, change, model, "", "")
	}
	parts := c.tally.ListOfKind(message.Get("content"), "content", jsonvalue.Object)
	text := messageText(message, parts)
	switch {
	case parserkit.IsText(speaker, "human"):
		var files []string
		for _, entry := range slices.Concat(message.Get("contextFiles").Items(), usedContextFiles.Items()) {
			file := fsPath(entry.Get("uri"))
			if fileName := entry.Get("fileName"); file == "" && fileName.IsString() {
				file = fileName.StringValue()
			}
			if entry.IsObject() && file != "" {
				files = append(files, file)
			}
		}
		if len(files) > 0 {
			text += "\n[context: " + strings.Join(files, ", ") + "]"
		}
		if text != "" {
			c.row(timeline.TurnUser, text, "", "", "")
		}
	case text != "" && parserkit.IsText(speaker, "assistant"):
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

// chat emits a heading row with the chat's account, then the rows of each
// interaction's human and assistant messages; accountPart is the heading's
// " | account=<endpoint>-<username>", empty for a chat with no account, and
// an interactions that is not an array is counted in the tally.
func (c *chatRows) chat(accountPart string, chat jsonvalue.Value) {
	c.timestamp = chatTime(parserkit.Text(chat.Get("id")), chat.Get("lastInteractionTimestamp"))
	interactions := c.tally.ListOfKind(chat.Get("interactions"), "interactions", jsonvalue.Object)
	c.row(timeline.TurnSystem, "chat: "+parserkit.Text(chat.Get("chatTitle"))+accountPart+
		" | interactions="+strconv.Itoa(len(interactions)), "", "", "")
	for _, interaction := range interactions {
		for _, key := range []string{"humanMessage", "assistantMessage"} {
			message := interaction.Get(key)
			c.tally.ObjectMember(message, key)
			if message.IsObject() {
				c.message(message, interaction.Get("usedContextFiles"))
			}
		}
	}
}

// splitHistory returns a history's top-level "chat", the chats of a history
// with no account, and the history without it and its "input", the typed
// prompts, which leaves the accounts. The extension reads a history with no
// key holding a hyphen as one with no account, and an account key,
// <endpoint>-<username>, always holds one, so neither member is an account:
// they are a history from before account keys, or the empty ones left beside
// the account it was moved under.
func splitHistory(history jsonvalue.Value) (jsonvalue.Value, jsonvalue.Value) {
	if !history.Has("chat") && !history.Has("input") {
		return jsonvalue.NewNull(), history
	}
	return history.Get("chat"), jsonvalue.NewObject(slices.DeleteFunc(slices.Clone(history.Members()),
		func(member jsonvalue.Member) bool { return member.Key == "chat" || member.Key == "input" })...)
}

// chatKeyCounts returns how many accounts of a history, and its chats with
// no account, hold each chat key: a key held more than once is told apart by
// its account.
func chatKeyCounts(history jsonvalue.Value) map[string]int {
	unkeyed, accounts := splitHistory(history)
	counts := map[string]int{}
	for _, key := range unkeyed.Keys() {
		counts[key]++
	}
	for _, account := range accounts.Members() {
		for _, key := range account.Value.Get("chat").Keys() {
			counts[key]++
		}
	}
	return counts
}

// historyRows emits the rows of every chat of a history: those with no
// account (splitHistory), then those of each account. A chat key found more
// than once takes the account before it in an account's session ids, and
// the account key each chat's heading repeats is cut (parserkit.Carried).
// Members of the wrong kind add one problem per member, at line, however
// many entries hold them.
func (p *Parser) historyRows(artifact *input.Artifact, history jsonvalue.Value, line int,
	options parserkit.Options, emit parserkit.Emit, problems *parserkit.Problems) {
	if !history.IsObject() {
		return
	}
	var tally parserkit.EntryTally
	carried := parserkit.NewCarried(artifact)
	seen := chatKeyCounts(history)
	emitChats := func(accountKey string, keyed bool, chats jsonvalue.Value) {
		for chatKey, chat := range tally.MembersOfKind(chats, "chat", jsonvalue.Object) {
			sessionID, accountPart := parserkit.Text(chat.Get("id")), ""
			if sessionID == "" {
				sessionID = chatKey
			}
			if keyed {
				accountPart = " | account=" + carried.Cut("account", accountKey)
				if seen[chatKey] > 1 {
					sessionID = accountKey + "/" + sessionID
				}
			}
			rows := chatRows{owner: p, artifact: artifact, sessionID: sessionID, line: line, emit: emit,
				names: map[string]string{}, models: parserkit.Tracker{}, tally: &tally}
			rows.chat(accountPart, chat)
		}
	}
	unkeyed, accounts := splitHistory(history)
	emitChats("", false, unkeyed)
	for accountKey, account := range parserkit.MembersOfKind(accounts, "chat history", jsonvalue.Object, line, problems) {
		emitChats(accountKey, true, account.Get("chat"))
	}
	tally.Report(line, problems)
	carried.Report(line, problems)
}
