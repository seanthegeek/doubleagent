package codex

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// uuidTail finds the thread id in a rollout's file name:
// rollout-<time>-<thread id>.jsonl, or <thread id>_<rollout id>.jsonl for a
// reverted thread; the first group is the thread id either way.
var uuidTail = regexp.MustCompile(`(` + uuidText + `)(?:_` + uuidText + `)?\.jsonl(?:\.zst)?\z`)

// uuidText is a UUID, its hexadecimal digits in either case.
const uuidText = `[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`

// responseCarriedItems are the turn items that only repeat a response
// item's message or reasoning, or the compacted line, under an id of their
// own.
var responseCarriedItems = []string{"UserMessage", "AgentMessage", "Reasoning", "HookPrompt",
	"ContextCompaction"}

// rollout is the state of one rollout's parse: the session's project,
// branch and model as the lines so far set them, what the model and
// permission rows have shown, and the line being read.
type rollout struct {
	parser   *Parser
	artifact *input.Artifact
	options  parser.Options
	emit     parser.Emit

	carried   map[string]bool
	exitCodes map[string]pyjson.Value

	sessionID, cwd, model, branch string
	metaSeen                      bool
	seenModel, seenMode           parser.Tracker
	policies                      map[string]policy
	// taskPending is set in a subagent's file until its first prompt.
	taskPending bool
	// guardianParent is the thread a guardian review thread reviews, and
	// started that thread's start time as written.
	guardianParent, started string

	line   int
	record pyjson.Value
}

// parseRollout emits the rows of a rollout, plain or zstd-compressed. The
// whole file is read first, so that a turn item can be checked against
// every response item's id. Problems come last, under the session.
func (p *Parser) parseRollout(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	var problems parser.Problems
	state := &rollout{parser: p, artifact: artifact, options: options, emit: emit,
		seenModel: parser.Tracker{}, seenMode: parser.Tracker{}, policies: map[string]policy{}}
	if match := uuidTail.FindStringSubmatch(artifact.Rel); match != nil {
		state.sessionID = match[1]
	}
	var records []numberedRecord
	if strings.HasSuffix(artifact.Rel, ".zst") {
		records = zstRecords(artifact.DiskPath, &problems)
	} else {
		for line, record := range parser.JSONLines(artifact.DiskPath, &problems) {
			records = append(records, numberedRecord{line: line, value: record})
		}
	}
	state.carried, state.exitCodes = prescan(records)
	for _, record := range records {
		state.line, state.record = record.line, record.value
		state.read()
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, state.sessionID, ""))
}

// read handles the current line by its type.
func (s *rollout) read() {
	payload := objectOr(s.record.Get("payload"))
	switch recordType := s.record.Get("type"); {
	case isText(recordType, "session_meta"):
		s.sessionMeta(payload)
	case isText(recordType, "turn_context"):
		s.turnContext(payload)
	case isText(recordType, "response_item"):
		if !s.record.Get("metadata").Get("inherited_user_message").Truthy() {
			s.responseItem(payload)
		}
	case isText(recordType, "event_msg"):
		s.event(payload)
	case isText(recordType, "compacted"):
		s.compacted(payload)
	case isText(recordType, "inter_agent_communication"):
		if !s.carried[parser.Str(payload.Get("id"))] {
			s.interAgent(payload)
		}
	case isText(recordType, "realtime_item"):
		s.realtime(payload)
	}
}

// row returns a row of the current line with the session's state: at
// timestamp when it gives one, else at the line's own time.
func (s *rollout) row(turnType string, timestamp pyjson.Value) timeline.Row {
	row := s.parser.BaseRow(s.artifact)
	row.SourceLine = s.line
	row.TimestampUTC = timeutil.FirstNonEmpty(timeutil.FromValue(timestamp),
		timeutil.FromValue(s.record.Get("timestamp")))
	row.SessionID = s.sessionID
	row.ProjectPath = s.cwd
	row.GitBranch = s.branch
	row.Model = s.model
	row.TurnType = turnType
	return row
}

// put emits row with text, compacted.
func (s *rollout) put(row timeline.Row, text string) {
	row.Text = timeline.Compact(text)
	s.emit(row)
}

// system emits a system row of the current line at the line's time.
func (s *rollout) system(text string) {
	s.put(s.row(timeline.TurnSystem, pyjson.NewNull()), text)
}

