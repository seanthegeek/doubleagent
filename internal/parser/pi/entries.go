package pi

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The customType of the entry pi writes when the user files a bug report.
const bugReportType = "pi.bug-report"

// gatePrefix starts the refusal text of little-coder's shell allow-list,
// whose other refusals are gateRefusals.
const gatePrefix = "shell whitelist:"

// gateRefusals are the refusal texts of little-coder's permission-gate
// extension.
func gateRefusals() []string {
	return []string{"edit cancelled by user", "command cancelled by user"}
}

// argumentKeys are the tool arguments that identify a call best, in the
// order ArgsSummary tries them.
func argumentKeys() []string {
	return []string{"command", "path", "file_path", "pattern", "code", "url", "query"}
}

// ArgsSummary returns a tool call's most identifying argument: for an
// object, the first of command, path, file_path, pattern, code, url and
// query that is neither null nor empty, as text when it is a string and as
// compact JSON otherwise, else the whole object as compact JSON; for any
// other value, "" when it is null or empty, else the value as text or JSON.
func ArgsSummary(arguments jsonvalue.Value) string {
	if arguments.IsObject() {
		for _, key := range argumentKeys() {
			if value := arguments.Get(key); parser.Present(value) {
				return parser.SettingValue(value)
			}
		}
		return parser.JSONText(arguments)
	}
	if !parser.Present(arguments) {
		return ""
	}
	return parser.SettingValue(arguments)
}

// exitSucceeded reports whether a bash exit code is no failure: null, false
// or a number equal to zero.
func exitSucceeded(value jsonvalue.Value) bool {
	return value.IsNull() || ((value.IsNumber() || value.IsBool()) && value.IsEmpty())
}

// label returns "<prefix>: <text>", or the prefix alone when text is empty.
func label(prefix, text string) string {
	if text == "" {
		return prefix
	}
	return prefix + ": " + text
}

// ModelText returns the text of the `model:` row when model is new or
// changed for key in seen, else "". The text ends with " provider=<name>"
// when provider is set and, when the provider answered with another model
// than the one asked for, " requested=<the model asked for>".
func ModelText(seen parser.Tracker, key, model, provider, requested string) string {
	text := seen.Change("model", key, model)
	if text != "" && provider != "" {
		text += " provider=" + provider
	}
	if text != "" && requested != "" && requested != model {
		text += " requested=" + requested
	}
	return text
}

// SwitchText returns the row text of a model change the user made: the
// ModelText row, or, when the change names the model already in force, the
// plain "model: <id> provider=<name>" row, so the switch keeps a row.
func SwitchText(seen parser.Tracker, key, model, provider string) string {
	text := ModelText(seen, key, model, provider, "")
	if text == "" && model != "" {
		text = "model: " + model
		if provider != "" {
			text += " provider=" + provider
		}
	}
	return text
}

// BugReportText returns the row text of a pi.bug-report entry's data:
// "session synced:" for a report uploaded, "bug report exported:" for one
// saved as a zip, and "" for any other delivery or data that is not an
// object. The hint is not written.
func BugReportText(data jsonvalue.Value) string {
	if !data.IsObject() {
		return ""
	}
	field := func(key string) string { return parser.SettingOrDash(data.Get(key)) }
	source := "(from session entry " + bugReportType + ", entry timestamp)"
	switch delivery := data.Get("delivery"); {
	case parser.IsText(delivery, "upload"):
		return "session synced: bug report " + field("id") + " uploaded to radius.pi.dev transcript=" +
			field("sessionIncluded") + " summary=" + field("summaryIncluded") + " " + source
	case parser.IsText(delivery, "zip"):
		return "bug report exported: " + field("id") + " path=" + field("path") + " " + source
	}
	return ""
}

// EntryRows emits the timeline rows of a pi-format session's entries, in
// file order, each with the given session id and project path.
func EntryRows(base parser.Base, artifact *input.Artifact, session *Session,
	options parser.Options, sessionID, projectPath string, emit parser.Emit) {
	entryRows(base, artifact, session, options, sessionID, projectPath, nil, emit)
}

