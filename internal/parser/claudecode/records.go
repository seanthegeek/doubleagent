package claudecode

import (
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// user emits the rows of a user record: its blocks, a "prompt trigger:" row
// when something other than the person typing sent it, and a "permission
// decision:" row when it records one. isTask marks a subagent's first
// prompt, the task it was given.
func (s *session) user(record pyjson.Value, isTask bool) {
	s.userBlocks(record, isTask)
	source, origin := pythonText(record.Get("promptSource")), pythonText(record.Get("turnOrigin"))
	if (source != "" && source != "typed") || (origin != "" && origin != "human") {
		s.emit(s.row(timeline.TurnSystem, "prompt trigger: promptSource="+orDash(source)+
			" turnOrigin="+orDash(origin)))
	}
	decision := record.Get("permissionDecision")
	if !decision.IsNull() || !isMissing(record.Get("toolDenialKind")) ||
		!isMissing(record.Get("userFeedback")) {
		s.emit(s.permissionRow(record, decision))
	}
}

// pythonText returns a value's str(), "" for null.
func pythonText(value pyjson.Value) string {
	if value.IsNull() {
		return ""
	}
	return value.Str()
}

// permissionRow returns the "permission decision:" row of a user record:
// the decision, its source and reason, the whole decision as JSON when it
// holds more, and the denial kind and the person's feedback. A record with
// a denial kind and no decision is a rejection that interrupted the turn.
func (s *session) permissionRow(record, decision pyjson.Value) timeline.Row {
	row := s.row(timeline.TurnSystem, "")
	for _, block := range record.Get("message").Get("content").Items() {
		if block.IsObject() && isText(block.Get("type"), "tool_result") {
			row.ToolUseID = parser.Str(block.Get("tool_use_id"))
			break
		}
	}
	denial := ""
	for _, key := range []string{"toolDenialKind", "userFeedback"} {
		if value := record.Get(key); !isMissing(value) {
			denial += " " + key + "=" + parser.SettingValue(value)
		}
	}
	text := ""
	switch {
	case decision.IsNull() && !isMissing(record.Get("toolDenialKind")):
		text = "permission decision: reject (interrupted)"
	case decision.IsNull():
		text = "permission decision: - source=- reason=-"
	case !decision.IsObject():
		text = "permission decision: " + parser.CompactJSON(decision)
	default:
		text = "permission decision: " + strOr(decision.Get("decision"), "-") + " source=" +
			strOr(decision.Get("source"), "-") + " reason=" + strOr(decision.Get("reasonType"), "-")
		for _, key := range decision.Keys() {
			if key != "decision" && key != "source" && key != "reasonType" {
				text += " " + parser.CompactJSON(decision)
				break
			}
		}
	}
	row.Text = timeline.Compact(text + denial)
	return row
}

// userBlocks emits the rows of a user record's content: its text and image
// blocks as user rows, or as labelled system rows when the agent wrote
// them, and its tool results with any persisted output read in full.
func (s *session) userBlocks(record pyjson.Value, isTask bool) {
	content := record.Get("message").Get("content")
	label := recordLabel(record, isTask)
	if !content.IsArray() {
		s.emit(s.textRow(record, label, parser.TextOf(content)))
		return
	}
	// The persisted output's path belongs to the line's one tool call, so
	// it is used only when the line has exactly one tool_result block.
	saved := record.Get("toolUseResult").Get("persistedOutputPath")
	results := 0
	for _, block := range content.Items() {
		if block.IsObject() && isText(block.Get("type"), "tool_result") {
			results++
		}
	}
	savedPath := ""
	if saved.IsString() && results == 1 {
		savedPath = saved.Text()
	}
	for _, block := range content.Items() {
		switch blockType := block.Get("type"); {
		case !block.IsObject():
		case isText(blockType, "tool_result"):
			s.toolResult(block, savedPath)
		case isText(blockType, "text"), isText(blockType, "image"):
			s.emit(s.textRow(record, label, parser.TextOf(pyjson.NewArray(block))))
		}
	}
}

// recordLabel returns the label every text row of a user record takes: a
// compaction summary, a subagent's task or an interrupted turn, else "".
func recordLabel(record pyjson.Value, isTask bool) string {
	switch {
	case record.Get("isCompactSummary").Truthy():
		return "compaction summary"
	case isTask:
		return "subagent task"
	case record.Get("interruptedMessageId").Truthy():
		return "interrupted"
	}
	return ""
}

// textRow returns the row of one text of a user record: a user row, or a
// system row "<label>: <text>" when the record's label, a marker at the
// text's start, the record's origin or its meta flag says the agent wrote
// it.
func (s *session) textRow(record pyjson.Value, label, text string) timeline.Row {
	if label == "" {
		label = markerLabel(text)
	}
	if label == "" {
		label = originLabel(record.Get("origin").Get("kind"))
	}
	if label == "" && record.Get("isMeta").Truthy() {
		label = "context"
	}
	if label != "" {
		return s.row(timeline.TurnSystem, label+": "+text)
	}
	return s.row(timeline.TurnUser, text)
}

// toolResult emits the row of a tool_result block.
func (s *session) toolResult(block pyjson.Value, savedPath string) {
	text := ResultText(block.Get("content"))
	if savedPath != "" || strings.HasPrefix(pytext.StripLeft(text), persistedMark) {
		text = persistedText(s.artifact, text, savedPath, &s.problems)
	}
	if block.Get("is_error").Truthy() {
		text = "[error] " + text
	}
	row := s.row(timeline.TurnToolResult, text)
	row.ToolUseID = parser.Str(block.Get("tool_use_id"))
	s.emit(row)
}

// assistant emits the rows of an assistant record: an "api error:" row for
// an error the API returned, else a row per tool call, text and, when asked
// for, thinking block.
func (s *session) assistant(record pyjson.Value) {
	message := record.Get("message")
	model := parser.Str(message.Get("model"))
	content := message.Get("content")
	if record.Get("isApiErrorMessage").Truthy() || model == "<synthetic>" {
		var status, parts []string
		for _, value := range []pyjson.Value{record.Get("apiErrorStatus"), record.Get("error")} {
			if !isMissing(value) {
				status = append(status, value.Str())
			}
		}
		for _, part := range []string{strings.Join(status, " "), parser.TextOf(content)} {
			if part != "" {
				parts = append(parts, part)
			}
		}
		s.emit(s.row(timeline.TurnSystem, "api error: "+strings.Join(parts, ": ")))
		return
	}
	if !content.IsArray() {
		if text := parser.TextOf(content); text != "" {
			s.modelRowOf(timeline.TurnAssistant, model, text)
		}
		return
	}
	for _, block := range content.Items() {
		if block.IsObject() {
			s.assistantBlock(record, block, model)
		}
	}
}

// assistantBlock emits the row of one block of an assistant record. A Bash
// call's input is the one sent on the wire when the record keeps it.
func (s *session) assistantBlock(record, block pyjson.Value, model string) {
	blockType := block.Get("type")
	switch {
	case isText(blockType, "tool_use"):
		row := s.base(s.timestamp)
		row.TurnType = timeline.TurnToolUse
		row.Model = model
		row.ToolName = parser.Str(block.Get("name"))
		row.ToolUseID = parser.Str(block.Get("id"))
		toolInput := block.Get("input")
		if row.ToolName == "Bash" {
			if sent := record.Get("wireToolInputs").Get(row.ToolUseID); sent.IsObject() &&
				sent.Get("command").Truthy() {
				toolInput = sent
			}
		}
		row.Text = timeline.Compact(ToolSummary(row.ToolName, toolInput))
		s.emit(row)
	case isText(blockType, "text"):
		if text := block.Get("text"); text.Truthy() {
			s.modelRowOf(timeline.TurnAssistant, model, text.Str())
		}
	case !s.options.IncludeThinking:
	case isText(blockType, "thinking"):
		if text := block.Get("thinking"); text.Truthy() {
			s.modelRowOf(timeline.TurnThinking, model, text.Str())
		}
	case isText(blockType, "redacted_thinking"):
		s.modelRowOf(timeline.TurnThinking, model, "[redacted]")
	}
}

// modelRowOf emits a row of the current record with the turn type, model
// and text given.
func (s *session) modelRowOf(turnType, model, text string) {
	row := s.row(turnType, text)
	row.Model = model
	s.emit(row)
}

// attachment emits the row of an attachment record, then the row of the
// scratch directory an environment attachment names. Before the end of a
// teleport's replayed block that directory is the cloud machine's, not this
// host's, and marks the session's origin instead.
func (s *session) attachment(record pyjson.Value) {
	if row, ok := s.attachmentRow(record.Get("attachment")); ok {
		s.emit(row)
	}
	scratch := scratchpad(record)
	if scratch == "" || !s.firstSeen("scratch", scratch) {
		return
	}
	if s.line <= s.scan.remoteLast {
		s.emit(s.stateRow(timeline.OriginPrefix + " (teleport, cloud scratchpad " + scratch + ")"))
		return
	}
	row := s.stateRow(timeline.ScratchPrefix + " " + scratch +
		" (session scratchpad, from the environment attachment)")
	row.ProjectPath = scratch
	s.emit(row)
}

// attachmentRow returns the row of a queued prompt, a hook's message or an
// edited file, and false for any other attachment.
func (s *session) attachmentRow(attachment pyjson.Value) (timeline.Row, bool) {
	switch kind := attachment.Get("type"); {
	case isText(kind, "queued_command"):
		text := parser.TextOf(attachment.Get("prompt"))
		origin := attachment.Get("origin").Get("kind")
		label := markerLabel(text)
		if label == "" && !(isText(origin, "human") && !attachment.Get("isMeta").Truthy()) {
			label = originLabel(origin)
			if label == "" {
				label = "queued command"
			}
		}
		if label != "" {
			return s.row(timeline.TurnSystem, label+": "+text), true
		}
		return s.row(timeline.TurnUser, text), true
	case isText(kind, "hook_system_message"):
		name := parser.Str(attachment.Get("hookName"))
		if name == "" {
			name = parser.Str(attachment.Get("hookEvent"))
		}
		var parts []string
		for _, part := range []string{name, parser.TextOf(attachment.Get("content"))} {
			if part != "" {
				parts = append(parts, part)
			}
		}
		return s.row(timeline.TurnSystem, "hook: "+strings.Join(parts, ": ")), true
	case isText(kind, "edited_text_file"):
		return s.row(timeline.TurnSystem, "edited file: "+parser.Str(attachment.Get("filename"))), true
	}
	return timeline.Row{}, false
}

// scratchpad returns an environment attachment's
// snapshot.scratchpadDirectory, or "".
func scratchpad(record pyjson.Value) string {
	attachment := record.Get("attachment")
	if !isText(attachment.Get("type"), "environment") {
		return ""
	}
	return attachment.Get("snapshot").Get("scratchpadDirectory").Text()
}

// bridge emits the rows of a bridge-session record: "remote control: bridge
// session ..." when the bridge id or sequence number changes, and "session
// synced:" the first time a session names a bridge id, with the URL of its
// bridge_status record.
func (s *session) bridge(record pyjson.Value) {
	bridge, sequence := record.Get("bridgeSessionId"), record.Get("lastSequenceNum")
	key := bridgeKey{bridge: equalityKey(bridge), sequence: equalityKey(sequence)}
	if last, ok := s.bridges[s.sessionID]; !ok || last != key {
		s.bridges[s.sessionID] = key
		name := "none"
		if bridge.Truthy() {
			name = bridge.Str()
		}
		s.emit(s.stateRow("remote control: bridge session " + name + " lastSequenceNum=" +
			strOr(sequence, "-")))
	}
	if bridge.Truthy() && s.firstSeen("sync", key.bridge) {
		s.emit(s.stateRow(timeline.SyncedPrefix + " remote control bridge " + bridge.Str() + " url " +
			s.bridgeURL(bridge)))
	}
}

// equalityKey returns a key two values share when Python's == finds them
// equal: a number or a boolean by its value (1, 1.0 and true are one), a
// string by its text, anything else by its compact JSON with sorted keys.
func equalityKey(value pyjson.Value) string {
	if flag, ok := value.Bool(); ok {
		if flag {
			return "n1"
		}
		return "n0"
	}
	if number, ok := value.Float(); ok && value.IsNumber() {
		return "n" + strconv.FormatFloat(number, 'g', -1, 64)
	}
	if value.IsString() {
		return "s" + value.Text()
	}
	return "j" + parser.CompactJSON(value)
}

// bridgeURL returns the URL of a bridge's first bridge_status record at or
// after the current line, else the nearest before it, among those between
// the bridge-session records of other bridges either side (a detach, with
// no id, counting as another bridge), or "-".
func (s *session) bridgeURL(bridge pyjson.Value) string {
	start, end := 0, -1
	for _, other := range s.scan.bridgeLines {
		if other.named && bridge.IsString() && other.bridgeID == bridge.Text() {
			continue
		}
		if other.line < s.line {
			start = max(start, other.line)
		} else if other.line > s.line && (end < 0 || other.line < end) {
			end = other.line
		}
	}
	before := "-"
	for _, status := range s.scan.bridgeURLs {
		if status.line >= s.line && (end < 0 || status.line < end) {
			return status.url
		}
		if start < status.line && status.line < s.line {
			before = status.url
		}
	}
	return before
}