// prompt emits a prompt that is not context: the person's, or a subagent's
// task when it is the subagent's first.
func (s *rollout) prompt(text string) {
	if s.taskPending {
		s.taskPending = false
		s.system("subagent task: " + text)
		return
	}
	s.put(s.row(timeline.TurnUser, pyjson.NewNull()), text)
}

// sessionMeta reads a session_meta line: the file's first sets the
// session, project and branch and gives the start rows; a later one with
// another id was copied from an ancestor thread.
func (s *rollout) sessionMeta(payload pyjson.Value) {
	threadID := parser.Str(firstTruthy(payload.Get("id"), payload.Get("session_id")))
	when := payload.Get("timestamp")
	if s.metaSeen {
		if threadID != "" && threadID != s.sessionID {
			s.put(s.row(timeline.TurnSystem, when), "copied from ancestor: session start "+
				strOr(payload.Get("originator"), "codex")+" "+parser.Str(payload.Get("cli_version"))+
				" id="+threadID+" cwd="+parser.Str(payload.Get("cwd")))
		}
		return
	}
	s.metaSeen = true
	s.sessionID = timeutil.FirstNonEmpty(threadID, s.sessionID)
	s.cwd = strOr(payload.Get("cwd"), s.cwd)
	git := objectOr(payload.Get("git"))
	s.branch = strOr(git.Get("branch"), s.branch)
	start := "session start: " + strOr(payload.Get("originator"), "codex") + " " +
		parser.Str(payload.Get("cli_version")) + " provider=" +
		parser.Str(payload.Get("model_provider")) + " cwd=" + s.cwd
	for _, field := range [][2]string{{"creator_account_id", "account"}, {"creator_user_id", "user"}} {
		if value := payload.Get(field[0]); value.Truthy() {
			start += " " + field[1] + "=" + value.Str()
		}
	}
	s.put(s.row(timeline.TurnSystem, when), start)
	if url := git.Get("repository_url"); url.IsString() && pytext.Strip(url.Text()) != "" {
		s.put(s.row(timeline.TurnSystem, when), timeline.GitRemotePrefix+url.Text())
	}
	s.lineage(payload, when)
}

// lineage emits the subagent and fork rows of the file's first
// session_meta, and notes whether the thread is a subagent's or a guardian
// review's.
func (s *rollout) lineage(payload, when pyjson.Value) {
	parent := parentOf(payload)
	if parent != "" {
		var extra []string
		for _, field := range []string{"agent_path", "agent_role", "agent_nickname"} {
			if value := payload.Get(field); value.Truthy() {
				extra = append(extra, field+"="+value.Str())
			}
		}
		s.put(s.row(timeline.TurnSystem, when), pytext.Strip("subagent "+s.sessionID+" of "+parent+
			" "+strings.Join(extra, " ")))
	}
	if forked := payload.Get("forked_from_id"); forked.Truthy() {
		s.put(s.row(timeline.TurnSystem, when), "forked from "+forked.Str())
	}
	s.taskPending = parent != "" || isSubagentMeta(payload)
	if isGuardian(payload) && parent != "" {
		s.guardianParent = parent
		s.started = parser.Str(firstTruthy(payload.Get("timestamp"), s.record.Get("timestamp")))
	}
}

// turnContext reads a turn_context line: it sets the project and model and
// gives the model and permission mode rows when they change.
func (s *rollout) turnContext(payload pyjson.Value) {
	s.cwd = strOr(payload.Get("cwd"), s.cwd)
	s.model = strOr(payload.Get("model"), s.model)
	s.policies[s.sessionID] = policyOf(payload)
	texts := []string{
		s.seenModel.Change("model", s.sessionID, parser.Str(payload.Get("model"))),
		s.seenMode.Change("permission mode", s.sessionID, s.policies[s.sessionID].text()),
	}
	s.systemTexts(texts)
}

// systemTexts emits a system row for each text that is not empty.
func (s *rollout) systemTexts(texts []string) {
	for _, text := range texts {
		if text != "" {
			s.system(text)
		}
	}
}