// entryRows emits the rows of the session's entries less those whose
// identity is in skip: the entries a fork copied from its parent.
func entryRows(base parser.Base, artifact *input.Artifact, session *Session,
	options parser.Options, sessionID, projectPath string, skip map[entryKey]bool, emit parser.Emit) {
	writer := &entryWriter{base: base, artifact: artifact, options: options, sessionID: sessionID,
		projectPath: projectPath, models: parser.Tracker{}, emit: emit}
	for _, entry := range session.Entries {
		if len(skip) > 0 && skip[keyOf(entry.Record)] {
			continue
		}
		writer.line = entry.Line
		writer.entry(entry.Record)
	}
}

// entryWriter writes the rows of one session's entries, keeping the model
// in force for the `model:` rows.
type entryWriter struct {
	base                   parser.Base
	artifact               *input.Artifact
	options                parser.Options
	sessionID, projectPath string
	models                 parser.Tracker
	emit                   parser.Emit
	// The line and time of the entry being written.
	line      int
	timestamp string
}

// put emits one row of the entry being written.
func (w *entryWriter) put(turn, text, model, toolName, toolUseID string) {
	row := w.base.BaseRow(w.artifact)
	row.SourceLine = w.line
	row.TimestampUTC = w.timestamp
	row.SessionID = w.sessionID
	row.ProjectPath = w.projectPath
	row.Model = model
	row.TurnType = turn
	row.ToolName = toolName
	row.ToolUseID = toolUseID
	row.Text = timeline.Compact(text)
	w.emit(row)
}

// system emits a system row of the entry being written.
func (w *entryWriter) system(text string) {
	w.put(timeline.TurnSystem, text, "", "", "")
}

// entry emits the rows of one entry. Other custom entries, labels, usage
// and unknown types carry no conversation content and give none.
func (w *entryWriter) entry(record jsonvalue.Value) {
	message := record.Get("message")
	w.timestamp = timeutil.FirstNonEmpty(timeutil.FromValue(record.Get("timestamp")),
		timeutil.FromValue(message.Get("timestamp")))
	switch kind := record.Get("type"); {
	case parser.IsText(kind, "message"):
		w.message(message, record)
	case parser.IsText(kind, "model_change"):
		model := parser.Text(record.Get("modelId"))
		if text := SwitchText(w.models, w.sessionID, model, parser.Text(record.Get("provider"))); text != "" {
			w.put(timeline.TurnSystem, text, model, "", "")
		}
	case parser.IsText(kind, "custom"):
		if parser.IsText(record.Get("customType"), bugReportType) {
			if text := BugReportText(record.Get("data")); text != "" {
				w.system(text)
			}
		}
	case kind.IsString():
		if text, ok := entryText(kind.StringValue(), record); ok {
			w.system(text)
		}
	}
}

// entryText returns the system row text of an entry type that gives one
// row from its own fields, or false for another type.
func entryText(kind string, record jsonvalue.Value) (string, bool) {
	field := func(key string) string { return parser.Text(record.Get(key)) }
	switch kind {
	case "thinking_level_change":
		return "thinking_level_change: " + field("thinkingLevel"), true
	case "compaction":
		return label("compaction: tokensBefore="+field("tokensBefore"), field("summary")), true
	case "branch_summary":
		return label("branch_summary from "+field("fromId"), field("summary")), true
	case "custom_message":
		return label("custom_message "+field("customType"), parser.TextOf(record.Get("content"))), true
	case "context_edit":
		what := "omitted from context"
		if replacement := record.Get("replacement"); replacement.IsObject() {
			what = parser.TextOf(replacement.Get("content"))
		}
		return label("context_edit target="+field("targetId"), what), true
	case "session_info":
		return "session_info: name=" + field("name"), true
	}
	return "", false
}

// message emits the rows of a message entry by its role.
func (w *entryWriter) message(message, record jsonvalue.Value) {
	content := message.Get("content")
	switch role := message.Get("role"); {
	case parser.IsText(role, "user"):
		if text := parser.TextOf(content); text != "" {
			w.put(timeline.TurnUser, text, "", "", "")
		}
	case parser.IsText(role, "assistant"):
		w.assistant(message)
	case parser.IsText(role, "toolResult"):
		w.toolResult(message)
	case parser.IsText(role, "bashExecution"):
		w.bash(message, parser.Text(record.Get("id")))
	case parser.IsText(role, "system"):
		w.system(label("system prompt", parser.TextOf(content)))
	case parser.IsText(role, "custom"):
		w.system(label("custom "+parser.Text(message.Get("customType")), parser.TextOf(content)))
	case parser.IsText(role, "branchSummary"), parser.IsText(role, "compactionSummary"):
		w.system(label(role.StringValue(), parser.Text(message.Get("summary"))))
	}
}

