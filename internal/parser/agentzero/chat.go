package agentzero

import (
	"cmp"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// logSize is how many log items an Agent Zero chat keeps; a chat with that
// many has lost its earliest turns from the log.
const logSize = 1000

// The log item types that are tool calls, and the argument names that stand
// for a call.
var (
	toolTypes = []string{"tool", "mcp", "code_exe", "browser", "subagent"}
	argKeys   = []string{"code", "command", "url", "query", "path"}
	overrides = []string{"chat", "utility", "embedding"}
)

// The headings of the warning log items that record a decision.
const mcpApprovalHeading = "Responses MCP approval required"

var infectionHeadings = [][2]string{
	{"Infection check: requesting clarification", "clarify"},
	{"Infection check: clarification passed", "passed"},
	{"Infection check: TERMINATED", "terminated"},
}

// historyItem is one entry of an agent's history: a message, or the summary
// of a topic or bulk.
type historyItem struct {
	agent   jsonvalue.Value
	summary bool
	node    jsonvalue.Value
}

// historyEntry is a history message and the number of the agent that holds
// it.
type historyEntry struct {
	agent   jsonvalue.Value
	message jsonvalue.Value
}

// walkHistory visits the history's messages and, with summaries, its
// summaries in order: bulks, then topics, then the current topic. It
// counts in wrong the entries it passes over that are present and neither
// null, an array nor an object.
func walkHistory(node jsonvalue.Value, summaries bool, wrong *int, visit func(summary bool, node jsonvalue.Value)) {
	if node.IsArray() {
		for _, child := range node.Items() {
			walkEntry(child, summaries, wrong, visit)
		}
		return
	}
	if !node.IsObject() {
		return
	}
	class := node.Get("_cls")
	switch {
	case parser.IsText(class, "Message"):
		visit(false, node)
	case parser.IsText(class, "History"):
		for _, name := range []string{"bulks", "topics", "current"} {
			walkEntry(node.Get(name), summaries, wrong, visit)
		}
	case parser.IsText(class, "Topic") || parser.IsText(class, "Bulk") || node.Has("messages") || node.Has("records"):
		if summaries && !node.Get("summary").IsEmpty() {
			visit(true, node.Get("summary"))
		}
		walkEntry(parser.FirstNonEmpty(node.Get("messages"), node.Get("records")), summaries, wrong, visit)
	}
}

// walkEntry walks a node inside the history as walkHistory does, counting
// it in wrong when it is present and neither null, an array nor an object.
func walkEntry(node jsonvalue.Value, summaries bool, wrong *int, visit func(summary bool, node jsonvalue.Value)) {
	if !node.IsNull() && !node.IsArray() && !node.IsObject() {
		*wrong++
		return
	}
	walkHistory(node, summaries, wrong, visit)
}

// parseAI returns the assistant's raw output parsed as the agent's JSON
// response, else null, noting its repeated member names in the parse of
// artifact.
func parseAI(artifact *input.Artifact, content jsonvalue.Value) jsonvalue.Value {
	if content.IsObject() {
		return content
	}
	if !content.IsString() {
		return jsonvalue.NewNull()
	}
	text := strings.TrimSpace(content.StringValue())
	if !strings.HasPrefix(text, "{") {
		return jsonvalue.NewNull()
	}
	value, err := parser.DecodeJSON(artifact, text)
	if err != nil || !value.IsObject() {
		return jsonvalue.NewNull()
	}
	return value
}

// headlineText returns a response's headline and thoughts, one per line.
func headlineText(response jsonvalue.Value) string {
	var parts []string
	if headline := response.Get("headline"); !headline.IsEmpty() {
		parts = append(parts, headline.Text())
	}
	if thoughts := response.Get("thoughts"); thoughts.IsArray() {
		for _, thought := range thoughts.Items() {
			if !thought.IsEmpty() {
				parts = append(parts, thought.Text())
			}
		}
	} else if !thoughts.IsEmpty() {
		parts = append(parts, thoughts.Text())
	}
	return strings.Join(parts, "\n")
}

// argsSummary returns the argument that names a tool call's action, else
// the arguments as JSON.
func argsSummary(args jsonvalue.Value) string {
	if args.IsObject() {
		var visible []jsonvalue.Member
		for _, member := range args.Members() {
			if !strings.HasPrefix(member.Key, "_") {
				visible = append(visible, member)
			}
		}
		object := jsonvalue.NewObject(visible...)
		for _, name := range argKeys {
			if value := object.Get(name); parser.Present(value) {
				return value.Text()
			}
		}
		return parser.JSONText(object)
	}
	if args.IsString() {
		return args.StringValue()
	}
	return parser.JSONText(args)
}

// modelOf returns a history message's provider_model_key, or "".
func modelOf(message jsonvalue.Value) string {
	return parser.Text(message.Get("metadata").Get("responses").Get("provider_model_key"))
}

// contentText returns a message's content as text: a list of blocks as their
// text, an object without text as compact JSON, anything else as the text
// of a chat message.
func contentText(value jsonvalue.Value) string {
	if value.IsObject() && !value.Has("text") {
		return parser.JSONText(value)
	}
	return parser.TextOf(value)
}

// recoverLogs returns the complete log items before the point where a
// damaged file breaks off, noting their repeated member names in the parse
// of artifact.
func (p *Parser) recoverLogs(artifact *input.Artifact, raw string) []jsonvalue.Value {
	match := p.logs.FindStringIndex(raw)
	if match == nil {
		return nil
	}
	document := jsonvalue.NewDocument(raw)
	defer func() { parser.NoteRepeats(artifact, document.TakeRepeats()) }()
	var items []jsonvalue.Value
	position := match[1]
	for {
		for position < len(raw) && strings.IndexByte(" \t\r\n,", raw[position]) >= 0 {
			position++
		}
		item, end, err := document.DecodePrefix(position)
		if err != nil || !item.IsObject() {
			return items
		}
		items = append(items, item)
		position = end
	}
}

// remoteCopyTexts returns the "session synced:" and "session relayed:" texts
// of a chat: a synced text names a provider response chain, a relayed text a
// Telegram, WhatsApp or email chat, with the Telegram bot's token and
// webhook secret through parser.SecretText. A telegram_bot_cfg that is not
// an object adds a problem.
func remoteCopyTexts(artifact *input.Artifact, chat, data jsonvalue.Value, problems *parser.Problems) []string {
	var texts []string
	for i, agent := range parser.Elements(chat.Get("agents")) {
		state := agent.Get("data").Get("responses_state")
		if !state.IsObject() || !parser.Present(state.Get("previous_response_id")) {
			continue
		}
		count := 0
		if ids := state.Get("response_ids"); ids.IsArray() {
			count = ids.Len()
		}
		number := strconv.Itoa(i)
		if declared, ok := agent.Lookup("number"); ok {
			number = declared.Text()
		}
		texts = append(texts, "session synced: provider response "+key(state, "response_id")+
			" previous "+key(state, "previous_response_id")+" model "+key(state, "provider_model_key")+
			" ("+strconv.Itoa(count)+" response ids) agent="+number+
			" (from chat.json agents[].data.responses_state, file mtime)")
	}
	if slices.ContainsFunc(telegramKeys, func(name string) bool { return parser.Present(data.Get(name)) }) {
		text := "session relayed: telegram bot " + key(data, "telegram_bot") + " chat " +
			key(data, "telegram_chat_id") + " user " + key(data, "telegram_user_id") + " @" +
			key(data, "telegram_username")
		parser.ObjectMember(data.Get("telegram_bot_cfg"), "telegram_bot_cfg", 0, problems)
		for _, name := range []string{"token", "webhook_secret"} {
			if secret := parser.Text(data.Get("telegram_bot_cfg").Get(name)); secret != "" {
				text += " " + name + "=" + parser.SecretText(artifact, secret)
			}
		}
		texts = append(texts, text+" (from chat.json data, file mtime)")
	}
	if parser.Present(data.Get("wa_chat_id")) {
		texts = append(texts, "session relayed: whatsapp chat "+key(data, "wa_chat_id")+" number "+
			key(data, "wa_sender_number")+" (from chat.json data, file mtime)")
	}
	if slices.ContainsFunc([]string{"email_thread_id", "email_sender"},
		func(name string) bool { return parser.Present(data.Get(name)) }) {
		texts = append(texts, "session relayed: email thread "+key(data, "email_thread_id")+" from "+
			key(data, "email_sender")+" (from chat.json data, file mtime)")
	}
	return texts
}

// modelName returns "<provider>/<name>" of a model configuration, or ""
// when it names neither.
func modelName(config jsonvalue.Value) string {
	var parts []string
	for _, name := range []string{"provider", "name"} {
		if part := strings.TrimSpace(parser.Text(config.Get(name))); part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, "/")
}

// overrideText returns the model data.chat_model_override requests, as row
// text: a preset by its name, a custom configuration by the provider and
// name of each slot, each followed by the api_key and api_base a custom
// configuration from the connector can carry.
func overrideText(artifact *input.Artifact, override jsonvalue.Value) string {
	if !override.IsObject() || override.Len() == 0 {
		return ""
	}
	if preset := override.Get("preset_name"); !preset.IsEmpty() {
		return "preset " + preset.Text()
	}
	var slots []string
	for _, slot := range overrides {
		if config := override.Get(slot); config.IsObject() && modelName(config) != "" {
			slots = append(slots, slot+"="+modelName(config)+endpointText(artifact, config, slot+"."))
		}
	}
	if len(slots) > 0 {
		return strings.Join(slots, " ")
	}
	return cmp.Or(modelName(override), "custom configuration") + endpointText(artifact, override, "")
}

// endpointText returns " <prefix>api_key=<key> <prefix>api_base=<url>" of
// a model configuration, each part when it is set, the key through
// parser.SecretText.
func endpointText(artifact *input.Artifact, config jsonvalue.Value, prefix string) string {
	var text string
	if key := parser.Text(config.Get("api_key")); key != "" {
		text += " " + prefix + "api_key=" + parser.SecretText(artifact, key)
	}
	if base := parser.Text(config.Get("api_base")); base != "" {
		text += " " + prefix + "api_base=" + base
	}
	return text
}

// chatSetting is the text of an editor-session or chat-model row and the
// project path it names, "" for none.
type chatSetting struct {
	text, project string
}

// chatSettingRows returns the editor-session and chat-model rows of a chat's
// data, each value the latest as of the file's modification time; ACP is the
// Agent Client Protocol an editor session uses. Only the scratch directory
// row names its own project path. A chat_model_override, or a slot of it,
// that is not an object adds a problem.
func chatSettingRows(artifact *input.Artifact, data jsonvalue.Value, problems *parser.Problems) []chatSetting {
	var rows []chatSetting
	if mode := data.Get("acp_mode"); !mode.IsEmpty() {
		rows = append(rows, chatSetting{"permission mode: " + mode.Text() + " (acp, as of file mtime)", ""})
	}
	if model := data.Get("acp_model_id"); !model.IsEmpty() {
		rows = append(rows, chatSetting{"model: requested " + model.Text() + " (acp, as of file mtime)", ""})
	}
	override := data.Get("chat_model_override")
	parser.ObjectMember(override, "chat_model_override", 0, problems)
	for _, slot := range overrides {
		parser.ObjectMember(override.Get(slot), "chat_model_override."+slot, 0, problems)
	}
	if requested := overrideText(artifact, override); requested != "" {
		rows = append(rows, chatSetting{"model: requested " + requested +
			" (chat override, as of file mtime)", ""})
	}
	if workdir := data.Get("workdir_path"); workdir.IsString() && strings.TrimSpace(workdir.StringValue()) != "" {
		path := strings.TrimSpace(workdir.StringValue())
		rows = append(rows, chatSetting{"scratch directory: " + path + " (acp container transport, " +
			"from chat.json data.workdir_path, as of file mtime)", path})
	}
	return rows
}

// warningDecision returns the text and tool name of a warning log item's
// "permission decision:" row, two empty texts when it records no decision.
func (p *Parser) warningDecision(item jsonvalue.Value) (string, string) {
	heading := parser.Text(item.Get("heading"))
	if content := item.Get("content"); content.IsString() {
		if match := p.blocked.FindStringSubmatch(content.StringValue()); match != nil {
			return "permission decision: blocked tool=" + match[1] + " profile=" + match[2], match[1]
		}
	}
	if strings.Contains(heading, mcpApprovalHeading) {
		return "permission decision: denied automatically (provider MCP approval request)",
			"mcp_approval_request"
	}
	for _, each := range infectionHeadings {
		if strings.Contains(heading, each[0]) {
			return "permission decision: " + each[1] + " (infection check)", ""
		}
	}
	return "", ""
}

// chat is the state of one chat file's parse: the row every row of it starts
// from, what the history holds, the models seen and the first failure that
// stops the parse.
type chat struct {
	parser   *Parser
	artifact *input.Artifact
	options  parser.Options
	emit     parser.Emit
	base     timeline.Row
	history  map[string]historyEntry
	// later holds the history messages whose id an earlier message has, in
	// history order, and trimmed says the log was cut to its last items.
	later   []historyEntry
	trimmed bool
	// fileRefs holds the message file each history id's message names, and
	// fileTexts the text each file gave, read at most once.
	fileRefs  map[string]fileRef
	fileTexts map[parser.EntryKey]string
	// replies holds the text of each history reply read as the agent's
	// JSON response, by id, and whether it read as one.
	replies map[string]decodedReply
	// keys holds the group key of each history id a log item names
	// (groupKey), grouped the keys more than one log item reaches, groups
	// the group of each (timeline.Group) and parented the ids with a
	// parent in it.
	keys     map[string]groupKey
	grouped  map[groupKey]bool
	groups   map[groupKey]*timeline.Group
	parented map[string]bool
	chosen   map[chosenKey]timeline.RowText
	shared   map[sharedKey]*timeline.SharedText
	// firstItems holds the time and source line of each id's first log
	// item.
	firstItems map[string]logPlace
	// work counts the message files read and the replies decoded, which
	// tests hold to one each.
	work   parser.Work
	models parser.Tracker
	// messages is the messages directory beside the chat, the zero
	// Directory when there is none.
	messages parser.Directory
	problems parser.Problems
	failure  error
}

// zeroAgent reports whether an agent number is the main agent's: null, zero,
// false or the text "0"; a number beyond the float64 range is not zero.
func zeroAgent(number jsonvalue.Value) bool {
	switch {
	case number.IsNull():
		return true
	case number.IsBool():
		flag, _ := number.Bool()
		return !flag
	case number.IsNumber():
		value, ok := number.Float()
		return ok && value == 0
	}
	return parser.IsText(number, "0")
}

// prefix returns text led by its agent: "[agent <n>] " for a subordinate
// agent, the text itself for the main one.
func prefix(agent jsonvalue.Value, text string) string {
	if zeroAgent(agent) {
		return text
	}
	return "[agent " + agent.Text() + "] " + text
}

// row returns a system, user, assistant, tool or thinking row of the chat.
func (c *chat) row(turn, text string, line int, timestamp string) timeline.Row {
	row := c.base
	row.TurnType = turn
	row.Text = timeline.Compact(text)
	row.SourceLine = line
	row.TimestampUTC = timestamp
	return row
}

// parseChat emits the rows of a chat file. A failure to look for a message
// file, other than the file being absent, stops the parse and is returned:
// the run reports it as a problem with the whole artifact.
func (p *Parser) parseChat(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	name := filepath.Base(artifact.DiskPath)
	legacy := !p.chat.MatchString(artifact.Rel)
	fallbackID := ""
	if directory, ok := parser.Beside(artifact); ok {
		fallbackID = filepath.Base(directory)
	}
	if legacy {
		fallbackID = parser.Stem(name)
	}
	state := &chat{parser: p, artifact: artifact, options: options, emit: emit,
		base: p.BaseRow(artifact), history: map[string]historyEntry{},
		replies: map[string]decodedReply{}, chosen: map[chosenKey]timeline.RowText{},
		shared: map[sharedKey]*timeline.SharedText{},
		work:   p.Work, models: parser.Tracker{}}
	state.base.SessionID = fallbackID
	content, ok := parser.FileBytes(artifact.DiskPath, &state.problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, state.problems, fallbackID, ""))
		return nil
	}
	raw := textlines.ValidUTF8(content)
	document, err := parser.DecodeJSON(artifact, raw)
	reason := ""
	switch {
	case err != nil:
		// The file holds the bot token and model API keys the parser shows,
		// and the reader's error can quote it.
		reason = parser.ReaderErrorText(artifact, err)
	case !document.IsObject():
		reason = "top level is not an object"
	}
	if reason != "" {
		emit(state.row(timeline.TurnSystem, "parser: chat.json did not parse ("+reason+
			"); log items recovered without history", 0, ""))
		state.logRows(p.recoverLogs(artifact, raw))
		return state.failure
	}
	if !legacy {
		state.messages = messagesBeside(artifact)
	}
	state.chatRows(document, fallbackID)
	return state.failure
}

