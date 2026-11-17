package copilotcli

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
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

// eventWalk is the state of one pass over an events.jsonl.
type eventWalk struct {
	parser     *Parser
	artifact   *input.Artifact
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
	permission struct{ mode, agent string }
}

// row returns a row of the event being read, with the project, branch and
// model as they stand now; model overrides the tracked model when given.
func (w *eventWalk) row(turn, text, toolName, toolUseID, model string) timeline.Row {
	return w.parser.row(w.artifact, w.line, turn, text, columns{timestamp: w.timestamp,
		sessionID: w.sessionID, project: w.project, branch: w.branch,
		model: firstNonEmpty(model, w.model), toolName: toolName, toolUseID: toolUseID})
}

// system emits a system row of the event being read.
func (w *eventWalk) system(text string) {
	w.emit(w.row(timeline.TurnSystem, text, "", "", ""))
}

// parseEvents emits the rows of one session's events.jsonl.
func (p *Parser) parseEvents(artifact *input.Artifact, sessionID string, options parser.Options,
	emit parser.Emit) {
	var problems parser.Problems
	walk := &eventWalk{parser: p, artifact: artifact, options: options, emit: emit, sessionID: sessionID,
		project: p.fallbackProject(artifact, parentOf(artifact.DiskPath), sessionID),
		names:   map[string]string{}, requested: map[string]bool{}, models: map[string]string{},
		modes: parser.Tracker{}}
	for number, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		walk.line = number
		walk.timestamp = timeutil.FromValue(record.Get("timestamp"))
		walk.event(text(record.Get("type")), record, record.Get("data"))
	}
	for _, row := range p.ErrorRows(artifact, problems, sessionID, walk.project) {
		row.GitBranch = walk.branch
		emit(row)
	}
}

