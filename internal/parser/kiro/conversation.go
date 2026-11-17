package kiro

import (
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The fixed prompts of a CancelledToolUses message.
const (
	denyText      = "I deny this tool request. Ask a follow up question clarifying the expected action"
	interruptText = "The user interrupted the tool execution."
)

// salvage parses a ConversationState, salvaging what precedes the damage
// when it does not parse: the conversation_id and every complete history
// entry before the damage. It returns the state, or false when there is
// none, and the error message, "" when the whole document parsed.
func (p *Parser) salvage(text string) (jsonvalue.Value, bool, string) {
	state, err := jsonvalue.DecodeString(text)
	if err == nil {
		if state.IsObject() {
			return state, true, ""
		}
		return jsonvalue.Value{}, false, "not a JSON object"
	}
	message := err.Error()
	found := p.historyStart.FindStringIndex(text)
	if found == nil {
		return jsonvalue.Value{}, false, message
	}
	var members []jsonvalue.Member
	if match := p.conversationID.FindStringSubmatch(text[:found[0]]); match != nil {
		if decoded, err := jsonvalue.DecodeString(`"` + match[1] + `"`); err == nil {
			members = append(members, jsonvalue.Member{Key: "conversation_id", Value: decoded})
		}
	}
	var entries []jsonvalue.Value
	for i := found[1]; i < len(text); {
		for i < len(text) && strings.IndexByte(" \t\r\n,", text[i]) >= 0 {
			i++
		}
		if i >= len(text) || text[i] == ']' {
			break
		}
		entry, next, err := jsonvalue.DecodePrefix(text, i)
		if err != nil {
			break
		}
		if entry.IsObject() {
			entries = append(entries, entry)
		}
		i = next
	}
	history := jsonvalue.Member{Key: "history", Value: jsonvalue.NewArray(entries...)}
	return jsonvalue.NewObject(append([]jsonvalue.Member{history}, members...)...), true, message
}

// variant unpacks an externally tagged enum, {"Tag": {...}}, as its tag
// and body, the body an empty object when it is not one.
func variant(value jsonvalue.Value) (string, jsonvalue.Value) {
	if !value.IsObject() || value.Len() != 1 {
		return "", jsonvalue.NewObject()
	}
	member := value.Members()[0]
	if !member.Value.IsObject() {
		return member.Key, jsonvalue.NewObject()
	}
	return member.Key, member.Value
}

// resultText returns the text of a tool use result, its status in
// brackets after it.
func resultText(result jsonvalue.Value) string {
	var parts []string
	for _, block := range parser.Elements(result.Get("content")) {
		switch {
		case !block.IsObject():
			parts = append(parts, block.Text())
		case block.Get("Text").IsString():
			parts = append(parts, block.Get("Text").StringValue())
		case block.Has("Json"):
			parts = append(parts, parser.JSONText(block.Get("Json")))
		default:
			parts = append(parts, parser.JSONText(block))
		}
	}
	text := strings.Join(parts, "\n")
	if status := result.Get("status"); !status.IsEmpty() {
		if text == "" {
			return "[" + status.Text() + "]"
		}
		text += " [" + status.Text() + "]"
	}
	return text
}

// decisionText returns the "permission decision:" text of a
// CancelledToolUses message's prompt: the fixed denial text of a bare n, a
// deny rule of the agent's toolsSettings, an interrupt, or the user's own
// words typed at the approval prompt.
func (p *Parser) decisionText(prompt jsonvalue.Value) string {
	if !prompt.IsString() || prompt.StringValue() == "" {
		return "permission decision: cancelled"
	}
	switch text := prompt.StringValue(); {
	case text == denyText:
		return "permission decision: denied by=n"
	case text == interruptText:
		return "permission decision: cancelled by=interrupt"
	case p.denyRule.MatchString(text):
		return "permission decision: denied by=deny-rule tool=" + p.denyRule.FindStringSubmatch(text)[1]
	}
	return "permission decision: denied by=user-text"
}

// entryKey returns what identifies a history entry across the copies one
// blob holds: the user content and time and the assistant message, on
// which a snapshot entry, a clone of the history entry, is equal to it.
func entryKey(entry jsonvalue.Value) string {
	user := entry.Get("user")
	return parser.JSONText(jsonvalue.NewArray(user.Get("content"), user.Get("timestamp"),
		entry.Get("assistant")))
}

// conversation is one ConversationState being read.
type conversation struct {
	parser             *Parser
	artifact           *input.Artifact
	options            parser.Options
	emit               parser.Emit
	line               int
	sessionID, project string
	stateModel, source string
	names              map[string]string
	lastTimestamp      string
}

// base returns a row of the conversation at rowTime, else at the last time
// a row had, with the model.
func (c *conversation) base(rowTime, model string) timeline.Row {
	row := c.parser.BaseRow(c.artifact)
	row.SourceLine = c.line
	if rowTime != "" {
		c.lastTimestamp = rowTime
	}
	row.TimestampUTC = c.lastTimestamp
	row.SessionID = c.sessionID
	row.ProjectPath = c.project
	row.Model = model
	return row
}

// fill sets a row's text and returns it.
func fill(row timeline.Row, turnType, text string) timeline.Row {
	row.TurnType = turnType
	row.Text = timeline.Compact(text)
	return row
}

// textOrJSON returns a string value's text, anything else as compact JSON.
func textOrJSON(value jsonvalue.Value) string {
	if value.IsString() {
		return value.StringValue()
	}
	return parser.JSONText(value)
}

// conversation emits the rows of one serialised ConversationState read
// from text; key is the working directory the database files it under and
// line the source line of its rows.
func (p *Parser) conversation(artifact *input.Artifact, options parser.Options, text, key string,
	line int, emit parser.Emit) {
	state, ok, problem := p.salvage(text)
	if !ok || !state.Get("history").IsArray() {
		if problem != "" {
			row := p.SystemRow(artifact, "parser: unparseable conversation: "+problem)
			row.SourceLine = line
			row.ProjectPath = key
			emit(row)
		}
		return
	}
	reader := &conversation{parser: p, artifact: artifact, options: options, emit: emit,
		line: line, sessionID: parser.Text(state.Get("conversation_id")), project: key,
		names: map[string]string{}}
	if info := state.Get("model_info"); info.IsObject() && !info.Get("model_id").IsEmpty() {
		reader.stateModel, reader.source = info.Get("model_id").Text(), "model_info"
	} else if model := state.Get("model"); model.IsString() {
		reader.stateModel, reader.source = model.StringValue(), "model"
	}
	if reader.project == "" {
		reader.project = firstCwd(state)
	}
	reader.read(state)
	if problem != "" {
		row := p.SystemRow(artifact, "parser: truncated conversation, "+
			strconv.Itoa(state.Get("history").Len())+" history entries recovered: "+problem)
		row.SourceLine = line
		row.SessionID = reader.sessionID
		row.ProjectPath = reader.project
		emit(row)
	}
}

// firstCwd returns the first working directory recorded in the history,
// or "".
func firstCwd(state jsonvalue.Value) string {
	for _, entry := range parser.Elements(state.Get("history")) {
		cwd, ok := lookupPath(entry, "user", "env_context", "env_state", "current_working_directory")
		if ok && !cwd.IsEmpty() {
			return cwd.Text()
		}
	}
	return ""
}

// lookupPath follows keys through nested objects and reports whether every
// one was there.
func lookupPath(value jsonvalue.Value, keys ...string) (jsonvalue.Value, bool) {
	for _, key := range keys {
		if !value.IsObject() {
			return jsonvalue.Value{}, false
		}
		next, ok := value.Lookup(key)
		if !ok {
			return jsonvalue.Value{}, false
		}
		value = next
	}
	return value, true
}

// read emits the rows of the history, then of the entries only a copy of
// it holds, then the summary, the pending message.
func (c *conversation) read(state jsonvalue.Value) {
	models := parser.Tracker{}
	seen := map[string]bool{}
	for _, entry := range state.Get("history").Items() {
		if entry.IsObject() {
			seen[entryKey(entry)] = true
			c.entryRows(entry, models, c.emit)
		}
	}
	copyModels := parser.Tracker{}
	for _, copied := range historyCopies(state) {
		var fresh []jsonvalue.Value
		for _, entry := range copied.entries {
			if entry.IsObject() && !seen[entryKey(entry)] {
				seen[entryKey(entry)] = true
				fresh = append(fresh, entry)
			}
		}
		if len(fresh) == 0 {
			continue
		}
		c.emit(fill(c.base(copied.timestamp, ""), timeline.TurnSystem,
			copied.head+" recovered="+strconv.Itoa(len(fresh))+" entries"+copied.tail))
		for _, entry := range fresh {
			c.entryRows(entry, copyModels, func(row timeline.Row) {
				if row.TurnType != timeline.TurnSystem {
					row.Recovered = "yes"
				}
				c.emit(row)
			})
		}
	}
	c.summary(state.Get("latest_summary"))
	c.pending(state.Get("next_message"))
}

// historyCopy is one copy of the history a blob holds: the row text before
// and after the entry count, its time and its entries.
type historyCopy struct {
	head, tail, timestamp string
	entries               []jsonvalue.Value
}

// historyCopies returns each checkpoint's history_snapshot, in checkpoint
// order, then tangent_state.main_history. The tangent copy has no time of
// its own.
func historyCopies(state jsonvalue.Value) []historyCopy {
	var copies []historyCopy
	if checkpoints := state.Get("checkpoint_manager").Get("checkpoints"); checkpoints.IsArray() {
		for _, checkpoint := range checkpoints.Items() {
			snapshot := checkpoint.Get("history_snapshot")
			if !checkpoint.IsObject() || !snapshot.IsArray() {
				continue
			}
			head := "checkpoint snapshot: tag=" + parser.Text(checkpoint.Get("tag"))
			if tool := checkpoint.Get("tool_name"); !tool.IsEmpty() {
				head += " tool=" + tool.Text()
			}
			tail := ""
			if description := checkpoint.Get("description"); !description.IsEmpty() {
				tail = ": " + description.Text()
			}
			copies = append(copies, historyCopy{head: head, tail: tail,
				timestamp: timeutil.FromValue(checkpoint.Get("timestamp")), entries: snapshot.Items()})
		}
	}
	if main := state.Get("tangent_state").Get("main_history"); main.IsArray() {
		copies = append(copies, historyCopy{head: "tangent mode: main conversation held aside,",
			entries: main.Items()})
	}
	return copies
}

// summary emits the row of the conversation's latest summary.
func (c *conversation) summary(summary jsonvalue.Value) {
	if !summary.IsArray() || summary.Len() == 0 {
		return
	}
	meta := summary.Index(1)
	if !meta.IsObject() {
		meta = jsonvalue.NewObject()
	}
	timestamp := timeutil.FirstNonEmpty(timeutil.FromValue(meta.Get("stream_end_timestamp_ms")),
		timeutil.FromValue(meta.Get("request_start_timestamp_ms")))
	c.emit(fill(c.base(timestamp, parser.Text(meta.Get("model_id"))), timeline.TurnSystem,
		"summary: "+textOrJSON(summary.Index(0))))
}

// pending emits the row of the message typed but not sent.
func (c *conversation) pending(pending jsonvalue.Value) {
	if !pending.IsObject() {
		return
	}
	text := ""
	if tag, body := variant(pending.Get("content")); tag == "Prompt" || tag == "CancelledToolUses" {
		text = parser.Text(body.Get("prompt"))
	}
	if text == "" {
		text = parser.JSONText(pending.Get("content"))
	}
	c.emit(fill(c.base(timeutil.FromValue(pending.Get("timestamp")), ""), timeline.TurnSystem,
		"pending message (not sent): "+text))
}

// entryRows emits the rows of one history entry: a model change, the user
// content, the assistant's response and tool uses, and a compaction mark.
// models tracks the model per session for this run over entries.
func (c *conversation) entryRows(entry jsonvalue.Value, models parser.Tracker, emit parser.Emit) {
	meta := entry.Get("request_metadata")
	if !meta.IsObject() {
		meta = jsonvalue.NewObject()
	}
	model := c.stateModel
	if modelID := meta.Get("model_id"); !modelID.IsEmpty() {
		model = modelID.Text()
	}
	start := timeutil.FromValue(meta.Get("request_start_timestamp_ms"))
	end := timeutil.FirstNonEmpty(timeutil.FromValue(meta.Get("stream_end_timestamp_ms")), start)
	user := entry.Get("user")
	userTime := ""
	if user.IsObject() {
		userTime = timeutil.FromValue(user.Get("timestamp"))
	}
	if text := models.Change("model", c.sessionID, model); text != "" {
		if meta.Get("model_id").IsEmpty() {
			text += " source=" + c.source
		}
		emit(fill(c.base(timeutil.FirstNonEmpty(userTime, start, end), model), timeline.TurnSystem, text))
	}
	if user.IsObject() {
		c.userRows(user.Get("content"), timeutil.FirstNonEmpty(userTime, start), model, emit)
	}
	if tag, body := variant(entry.Get("assistant")); tag == "Response" || tag == "ToolUse" {
		if content := body.Get("content"); !content.IsEmpty() {
			emit(fill(c.base(end, model), timeline.TurnAssistant, textOrJSON(content)))
		}
		c.toolUses(body.Get("tool_uses"), end, model, emit)
	}
	if tags := meta.Get("message_meta_tags"); tags.IsArray() {
		for _, tag := range tags.Items() {
			if tag.IsString() && tag.StringValue() == "Compact" {
				emit(fill(c.base(end, model), timeline.TurnSystem,
					"compact: conversation history summarised"))
				break
			}
		}
	}
}

// toolUses emits a tool_use row per call of an assistant message, and
// remembers each call's tool name for its result.
func (c *conversation) toolUses(uses jsonvalue.Value, end, model string, emit parser.Emit) {
	for _, use := range parser.Elements(uses) {
		if !use.IsObject() {
			continue
		}
		toolID := parser.Text(use.Get("id"))
		toolName := parser.Text(use.Get("name"))
		if toolName == "" {
			toolName = parser.Text(use.Get("orig_name"))
		}
		c.names[toolID] = toolName
		row := c.base(end, model)
		row.ToolName, row.ToolUseID = toolName, toolID
		arguments := textOrJSON(use.Get("args"))
		if orig := use.Get("orig_name"); !orig.IsEmpty() && !(orig.IsString() && orig.StringValue() == toolName) {
			arguments = "orig_name=" + orig.Text() + " " + arguments
		}
		emit(fill(row, timeline.TurnToolUse, arguments))
	}
}

// userRows emits the rows of a history entry's user content: the prompt,
// a deny-rule rejection sent as a plain prompt, and each tool result with,
// for a cancelled call, its permission decision.
func (c *conversation) userRows(content jsonvalue.Value, timestamp, model string, emit parser.Emit) {
	tag, body := variant(content)
	prompt := body.Get("prompt")
	cancelled := tag == "CancelledToolUses"
	if (tag == "Prompt" || cancelled) && !prompt.IsEmpty() {
		emit(fill(c.base(timestamp, model), timeline.TurnUser, prompt.Text()))
	}
	if tag == "Prompt" && prompt.IsString() {
		if match := c.parser.denyRule.FindStringSubmatch(prompt.StringValue()); match != nil {
			// The rejection reached the model as a plain prompt: it names the
			// tool but not the call.
			row := c.base(timestamp, model)
			row.ToolName = match[1]
			emit(fill(row, timeline.TurnSystem, c.parser.decisionText(prompt)))
		}
	}
	if tag != "ToolUseResults" && !cancelled {
		return
	}
	for _, result := range parser.Elements(body.Get("tool_use_results")) {
		if !result.IsObject() {
			continue
		}
		row := c.base(timestamp, model)
		row.ToolUseID = parser.Text(result.Get("tool_use_id"))
		row.ToolName = c.names[row.ToolUseID]
		text := resultText(result)
		switch {
		case cancelled && text != "":
			text = "cancelled " + text
		case cancelled:
			text = "cancelled"
		}
		emit(fill(row, timeline.TurnToolResult, text))
		if cancelled {
			decision := c.base(timestamp, model)
			decision.ToolUseID, decision.ToolName = row.ToolUseID, row.ToolName
			emit(fill(decision, timeline.TurnSystem, c.parser.decisionText(prompt)))
		}
	}
}
