package codex

import (
	"cmp"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// uuidTail finds the thread id in a rollout's file name:
// rollout-<time>-<thread id>.jsonl, or <thread id>_<rollout id>.jsonl for a
// reverted thread; the first group is the thread id either way.
var uuidTail = parserkit.MustRelPattern(`(` + uuidText + `)(?:_` + uuidText + `)?\.jsonl(?:\.zst)?\z`)

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
	options  parserkit.Options
	emit     parserkit.Emit

	carried   map[string]bool
	exitCodes map[string]string
	// cuts cuts the session id each guardian verdict's text repeats and
	// the policy values each thread_settings_applied row repeats.
	cuts *parserkit.Carried

	sessionID, cwd, model, branch string
	metaSeen                      bool
	seenModel                     parserkit.Tracker
	// modes holds each session's permission mode, and shownModes the
	// composite as its last "permission mode:" row wrote it.
	modes      map[string]*mode
	shownModes map[string]string
	// taskPending is set in a subagent's file until its first prompt.
	taskPending bool
	// guardianParent is the thread a guardian review thread reviews, and
	// started that thread's start time as written.
	guardianParent, started string
	// guardian holds the indexes of the log databases once a
	// task_complete has looked for them, so the rollout lists them once.
	guardian     []*guardianIndex
	guardianRead bool

	line   int
	record jsonvalue.Value
}

// parseRollout emits the rows of a rollout, plain or zstd-compressed. The
// whole file is read first, so that a turn item can be checked against
// every response item's id. Problems come last, under the session.
func (p *Parser) parseRollout(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) {
	var problems parserkit.Problems
	state := &rollout{parser: p, artifact: artifact, options: options, emit: emit,
		seenModel: parserkit.Tracker{}, modes: map[string]*mode{},
		shownModes: map[string]string{}, cuts: parserkit.NewCarried(artifact)}
	if match := uuidTail.FindStringSubmatch(artifact.Rel); match != nil {
		state.sessionID = match[1]
	}
	var records []numberedRecord
	if textlines.HasSuffixFoldASCII(artifact.Rel, ".zst") {
		records = p.zstRecords(artifact.DiskPath, &problems)
	} else {
		for line, record := range parserkit.JSONLines(artifact.DiskPath, &problems) {
			records = append(records, numberedRecord{line: line, value: record})
		}
	}
	state.carried, state.exitCodes = prescan(records)
	for _, record := range records {
		state.line, state.record = record.line, record.value
		parserkit.ObjectMember(record.value.Get("payload"), "payload", record.line, &problems)
		state.read()
	}
	state.cuts.Report(0, &problems)
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, state.sessionID, ""))
}

// read handles the current line by its type.
func (s *rollout) read() {
	payload := objectOr(s.record.Get("payload"))
	switch recordType := s.record.Get("type"); {
	case parserkit.IsText(recordType, "session_meta"):
		s.sessionMeta(payload)
	case parserkit.IsText(recordType, "turn_context"):
		s.turnContext(payload)
	case parserkit.IsText(recordType, "response_item"):
		if s.record.Get("metadata").Get("inherited_user_message").IsEmpty() {
			s.responseItem(payload)
		}
	case parserkit.IsText(recordType, "event_msg"):
		s.event(payload)
	case parserkit.IsText(recordType, "compacted"):
		s.compacted(payload)
	case parserkit.IsText(recordType, "inter_agent_communication"):
		if !s.carried[parserkit.Text(payload.Get("id"))] {
			s.interAgent(payload)
		}
	case parserkit.IsText(recordType, "realtime_item"):
		s.realtime(payload)
	}
}

// row returns a row of the current line with the session's state: at
// timestamp when it gives one, else at the line's own time.
func (s *rollout) row(turnType string, timestamp jsonvalue.Value) timeline.Row {
	row := s.parser.BaseRow(s.artifact)
	row.SourceLine = s.line
	row.TimestampUTC = cmp.Or(timeutil.FromValue(timestamp),
		timeutil.FromValue(s.record.Get("timestamp")))
	row.SessionID = s.sessionID
	row.ProjectPath = s.cwd
	row.GitBranch = s.branch
	row.Model = s.model
	row.TurnType = turnType
	return row
}

// exitCode returns the failed exit code of the call callID, carried from its
// CommandExecution item into each output of the call, cut as a carried
// value; "" when it has none.
func (s *rollout) exitCode(callID string) string {
	return s.cuts.Cut("exit code", s.exitCodes[callID])
}

// put emits row with text, compacted.
func (s *rollout) put(row timeline.Row, text string) {
	row.Text = timeline.Compact(text)
	s.emit(row)
}

// system emits a system row of the current line at the line's time.
func (s *rollout) system(text string) {
	s.put(s.row(timeline.TurnSystem, jsonvalue.NewNull()), text)
}