// responseItem reads a response_item line: a message, a tool call or
// result, a message between agents, an image generation or reasoning.
func (s *rollout) responseItem(payload pyjson.Value) {
	payloadType := payload.Get("type")
	switch {
	case isText(payloadType, "message"):
		s.message(payload)
	case textIn(payloadType, callTypes...):
		row := s.row(timeline.TurnToolUse, pyjson.NewNull())
		row.ToolName = callName(payload)
		row.ToolUseID = parser.Str(firstTruthy(payload.Get("call_id"), payload.Get("id")))
		s.put(row, callSummary(payload))
	case textIn(payloadType, outputTypes...):
		row := s.row(timeline.TurnToolResult, pyjson.NewNull())
		row.ToolUseID = parser.Str(payload.Get("call_id"))
		s.put(row, withExit(parser.TextOf(payload.Get("output")), s.exitCodes[row.ToolUseID]))
	case isText(payloadType, "tool_search_call"):
		row := s.row(timeline.TurnToolUse, pyjson.NewNull())
		row.ToolName = "tool_search"
		row.ToolUseID = parser.Str(firstTruthy(payload.Get("call_id"), payload.Get("id")))
		s.put(row, parser.CompactJSON(payload.Get("arguments")))
	case isText(payloadType, "tool_search_output"):
		row := s.row(timeline.TurnToolResult, pyjson.NewNull())
		row.ToolUseID = parser.Str(payload.Get("call_id"))
		s.put(row, toolNames(payload.Get("tools")))
	case isText(payloadType, "agent_message"):
		s.agentMessage(payload, agentText(payload))
	case isText(payloadType, "image_generation_call"):
		s.system("image generation: " + parser.Str(payload.Get("revised_prompt")) + " status=" +
			parser.Str(payload.Get("status")))
	case isText(payloadType, "reasoning") && s.options.IncludeThinking:
		text := parser.TextOf(payload.Get("summary"))
		if text == "" {
			text = parser.TextOf(payload.Get("content"))
		}
		if text != "" {
			s.put(s.row(timeline.TurnThinking, pyjson.NewNull()), text)
		}
	}
}

// message reads a message response item: a user-role message by its runs,
// any other role's text as an assistant row or "<role>: <text>".
func (s *rollout) message(payload pyjson.Value) {
	role := payload.Get("role")
	if isText(role, "user") {
		for _, part := range userParts(payload) {
			if part.kind == "" {
				s.prompt(part.text)
			} else {
				s.system("context: " + part.kind + ": " + part.text)
			}
		}
		return
	}
	text := parser.TextOf(payload.Get("content"))
	switch {
	case text == "":
	case isText(role, "assistant"):
		s.put(s.row(timeline.TurnAssistant, pyjson.NewNull()), text)
	default:
		s.system(role.Str() + ": " + text)
	}
}

// agentMessage emits a message between agents: a subagent's first prompt,
// or an "agent message" row.
func (s *rollout) agentMessage(payload pyjson.Value, text string) {
	if s.taskPending {
		s.prompt(text)
		return
	}
	s.system(agentLabel(payload) + text)
}

// event reads an event_msg line.
func (s *rollout) event(payload pyjson.Value) {
	switch payloadType := payload.Get("type"); {
	case isText(payloadType, "task_started"):
		s.system("task_started turn=" + parser.Str(payload.Get("turn_id")))
		attribution := payload.Get("turn_attribution")
		if trigger := attribution.Get("turn_trigger"); attribution.IsObject() && trigger.Truthy() {
			turn := parser.Str(firstTruthy(payload.Get("turn_id"), attribution.Get("turn_id")))
			s.system("turn trigger: " + trigger.Str() + " turn=" + turn)
		}
	case isText(payloadType, "task_complete"):
		s.system("task_complete turn=" + parser.Str(payload.Get("turn_id")) + " duration_ms=" +
			parser.Str(payload.Get("duration_ms")))
		s.guardianDecision(payload)
	case isText(payloadType, "thread_settings_applied"):
		s.threadSettings(payload.Get("thread_settings"))
	case isText(payloadType, "thread_goal_updated"):
		s.system(goalText(payload))
	case isText(payloadType, "turn_aborted"):
		detail := ""
		if failure := payload.Get("error"); failure.IsObject() {
			detail = parser.Str(failure.Get("message"))
		}
		s.system(pytext.Strip("turn aborted: " + parser.Str(payload.Get("reason")) + " " + detail))
	case isText(payloadType, "thread_rolled_back"):
		count := ""
		if value, ok := payload.Lookup("num_turns"); ok {
			count = value.Str()
		}
		s.system("rolled back " + count + " turns")
	case isText(payloadType, "item_completed"):
		s.itemCompleted(payload)
	}
}

