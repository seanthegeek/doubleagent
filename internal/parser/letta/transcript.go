package letta

import (
	"io/fs"
	"sort"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// projectTime is one sessions.jsonl line of an agent: when it was written
// and its project.
type projectTime struct {
	timestamp, project string
}

// sessionProject returns the project of a transcript line captured at
// timestamp: that of the agent's earliest sessions.jsonl line at or after
// it (the exit line of the CLI session the turn belongs to), else, when no
// line is later, that of the latest line before it.
func sessionProject(sessions []projectTime, timestamp string) string {
	if timestamp == "" {
		return ""
	}
	before := ""
	for _, session := range sessions {
		if session.timestamp >= timestamp {
			return session.project
		}
		before = session.project
	}
	return before
}

// lettaFile returns the path of name in the .letta directory of the
// transcript's home (a transcript lies four directories below it), when
// neither it nor .letta is a symbolic link and it is a regular file.
func lettaFile(artifact *input.Artifact, name string) (string, bool) {
	letta, ok := parser.Above(artifact, 3)
	path, _ := parser.Above(artifact, 3, name)
	if !ok {
		return "", false
	}
	info, err := osfile.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	if found, err := osfile.Lstat(letta); err == nil && found.Mode()&fs.ModeSymlink != 0 {
		return "", false
	}
	return path, true
}

// agentSessions returns (time, project) of the agent's sessions.jsonl lines
// that name a project and a time, sorted by time then project.
func agentSessions(artifact *input.Artifact, agentDirectory string) []projectTime {
	path, ok := lettaFile(artifact, "sessions.jsonl")
	if !ok {
		return nil
	}
	var sessions []projectTime
	var ignored parser.Problems
	for _, record := range parser.JSONLines(path, &ignored) {
		if sanitize(parser.Str(record.Get("agent_id"))) != agentDirectory || !record.Get("project").Truthy() {
			continue
		}
		if timestamp := timeutil.FromValue(record.Get("timestamp")); timestamp != "" {
			sessions = append(sessions, projectTime{timestamp, record.Get("project").Str()})
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].timestamp != sessions[j].timestamp {
			return sessions[i].timestamp < sessions[j].timestamp
		}
		return sessions[i].project < sessions[j].project
	})
	return sessions
}

// server returns the server of a transcript's agent: the baseUrl of its
// agents[] entry in ~/.letta/settings.json, api.letta.com when the entry has
// none, "-" when there is no entry or no file. It is a pre-scan: the
// settings file reports its own problems when it is parsed itself.
func server(artifact *input.Artifact, agentDirectory string) string {
	path, ok := lettaFile(artifact, "settings.json")
	if !ok {
		return "-"
	}
	var ignored parser.Problems
	document, _ := readSettings(path, &ignored)
	for _, entry := range document.Get("agents").Items() {
		if entry.IsObject() && sanitize(parser.Str(entry.Get("agentId"))) == agentDirectory {
			if base := parser.Str(entry.Get("baseUrl")); base != "" {
				return base
			}
			return lettaCloud
		}
	}
	return "-"
}

// readSettings returns a Letta JSON file as an object, or false when it
// cannot be read (its unreadable problem added) or is not an object (no
// problem added).
func readSettings(path string, problems *parser.Problems) (pyjson.Value, bool) {
	content, ok := parser.FileBytes(path, problems)
	if !ok {
		return pyjson.NewNull(), false
	}
	document, err := pyjson.Decode(content)
	if err != nil || !document.IsObject() {
		return pyjson.NewNull(), false
	}
	return document, true
}

// transcriptRows writes the rows of one transcript.
type transcriptRows struct {
	parser       *Parser
	artifact     *input.Artifact
	conversation string
	line         int
	timestamp    string
	project      string
	emit         parser.Emit
}

// put emits a row of the line being written.
func (w *transcriptRows) put(turn, text, toolName string) {
	row := w.parser.BaseRow(w.artifact)
	row.SourceLine = w.line
	row.TimestampUTC = w.timestamp
	row.SessionID = w.conversation
	row.ProjectPath = w.project
	row.TurnType = turn
	row.ToolName = toolName
	row.Text = timeline.Compact(text)
	w.emit(row)
}

// parseTranscript emits the rows of a transcript, preceded, for a server
// agent, by its `session synced:` row at the first line.
func (p *Parser) parseTranscript(artifact *input.Artifact, agentDirectory, conversation string,
	options parser.Options, emit parser.Emit) {
	sessions := agentSessions(artifact, agentDirectory)
	serverSide := !strings.HasPrefix(agentDirectory, localAgentPrefix)
	writer := &transcriptRows{parser: p, artifact: artifact, conversation: conversation, emit: emit}
	var problems parser.Problems
	for line, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		writer.line = line
		writer.timestamp = timeutil.FromValue(record.Get("captured_at"))
		writer.project = sessionProject(sessions, writer.timestamp)
		if serverSide {
			serverSide = false
			writer.put(timeline.TurnSystem, "session synced: server conversation "+conversation+
				" agent "+agentDirectory+" server "+server(artifact, agentDirectory)+
				" (from .letta/transcripts path, first captured_at)", "")
		}
		writer.record(record, options)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, conversation, ""))
}

// record emits the rows of one transcript line by its kind; a kind that
// is not a string matches none.
func (w *transcriptRows) record(record pyjson.Value, options parser.Options) {
	text := parser.Str(record.Get("text"))
	switch kind := record.Get("kind").Text(); kind {
	case "user", "assistant":
		w.put(kind, text, "")
	case "reasoning":
		if options.IncludeThinking {
			w.put(timeline.TurnThinking, text, "")
		}
	case "error":
		w.put(timeline.TurnSystem, "error: "+text, "")
	case "tool_call":
		w.toolCall(record)
	}
}

// toolCall emits a tool call line's call row and, when it holds a result,
// its result row.
func (w *transcriptRows) toolCall(record pyjson.Value) {
	name := parser.Str(record.Get("name"))
	w.put(timeline.TurnToolUse, argumentsText(record.Get("argsText")), name)
	if record.Has("resultText") || record.Has("resultOk") {
		result := parser.Str(record.Get("resultText"))
		if succeeded, isBool := record.Get("resultOk").Bool(); isBool && !succeeded {
			result = "[error] " + result
		}
		w.put(timeline.TurnToolResult, result, name)
	}
}