// messagesBeside returns the messages directory beside a chat file, the
// zero Directory when there is none or it is a link.
func messagesBeside(artifact *input.Artifact) parser.Directory {
	path, found := parser.Beside(artifact, "messages")
	if !found {
		return parser.Directory{}
	}
	messages, _ := parser.DirectoryAt(path)
	return messages
}

// chatRows emits the rows of a chat that decoded.
func (c *chat) chatRows(document jsonvalue.Value, fallbackID string) {
	c.base.SessionID = parser.TextOr(document.Get("id"), fallbackID)
	data := parser.ObjectOrEmptyMember(document.Get("data"), "data", 0, &c.problems)
	if project := data.Get("project"); !project.IsEmpty() {
		c.base.ProjectPath = projectOf(project)
	}
	// An editor (ACP) session's workspace is the user's project on the host
	// that ran the editor.
	if cwd := data.Get("acp_cwd"); cwd.IsString() && strings.TrimSpace(cwd.StringValue()) != "" {
		c.base.ProjectPath = strings.TrimSpace(cwd.StringValue())
	}
	c.startRows(document, data)
	logs := c.logItems(document)
	c.trimmed = len(logs) >= logSize
	ordered := c.agentHistories(document, c.trimmed)
	if c.trimmed {
		c.trimmedRows(logs, ordered)
	}
	c.logRows(logs)
	if c.failure == nil {
		parser.EmitAll(c.emit, c.parser.ErrorRows(c.artifact, c.problems, c.base.SessionID,
			c.base.ProjectPath))
	}
}

