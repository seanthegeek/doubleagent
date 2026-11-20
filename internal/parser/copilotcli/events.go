package copilotcli

import (
	"cmp"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The event types that are noted by their main field, and the fields tried.
var (
	noteTypes = []string{"session.task_complete", "system.notification", "session.schedule_created",
		"skill.invoked", "session.handoff"}
	noteKeys = []string{"summary", "content", "prompt", "message", "reason", "name", "agentName"}
	// allowAllKeys are the Copilot Desktop app's form of
	// session.permissions_changed.
	allowAllKeys = []string{"previousAllowAllPermissions", "allowAllPermissions",
		"previousAllowAllPermissionMode", "allowAllPermissionMode"}
)

// eventWalk is the state of one pass over an events.jsonl, with the
// problems of its lines.
type eventWalk struct {
	parser     *Parser
	artifact   *input.Artifact
	problems   *parser.Problems
	options    parser.Options
	emit       parser.Emit
	sessionID  string
	project    string
	branch     string
	model      string
	line       int
	timestamp  string
	names      map[string]string
	requested  map[string]bool
	models     map[string]string
	modes      parser.Tracker
	permission permissionMode
	// stored is the permission mode modes last took, and cuts cuts its
	// halves where a later event's row repeats them.
	stored permissionMode
	cuts   *parser.Carried
}

// permissionMode is the tracked permission mode: the permissionMode and
// the agentMode last recorded.
type permissionMode struct{ mode, agent string }

// row returns a row of the event being read, with the project, branch and
// model as they stand now; model overrides the tracked model when given.
func (w *eventWalk) row(turn, text, toolName, toolUseID, model string) timeline.Row {
	return w.parser.row(w.artifact, w.line, turn, text, columns{timestamp: w.timestamp,
		sessionID: w.sessionID, project: w.project, branch: w.branch,
		model: cmp.Or(model, w.model), toolName: toolName, toolUseID: toolUseID})
}

// system emits a system row of the event being read.
func (w *eventWalk) system(text string) {
	w.emit(w.row(timeline.TurnSystem, text, "", "", ""))
}

// parseEvents emits the rows of one session's events.jsonl, then its
// problems; an event's data that is not a JSON object is one.
func (p *Parser) parseEvents(artifact *input.Artifact, sessionID string, options parser.Options,
	emit parser.Emit) {
	var problems parser.Problems
	walk := &eventWalk{parser: p, artifact: artifact, options: options, emit: emit, sessionID: sessionID,
		project: p.fallbackProject(artifact, 0, sessionID),
		names:   map[string]string{}, requested: map[string]bool{}, models: map[string]string{},
		modes: parser.Tracker{}, cuts: parser.NewCarried(artifact)}
	walk.problems = &problems
	for number, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		walk.line = number
		walk.timestamp = timeutil.FromValue(record.Get("timestamp"))
		data := record.Get("data")
		parser.ObjectMember(data, "data", number, &problems)
		walk.event(record.Get("type").Text(), record, data)
	}
	walk.cuts.Report(0, &problems)
	for _, row := range p.ErrorRows(artifact, problems, sessionID, walk.project) {
		row.GitBranch = walk.branch
		emit(row)
	}
}

// event emits the rows of one event, by its type. The assistant turn and
// usage events, the other model events and unknown types are bookkeeping and
// give no row.
func (w *eventWalk) event(kind string, record, data jsonvalue.Value) {
	switch {
	case kind == "session.start" || kind == "session.resume" || kind == "session.context_changed":
		w.sessionEvent(kind, data)
	case kind == "system.message":
		label := "system prompt"
		if parser.IsText(data.Get("role"), "developer") {
			label = "developer message"
		}
		w.system(label + ": " + data.Get("content").Text())
	case kind == "user.message":
		w.userRow(record, data)
	case kind == "assistant.message":
		w.assistantRows(data)
	case kind == "assistant.reasoning":
		if w.options.IncludeThinking && !data.Get("content").IsEmpty() {
			w.emit(w.row(timeline.TurnThinking, data.Get("content").Text(), "", "", ""))
		}
	case kind == "tool.execution_start" || kind == "tool.user_requested":
		w.toolStart(kind, data)
	case kind == "tool.execution_complete":
		callID := data.Get("toolCallId").Text()
		w.emit(w.row(timeline.TurnToolResult, resultText(data), w.names[callID], callID,
			data.Get("model").Text()))
	case strings.HasPrefix(kind, "permission.") || kind == "session.permissions_changed" ||
		kind == "session.mode_changed":
		w.permissionRows(kind, data)
	case kind == "session.model_change" || kind == "session.auto_mode_resolved":
		w.modelRows(kind, data)
	default:
		w.noteEvent(kind, data)
	}
}

