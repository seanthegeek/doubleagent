package kiro

import (
	"errors"
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
// none, and the error that stopped the decoding, nil when the whole
// document parsed.
func (p *Parser) salvage(text string) (jsonvalue.Value, bool, error) {
	state, err := jsonvalue.DecodeString(text)
	if err == nil {
		if state.IsObject() {
			return state, true, nil
		}
		return jsonvalue.Value{}, false, errors.New("not a JSON object")
	}
	found := p.historyStart.FindStringIndex(text)
	if found == nil {
		return jsonvalue.Value{}, false, err
	}
	var members []jsonvalue.Member
	if match := p.conversationID.FindStringSubmatch(text[:found[0]]); match != nil {
		if decoded, decodeErr := jsonvalue.DecodeString(`"` + match[1] + `"`); decodeErr == nil {
			members = append(members, jsonvalue.Member{Key: "conversation_id", Value: decoded})
		}
	}
	entries := parser.ObjectsFrom(text, found[1])
	history := jsonvalue.Member{Key: "history", Value: jsonvalue.NewArray(entries...)}
	return jsonvalue.NewObject(append([]jsonvalue.Member{history}, members...)...), true, err
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

// variantOf is variant for a member of a history entry; a variant whose
// body is present but not an object, which the format never writes, is
// counted in c.entries as "<member>.<tag>".
func (c *conversation) variantOf(value jsonvalue.Value, member string) (string, jsonvalue.Value) {
	tag, body := variant(value)
	if tag != "" {
		c.entries.Check(value.Get(tag), member+"."+tag, jsonvalue.Object)
	}
	return tag, body
}

// resultText returns the text of a tool use result, its status in
// brackets after it. The content is a list of blocks; a content that is a
// string is its own text.
func resultText(result jsonvalue.Value) string {
	var parts []string
	content := result.Get("content")
	if content.IsString() {
		parts = append(parts, content.StringValue())
	}
	for _, block := range parser.Elements(content) {
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
	problems           parser.Problems
	// entries counts the problems of the members of history entries and
	// checkpoints, each reported once for the conversation.
	entries parser.EntryTally
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

// conversation emits the rows of one serialised ConversationState read
// from text; key is the working directory the database files it under and
// line the source line of its rows. A member of the wrong kind adds a
// problem row, after the conversation's rows.
func (p *Parser) conversation(artifact *input.Artifact, options parser.Options, text, key string,
	line int, emit parser.Emit) {
	state, ok, err := p.salvage(text)
	if !ok || !state.Get("history").IsArray() {
		if ok && err == nil {
			var found parser.Problems
			parser.ArrayMember(state.Get("history"), "history", line, &found)
			parser.EmitAll(emit, p.ErrorRows(artifact, found, parser.Text(state.Get("conversation_id")), key))
		}
		if err != nil {
			row := p.SystemRow(artifact, "parser: unparseable conversation: "+err.Error())
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
	reader.entries.Report(line, &reader.problems)
	parser.EmitAll(emit, p.ErrorRows(artifact, reader.problems, reader.sessionID, reader.project))
	if err != nil {
		row := p.SystemRow(artifact, "parser: truncated conversation, "+
			strconv.Itoa(state.Get("history").Len())+" history entries recovered: "+err.Error())
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
	if history := state.Get("history"); history.IsArray() {
		for _, entry := range parser.EntriesOfKind(history, "history", jsonvalue.Object, c.line, &c.problems) {
			seen[entryKey(entry)] = true
			c.entryRows(entry, models, c.emit)
		}
	}
	copyModels := parser.Tracker{}
	for _, copied := range c.historyCopies(state) {
		var fresh []jsonvalue.Value
		for _, entry := range copied.entries {
			if !seen[entryKey(entry)] {
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
// its own. A checkpoint_manager, tangent_state, checkpoints or
// main_history of the wrong kind adds a problem; a history_snapshot of the
// wrong kind is counted in c.entries.
func (c *conversation) historyCopies(state jsonvalue.Value) []historyCopy {
	var copies []historyCopy
	manager := parser.ObjectOrEmptyMember(state.Get("checkpoint_manager"), "checkpoint_manager", c.line, &c.problems)
	checkpoints := parser.EntriesOfKind(manager.Get("checkpoints"), "checkpoint_manager.checkpoints",
		jsonvalue.Object, c.line, &c.problems)
	for _, checkpoint := range checkpoints {
		snapshot := checkpoint.Get("history_snapshot")
		entries := c.entries.ListOfKind(snapshot, "checkpoint_manager.checkpoints[].history_snapshot",
			jsonvalue.Object)
		if !snapshot.IsArray() {
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
			timestamp: timeutil.FromValue(checkpoint.Get("timestamp")), entries: entries})
	}
	tangent := parser.ObjectOrEmptyMember(state.Get("tangent_state"), "tangent_state", c.line, &c.problems)
	if main := parser.ListOfKind(tangent.Get("main_history"), "tangent_state.main_history", jsonvalue.Object,
		c.line, &c.problems); len(main) > 0 {
		copies = append(copies, historyCopy{head: "tangent mode: main conversation held aside,",
			entries: main})
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
		"summary: "+parser.SettingValue(summary.Index(0))))
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
// models tracks the model per session for this run over entries. A
// request_metadata, user, assistant or message_meta_tags of the wrong kind
// is counted in c.entries.
func (c *conversation) entryRows(entry jsonvalue.Value, models parser.Tracker, emit parser.Emit) {
	meta := c.entries.ObjectOrEmptyMember(entry.Get("request_metadata"), "request_metadata")
	model := c.stateModel
	if modelID := meta.Get("model_id"); !modelID.IsEmpty() {
		model = modelID.Text()
	}
	start := timeutil.FromValue(meta.Get("request_start_timestamp_ms"))
	end := timeutil.FirstNonEmpty(timeutil.FromValue(meta.Get("stream_end_timestamp_ms")), start)
	user := c.entries.ObjectOrEmptyMember(entry.Get("user"), "user")
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
	assistant := entry.Get("assistant")
	c.entries.Check(assistant, "assistant", jsonvalue.Object)
	if tag, body := c.variantOf(assistant, "assistant"); tag == "Response" || tag == "ToolUse" {
		if content := body.Get("content"); !content.IsEmpty() {
			emit(fill(c.base(end, model), timeline.TurnAssistant, parser.SettingValue(content)))
		}
		c.toolUses(body.Get("tool_uses"), end, model, emit)
	}
	for _, tag := range c.entries.ArrayMember(meta.Get("message_meta_tags"), "request_metadata.message_meta_tags") {
		if tag.IsString() && tag.StringValue() == "Compact" {
			emit(fill(c.base(end, model), timeline.TurnSystem,
				"compact: conversation history summarised"))
			break
		}
	}
}

// toolUses emits a tool_use row per call of an assistant message, and
// remembers each call's tool name for its result. A tool_uses that is not
// an array is counted in c.entries.
func (c *conversation) toolUses(uses jsonvalue.Value, end, model string, emit parser.Emit) {
	for _, use := range c.entries.EntriesOfKind(uses, "tool_uses", jsonvalue.Object) {
		toolID := parser.Text(use.Get("id"))
		toolName := parser.Text(use.Get("name"))
		if toolName == "" {
			toolName = parser.Text(use.Get("orig_name"))
		}
		c.names[toolID] = toolName
		row := c.base(end, model)
		row.ToolName, row.ToolUseID = toolName, toolID
		arguments := parser.SettingValue(use.Get("args"))
		if orig := use.Get("orig_name"); !orig.IsEmpty() && !(orig.IsString() && orig.StringValue() == toolName) {
			arguments = "orig_name=" + orig.Text() + " " + arguments
		}
		emit(fill(row, timeline.TurnToolUse, arguments))
	}
}

// userRows emits the rows of a history entry's user content: the prompt,
// a deny-rule rejection sent as a plain prompt, and each tool result with,
// for a cancelled call, its permission decision. A content that is not an
// object, or a tool_use_results that is not an array, is counted in
// c.entries.
func (c *conversation) userRows(content jsonvalue.Value, timestamp, model string, emit parser.Emit) {
	tag, body := c.variantOf(c.entries.ObjectOrEmptyMember(content, "user.content"), "user.content")
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
	for _, result := range c.entries.EntriesOfKind(body.Get("tool_use_results"), "tool_use_results",
		jsonvalue.Object) {
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