// startRows emits the chat start row and the standing rows from the chat's
// data.
func (c *chat) startRows(document, data jsonvalue.Value) {
	label := func(name string, value jsonvalue.Value) string {
		if !value.IsEmpty() {
			return name + "=" + value.Text()
		}
		return ""
	}
	text := parser.JoinNonEmpty(" | ", "chat start", parser.Text(document.Get("name")),
		label("type", document.Get("type")), label("profile", document.Get("agent_profile")),
		label("project", data.Get("project")))
	start := c.row(timeline.TurnSystem, text, 0, "")
	start.TimestampUTC = timeutil.FromValue(document.Get("created_at"))
	c.emit(start)
	for _, text := range remoteCopyTexts(c.artifact, document, data, &c.problems) {
		c.emit(c.row(timeline.TurnSystem, text, 0, parser.FileMtime(c.artifact)))
	}
	for _, setting := range chatSettingRows(c.artifact, data, &c.problems) {
		row := c.row(timeline.TurnSystem, setting.text, 0, parser.FileMtime(c.artifact))
		if setting.project != "" {
			row.ProjectPath = setting.project
		}
		c.emit(row)
	}
}

// logItems returns the chat's log items that are objects; a log that is
// not an object, or logs that is not an array, adds a problem, and so do
// the items that are not objects, one for them all.
func (c *chat) logItems(document jsonvalue.Value) []jsonvalue.Value {
	log := document.Get("log")
	if !log.IsObject() && !log.IsNull() {
		c.problems.Add(0, parser.WrongKind("log", jsonvalue.Object))
		return nil
	}
	return parser.ListOfKind(log.Get("logs"), "log.logs", jsonvalue.Object, 0, &c.problems)
}