// noteEvent emits the rows of the events that are described in one line.
func (w *eventWalk) noteEvent(kind string, data jsonvalue.Value) {
	switch {
	case kind == "session.info":
		w.system(infoText(data))
	case kind == "model.message":
		callModel := data.Get("modelCall").Get("model").Text()
		w.emit(w.row(timeline.TurnSystem, "utility model "+callModel+": "+
			data.Get("message").Get("content").Text(), "", "", callModel))
	case kind == "session.compaction_complete":
		w.system("compaction summary: " + data.Get("summaryContent").Text())
	case kind == "abort" || kind == "session.error" || kind == "session.warning":
		w.system(kind + ": " + cmp.Or(data.Get("reason").Text(), data.Get("message").Text(),
			parser.JSONText(data)))
	case kind == "hook.start" || kind == "hook.end":
		w.emit(w.row(timeline.TurnSystem, hookText(kind, data), "", data.Get("parentToolCallId").Text(), ""))
	case isNoteType(kind):
		main := ""
		for _, key := range noteKeys {
			if value := data.Get(key); parser.Present(value) {
				main = value.Text()
				break
			}
		}
		w.system(kind + ": " + cmp.Or(main, parser.JSONText(data)))
	case kind == "session.shutdown":
		w.system(shutdownText(data))
	}
}

// isNoteType reports whether an event type is noted by its main field.
func isNoteType(kind string) bool {
	return slices.Contains(noteTypes, kind) || strings.HasPrefix(kind, "subagent.")
}

// sessionEvent handles session.start, session.resume and
// session.context_changed: the working directory and branch they carry.
func (w *eventWalk) sessionEvent(kind string, data jsonvalue.Value) {
	context := data
	if data.Has("context") {
		context = data.Get("context")
	}
	if !context.Get("cwd").IsEmpty() {
		w.project = context.Get("cwd").Text()
	}
	if context.Has("branch") || kind == "session.context_changed" {
		w.branch = context.Get("branch").Text()
	}
	switch kind {
	case "session.start":
		w.system("session start: copilot " + cmp.Or(data.Get("copilotVersion").Text(), "-") +
			" producer=" + cmp.Or(data.Get("producer").Text(), "-") + " cwd=" +
			cmp.Or(context.Get("cwd").Text(), "-"))
	case "session.resume":
		w.system("session resumed")
	}
}

// toolStart emits the tool_use row of a tool the model or the user started,
// once per call id.
func (w *eventWalk) toolStart(kind string, data jsonvalue.Value) {
	callID, name := data.Get("toolCallId").Text(), data.Get("toolName").Text()
	if _, held := w.names[callID]; callID != "" && name != "" && !held {
		w.names[callID] = name
	}
	if kind == "tool.execution_start" && w.requested[callID] {
		return
	}
	w.requested[callID] = true
	body := argsText(data.Get("arguments"))
	if kind == "tool.user_requested" {
		body = "user requested: " + body
	}
	w.emit(w.row(timeline.TurnToolUse, body, name, callID, data.Get("model").Text()))
}

// userRow emits a user message: a user row for a typed prompt, else a system
// row naming what sent it.
func (w *eventWalk) userRow(record, data jsonvalue.Value) {
	content, source := data.Get("content").Text(), data.Get("source").Text()
	label := ""
	switch {
	case !data.Get("isAutopilotContinuation").IsEmpty():
		label = "auto-continuation"
	case !record.Get("agentId").IsEmpty():
		label = "subagent task"
	case source != "" && source != "user":
		label = source
	default:
		w.emit(w.row(timeline.TurnUser, content, "", "", ""))
		return
	}
	w.system(label + ": " + content)
}

// assistantRows emits an assistant message's reply, reasoning and tool
// requests; a toolRequests that is not a JSON array is a problem.
func (w *eventWalk) assistantRows(data jsonvalue.Value) {
	model, content := data.Get("model").Text(), data.Get("content").Text()
	if strings.TrimSpace(content) != "" {
		w.emit(w.row(timeline.TurnAssistant, content, "", "", model))
	}
	if w.options.IncludeThinking {
		thought := data.Get("reasoningText").Text()
		if thought == "" {
			thought = reasoningSummaries(data, w.line, w.problems)
		}
		if thought != "" {
			w.emit(w.row(timeline.TurnThinking, thought, "", "", model))
		}
	}
	for _, request := range parser.ArrayMember(data.Get("toolRequests"), "toolRequests", w.line, w.problems) {
		callID, name := request.Get("toolCallId").Text(), request.Get("name").Text()
		if callID != "" {
			w.names[callID] = name
			w.requested[callID] = true
		}
		w.emit(w.row(timeline.TurnToolUse, argsText(request.Get("arguments")), name, callID, model))
	}
}