// event emits the rows of one event, by its type. The assistant turn and
// usage events, the other model events and unknown types are bookkeeping and
// give no row.
func (w *eventWalk) event(kind string, record, data pyjson.Value) {
	switch {
	case kind == "session.start" || kind == "session.resume" || kind == "session.context_changed":
		w.sessionEvent(kind, data)
	case kind == "system.message":
		label := "system prompt"
		if isString(data.Get("role"), "developer") {
			label = "developer message"
		}
		w.system(label + ": " + text(data.Get("content")))
	case kind == "user.message":
		w.userRow(record, data)
	case kind == "assistant.message":
		w.assistantRows(data)
	case kind == "assistant.reasoning":
		if w.options.IncludeThinking && data.Get("content").Truthy() {
			w.emit(w.row(timeline.TurnThinking, text(data.Get("content")), "", "", ""))
		}
	case kind == "tool.execution_start" || kind == "tool.user_requested":
		w.toolStart(kind, data)
	case kind == "tool.execution_complete":
		callID := text(data.Get("toolCallId"))
		w.emit(w.row(timeline.TurnToolResult, resultText(data), w.names[callID], callID,
			text(data.Get("model"))))
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
func (w *eventWalk) noteEvent(kind string, data pyjson.Value) {
	switch {
	case kind == "session.info":
		w.system(infoText(data))
	case kind == "model.message":
		callModel := text(data.Get("modelCall").Get("model"))
		w.emit(w.row(timeline.TurnSystem, "utility model "+callModel+": "+
			text(data.Get("message").Get("content")), "", "", callModel))
	case kind == "session.compaction_complete":
		w.system("compaction summary: " + text(data.Get("summaryContent")))
	case kind == "abort" || kind == "session.error" || kind == "session.warning":
		w.system(kind + ": " + firstNonEmpty(text(data.Get("reason")), text(data.Get("message")),
			parser.CompactJSON(data)))
	case kind == "hook.start" || kind == "hook.end":
		w.emit(w.row(timeline.TurnSystem, hookText(kind, data), "", text(data.Get("parentToolCallId")), ""))
	case isNoteType(kind):
		main := ""
		for _, key := range noteKeys {
			if value := data.Get(key); !blank(value) {
				main = text(value)
				break
			}
		}
		w.system(kind + ": " + firstNonEmpty(main, parser.CompactJSON(data)))
	case kind == "session.shutdown":
		w.system(shutdownText(data))
	}
}

// isNoteType reports whether an event type is noted by its main field.
func isNoteType(kind string) bool {
	for _, each := range noteTypes {
		if kind == each {
			return true
		}
	}
	return strings.HasPrefix(kind, "subagent.")
}

// sessionEvent handles session.start, session.resume and
// session.context_changed: the working directory and branch they carry.
func (w *eventWalk) sessionEvent(kind string, data pyjson.Value) {
	context := data
	if data.Has("context") {
		context = data.Get("context")
	}
	if context.Get("cwd").Truthy() {
		w.project = text(context.Get("cwd"))
	}
	if context.Has("branch") || kind == "session.context_changed" {
		w.branch = text(context.Get("branch"))
	}
	switch kind {
	case "session.start":
		w.system("session start: copilot " + orDash(text(data.Get("copilotVersion"))) +
			" producer=" + orDash(text(data.Get("producer"))) + " cwd=" +
			orDash(text(context.Get("cwd"))))
	case "session.resume":
		w.system("session resumed")
	}
}

// toolStart emits the tool_use row of a tool the model or the user started,
// once per call id.
func (w *eventWalk) toolStart(kind string, data pyjson.Value) {
	callID, name := text(data.Get("toolCallId")), text(data.Get("toolName"))
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
	w.emit(w.row(timeline.TurnToolUse, body, name, callID, text(data.Get("model"))))
}

// userRow emits a user message: a user row for a typed prompt, else a system
// row naming what sent it.
func (w *eventWalk) userRow(record, data pyjson.Value) {
	content, source := text(data.Get("content")), text(data.Get("source"))
	label := ""
	switch {
	case data.Get("isAutopilotContinuation").Truthy():
		label = "auto-continuation"
	case record.Get("agentId").Truthy():
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
// requests.
func (w *eventWalk) assistantRows(data pyjson.Value) {
	model, content := text(data.Get("model")), text(data.Get("content"))
	if pytext.Strip(content) != "" {
		w.emit(w.row(timeline.TurnAssistant, content, "", "", model))
	}
	if w.options.IncludeThinking {
		thought := text(data.Get("reasoningText"))
		if thought == "" {
			thought = reasoningSummaries(data)
		}
		if thought != "" {
			w.emit(w.row(timeline.TurnThinking, thought, "", "", model))
		}
	}
	for _, request := range parser.Elements(data.Get("toolRequests")) {
		callID, name := text(request.Get("toolCallId")), text(request.Get("name"))
		if callID != "" {
			w.names[callID] = name
			w.requested[callID] = true
		}
		w.emit(w.row(timeline.TurnToolUse, argsText(request.Get("arguments")), name, callID, model))
	}
}

// reasoningSummaries returns the text of a message's reasoning blocks'
// summaries, separated by blank lines.
func reasoningSummaries(data pyjson.Value) string {
	var parts []string
	for _, block := range parser.Elements(data.Get("reasoningBlocks").Get("blocks")) {
		for _, summary := range parser.Elements(block.Get("summary")) {
			if body := summary.Get("text"); body.IsString() && body.Text() != "" {
				parts = append(parts, body.Text())
			}
		}
	}
	return strings.Join(parts, "\n\n")
}

// modeValue returns the tracked permission mode with the agent mode
// appended when there is one.
func (w *eventWalk) modeValue() string {
	if w.permission.agent != "" {
		return w.permission.mode + " agent=" + w.permission.agent
	}
	return w.permission.mode
}

// permissionRows emits the permission request, decision and mode rows of one
// event. The tracked mode is "<permissionMode> agent=<agentMode>"; the two
// change events update their half of it.
func (w *eventWalk) permissionRows(kind string, data pyjson.Value) {
	callID := text(data.Get("toolCallId"))
	switch kind {
	case "permission.requested":
		w.permissionRequest(data)
	case "permission.completed":
		w.emit(w.row(timeline.TurnSystem, decisionText(data), w.names[callID], callID, ""))
	case "permission.carriedForward":
		w.emit(w.row(timeline.TurnSystem, "permission decision: carried-forward source="+
			orDash(text(data.Get("decisionSource"))), w.names[callID], callID, ""))
	case "session.permissions_changed":
		w.permissionsChanged(data)
	case "session.mode_changed":
		old, next := orDash(text(data.Get("previousMode"))), orDash(text(data.Get("newMode")))
		w.system("agent mode: " + old + " -> " + next)
		w.permission.agent = text(data.Get("newMode"))
		if w.permission.mode != "" {
			w.modes[w.sessionID] = w.modeValue()
		}
	}
}

// permissionRequest emits the row of a permission request and the mode row
// when the request shows the mode changed.
func (w *eventWalk) permissionRequest(data pyjson.Value) {
	request := data.Get("permissionRequest")
	callID := text(request.Get("toolCallId"))
	line := "permission request: " + orDash(text(request.Get("kind")))
	if target := requestTarget(request); target != "" {
		line += " " + target
	}
	if intention := request.Get("intention"); !blank(intention) {
		line += " intention=" + text(intention)
	}
	w.emit(w.row(timeline.TurnSystem, line, w.names[callID], callID, ""))
	if data.Get("permissionMode").Truthy() {
		w.permission.mode = text(data.Get("permissionMode"))
	}
	if data.Get("agentMode").Truthy() {
		w.permission.agent = text(data.Get("agentMode"))
	}
	if w.permission.mode == "" {
		return
	}
	if change := w.modes.Change("permission mode", w.sessionID, w.modeValue()); change != "" {
		w.emit(w.row(timeline.TurnSystem, change, w.names[callID], callID, ""))
	}
}

// permissionsChanged emits the row of a permission mode change, in the form
// the app's bundled CLI writes (allow-all keys) or the CLI's own.
func (w *eventWalk) permissionsChanged(data pyjson.Value) {
	allowAll := !data.Has("mode") && !data.Has("previousMode")
	if allowAll {
		allowAll = false
		for _, key := range allowAllKeys {
			allowAll = allowAll || data.Has(key)
		}
	}
	if allowAll {
		old, next := orDash(text(data.Get("previousAllowAllPermissionMode"))),
			text(data.Get("allowAllPermissionMode"))
		w.system("permission mode: allow-all " + old + " -> " + orDash(next))
		if next != "" {
			w.permission.mode = "allow-all " + next
		}
	} else {
		w.system("permission mode: " + orDash(text(data.Get("previousMode"))) + " -> " +
			orDash(text(data.Get("mode"))))
		w.permission.mode = text(data.Get("mode"))
	}
	if w.permission.mode != "" {
		w.modes[w.sessionID] = w.modeValue()
	}
}

// modelRows emits "model: <id>" at the first model and "model: <old> -> <new>"
// after. A change to the same model is kept, since a change event is a
// decision even when it resolves to the same model.
func (w *eventWalk) modelRows(kind string, data pyjson.Value) {
	var next, suffix string
	if kind == "session.auto_mode_resolved" {
		next = text(data.Get("chosenModel"))
		suffix = " (auto, " + orDash(text(data.Get("routingMethod"))) + ")"
	} else {
		next = text(data.Get("newModel"))
		for _, key := range []string{"cause", "source"} {
			if value := data.Get(key); !blank(value) {
				suffix += " " + key + "=" + text(value)
			}
		}
		if effort := data.Get("reasoningEffort"); !blank(effort) {
			suffix += " effort=" + text(effort)
		}
	}
	if next == "" {
		return
	}
	old := firstNonEmpty(w.models[w.sessionID], text(data.Get("previousModel")))
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
func hookText(kind string, data pyjson.Value) string {
	hook := orDash(text(data.Get("hookType")))
	if kind == "hook.start" {
		payload := "{}"
		if input := data.Get("input"); !input.IsNull() {
			payload = parser.CompactJSON(input)
		}
		return "hook " + hook + " start: " + payload
	}
	success, output := "-", "{}"
	if flag := data.Get("success"); flag.IsBool() {
		success = parser.SettingValue(flag)
	}
	if value := data.Get("output"); !value.IsNull() {
		output = parser.CompactJSON(value)
	}
	line := "hook " + hook + " end: success=" + success + " output=" + output
	if failure := data.Get("error"); !failure.IsNull() {
		line += " error=" + parser.CompactJSON(failure)
	}
	return line
}

// infoText returns "session.info <infoType>: <message>" and the url and tip
// present.
func infoText(data pyjson.Value) string {
	line := "session.info " + text(data.Get("infoType")) + ": " + text(data.Get("message"))
	for _, key := range []string{"url", "tip"} {
		if value := data.Get(key); !blank(value) {
			line += " " + key + "=" + text(value)
		}
	}
	return line
}

// shutdownText returns the text of a session.shutdown event.
func shutdownText(data pyjson.Value) string {
	files := data.Get("codeChanges").Get("filesModified")
	var listed string
	if files.IsArray() {
		names := make([]string, 0, files.Len())
		for _, path := range files.Items() {
			names = append(names, text(path))
		}
		listed = strings.Join(names, ", ")
	} else {
		listed = text(files)
	}
	parts := []string{"session end:", orDash(text(data.Get("shutdownType")))}
	if reason := data.Get("errorReason"); !blank(reason) {
		parts = append(parts, text(reason))
	}
	parts = append(parts, "files modified: "+firstNonEmpty(listed, "none"))
	return strings.Join(parts, " ")
}