// agentHistories reads each agent's history, filling the chat's message
// index and returning its items in order. An agents member that is not an
// array adds a problem, and so do the agents that are not objects, one for
// them all, and the entries of an agent's history that are not objects,
// the history itself among them, one per agent, and so do the messages
// whose id an earlier message has, one per agent: the first message of an
// id is the one its log items join (newLogItem), and each later one is
// kept for laterRows. An agent's number, which each of its rows starts
// with, is cut by the kit (parser.Carried).
func (c *chat) agentHistories(document jsonvalue.Value, summaries bool) []historyItem {
	var ordered []historyItem
	carried := parser.NewCarried(c.artifact)
	defer carried.Report(0, &c.problems)
	for i, agent := range parser.EntriesOfKind(document.Get("agents"), "agents", jsonvalue.Object, 0, &c.problems) {
		number := jsonvalue.NewInt(int64(i))
		if declared, ok := agent.Lookup("number"); ok {
			number = declared
		}
		if text := number.Text(); len(text) > parser.CarriedLimit && !zeroAgent(number) {
			number = jsonvalue.NewString(carried.Cut("agent number", text))
		}
		history := agent.Get("history")
		if history.IsString() {
			history = jsonvalue.NewObject()
			if text := agent.Get("history").StringValue(); text != "" {
				decoded, err := parser.DecodeJSON(c.artifact, text)
				if err != nil {
					c.emit(c.row(timeline.TurnSystem, "parser: history of agent "+number.Text()+
						" did not parse ("+err.Error()+")", 0, ""))
					continue
				}
				history = decoded
			}
		}
		wrong, repeats := 0, 0
		walkEntry(history, summaries, &wrong, func(summary bool, node jsonvalue.Value) {
			ordered = append(ordered, historyItem{agent: number, summary: summary, node: node})
			if messageID := node.Get("id"); !summary && !messageID.IsEmpty() {
				entry := historyEntry{agent: number, message: node}
				if _, held := c.history[messageID.Text()]; held {
					c.later = append(c.later, entry)
					repeats++
				} else {
					c.history[messageID.Text()] = entry
				}
			}
		})
		if wrong > 0 {
			c.problems.Add(0, parser.WrongEntries("history of agent "+number.Text(), jsonvalue.Object, wrong))
		}
		if repeats > 0 {
			c.problems.Add(0, repeatsText(number, repeats))
		}
	}
	return ordered
}