// reasoningSummaries returns the text of a message's reasoning blocks'
// summaries, separated by blank lines. A reasoningBlocks or blocks of the
// wrong kind adds a problem at line, and so does a block that is not an
// object; a summary of the wrong kind, or one that is not an object, is one
// problem however many blocks hold it so.
func reasoningSummaries(data jsonvalue.Value, line int, problems *parser.Problems) string {
	var parts []string
	var tally parser.EntryTally
	reasoning := data.Get("reasoningBlocks")
	parser.ObjectMember(reasoning, "reasoningBlocks", line, problems)
	for _, block := range parser.EntriesOfKind(reasoning.Get("blocks"), "reasoningBlocks.blocks",
		jsonvalue.Object, line, problems) {
		for _, summary := range tally.EntriesOfKind(block.Get("summary"), "summary", jsonvalue.Object) {
			if body := summary.Get("text"); body.IsString() && body.StringValue() != "" {
				parts = append(parts, body.StringValue())
			}
		}
	}
	tally.Report(line, problems)
	return strings.Join(parts, "\n\n")
}

// text returns the permission mode with the agent mode appended when
// there is one, each half as shown gives it.
func (m permissionMode) text(shown func(name, value string) string) string {
	if m.agent != "" {
		return shown("permission mode", m.mode) + " agent=" + shown("agent mode", m.agent)
	}
	return shown("permission mode", m.mode)
}

// whole returns a half of the mode as it is.
func whole(_, value string) string {
	return value
}

// modeValue returns the tracked permission mode, whole.
func (w *eventWalk) modeValue() string {
	return w.permission.text(whole)
}

// storeMode makes the tracked permission mode the one modes holds.
func (w *eventWalk) storeMode() {
	w.modes[w.sessionID] = w.modeValue()
	w.stored = w.permission
}

// permissionRows emits the permission request, decision and mode rows of one
// event. The tracked mode is "<permissionMode> agent=<agentMode>"; the two
// change events update their half of it.
func (w *eventWalk) permissionRows(kind string, data jsonvalue.Value) {
	callID := data.Get("toolCallId").Text()
	switch kind {
	case "permission.requested":
		w.permissionRequest(data)
	case "permission.completed":
		w.emit(w.row(timeline.TurnSystem, decisionText(data), w.names[callID], callID, ""))
	case "permission.carriedForward":
		w.emit(w.row(timeline.TurnSystem, "permission decision: carried-forward source="+
			cmp.Or(data.Get("decisionSource").Text(), "-"), w.names[callID], callID, ""))
	case "session.permissions_changed":
		w.permissionsChanged(data)
	case "session.mode_changed":
		old, next := cmp.Or(data.Get("previousMode").Text(), "-"), cmp.Or(data.Get("newMode").Text(), "-")
		w.system("agent mode: " + old + " -> " + next)
		w.permission.agent = data.Get("newMode").Text()
		if w.permission.mode != "" {
			w.storeMode()
		}
	}
}

// permissionRequest emits the row of a permission request and the mode row
// when the request shows the mode changed.
func (w *eventWalk) permissionRequest(data jsonvalue.Value) {
	request := data.Get("permissionRequest")
	callID := request.Get("toolCallId").Text()
	line := "permission request: " + cmp.Or(request.Get("kind").Text(), "-")
	if target := requestTarget(request); target != "" {
		line += " " + target
	}
	if intention := request.Get("intention"); parser.Present(intention) {
		line += " intention=" + intention.Text()
	}
	w.emit(w.row(timeline.TurnSystem, line, w.names[callID], callID, ""))
	set := map[string]bool{"permission mode": !data.Get("permissionMode").IsEmpty(),
		"agent mode": !data.Get("agentMode").IsEmpty()}
	if set["permission mode"] {
		w.permission.mode = data.Get("permissionMode").Text()
	}
	if set["agent mode"] {
		w.permission.agent = data.Get("agentMode").Text()
	}
	if w.permission.mode != "" {
		w.modeChange(set, callID)
	}
}

// modeChange emits the "permission mode:" row when a request changes the
// tracked mode. Every half of the earlier mode, and each half of the new
// one the request did not set, came from an earlier event and is repeated
// in each such row, so it is cut (parser.Carried); whether the mode
// changed is decided on the whole values.
func (w *eventWalk) modeChange(set map[string]bool, callID string) {
	_, seen := w.modes[w.sessionID]
	old := w.stored
	if w.modes.Change("permission mode", w.sessionID, w.modeValue()) == "" {
		return
	}
	w.stored = w.permission
	next := w.permission.text(func(name, value string) string {
		if set[name] {
			return value
		}
		return w.cuts.Cut(name, value)
	})
	text := "permission mode: " + next
	if seen {
		text = "permission mode: " + old.text(w.cuts.Cut) + " -> " + next
	}
	w.emit(w.row(timeline.TurnSystem, text, w.names[callID], callID, ""))
}