// guardianDecision emits, for a guardian review thread's task_complete, the
// "permission decision:" row of its verdict under the reviewed thread, with
// the governed call when the log databases name exactly one.
func (s *rollout) guardianDecision(payload pyjson.Value) {
	message := payload.Get("last_agent_message")
	if s.guardianParent == "" || !message.IsString() {
		return
	}
	verdict, ok := guardianVerdict(message.Text())
	if !ok {
		return
	}
	row := s.row(timeline.TurnSystem, pyjson.NewNull())
	row.SessionID = s.guardianParent
	row.Model = "" // the reviewer's model is not the parent thread's
	start := timeutil.FirstNonEmpty(s.started, row.TimestampUTC)
	var calls []string
	if home, ok := s.parser.codexHome(s.artifact); ok {
		calls = guardianCalls(home, s.guardianParent, start, row.TimestampUTC)
	}
	if len(calls) == 1 {
		row.ToolUseID = calls[0]
	}
	s.put(row, guardianText(verdict, orDash(row.ToolUseID), s.sessionID))
}

// threadSettings reads a thread_settings_applied event: the model and
// permission mode tracked as for a turn_context. Before the thread's first
// turn_context the event's policy is folded into that turn's row.
func (s *rollout) threadSettings(settings pyjson.Value) {
	if !settings.IsObject() {
		return
	}
	newModel := settingsModel(settings)
	s.model = timeutil.FirstNonEmpty(newModel, s.model)
	texts := []string{s.seenModel.Change("model", s.sessionID, newModel)}
	if previous, ok := s.policies[s.sessionID]; ok {
		state := previous.withSettings(settings)
		s.policies[s.sessionID] = state
		texts = append(texts, s.seenMode.Change("permission mode", s.sessionID, state.text()))
	}
	s.systemTexts(texts)
}

// itemCompleted reads an item_completed event: a turn item that no
// response item carries gives its rows.
func (s *rollout) itemCompleted(payload pyjson.Value) {
	item := payload.Get("item")
	if !item.IsObject() || textIn(item.Get("type"), responseCarriedItems...) ||
		s.carried[parser.Str(item.Get("id"))] {
		return
	}
	for _, part := range itemRows(item) {
		when := payload.Get("completed_at_ms")
		if part.when == atStart {
			when = payload.Get("started_at_ms")
		}
		row := s.row(part.turnType, when)
		row.ToolName = part.toolName
		row.ToolUseID = part.toolUseID
		s.put(row, part.text)
	}
}

// compacted reads a compacted line: its summary, or the size of the
// history that replaced the context.
func (s *rollout) compacted(payload pyjson.Value) {
	if message := payload.Get("message"); message.IsString() && message.Text() != "" {
		s.system("compaction summary: " + message.Text())
		return
	}
	count := 0
	if history := payload.Get("replacement_history"); history.IsArray() {
		count = history.Len()
	}
	s.system("compaction: replacement history of " + strconv.Itoa(count) + " items")
}

// interAgent reads an inter_agent_communication line that no response
// item carries.
func (s *rollout) interAgent(payload pyjson.Value) {
	text := ""
	switch content := payload.Get("content"); {
	case content.IsString() && content.Text() != "":
		text = content.Text()
	case payload.Get("encrypted_content").Truthy():
		text = "[encrypted]"
	}
	s.agentMessage(payload, text)
}

// realtime reads a realtime_item line: a transcript segment of the user or
// the assistant, or any other item as its JSON.
func (s *rollout) realtime(payload pyjson.Value) {
	kind, role := payload.Get("type"), payload.Get("role")
	if isText(kind, "transcript_segment") && textIn(role, "user", "assistant") {
		s.put(s.row(role.Text(), pyjson.NewNull()), parser.Str(payload.Get("text")))
		return
	}
	text := "realtime: " + parser.Str(kind) + " " +
		parser.CompactJSON(without(payload, "type", "id", "text"))
	if spoken := payload.Get("text"); spoken.Truthy() {
		text += " " + spoken.Str()
	}
	s.system(text)
}

// codexHome returns the directory of the agent's stores that holds the
// artifact, as many levels above the artifact's directory as its relative
// path has directories below the home prefix; false when that would leave
// the artifact's home, which a path the parser wants never does.
func (p *Parser) codexHome(artifact *input.Artifact) (string, bool) {
	return parser.Above(artifact, strings.Count(artifact.Rel, "/")-p.homeDepth())
}