// repeatsText returns the problem of an agent's history holding count
// messages whose id an earlier message has.
func repeatsText(agent jsonvalue.Value, count int) string {
	messages := strconv.Itoa(count) + " messages"
	if count == 1 {
		messages = "1 message"
	}
	return "history of agent " + agent.Text() + " holds " + messages +
		" repeating an earlier message's id; each gives its own rows, at a log item of the id or, " +
		"with no such item, without a time"
}

// trimmedRows emits, for a chat whose log was cut to its last items, the
// note and the turns the log no longer holds, from the agent history.
func (c *chat) trimmedRows(logs []jsonvalue.Value, ordered []historyItem) {
	c.emit(c.row(timeline.TurnSystem, "log trimmed to its last "+strconv.Itoa(logSize)+
		" items; earlier turns follow from the agent history without timestamps", 0, ""))
	inLog := map[string]bool{}
	for _, item := range logs {
		if logID := item.Get("id"); !logID.IsEmpty() {
			inLog[logID.Text()] = true
		}
	}
	for _, each := range ordered {
		switch {
		case each.summary:
			c.emit(c.row(timeline.TurnSystem, prefix(each.agent, "[summary] "+contentText(each.node)), 0, ""))
		case !inLog[parser.Text(each.node.Get("id"))]:
			c.historyRows(each.agent, each.node)
		}
	}
}

// modelRow emits the "model:" row of a new or changed model. The model is
// tracked per chat and agent number; a subordinate agent's row ends " agent=<n>".
func (c *chat) modelRow(agent jsonvalue.Value, model string, line int, timestamp string) {
	text := c.models.Change("model", c.base.SessionID+"\x00"+agent.Text(), model)
	if text == "" {
		return
	}
	if !zeroAgent(agent) {
		text += " agent=" + agent.Text()
	}
	row := c.row(timeline.TurnSystem, text, line, timestamp)
	row.Model = model
	c.emit(row)
}

// historyRows emits the rows of one history message.
func (c *chat) historyRows(agent, message jsonvalue.Value) {
	content := message.Get("content")
	if !message.Get("ai").IsEmpty() {
		c.assistantHistory(agent, message, content)
		return
	}
	switch {
	case content.IsObject() && content.Has("tool_result"):
		row := c.row(timeline.TurnToolResult, prefix(agent, contentText(content.Get("tool_result"))), 0, "")
		row.ToolName, row.ToolUseID = parser.Text(content.Get("tool_name")), parser.Text(message.Get("id"))
		c.emit(row)
	case content.IsObject() && content.Has("user_message"):
		c.emit(c.row(timeline.TurnUser, prefix(agent, contentText(content.Get("user_message"))), 0, ""))
	default:
		c.emit(c.row(timeline.TurnSystem, prefix(agent, contentText(content)), 0, ""))
	}
}

// assistantHistory emits the model row, the reply and the tool call of an
// assistant history message.
func (c *chat) assistantHistory(agent, message, content jsonvalue.Value) {
	parsed := parseAI(c.artifact, content)
	model := modelOf(message)
	c.modelRow(agent, model, 0, "")
	text := contentText(content)
	if !parsed.IsEmpty() {
		text = headlineText(parsed)
	}
	if text != "" {
		row := c.row(timeline.TurnAssistant, prefix(agent, text), 0, "")
		row.Model = model
		c.emit(row)
	}
	if !parsed.IsEmpty() && !parsed.Get("tool_name").IsEmpty() {
		row := c.row(timeline.TurnToolUse, prefix(agent, argsSummary(parsed.Get("tool_args"))), 0, "")
		row.ToolName, row.Model = parsed.Get("tool_name").Text(), model
		c.emit(row)
	}
}

// fileRef is the message file a history message names: its path in the
// messages directory and the key of the entry it opens.
type fileRef struct {
	path string
	key  parser.EntryKey
}