// permissionsChanged emits the row of a permission mode change, in the form
// the app's bundled CLI writes (allow-all keys) or the CLI's own.
func (w *eventWalk) permissionsChanged(data jsonvalue.Value) {
	allowAll := !data.Has("mode") && !data.Has("previousMode")
	if allowAll {
		allowAll = false
		for _, key := range allowAllKeys {
			allowAll = allowAll || data.Has(key)
		}
	}
	if allowAll {
		old, next := cmp.Or(data.Get("previousAllowAllPermissionMode").Text(), "-"),
			data.Get("allowAllPermissionMode").Text()
		w.system("permission mode: allow-all " + old + " -> " + cmp.Or(next, "-"))
		if next != "" {
			w.permission.mode = "allow-all " + next
		}
	} else {
		w.system("permission mode: " + cmp.Or(data.Get("previousMode").Text(), "-") + " -> " +
			cmp.Or(data.Get("mode").Text(), "-"))
		w.permission.mode = data.Get("mode").Text()
	}
	if w.permission.mode != "" {
		w.storeMode()
	}
}

// modelRows emits "model: <id>" at the first model and "model: <old> -> <new>"
// after. A change to the same model is kept, since a change event is a
// decision even when it resolves to the same model.
func (w *eventWalk) modelRows(kind string, data jsonvalue.Value) {
	var next, suffix string
	if kind == "session.auto_mode_resolved" {
		next = data.Get("chosenModel").Text()
		suffix = " (auto, " + cmp.Or(data.Get("routingMethod").Text(), "-") + ")"
	} else {
		next = data.Get("newModel").Text()
		for _, key := range []string{"cause", "source"} {
			if value := data.Get(key); parser.Present(value) {
				suffix += " " + key + "=" + value.Text()
			}
		}
		if effort := data.Get("reasoningEffort"); parser.Present(effort) {
			suffix += " effort=" + effort.Text()
		}
	}
	if next == "" {
		return
	}
	old := cmp.Or(w.models[w.sessionID], data.Get("previousModel").Text())
	w.models[w.sessionID] = next
	w.model = next
	line := "model: " + next
	if old != "" {
		line = "model: " + old + " -> " + next
	}
	w.emit(w.row(timeline.TurnSystem, line+suffix, "", "", next))
}

// hookText returns the text of a hook.start or hook.end event: "hook <type>
// start: <input>", or "hook <type> end: success=<b> output=<output>" and
// " error=<error>" when present; each payload is compact JSON in full, "{}"
// when absent.
func hookText(kind string, data jsonvalue.Value) string {
	hook := cmp.Or(data.Get("hookType").Text(), "-")
	if kind == "hook.start" {
		payload := "{}"
		if input := data.Get("input"); !input.IsNull() {
			payload = parser.JSONText(input)
		}
		return "hook " + hook + " start: " + payload
	}
	success, output := "-", "{}"
	if flag := data.Get("success"); flag.IsBool() {
		success = parser.SettingValue(flag)
	}
	if value := data.Get("output"); !value.IsNull() {
		output = parser.JSONText(value)
	}
	line := "hook " + hook + " end: success=" + success + " output=" + output
	if failure := data.Get("error"); !failure.IsNull() {
		line += " error=" + parser.JSONText(failure)
	}
	return line
}

// infoText returns "session.info <infoType>: <message>" and the url and tip
// present.
func infoText(data jsonvalue.Value) string {
	line := "session.info " + data.Get("infoType").Text() + ": " + data.Get("message").Text()
	for _, key := range []string{"url", "tip"} {
		if value := data.Get(key); parser.Present(value) {
			line += " " + key + "=" + value.Text()
		}
	}
	return line
}

// shutdownText returns the text of a session.shutdown event.
func shutdownText(data jsonvalue.Value) string {
	files := data.Get("codeChanges").Get("filesModified")
	var listed string
	if files.IsArray() {
		names := make([]string, 0, files.Len())
		for _, path := range files.Items() {
			names = append(names, path.Text())
		}
		listed = strings.Join(names, ", ")
	} else {
		listed = files.Text()
	}
	parts := []string{"session end:", cmp.Or(data.Get("shutdownType").Text(), "-")}
	if reason := data.Get("errorReason"); parser.Present(reason) {
		parts = append(parts, reason.Text())
	}
	parts = append(parts, "files modified: "+cmp.Or(listed, "none"))
	return strings.Join(parts, " ")
}
