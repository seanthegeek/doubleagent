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
// summaries in order: bulks, then topics, then the current topic.
func walkHistory(node jsonvalue.Value, summaries bool, visit func(summary bool, node jsonvalue.Value)) {
	if node.IsArray() {
		for _, child := range node.Items() {
			walkHistory(child, summaries, visit)
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
			walkHistory(node.Get(name), summaries, visit)
		}
	case parser.IsText(class, "Topic") || parser.IsText(class, "Bulk") || node.Has("messages") || node.Has("records"):
		if summaries && !node.Get("summary").IsEmpty() {
			visit(true, node.Get("summary"))
		}
		walkHistory(parser.FirstNonEmpty(node.Get("messages"), node.Get("records")), summaries, visit)
	}
}

// parseAI returns the assistant's raw output parsed as the agent's JSON
// response, else null.
func parseAI(content jsonvalue.Value) jsonvalue.Value {
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
	value, err := jsonvalue.DecodeString(text)
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
// damaged file breaks off.
func (p *Parser) recoverLogs(raw string) []jsonvalue.Value {
	match := p.logs.FindStringIndex(raw)
	if match == nil {
		return nil
	}
	var items []jsonvalue.Value
	position := match[1]
	for {
		for position < len(raw) && strings.IndexByte(" \t\r\n,", raw[position]) >= 0 {
			position++
		}
		item, end, err := jsonvalue.DecodePrefix(raw, position)
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
	// joined holds the history ids whose message has joined a log item,
	// and files the message files already looked for: each joins the
	// first log item that reaches it and is read at most once.
	joined map[string]bool
	files  map[string]bool
	// work counts the message files read, which a test holds to one each.
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
		base: p.BaseRow(artifact), history: map[string]historyEntry{}, joined: map[string]bool{},
		work: p.Work, models: parser.Tracker{}}
	state.base.SessionID = fallbackID
	content, ok := parser.FileBytes(artifact.DiskPath, &state.problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, state.problems, fallbackID, ""))
		return nil
	}
	raw := textlines.ValidUTF8(content)
	document, err := jsonvalue.DecodeString(raw)
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
		state.logRows(p.recoverLogs(raw))
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
	trimmed := len(logs) >= logSize
	ordered := c.agentHistories(document, trimmed)
	if trimmed {
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
// not an object, or logs that is not an array, adds a problem.
func (c *chat) logItems(document jsonvalue.Value) []jsonvalue.Value {
	log := document.Get("log")
	if !log.IsObject() && !log.IsNull() {
		c.problems.Add(0, parser.WrongKind("log", jsonvalue.Object))
		return nil
	}
	var logs []jsonvalue.Value
	for _, item := range parser.ArrayMember(log.Get("logs"), "log.logs", 0, &c.problems) {
		if item.IsObject() {
			logs = append(logs, item)
		}
	}
	return logs
}

// agentHistories reads each agent's history, filling the chat's message
// index and returning its items in order. An agents member that is not an
// array adds a problem.
func (c *chat) agentHistories(document jsonvalue.Value, summaries bool) []historyItem {
	var ordered []historyItem
	for i, agent := range parser.ArrayMember(document.Get("agents"), "agents", 0, &c.problems) {
		if !agent.IsObject() {
			continue
		}
		number := jsonvalue.NewInt(int64(i))
		if declared, ok := agent.Lookup("number"); ok {
			number = declared
		}
		history := agent.Get("history")
		if history.IsString() {
			history = jsonvalue.NewObject()
			if text := agent.Get("history").StringValue(); text != "" {
				decoded, err := jsonvalue.DecodeString(text)
				if err != nil {
					c.emit(c.row(timeline.TurnSystem, "parser: history of agent "+number.Text()+
						" did not parse ("+err.Error()+")", 0, ""))
					continue
				}
				history = decoded
			}
		}
		walkHistory(history, summaries, func(summary bool, node jsonvalue.Value) {
			ordered = append(ordered, historyItem{agent: number, summary: summary, node: node})
			if messageID := node.Get("id"); !summary && !messageID.IsEmpty() {
				if _, held := c.history[messageID.Text()]; !held {
					c.history[messageID.Text()] = historyEntry{agent: number, message: node}
				}
			}
		})
	}
	return ordered
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
	parsed := parseAI(content)
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

// messageFile returns the text of the message file a history entry names, ""
// when there is none or an earlier entry of the chat named it: a file is
// looked for once, and its text joins the first log item that reaches it.
// A file that exists but cannot be read adds its unreadable problem; a
// failure to look for it stops the parse.
func (c *chat) messageFile(content jsonvalue.Value) string {
	if !content.IsObject() || content.Get("file").IsEmpty() {
		return ""
	}
	name := strings.ReplaceAll(content.Get("file").Text(), `\`, "/")
	name = name[strings.LastIndex(name, "/")+1:]
	if c.files[name] {
		return ""
	}
	if c.files == nil {
		c.files = map[string]bool{}
	}
	c.files[name] = true
	path, named := c.messages.Child(name)
	if !named {
		return ""
	}
	info, err := osfile.Lstat(path)
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
	data, ok := parser.FileBytes(path, &c.problems)
	if !ok {
		return ""
	}
	return textlines.ValidUTF8(data)
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
		// The message's text joins the first log item that names its id;
		// a later one keeps the id and the model and reads its own content.
		if !c.joined[current.itemID] {
			c.joined[current.itemID] = true
			current.body = held.message.Get("content")
		}
	}
	return current
}

// logRows emits the rows of the log items; a message file that cannot be
// read adds a problem.
func (c *chat) logRows(logs []jsonvalue.Value) {
	for _, item := range logs {
		if c.failure != nil {
			return
		}
		current := c.newLogItem(item)
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
}

// userLogRow emits a user log item, with the text the history holds for it
// and the names of its attachments.
func (c *chat) userLogRow(current *logItem, row func(turn, text string) timeline.Row) {
	text := contentText(current.content)
	if current.body.IsObject() && current.body.Has("user_message") {
		text = contentText(current.body.Get("user_message"))
	}
	if attachments := current.kvps.Get("attachments"); attachments.IsArray() && attachments.Len() > 0 {
		names := make([]string, 0, attachments.Len())
		for _, attachment := range attachments.Items() {
			names = append(names, attachment.Text())
		}
		text += " [attachments: " + strings.Join(names, ", ") + "]"
	}
	c.emit(row(timeline.TurnUser, text))
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
	parsed := jsonvalue.NewNull()
	if current.hasMessage {
		parsed = parseAI(current.body)
	}
	text := headlineText(current.kvps)
	if text == "" && !parsed.IsEmpty() {
		text = headlineText(parsed)
	}
	if text == "" && current.hasMessage && parsed.IsEmpty() {
		text = contentText(current.body)
	}
	if text == "" {
		text = contentText(current.content)
	}
	if text != "" {
		made := row(timeline.TurnAssistant, text)
		made.Model = model
		c.emit(made)
	}
}

// toolLogRows emits the call and the result of a tool log item.
func (c *chat) toolLogRows(current *logItem, row func(turn, text string) timeline.Row) {
	name := parser.TextOr(parser.FirstNonEmpty(current.kvps.Get("_tool_name"), current.body.Get("tool_name")), current.kind)
	toolID := current.itemID
	if toolID == "" {
		toolID = "log:" + strconv.Itoa(current.line)
	}
	call := row(timeline.TurnToolUse, argsSummary(current.kvps))
	call.ToolName, call.ToolUseID = name, toolID
	c.emit(call)
	result := c.messageFile(current.body)
	if c.failure != nil {
		return
	}
	if result == "" && current.body.IsObject() && current.body.Has("tool_result") {
		result = contentText(current.body.Get("tool_result"))
	}
	if result == "" {
		result = contentText(current.content)
	}
	if result != "" || current.hasMessage {
		made := row(timeline.TurnToolResult, result)
		made.ToolName, made.ToolUseID = name, toolID
		c.emit(made)
	}
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