// prompt emits a prompt that is not context: the person's, or a subagent's
// task when it is the subagent's first.
func (s *rollout) prompt(text string) {
	if s.taskPending {
		s.taskPending = false
		s.system("subagent task: " + text)
		return
	}
	s.put(s.row(timeline.TurnUser, jsonvalue.NewNull()), text)
}

// sessionMeta reads a session_meta line: the file's first sets the
// session, project and branch and gives the start rows; a later one with
// another id was copied from an ancestor thread.
func (s *rollout) sessionMeta(payload jsonvalue.Value) {
	threadID := parserkit.Text(parserkit.FirstNonEmpty(payload.Get("id"), payload.Get("session_id")))
	when := payload.Get("timestamp")
	if s.metaSeen {
		if threadID != "" && threadID != s.sessionID {
			s.put(s.row(timeline.TurnSystem, when), "copied from ancestor: session start "+
				cmp.Or(parserkit.Text(payload.Get("originator")), "codex")+" "+parserkit.Text(payload.Get("cli_version"))+
				" id="+threadID+" cwd="+parserkit.Text(payload.Get("cwd")))
		}
		return
	}
	s.metaSeen = true
	s.sessionID = cmp.Or(threadID, s.sessionID)
	s.cwd = cmp.Or(parserkit.Text(payload.Get("cwd")), s.cwd)
	git := objectOr(payload.Get("git"))
	s.branch = cmp.Or(parserkit.Text(git.Get("branch")), s.branch)
	start := []string{"session start: " + cmp.Or(parserkit.Text(payload.Get("originator")), "codex") + " " +
		parserkit.Text(payload.Get("cli_version")) + " provider=" +
		parserkit.Text(payload.Get("model_provider")) + " cwd=" + s.cwd}
	for _, field := range [][2]string{{"creator_account_id", "account"}, {"creator_user_id", "user"}} {
		if value := payload.Get(field[0]); !value.IsEmpty() {
			start = append(start, field[1]+"="+value.Text())
		}
	}
	s.put(s.row(timeline.TurnSystem, when), strings.Join(start, " "))
	if url := git.Get("repository_url"); url.IsString() && strings.TrimSpace(url.StringValue()) != "" {
		s.put(s.row(timeline.TurnSystem, when), timeline.GitRemotePrefix+url.StringValue())
	}
	s.lineage(payload, when)
}

// lineage emits the subagent and fork rows of the file's first
// session_meta, and notes whether the thread is a subagent's or a guardian
// review's.
func (s *rollout) lineage(payload, when jsonvalue.Value) {
	parent := parentOf(payload)
	if parent != "" {
		var extra []string
		for _, field := range []string{"agent_path", "agent_role", "agent_nickname"} {
			if value := payload.Get(field); !value.IsEmpty() {
				extra = append(extra, field+"="+value.Text())
			}
		}
		s.put(s.row(timeline.TurnSystem, when), strings.TrimSpace("subagent "+s.sessionID+" of "+parent+
			" "+strings.Join(extra, " ")))
	}
	if forked := payload.Get("forked_from_id"); !forked.IsEmpty() {
		s.put(s.row(timeline.TurnSystem, when), "forked from "+forked.Text())
	}
	s.taskPending = parent != "" || isSubagentMeta(payload)
	if isGuardian(payload) && parent != "" {
		s.guardianParent = parent
		s.started = parserkit.Text(parserkit.FirstNonEmpty(payload.Get("timestamp"), s.record.Get("timestamp")))
	}
}

// turnContext reads a turn_context line: it sets the project and model and
// gives the model and permission mode rows when they change.
func (s *rollout) turnContext(payload jsonvalue.Value) {
	s.cwd = cmp.Or(parserkit.Text(payload.Get("cwd")), s.cwd)
	s.model = cmp.Or(parserkit.Text(payload.Get("model")), s.model)
	current, ok := s.modes[s.sessionID]
	if !ok {
		current = newMode(s.parser.Work)
		s.modes[s.sessionID] = current
	}
	current.fromContext(payload)
	texts := []string{
		s.seenModel.Change("model", s.sessionID, parserkit.Text(payload.Get("model"))),
		s.modeChange(current, current.wholeText),
	}
	s.systemTexts(texts)
}

