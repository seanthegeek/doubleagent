package nanobot

import (
	"encoding/base64"
	"io/fs"
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
	// namespaces holds the session namespace directories, each listed once
	// for the file, made on the first lookup; listed says it was made.
	namespaces []namespace
	listed     bool
}

// namespace is one session namespace directory and its listing.
type namespace struct {
	directory parser.Directory
	listing   parser.Listing
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
// cannot be read adds its unreadable problem. The file is looked up by its
// exact name in each namespace's listing, since the agent names it by the
// same encoding of the session id.
func (w *webui) workspace(sessionID string) string {
	name, named := parser.Component(base64.RawURLEncoding.EncodeToString([]byte(sessionID)) + ".jsonl")
	if !named {
		return ""
	}
	found := map[string]bool{}
	for _, each := range w.sessionNamespaces() {
		if _, ok := each.listing.RegularFile(name); !ok {
			continue
		}
		if marker, ok := each.directory.Child(".workspace"); ok {
			found[w.marker(marker, &w.problems)] = true
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

// sessionNamespaces returns the session namespace directories beside the
// transcript's sessions directory, those that are plain directories and
// not hidden, each listed once for the file.
func (w *webui) sessionNamespaces() []namespace {
	if w.listed {
		return w.namespaces
	}
	w.listed = true
	levels := 1
	if w.segmented {
		levels = 2
	}
	sessions, ok := parser.Above(w.artifact, levels, "sessions")
	if !ok {
		return nil
	}
	listed, ok := parser.DirectoryAt(sessions)
	entries, err := parser.ReadDir(sessions)
	if !ok || err != nil {
		return nil
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") || entry.Type()&fs.ModeSymlink != 0 || !entry.IsDir() {
			continue
		}
		path, ok := listed.Child(entry.Name())
		if !ok {
			continue
		}
		if directory, ok := parser.DirectoryAt(path); ok {
			w.Work.Add("namespace listings", 1)
			w.namespaces = append(w.namespaces, namespace{directory: directory, listing: parser.ListingOf(directory)})
		}
	}
	return w.namespaces
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
		for _, each := range webuiEvents(record, line, options, &state.problems) {
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
// other a `webui event:` row. A tool_events or edits that is not an array
// adds a problem at line.
func webuiEvents(record jsonvalue.Value, line int, options parser.Options, problems *parser.Problems) []event {
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
		return messageEvents(record, text, options, line, problems)
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
		return fileEdits(record, line, problems)
	case "fork_marker":
		return only(timeline.TurnSystem, "webui fork marker: the fork starts here")
	case "context_compaction":
		return only(timeline.TurnSystem, "context compaction: "+parser.Text(record.Get("phase")))
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
func messageEvents(record jsonvalue.Value, text string, options parser.Options, line int,
	problems *parser.Problems) []event {
	var toolEvents []jsonvalue.Value
	for _, entry := range parser.ArrayMember(record.Get("tool_events"), "tool_events", line, problems) {
		if entry.IsObject() {
			toolEvents = append(toolEvents, entry)
		}
	}
	events := toolEventRows(toolEvents)
	kind := record.Get("kind")
	switch {
	case parser.IsText(kind, "reasoning"):
		if options.IncludeThinking && text != "" {
			events = append(events, event{turnType: timeline.TurnThinking, text: text})
		}
	case parser.IsText(kind, "tool_hint"), parser.IsText(kind, "progress"):
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
func fileEdits(record jsonvalue.Value, line int, problems *parser.Problems) []event {
	var events []event
	for _, entry := range parser.ArrayMember(record.Get("edits"), "edits", line, problems) {
		if !entry.IsObject() || parser.IsText(entry.Get("phase"), "start") {
			continue
		}
		operation := ""
		if value := entry.Get("operation"); !value.IsEmpty() {
			operation = " (" + value.Text() + ")"
		}
		text := "file edit: " +
			parser.Text(parser.FirstNonEmpty(entry.Get("status"), entry.Get("phase"))) + " " +
			parser.Text(parser.FirstNonEmpty(entry.Get("absolute_path"), entry.Get("path"))) + " +" +
			getOr(entry, "added", "0") + " -" + getOr(entry, "deleted", "0") + operation
		events = append(events, event{timeline.TurnSystem, text, parser.Text(entry.Get("tool")),
			parser.Text(entry.Get("call_id"))})
	}
	return events
}

// toolEventRows returns the tool call and result rows of a message's tool
// events, joined on call_id.
func toolEventRows(toolEvents []jsonvalue.Value) []event {
	var events []event
	for _, entry := range toolEvents {
		name, callID := parser.Text(entry.Get("name")), parser.Text(entry.Get("call_id"))
		phase := entry.Get("phase")
		switch {
		case parser.IsText(phase, "start"):
			events = append(events, event{timeline.TurnToolUse,
				openclaw.ToolArgsText(entry.Get("arguments")), name, callID})
		case parser.IsText(phase, "end"):
			result := entry.Get("result")
			body := parser.JSONText(result)
			if result.IsString() {
				body = result.StringValue()
			}
			events = append(events, event{timeline.TurnToolResult, body, name, callID})
		case parser.IsText(phase, "error"):
			events = append(events, event{timeline.TurnToolResult, parser.Text(entry.Get("error")),
				name, callID})
		}
	}
	return events
}
