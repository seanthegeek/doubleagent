package claudecode

import (
	"cmp"
	"strconv"
	"strings"
	"unicode"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// user emits the rows of a user record: its blocks, a "prompt trigger:" row
// when something other than the person typing sent it, and a "permission
// decision:" row when it records one. isTask marks a subagent's first
// prompt, the task it was given.
func (s *session) user(record jsonvalue.Value, isTask bool) {
	s.userBlocks(record, isTask)
	source, origin := record.Get("promptSource").Text(), record.Get("turnOrigin").Text()
	if (source != "" && source != "typed") || (origin != "" && origin != "human") {
		s.emit(s.row(timeline.TurnSystem, "prompt trigger: promptSource="+cmp.Or(source, "-")+
			" turnOrigin="+cmp.Or(origin, "-")))
	}
	decision := record.Get("permissionDecision")
	if !decision.IsNull() || parser.Present(record.Get("toolDenialKind")) ||
		parser.Present(record.Get("userFeedback")) {
		s.emit(s.permissionRow(record, decision))
	}
}

// permissionRow returns the "permission decision:" row of a user record:
// the decision, its source and reason, the whole decision as JSON when it
// holds more, and the denial kind and the person's feedback. A record with
// a denial kind and no decision is a rejection that interrupted the turn.
func (s *session) permissionRow(record, decision jsonvalue.Value) timeline.Row {
	row := s.row(timeline.TurnSystem, "")
	for _, block := range record.Get("message").Get("content").Items() {
		if block.IsObject() && parser.IsText(block.Get("type"), "tool_result") {
			row.ToolUseID = parser.Text(block.Get("tool_use_id"))
			break
		}
	}
	var denial strings.Builder
	for _, key := range []string{"toolDenialKind", "userFeedback"} {
		if value := record.Get(key); parser.Present(value) {
			denial.WriteString(" " + key + "=" + parser.SettingValue(value))
		}
	}
	text := ""
	switch {
	case decision.IsNull() && parser.Present(record.Get("toolDenialKind")):
		text = "permission decision: reject (interrupted)"
	case decision.IsNull():
		text = "permission decision: - source=- reason=-"
	case !decision.IsObject():
		text = "permission decision: " + parser.JSONText(decision)
	default:
		text = "permission decision: " + textOr(decision.Get("decision"), "-") + " source=" +
			textOr(decision.Get("source"), "-") + " reason=" + textOr(decision.Get("reasonType"), "-")
		for _, key := range decision.Keys() {
			if key != "decision" && key != "source" && key != "reasonType" {
				text += " " + parser.JSONText(decision)
				break
			}
		}
	}
	row.Text = timeline.Compact(text + denial.String())
	return row
}

// userBlocks emits the rows of a user record's content: its text and image
// blocks as user rows, or as labelled system rows when the agent wrote
// them, and its tool results with any persisted output read in full.
func (s *session) userBlocks(record jsonvalue.Value, isTask bool) {
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
		if block.IsObject() && parser.IsText(block.Get("type"), "tool_result") {
			results++
		}
	}
	savedPath := ""
	if saved.IsString() && results == 1 {
		savedPath = saved.StringValue()
	}
	for _, block := range parser.EntriesOfKind(content, "message.content", jsonvalue.Object, s.line, &s.problems) {
		switch blockType := block.Get("type"); {
		case parser.IsText(blockType, "tool_result"):
			s.toolResult(block, savedPath)
		case parser.IsText(blockType, "text"), parser.IsText(blockType, "image"):
			s.emit(s.textRow(record, label, parser.TextOf(jsonvalue.NewArray(block))))
		}
	}
}

// recordLabel returns the label every text row of a user record takes: a
// compaction summary, a subagent's task or an interrupted turn, else "".
func recordLabel(record jsonvalue.Value, isTask bool) string {
	switch {
	case !record.Get("isCompactSummary").IsEmpty():
		return "compaction summary"
	case isTask:
		return "subagent task"
	case !record.Get("interruptedMessageId").IsEmpty():
		return "interrupted"
	}
	return ""
}

// textRow returns the row of one text of a user record: a user row, or a
// system row "<label>: <text>" when the record's label, a marker at the
// text's start, the record's origin or its meta flag says the agent wrote
// it.
func (s *session) textRow(record jsonvalue.Value, label, text string) timeline.Row {
	if label == "" {
		label = markerLabel(text)
	}
	if label == "" {
		label = originLabel(record.Get("origin").Get("kind"))
	}
	if label == "" && !record.Get("isMeta").IsEmpty() {
		label = "context"
	}
	if label != "" {
		return s.row(timeline.TurnSystem, label+": "+text)
	}
	return s.row(timeline.TurnUser, text)
}

// toolResult emits the row of a tool_result block.
func (s *session) toolResult(block jsonvalue.Value, savedPath string) {
	text := ResultText(block.Get("content"))
	if savedPath != "" || strings.HasPrefix(strings.TrimLeftFunc(text, unicode.IsSpace), persistedMark) {
		text = persistedText(s.artifact, text, savedPath, s.persisted, &s.problems)
	}
	if !block.Get("is_error").IsEmpty() {
		text = "[error] " + text
	}
	row := s.row(timeline.TurnToolResult, text)
	row.ToolUseID = parser.Text(block.Get("tool_use_id"))
	s.emit(row)
}

