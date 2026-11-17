package nanobot

import (
	"encoding/base64"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/openclaw"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// webuiPrefix starts the session id of a WebUI chat.
const webuiPrefix = "websocket:"

// webuiSilent are the events that give no row: stream chunks, folded into
// their end records, and the turn boundary.
func webuiSilent() []string {
	return []string{"delta", "reasoning_delta", "turn_end"}
}

// event is one row of a WebUI record before it gets its session.
type event struct {
	turnType  string
	text      string
	toolName  string
	toolUseID string
}

// webui is the state of one WebUI transcript file while its rows are made:
// the project path found for each session id, and the problems met.
type webui struct {
	*Parser
	artifact  *input.Artifact
	segmented bool
	projects  map[string]string
	problems  parser.Problems
}

// project returns the project path of a session id, looked up once.
func (w *webui) project(sessionID string) string {
	if project, ok := w.projects[sessionID]; ok {
		return project
	}
	project := w.workspace(sessionID)
	w.projects[sessionID] = project
	return project
}

// workspace returns the .workspace of the session file for sessionID when
// exactly one session namespace that has the file gives one. A marker that
// cannot be read adds its unreadable problem.
func (w *webui) workspace(sessionID string) string {
	levels := 1
	if w.segmented {
		levels = 2
	}
	sessions, ok := parser.Above(w.artifact, levels, "sessions")
	stem := strings.TrimRight(base64.URLEncoding.EncodeToString([]byte(sessionID)), "=")
	name, named := parser.Component(stem + ".jsonl")
	if !ok || !named {
		return ""
	}
	entries, err := parser.ReadDir(sessions)
	if err != nil {
		return ""
	}
	found := map[string]bool{}
	for _, entry := range entries {
		hidden := strings.HasPrefix(entry.Name(), ".")
		if hidden || entry.Type()&fs.ModeSymlink != 0 || !entry.IsDir() {
			continue
		}
		namespace := filepath.Join(sessions, entry.Name())
		if parser.RegularFile(filepath.Join(namespace, name)) {
			found[readMarker(filepath.Join(namespace, ".workspace"), &w.problems)] = true
		}
	}
	delete(found, "")
	if len(found) != 1 {
		return ""
	}
	for project := range found {
		return project
	}
	return ""
}

// parseWebUI emits the rows of a WebUI transcript file, each record's
// session id websocket:<chat_id>, else one named after the file.
func (p *Parser) parseWebUI(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	match := p.files.webui.FindStringSubmatch(artifact.Rel)
	segment := group(p.files.webui, match, "seg")
	stem := segment
	if stem == "" {
		stem = group(p.files.webui, match, "stem")
	}
	fallback := stem
	if rest, ok := strings.CutPrefix(stem, "websocket_"); ok {
		fallback = webuiPrefix + rest
	}
	state := &webui{Parser: p, artifact: artifact, segmented: segment != "", projects: map[string]string{}}
	sessionID := fallback
	for line, record := range parser.JSONLines(artifact.DiskPath, &state.problems) {
		sessionID = fallback
		if chat := record.Get("chat_id"); chat.IsString() && chat.StringValue() != "" {
			sessionID = webuiPrefix + chat.StringValue()
		}
		for _, each := range webuiEvents(record, options) {
			row := p.BaseRow(artifact)
			row.SourceLine = line
			row.SessionID = sessionID
			row.ProjectPath = state.project(sessionID)
			row.TimestampUTC = timeutil.FromValue(record.Get("created_at_ms"))
			row.TurnType, row.ToolName, row.ToolUseID = each.turnType, each.toolName, each.toolUseID
			row.Text = timeline.Compact(each.text)
			emit(row)
		}
	}
	if len(state.problems) > 0 {
		project := state.project(sessionID)
		parser.EmitAll(emit, p.ErrorRows(artifact, state.problems, sessionID, project))
	}
}

// webuiEvents returns the rows of one WebUI record by its event: a user
// prompt, a message (tool events joined on call_id, a breadcrumb only
// without them, reasoning, else assistant text), the end of a stream unless
// it merges into the next, the end of reasoning, finished file edits, a fork
// marker and a context compaction. The silent events give nothing and any
// other a `webui event:` row.
func webuiEvents(record jsonvalue.Value, options parser.Options) []event {
	name := record.Get("event")
	text := ""
	if value := record.Get("text"); value.IsString() {
		text = value.StringValue()
	}
	if !name.IsString() || name.StringValue() == "" || slices.Contains(webuiSilent(), name.StringValue()) {
		return nil
	}
	only := func(turnType, text string) []event {
		return []event{{turnType: turnType, text: text}}
	}
	switch name.StringValue() {
	case "user":
		return only(timeline.TurnUser, userText(record, text))
	case "message":
		return messageEvents(record, text, options)
	case "stream_end":
		if text != "" && record.Get("merge_next").IsEmpty() {
			return only(timeline.TurnAssistant, text)
		}
		return nil
	case "reasoning_end":
		if options.IncludeThinking && text != "" {
			return only(timeline.TurnThinking, text)
		}
		return nil
	case "file_edit":
		return fileEdits(record)
	case "fork_marker":
		return only(timeline.TurnSystem, "webui fork marker: the fork starts here")
	case "context_compaction":
		return only(timeline.TurnSystem, "context compaction: "+parser.Str(record.Get("phase")))
	}
	return only(timeline.TurnSystem, "webui event: "+name.StringValue())
}

// userText returns the text of a user prompt, else its media paths.
func userText(record jsonvalue.Value, text string) string {
	if text != "" {
		return text
	}
	var media []string
	for _, path := range parser.Elements(record.Get("media_paths")) {
		if !path.IsEmpty() {
			media = append(media, "[media: "+path.Text()+"]")
		}
	}
	return strings.Join(media, " ")
}

// messageEvents returns the rows of a message record: its tool events,
// then its text by kind.
func messageEvents(record jsonvalue.Value, text string, options parser.Options) []event {
	var toolEvents []jsonvalue.Value
	for _, entry := range parser.Elements(record.Get("tool_events")) {
		if entry.IsObject() {
			toolEvents = append(toolEvents, entry)
		}
	}
	events := toolEventRows(toolEvents)
	kind := record.Get("kind")
	switch {
	case isText(kind, "reasoning"):
		if options.IncludeThinking && text != "" {
			events = append(events, event{turnType: timeline.TurnThinking, text: text})
		}
	case isText(kind, "tool_hint"), isText(kind, "progress"):
		if text != "" && len(toolEvents) == 0 {
			label := strings.ReplaceAll(kind.StringValue(), "_", " ")
			events = append(events, event{turnType: timeline.TurnSystem, text: label + ": " + text})
		}
	case text != "":
		events = append(events, event{turnType: timeline.TurnAssistant, text: text})
	}
	return events
}

// fileEdits returns a `file edit:` row per edit of a file_edit record that
// is past its start.
func fileEdits(record jsonvalue.Value) []event {
	var events []event
	for _, entry := range parser.Elements(record.Get("edits")) {
		if !entry.IsObject() || isText(entry.Get("phase"), "start") {
			continue
		}
		operation := ""
		if value := entry.Get("operation"); !value.IsEmpty() {
			operation = " (" + value.Text() + ")"
		}
		text := fmt.Sprintf("file edit: %s %s +%s -%s%s",
			firstStr(entry.Get("status"), entry.Get("phase")),
			firstStr(entry.Get("absolute_path"), entry.Get("path")),
			getOr(entry, "added", "0"), getOr(entry, "deleted", "0"), operation)
		events = append(events, event{timeline.TurnSystem, text, parser.Str(entry.Get("tool")),
			parser.Str(entry.Get("call_id"))})
	}
	return events
}

// toolEventRows returns the tool call and result rows of a message's tool
// events, joined on call_id.
func toolEventRows(toolEvents []jsonvalue.Value) []event {
	var events []event
	for _, entry := range toolEvents {
		name, callID := parser.Str(entry.Get("name")), parser.Str(entry.Get("call_id"))
		phase := entry.Get("phase")
		switch {
		case isText(phase, "start"):
			events = append(events, event{timeline.TurnToolUse,
				openclaw.ToolArgsText(entry.Get("arguments")), name, callID})
		case isText(phase, "end"):
			result := entry.Get("result")
			body := parser.CompactJSON(result)
			if result.IsString() {
				body = result.StringValue()
			}
			events = append(events, event{timeline.TurnToolResult, body, name, callID})
		case isText(phase, "error"):
			events = append(events, event{timeline.TurnToolResult, parser.Str(entry.Get("error")),
				name, callID})
		}
	}
	return events
}