// assistant emits an assistant message's model row, its text, thinking and
// tool call rows, and a row for a turn that stopped on an error.
func (w *entryWriter) assistant(message jsonvalue.Value) {
	requested := parser.Text(message.Get("model"))
	model := parser.Text(message.Get("responseModel"))
	if model == "" {
		model = requested
	}
	provider := parser.Text(message.Get("provider"))
	if text := ModelText(w.models, w.sessionID, model, provider, requested); text != "" {
		w.put(timeline.TurnSystem, text, model, "", "")
	}
	content := message.Get("content")
	if content.IsString() {
		content = jsonvalue.NewArray(jsonvalue.NewObject(
			jsonvalue.Member{Key: "type", Value: jsonvalue.NewString("text")},
			jsonvalue.Member{Key: "text", Value: content}))
	}
	if content.IsArray() {
		var problems parser.Problems
		for _, part := range parser.EntriesOfKind(content, "content", jsonvalue.Object, w.line, &problems) {
			w.assistantPart(part, model)
		}
		for _, problem := range problems {
			w.put(timeline.TurnSystem, parser.ProblemPrefix+problem.Message, "", "", "")
		}
	}
	if stop := message.Get("stopReason"); parser.IsText(stop, "error") || parser.IsText(stop, "aborted") {
		w.put(timeline.TurnSystem, label("assistant stopReason="+stop.StringValue(),
			parser.Text(message.Get("errorMessage"))), model, "", "")
	}
}

// assistantPart emits the row of one part of an assistant message's
// content: text, thinking when it is asked for, or a tool call.
func (w *entryWriter) assistantPart(part jsonvalue.Value, model string) {
	switch kind := part.Get("type"); {
	case parser.IsText(kind, "text") && !part.Get("text").IsEmpty():
		w.put(timeline.TurnAssistant, part.Get("text").Text(), model, "", "")
	case parser.IsText(kind, "thinking") && w.options.IncludeThinking:
		text := parser.Text(part.Get("thinking"))
		if !part.Get("redacted").IsEmpty() && text == "" {
			text = "[redacted]"
		}
		if text != "" {
			w.put(timeline.TurnThinking, text, model, "", "")
		}
	case parser.IsText(kind, "toolCall"):
		w.put(timeline.TurnToolUse, ArgsSummary(part.Get("arguments")), model,
			parser.Text(part.Get("name")), parser.Text(part.Get("id")))
	}
}

// toolResult emits a tool result row, and a permission row when the result
// is a refusal of little-coder's permission gate.
func (w *entryWriter) toolResult(message jsonvalue.Value) {
	reason := parser.TextOf(message.Get("content"))
	failed := !message.Get("isError").IsEmpty()
	text := reason
	if failed {
		text = "[error] " + reason
	}
	name, callID := parser.Text(message.Get("toolName")), parser.Text(message.Get("toolCallId"))
	w.put(timeline.TurnToolResult, text, "", name, callID)
	if failed && (slices.Contains(gateRefusals(), reason) || strings.HasPrefix(reason, gatePrefix)) {
		w.put(timeline.TurnSystem, "permission decision: denied ("+reason+")", "", name, callID)
	}
}

// bash emits the call and result rows of a command the user ran with !,
// the result ending with its exit code, cancellation and truncation.
func (w *entryWriter) bash(message jsonvalue.Value, entryID string) {
	w.put(timeline.TurnToolUse, parser.Text(message.Get("command")), "", "bash", entryID)
	output := parser.Text(message.Get("output"))
	var notes []string
	if code := message.Get("exitCode"); !exitSucceeded(code) {
		notes = append(notes, "exit "+code.Text())
	}
	if !message.Get("cancelled").IsEmpty() {
		notes = append(notes, "cancelled")
	}
	if !message.Get("truncated").IsEmpty() {
		notes = append(notes, "truncated")
	}
	if len(notes) > 0 {
		if output != "" {
			output += "\n"
		}
		output += "[" + strings.Join(notes, ", ") + "]"
	}
	w.put(timeline.TurnToolResult, output, "", "bash", entryID)
}