// messageFileRef returns the message file a history message's content
// names, and whether it names a file name in the messages directory.
func (c *chat) messageFileRef(content jsonvalue.Value) (fileRef, bool) {
	if !content.IsObject() || content.Get("file").IsEmpty() {
		return fileRef{}, false
	}
	name := strings.ReplaceAll(content.Get("file").Text(), `\`, "/")
	path, named := c.messages.Child(name[strings.LastIndex(name, "/")+1:])
	if !named {
		return fileRef{}, false
	}
	return fileRef{path: path, key: parser.KeyOf(path)}, true
}

// messageFileText returns the text of a message file, "" when it is absent
// or not a regular file. A file is looked for once per chat, by the entry
// it opens, so every spelling of its name that opens it gives the text of
// that one read. A file that exists but cannot be read adds its unreadable
// problem; a failure to look for it stops the parse.
func (c *chat) messageFileText(ref fileRef) string {
	if text, ok := c.fileTexts[ref.key]; ok {
		return text
	}
	if c.fileTexts == nil {
		c.fileTexts = map[parser.EntryKey]string{}
	}
	c.fileTexts[ref.key] = ""
	info, err := osfile.Lstat(ref.path)
	if err != nil {
		if !absentLookup(err) {
			c.failure = err
		}
		return ""
	}
	if !info.Mode().IsRegular() {
		return ""
	}
	c.work.Add("message files read", 1)
	data, ok := parser.FileBytes(ref.path, &c.problems)
	if !ok {
		return ""
	}
	text := textlines.ValidUTF8(data)
	c.fileTexts[ref.key] = text
	return text
}

// logItem is one log item with what its rows are built from.
type logItem struct {
	item, kvps, content jsonvalue.Value
	kind, itemID        string
	line                int
	timestamp           string
	agent               jsonvalue.Value
	message, body       jsonvalue.Value
	hasMessage          bool
}

// newLogItem reads a log item against the chat's history.
func (c *chat) newLogItem(item jsonvalue.Value) *logItem {
	current := &logItem{item: item, kind: parser.Text(item.Get("type")),
		timestamp: timeutil.FromValue(item.Get("timestamp")), kvps: item.Get("kvps"),
		itemID: parser.Text(item.Get("id")), agent: jsonvalue.NewInt(0), content: jsonvalue.NewString(""),
		message: jsonvalue.NewNull(), body: jsonvalue.NewNull()}
	if number, ok := item.Get("no").Int(); ok && item.Get("no").IsInt() {
		current.line = int(number)
	}
	if agent, ok := item.Lookup("agentno"); ok {
		current.agent = agent
	}
	if !current.kvps.IsObject() {
		current.kvps = jsonvalue.NewObject()
	}
	if content := item.Get("content"); !content.IsEmpty() {
		current.content = content
	}
	if held, ok := c.history[current.itemID]; ok && current.itemID != "" && !held.message.IsEmpty() {
		current.message, current.hasMessage = held.message, true
		current.body = held.message.Get("content")
	}
	return current
}

// groupKey names the group of the log items that could show one text:
// the message file a history message names, by the entry it opens, so
// the items of every id whose message names it are one group; else the
// history id (file the zero key).
type groupKey struct {
	itemID string
	file   parser.EntryKey
}

// logPlace is the time and source line of a log item.
type logPlace struct {
	timestamp string
	line      int
}

// groupKeys sets the group key of each history id a log item names, the
// message file of each id whose message names one, and the keys that
// more than one log item reaches; only those have a group.
func (c *chat) groupKeys(logs []jsonvalue.Value) {
	c.keys, c.fileRefs, c.grouped = map[string]groupKey{}, map[string]fileRef{}, map[groupKey]bool{}
	c.groups, c.parented, c.firstItems = map[groupKey]*timeline.Group{}, map[string]bool{}, map[string]logPlace{}
	counts := map[groupKey]int{}
	for _, item := range logs {
		itemID := parser.Text(item.Get("id"))
		held, ok := c.history[itemID]
		if !ok || itemID == "" || held.message.IsEmpty() {
			continue
		}
		key, known := c.keys[itemID]
		if !known {
			key = groupKey{itemID: itemID}
			if ref, named := c.messageFileRef(held.message.Get("content")); named {
				c.fileRefs[itemID] = ref
				key = groupKey{file: ref.key}
			}
			c.keys[itemID] = key
		}
		counts[key]++
		if counts[key] > 1 {
			c.grouped[key] = true
		}
	}
}

// joined is how a log item's row of turn reads its id's history message:
// text gives the row's text with the message's content as body, or null
// for the item's own, and whether the message gave it; with file, the
// text of a message file the message names stands in its place when the
// file gives one; suffix ends the text either way; with optional the item
// writes no row without text.
type joined struct {
	turn     string
	text     func(body jsonvalue.Value) (string, bool)
	file     bool
	suffix   string
	optional bool
}

// sharedKey names one text a group's rows can show: its source (the
// message file, else the history id) and the turn that reads it.
type sharedKey struct {
	source groupKey
	turn   string
}

// chosenKey names the text a row shows with a shared text: led by the
// item's agent and followed by the suffix.
type chosenKey struct {
	shared        sharedKey
	agent, suffix string
}

// joinedRow returns a log item's row, with its history message's text (or
// its message file's) when the message gives one, else its own, and
// whether that text is set. When more than one log item reaches the
// text's group (groupKeys: one per message file, holding the items of
// every id whose message names it, else one per id), the row is a parent
// of it with both texts (Parent.Texts): the text is shown only in the
// first such item the filters keep, the others showing their own content
// but for the first item of each later id, which shows what its message
// gives apart from the file. An optional row with no text of its own is
// shown only when chosen (timeline.WhenChosen).
func (c *chat) joinedRow(current *logItem, how joined,
	row func(turn, text string) timeline.Row) (timeline.Row, bool) {
	shown, fromHistory := how.text(current.body)
	full, shows, source := shown, fromHistory, groupKey{itemID: current.itemID}
	if how.file {
		if text := c.historyFile(current); text != "" {
			full, shows, source = text, true, c.keys[current.itemID]
		}
	}
	hasText := full != "" || how.suffix != ""
	key := c.keys[current.itemID]
	if !shows || !c.grouped[key] || c.failure != nil {
		return row(how.turn, full+how.suffix), hasText
	}
	group, later := c.groups[key]
	if !later {
		group = timeline.NewGroup()
		c.groups[key] = group
	}
	chosen := c.chosenText(current, how, full, source)
	made := row(how.turn, "")
	made.Text, made.Shared = chosen.Text, chosen.Shared
	parent := group.Parent(&made)
	firstOfID := !c.parented[current.itemID]
	c.parented[current.itemID] = true
	own, _ := how.text(jsonvalue.NewNull())
	switch {
	case later && firstOfID && fromHistory:
		parent.Texts(&made, chosen, timeline.RowText{Text: row(how.turn, shown+how.suffix).Text})
	case !how.optional || own != "" || how.suffix != "":
		parent.Texts(&made, chosen, timeline.RowText{Text: row(how.turn, own+how.suffix).Text})
	default:
		parent.Row(&made, timeline.WhenChosen)
	}
	return made, true
}

// chosenText returns the text an item's row of turn shows with the text of
// its group, shown: led by the item's agent and followed by the suffix,
// built once per source, turn, agent and suffix. The text itself is one
// shared text (timeline.SharedText) per source and turn, so memory holds
// it once however many items and ids hold it, and only the chosen item's
// row writes it.
func (c *chat) chosenText(current *logItem, how joined, shown string, source groupKey) timeline.RowText {
	key := chosenKey{sharedKey{source, how.turn}, current.agent.Text(), how.suffix}
	if chosen, ok := c.chosen[key]; ok {
		return chosen
	}
	shared, ok := c.shared[key.shared]
	if !ok {
		shared = timeline.NewSharedText(shown)
		c.shared[key.shared] = shared
	}
	var builder timeline.TextBuilder
	builder.Write(prefix(current.agent, ""))
	builder.Share(shared, "[history message]", historyPointer)
	builder.Write(how.suffix)
	chosen := builder.Text()
	c.chosen[key] = chosen
	return chosen
}

// historyPointer is what a row writes for the history message or message
// file of a group another row holds in full; only one item of the group
// shows it, so no row writes it but a copy of that item's row.
func historyPointer(holder *timeline.Row, _ bool) string {
	return "[history message shown in full above, at source line " + strconv.Itoa(holder.SourceLine) + "]"
}

// logRows emits the rows of the log items, then those of the history
// messages whose id an earlier message has; a message file that cannot be
// read adds a problem.
func (c *chat) logRows(logs []jsonvalue.Value) {
	c.groupKeys(logs)
	for _, item := range logs {
		if c.failure != nil {
			return
		}
		current := c.newLogItem(item)
		if _, seen := c.firstItems[current.itemID]; current.hasMessage && !seen {
			c.firstItems[current.itemID] = logPlace{current.timestamp, current.line}
		}
		row := func(turn, text string) timeline.Row {
			return c.row(turn, prefix(current.agent, text), current.line, current.timestamp)
		}
		switch {
		case current.kind == "user":
			c.userLogRow(current, row)
		case current.kind == "agent":
			c.agentLogRows(current, row)
		case current.kind == "response":
			c.emit(row(timeline.TurnAssistant, contentText(current.content)))
		case slices.Contains(toolTypes, current.kind):
			c.toolLogRows(current, row)
		default:
			c.otherLogRows(current, row)
		}
	}
	if c.failure == nil {
		c.laterRows()
	}
}

// laterRows emits the rows of each history message whose id an earlier
// message has, as a history message's rows (laterMessageRows). When a log
// item names the id, each row is a member of the group the id's items are
// parents of (timeline.Member), at the time and source line of the item
// the group goes with, else at those of the id's first log item. When none
// does, every message of the id gives its rows without a time, as a
// trimmed log's history does; in a trimmed log trimmedRows has given them.
func (c *chat) laterRows() {
	unnamed := map[string]bool{}
	for _, entry := range c.later {
		itemID := entry.message.Get("id").Text()
		place, named := c.firstItems[itemID]
		if !named {
			if !c.trimmed {
				if !unnamed[itemID] {
					unnamed[itemID] = true
					parser.EmitAll(c.emit, c.laterMessageRows(c.history[itemID]))
				}
				parser.EmitAll(c.emit, c.laterMessageRows(entry))
			}
			continue
		}
		for _, row := range c.laterMessageRows(entry) {
			if c.parented[itemID] {
				c.groups[c.keys[itemID]].Member(&row, timeline.TakeTime|timeline.TakeSourceLine)
			} else {
				row.TimestampUTC, row.SourceLine = place.timestamp, place.line
			}
			c.emit(row)
		}
	}
}

// laterMessageRows returns the rows of a history message of a repeated id
// (historyRows). Such a message holds no place in the chat's sequence of
// models, so its model row names its own model alone (a tracker of its
// own), and the chat's tracker is left as the log left it.
func (c *chat) laterMessageRows(entry historyEntry) []timeline.Row {
	emit, models := c.emit, c.models
	defer func() { c.emit, c.models = emit, models }()
	var rows []timeline.Row
	c.emit = func(row timeline.Row) { rows = append(rows, row) }
	c.models = parser.Tracker{}
	c.historyRows(entry.agent, entry.message)
	return rows
}

// userLogRow emits a user log item, with the text the history holds for it
// and the names of its attachments.
func (c *chat) userLogRow(current *logItem, row func(turn, text string) timeline.Row) {
	suffix := ""
	if attachments := current.kvps.Get("attachments"); attachments.IsArray() && attachments.Len() > 0 {
		names := make([]string, 0, attachments.Len())
		for _, attachment := range attachments.Items() {
			names = append(names, attachment.Text())
		}
		suffix = " [attachments: " + strings.Join(names, ", ") + "]"
	}
	made, _ := c.joinedRow(current, joined{turn: timeline.TurnUser, suffix: suffix,
		text: func(body jsonvalue.Value) (string, bool) {
			if body.IsObject() && body.Has("user_message") {
				return contentText(body.Get("user_message")), true
			}
			return contentText(current.content), false
		}}, row)
	c.emit(made)
}

// agentLogRows emits the model row, the reasoning and the reply of an agent
// log item.
func (c *chat) agentLogRows(current *logItem, row func(turn, text string) timeline.Row) {
	model := modelOf(current.message)
	c.modelRow(current.agent, model, current.line, current.timestamp)
	if reasoning := current.kvps.Get("reasoning"); c.options.IncludeThinking && !reasoning.IsEmpty() {
		made := row(timeline.TurnThinking, contentText(reasoning))
		made.Model = model
		c.emit(made)
	}
	headline := headlineText(current.kvps)
	made, hasText := c.joinedRow(current, joined{turn: timeline.TurnAssistant, optional: true,
		text: func(body jsonvalue.Value) (string, bool) {
			text := headline
			if text == "" {
				if replied, parsed := c.reply(current, body); parsed {
					text = replied
				} else if current.hasMessage {
					text = contentText(body)
				}
				if text != "" {
					return text, true
				}
			}
			return cmp.Or(text, contentText(current.content)), false
		}}, row)
	if hasText {
		made.Model = model
		c.emit(made)
	}
}

// decodedReply is a history reply read as the agent's JSON response
// (parseAI): its headline and thoughts, and whether it read as one.
type decodedReply struct {
	text   string
	parsed bool
}

// reply returns the history reply body an agent item joins, read as the
// agent's JSON response, and whether it read as one: it is decoded once
// per id, since any number of items of the id read it.
func (c *chat) reply(current *logItem, body jsonvalue.Value) (string, bool) {
	if !current.hasMessage || body.IsNull() {
		return "", false
	}
	if held, ok := c.replies[current.itemID]; ok {
		return held.text, held.parsed
	}
	c.work.Add("history replies decoded", 1)
	parsed := parseAI(c.artifact, body)
	held := decodedReply{parsed: !parsed.IsEmpty()}
	if held.parsed {
		held.text = headlineText(parsed)
	}
	c.replies[current.itemID] = held
	return held.text, held.parsed
}

// toolLogRows emits the call and the result of a tool log item.
func (c *chat) toolLogRows(current *logItem, row func(turn, text string) timeline.Row) {
	name := parser.TextOr(parser.FirstNonEmpty(current.kvps.Get("_tool_name"),
		current.message.Get("content").Get("tool_name")), current.kind)
	toolID := current.itemID
	if toolID == "" {
		toolID = "log:" + strconv.Itoa(current.line)
	}
	call := row(timeline.TurnToolUse, argsSummary(current.kvps))
	call.ToolName, call.ToolUseID = name, toolID
	c.emit(call)
	made, hasResult := c.joinedRow(current, joined{turn: timeline.TurnToolResult, file: true,
		text: func(body jsonvalue.Value) (string, bool) {
			if body.IsObject() && body.Has("tool_result") {
				if result := contentText(body.Get("tool_result")); result != "" {
					return result, true
				}
			}
			return contentText(current.content), false
		}}, row)
	if c.failure != nil {
		return
	}
	if hasResult || current.hasMessage {
		made.ToolName, made.ToolUseID = name, toolID
		c.emit(made)
	}
}

// historyFile returns the text of the message file a log item's history
// message names (groupKeys), read once per file however many items and
// ids reach it.
func (c *chat) historyFile(current *logItem) string {
	ref, named := c.fileRefs[current.itemID]
	if !named || !current.hasMessage {
		return ""
	}
	return c.messageFileText(ref)
}

// otherLogRows emits the row of any other log item: its type, heading and
// content, and for a warning the permission decision it records.
func (c *chat) otherLogRows(current *logItem, row func(turn, text string) timeline.Row) {
	heading := strings.TrimSpace(c.parser.icon.ReplaceAllString(parser.Text(current.item.Get("heading")), ""))
	text := parser.JoinNonEmpty(" | ", heading, contentText(current.content))
	kind := cmp.Or(current.kind, "log")
	if text != "" {
		text = kind + ": " + text
	} else {
		text = kind
	}
	c.emit(row(timeline.TurnSystem, text))
	if current.kind == "warning" {
		if decision, tool := c.parser.warningDecision(current.item); decision != "" {
			made := row(timeline.TurnSystem, decision)
			made.ToolName = tool
			c.emit(made)
		}
	}
}