// modeChange returns the "permission mode:" text when the session's
// composite mode changes (mode.changed), written with what shown returns,
// the composite as this line writes it, after the one the last such row
// wrote; "" when it does not change, without calling shown.
func (s *rollout) modeChange(current *mode, shown func() string) string {
	if !current.changed() {
		return ""
	}
	old := s.shownModes[s.sessionID]
	s.shownModes[s.sessionID] = shown()
	if old != "" {
		return "permission mode: " + old + " -> " + s.shownModes[s.sessionID]
	}
	return "permission mode: " + s.shownModes[s.sessionID]
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
func (s *rollout) responseItem(payload jsonvalue.Value) {
	payloadType := payload.Get("type")
	switch {
	case parserkit.IsText(payloadType, "message"):
		s.message(payload)
	case textIn(payloadType, callTypes...):
		row := s.row(timeline.TurnToolUse, jsonvalue.NewNull())
		row.ToolName = callName(payload)
		row.ToolUseID = parserkit.Text(parserkit.FirstNonEmpty(payload.Get("call_id"), payload.Get("id")))
		s.put(row, callSummary(payload))
	case textIn(payloadType, outputTypes...):
		row := s.row(timeline.TurnToolResult, jsonvalue.NewNull())
		row.ToolUseID = parserkit.Text(payload.Get("call_id"))
		s.put(row, withExit(parserkit.TextOf(payload.Get("output")), s.exitCode(row.ToolUseID)))
	case parserkit.IsText(payloadType, "tool_search_call"):
		row := s.row(timeline.TurnToolUse, jsonvalue.NewNull())
		row.ToolName = "tool_search"
		row.ToolUseID = parserkit.Text(parserkit.FirstNonEmpty(payload.Get("call_id"), payload.Get("id")))
		s.put(row, parserkit.JSONText(payload.Get("arguments")))
	case parserkit.IsText(payloadType, "tool_search_output"):
		row := s.row(timeline.TurnToolResult, jsonvalue.NewNull())
		row.ToolUseID = parserkit.Text(payload.Get("call_id"))
		s.put(row, toolNames(payload.Get("tools")))
	case parserkit.IsText(payloadType, "agent_message"):
		s.agentMessage(payload, agentText(payload))
	case parserkit.IsText(payloadType, "image_generation_call"):
		s.system("image generation: " + parserkit.Text(payload.Get("revised_prompt")) + " status=" +
			parserkit.Text(payload.Get("status")))
	case parserkit.IsText(payloadType, "reasoning") && s.options.IncludeThinking:
		text := parserkit.TextOf(payload.Get("summary"))
		if text == "" {
			text = parserkit.TextOf(payload.Get("content"))
		}
		if text != "" {
			s.put(s.row(timeline.TurnThinking, jsonvalue.NewNull()), text)
		}
	}
}

// message reads a message response item: a user-role message by its runs,
// any other role's text as an assistant row or "<role>: <text>".
func (s *rollout) message(payload jsonvalue.Value) {
	role := payload.Get("role")
	if parserkit.IsText(role, "user") {
		for _, part := range userParts(payload, s.parser.Work) {
			if part.kind == "" {
				s.prompt(part.text)
			} else {
				s.system("context: " + part.kind + ": " + part.text)
			}
		}
		return
	}
	text := parserkit.TextOf(payload.Get("content"))
	switch {
	case text == "":
	case parserkit.IsText(role, "assistant"):
		s.put(s.row(timeline.TurnAssistant, jsonvalue.NewNull()), text)
	default:
		s.system(role.Text() + ": " + text)
	}
}

// agentMessage emits a message between agents: a subagent's first prompt,
// or an "agent message" row.
func (s *rollout) agentMessage(payload jsonvalue.Value, text string) {
	if s.taskPending {
		s.prompt(text)
		return
	}
	s.system(agentLabel(payload) + text)
}

// event reads an event_msg line.
func (s *rollout) event(payload jsonvalue.Value) {
	switch payloadType := payload.Get("type"); {
	case parserkit.IsText(payloadType, "task_started"):
		s.system("task_started turn=" + parserkit.Text(payload.Get("turn_id")))
		attribution := payload.Get("turn_attribution")
		if trigger := attribution.Get("turn_trigger"); attribution.IsObject() && !trigger.IsEmpty() {
			turn := parserkit.Text(parserkit.FirstNonEmpty(payload.Get("turn_id"), attribution.Get("turn_id")))
			s.system("turn trigger: " + trigger.Text() + " turn=" + turn)
		}
	case parserkit.IsText(payloadType, "task_complete"):
		s.system("task_complete turn=" + parserkit.Text(payload.Get("turn_id")) + " duration_ms=" +
			parserkit.Text(payload.Get("duration_ms")))
		s.guardianDecision(payload)
	case parserkit.IsText(payloadType, "thread_settings_applied"):
		s.threadSettings(payload.Get("thread_settings"))
	case parserkit.IsText(payloadType, "thread_goal_updated"):
		s.system(goalText(payload))
	case parserkit.IsText(payloadType, "turn_aborted"):
		detail := ""
		if failure := payload.Get("error"); failure.IsObject() {
			detail = parserkit.Text(failure.Get("message"))
		}
		s.system(strings.TrimSpace("turn aborted: " + parserkit.Text(payload.Get("reason")) + " " + detail))
	case parserkit.IsText(payloadType, "thread_rolled_back"):
		count := ""
		if value, ok := payload.Lookup("num_turns"); ok {
			count = value.Text()
		}
		s.system("rolled back " + count + " turns")
	case parserkit.IsText(payloadType, "item_completed"):
		s.itemCompleted(payload)
	}
}

// guardianDecision emits, for a guardian review thread's task_complete, the
// "permission decision:" row of its verdict under the reviewed thread, with
// the governed call when the log databases name exactly one.
func (s *rollout) guardianDecision(payload jsonvalue.Value) {
	message := payload.Get("last_agent_message")
	if s.guardianParent == "" || !message.IsString() {
		return
	}
	verdict, ok := guardianVerdict(s.artifact, message.StringValue())
	if !ok {
		return
	}
	row := s.row(timeline.TurnSystem, jsonvalue.NewNull())
	row.SessionID = s.guardianParent
	row.Model = "" // the reviewer's model is not the parent thread's
	start := cmp.Or(s.started, row.TimestampUTC)
	var calls []string
	if home, ok := s.parser.codexHome(s.artifact); ok {
		calls = guardianCalls(func() []*guardianIndex { return s.guardianLogs(home) }, s.guardianParent, start,
			row.TimestampUTC)
	}
	if len(calls) == 1 {
		row.ToolUseID = calls[0]
	}
	s.put(row, guardianText(verdict, cmp.Or(row.ToolUseID, "-"), s.cuts.Cut("session id", s.sessionID)))
}

// guardianLogs returns the indexes of the log databases in home, looking
// for them on the rollout's first call.
func (s *rollout) guardianLogs(home string) []*guardianIndex {
	if !s.guardianRead {
		s.guardian, s.guardianRead = s.parser.guardianIndexes(s.artifact, home), true
		s.parser.Work.Add("guardian log reads", 1)
	}
	return s.guardian
}

// threadSettings reads a thread_settings_applied event: the model and
// permission mode tracked as for a turn_context. Before the thread's first
// turn_context the event's policy is folded into that turn's row.
func (s *rollout) threadSettings(settings jsonvalue.Value) {
	if !settings.IsObject() {
		return
	}
	newModel := settingsModel(settings)
	s.model = cmp.Or(newModel, s.model)
	texts := []string{s.seenModel.Change("model", s.sessionID, newModel)}
	if current, ok := s.modes[s.sessionID]; ok {
		set := settingsSet(settings)
		current.withSettings(settings, set)
		texts = append(texts, s.modeChange(current, func() string {
			return current.carriedText(set, s.cuts)
		}))
	}
	s.systemTexts(texts)
}

// itemCompleted reads an item_completed event: a turn item that no
// response item carries gives its rows.
func (s *rollout) itemCompleted(payload jsonvalue.Value) {
	item := payload.Get("item")
	if !item.IsObject() || textIn(item.Get("type"), responseCarriedItems...) ||
		s.carried[parserkit.Text(item.Get("id"))] {
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
func (s *rollout) compacted(payload jsonvalue.Value) {
	if message := payload.Get("message"); message.IsString() && message.StringValue() != "" {
		s.system("compaction summary: " + message.StringValue())
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
func (s *rollout) interAgent(payload jsonvalue.Value) {
	text := ""
	switch content := payload.Get("content"); {
	case content.IsString() && content.StringValue() != "":
		text = content.StringValue()
	case !payload.Get("encrypted_content").IsEmpty():
		text = "[encrypted]"
	}
	s.agentMessage(payload, text)
}

// realtime reads a realtime_item line: a transcript segment of the user or
// the assistant, or any other item as its JSON.
func (s *rollout) realtime(payload jsonvalue.Value) {
	kind, role := payload.Get("type"), payload.Get("role")
	if parserkit.IsText(kind, "transcript_segment") && textIn(role, "user", "assistant") {
		s.put(s.row(role.StringValue(), jsonvalue.NewNull()), parserkit.Text(payload.Get("text")))
		return
	}
	text := "realtime: " + parserkit.Text(kind) + " " +
		parserkit.JSONText(without(payload, "type", "id", "text"))
	if spoken := payload.Get("text"); !spoken.IsEmpty() {
		text += " " + spoken.Text()
	}
	s.system(text)
}

// codexHome returns the directory of the agent's stores that holds the
// artifact, as many levels above the artifact's directory as its relative
// path has directories below the home prefix; false when that would leave
// the artifact's home, which a path the parser wants never does.
func (p *Parser) codexHome(artifact *input.Artifact) (string, bool) {
	return parserkit.Above(artifact, strings.Count(artifact.Rel, "/")-p.homeDepth())
}