// assistant emits the rows of an assistant record: an "api error:" row for
// an error the API returned, else a row per tool call, text and, when asked
// for, thinking block.
func (s *session) assistant(record jsonvalue.Value) {
	message := record.Get("message")
	model := parser.Text(message.Get("model"))
	content := message.Get("content")
	if !record.Get("isApiErrorMessage").IsEmpty() || model == "<synthetic>" {
		var status, parts []string
		for _, value := range []jsonvalue.Value{record.Get("apiErrorStatus"), record.Get("error")} {
			if parser.Present(value) {
				status = append(status, value.Text())
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
	for _, block := range parser.EntriesOfKind(content, "message.content", jsonvalue.Object, s.line, &s.problems) {
		s.assistantBlock(record, block, model)
	}
}

// assistantBlock emits the row of one block of an assistant record. A Bash
// call's input is the one sent on the wire when the record keeps it.
func (s *session) assistantBlock(record, block jsonvalue.Value, model string) {
	blockType := block.Get("type")
	switch {
	case parser.IsText(blockType, "tool_use"):
		row := s.base(s.timestamp)
		row.TurnType = timeline.TurnToolUse
		row.Model = model
		row.ToolName = parser.Text(block.Get("name"))
		row.ToolUseID = parser.Text(block.Get("id"))
		toolInput := block.Get("input")
		if row.ToolName == "Bash" {
			if sent := record.Get("wireToolInputs").Get(row.ToolUseID); sent.IsObject() &&
				!sent.Get("command").IsEmpty() {
				toolInput = sent
			}
		}
		row.Text = timeline.Compact(ToolSummary(row.ToolName, toolInput))
		s.emit(row)
	case parser.IsText(blockType, "text"):
		if text := block.Get("text"); !text.IsEmpty() {
			s.modelRowOf(timeline.TurnAssistant, model, text.Text())
		}
	case !s.options.IncludeThinking:
	case parser.IsText(blockType, "thinking"):
		if text := block.Get("thinking"); !text.IsEmpty() {
			s.modelRowOf(timeline.TurnThinking, model, text.Text())
		}
	case parser.IsText(blockType, "redacted_thinking"):
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
func (s *session) attachment(record jsonvalue.Value) {
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
func (s *session) attachmentRow(attachment jsonvalue.Value) (timeline.Row, bool) {
	switch kind := attachment.Get("type"); {
	case parser.IsText(kind, "queued_command"):
		text := parser.TextOf(attachment.Get("prompt"))
		origin := attachment.Get("origin").Get("kind")
		label := markerLabel(text)
		if label == "" && !(parser.IsText(origin, "human") && attachment.Get("isMeta").IsEmpty()) {
			label = originLabel(origin)
			if label == "" {
				label = "queued command"
			}
		}
		if label != "" {
			return s.row(timeline.TurnSystem, label+": "+text), true
		}
		return s.row(timeline.TurnUser, text), true
	case parser.IsText(kind, "hook_system_message"):
		name := parser.Text(attachment.Get("hookName"))
		if name == "" {
			name = parser.Text(attachment.Get("hookEvent"))
		}
		var parts []string
		for _, part := range []string{name, parser.TextOf(attachment.Get("content"))} {
			if part != "" {
				parts = append(parts, part)
			}
		}
		return s.row(timeline.TurnSystem, "hook: "+strings.Join(parts, ": ")), true
	case parser.IsText(kind, "edited_text_file"):
		return s.row(timeline.TurnSystem, "edited file: "+parser.Text(attachment.Get("filename"))), true
	}
	return timeline.Row{}, false
}

// scratchpad returns an environment attachment's
// snapshot.scratchpadDirectory, or "".
func scratchpad(record jsonvalue.Value) string {
	attachment := record.Get("attachment")
	if !parser.IsText(attachment.Get("type"), "environment") {
		return ""
	}
	return attachment.Get("snapshot").Get("scratchpadDirectory").StringValue()
}

// bridge emits the rows of a bridge-session record: "remote control: bridge
// session ..." when the bridge id or sequence number changes, and "session
// synced:" the first time a session names a bridge id, with the URL of its
// bridge_status record.
func (s *session) bridge(record jsonvalue.Value) {
	bridge, sequence := record.Get("bridgeSessionId"), record.Get("lastSequenceNum")
	key := bridgeKey{bridge: equalityKey(bridge), sequence: equalityKey(sequence)}
	if last, ok := s.bridges[s.sessionID]; !ok || last != key {
		s.bridges[s.sessionID] = key
		name := "none"
		if !bridge.IsEmpty() {
			name = bridge.Text()
		}
		s.emit(s.stateRow("remote control: bridge session " + name + " lastSequenceNum=" +
			textOr(sequence, "-")))
	}
	if !bridge.IsEmpty() && s.firstSeen("sync", key.bridge) {
		s.emit(s.stateRow(timeline.SyncedPrefix + " remote control bridge " + bridge.Text() + " url " +
			s.bridgeURL(bridge)))
	}
}

// equalityKey returns a key two values share when they hold the same
// value: a number by its value, a boolean as the number 0 or 1 (so 1, 1.0
// and true are one key), a string by its text, and anything else by its
// JSON text in the evidence's key order.
func equalityKey(value jsonvalue.Value) string {
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
		return "s" + value.StringValue()
	}
	return "j" + parser.JSONText(value)
}

// bridgeURL returns the URL of a bridge's first bridge_status record at or
// after the current line, else the nearest before it, among those between
// the bridge-session records of other bridges either side (a detach, with
// no id, counting as another bridge), or "-".
func (s *session) bridgeURL(bridge jsonvalue.Value) string {
	return s.scan.bridgeURL(bridge, s.line, s.reader.Work)
}
