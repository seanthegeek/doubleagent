package letta

import (
	"io/fs"
	"sort"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
// line is later, that of the latest line before it. The lines are sorted
// by time, so it is a binary search; work counts its comparisons.
func sessionProject(sessions []projectTime, timestamp string, work parserkit.Work) string {
	if timestamp == "" || len(sessions) == 0 {
		return ""
	}
	next := sort.Search(len(sessions), func(i int) bool {
		work.Add("session comparisons", 1)
		return sessions[i].timestamp >= timestamp
	})
	return sessions[min(next, len(sessions)-1)].project
}

// fileState names a side file as read: its path, modification time in
// nanoseconds and size, the key of what is read from it once.
type fileState struct {
	path        string
	mtime, size int64
}

// homeSideFile returns where the home holds rel, a file beside the one
// parsed, when neither it nor .letta is a symbolic link and it is a regular
// file.
func homeSideFile(artifact *input.Artifact, rel string) (string, bool) {
	if artifact.Home == nil {
		return "", false
	}
	path, found := artifact.Home.PathOf(rel)
	if !found {
		return "", false
	}
	info, err := osfile.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return "", false
	}
	letta, found := artifact.Home.PathOf(".letta")
	if !found {
		return "", false
	}
	if info, err := osfile.Lstat(letta); err == nil && info.Mode()&fs.ModeSymlink != 0 {
		return "", false
	}
	return path, true
}

// sideFileState returns the state of the home's side file rel, false when
// homeSideFile refuses it.
func sideFileState(artifact *input.Artifact, rel string) (fileState, bool) {
	path, ok := homeSideFile(artifact, rel)
	if !ok {
		return fileState{}, false
	}
	info, err := osfile.Lstat(path)
	if err != nil {
		return fileState{}, false
	}
	return fileState{path: path, mtime: info.ModTime().UnixNano(), size: info.Size()}, true
}

// readSessions returns the sessions.jsonl lines at path that name a
// project and a time, by agent directory, each agent's sorted by time then
// project. The file's problems are its own parse's to report.
func readSessions(path string) map[string][]projectTime {
	byAgent := map[string][]projectTime{}
	var ignored parserkit.Problems
	for _, record := range parserkit.JSONLines(path, &ignored) {
		if record.Get("project").IsEmpty() {
			continue
		}
		if timestamp := timeutil.FromValue(record.Get("timestamp")); timestamp != "" {
			agent := sanitize(parserkit.Text(record.Get("agent_id")))
			byAgent[agent] = append(byAgent[agent], projectTime{timestamp, record.Get("project").Text()})
		}
	}
	for _, sessions := range byAgent {
		sort.Slice(sessions, func(i, j int) bool {
			if sessions[i].timestamp != sessions[j].timestamp {
				return sessions[i].timestamp < sessions[j].timestamp
			}
			return sessions[i].project < sessions[j].project
		})
	}
	return byAgent
}

// agentSessions returns (time, project) of the agent's sessions.jsonl lines
// that name a project and a time, sorted by time then project. The file is
// read once per state for every agent, since each transcript looks itself
// up in it; work counts the reads.
func (p *Parser) agentSessions(artifact *input.Artifact, agentDirectory string) []projectTime {
	state, ok := sideFileState(artifact, sessionsRel)
	if !ok {
		return nil
	}
	byAgent, held := p.sessions[state]
	if !held {
		p.Work.Add("sessions.jsonl reads", 1)
		byAgent = readSessions(state.path)
		p.sessions[state] = byAgent
	}
	return byAgent[agentDirectory]
}

// readSettings returns a Letta JSON file as an object, or false when it
// cannot be read (its unreadable problem added) or is not an object (no
// problem added).
func readSettings(artifact *input.Artifact, path string, problems *parserkit.Problems) (jsonvalue.Value, bool) {
	content, ok := parserkit.FileBytes(path, problems)
	if !ok {
		return jsonvalue.NewNull(), false
	}
	document, err := parserkit.DecodeJSONBytes(artifact, content)
	if err != nil || !document.IsObject() {
		return jsonvalue.NewNull(), false
	}
	return document, true
}

// readServers returns the server of each agent directory the settings file
// at path names, from the first agents[] entry naming it.
func readServers(artifact *input.Artifact, path string) map[string]string {
	byAgent := map[string]string{}
	var ignored parserkit.Problems
	document, _ := readSettings(artifact, path, &ignored)
	for _, entry := range document.Get("agents").Items() {
		if !entry.IsObject() {
			continue
		}
		agent := sanitize(parserkit.Text(entry.Get("agentId")))
		if _, seen := byAgent[agent]; seen {
			continue
		}
		byAgent[agent] = lettaCloud
		if base := parserkit.Text(entry.Get("baseUrl")); base != "" {
			byAgent[agent] = base
		}
	}
	return byAgent
}

// server returns the server of a transcript's agent: the baseUrl of its
// first agents[] entry in ~/.letta/settings.json, api.letta.com when the
// entry has none, "-" when there is
// no entry or no file. It is a pre-scan: the settings file reports its own
// problems when it is parsed itself. The file is read once per state for
// every agent; work counts the reads.
func (p *Parser) server(artifact *input.Artifact, agentDirectory string) string {
	state, ok := sideFileState(artifact, settingsRel)
	if !ok {
		return "-"
	}
	byAgent, held := p.servers[state]
	if !held {
		p.Work.Add("settings.json reads", 1)
		// settings.json is parsed as its own artifact, whose parse reports
		// its repeated member names.
		byAgent = readServers(nil, state.path)
		p.servers[state] = byAgent
	}
	if found, ok := byAgent[agentDirectory]; ok {
		return found
	}
	return "-"
}

// transcriptRows writes the rows of one transcript.
type transcriptRows struct {
	parser       *Parser
	artifact     *input.Artifact
	conversation string
	line         int
	timestamp    string
	project      string
	emit         parserkit.Emit
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

// toolCall emits a tool call line's call row and, when it holds a result,
// its result row.
func (w *transcriptRows) toolCall(record jsonvalue.Value) {
	name := parserkit.Text(record.Get("name"))
	w.put(timeline.TurnToolUse, argumentsText(w.artifact, record.Get("argsText")), name)
	if record.Has("resultText") || record.Has("resultOk") {
		result := parserkit.Text(record.Get("resultText"))
		if succeeded, isBool := record.Get("resultOk").Bool(); isBool && !succeeded {
			result = "[error] " + result
		}
		w.put(timeline.TurnToolResult, result, name)
	}
}

// record emits the rows of one transcript line by its kind; a kind that
// is not a string matches none.
func (w *transcriptRows) record(record jsonvalue.Value, options parserkit.Options) {
	text := parserkit.Text(record.Get("text"))
	switch kind := record.Get("kind").StringValue(); kind {
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

// parseTranscript emits the rows of a transcript, preceded, for a server
// agent, by its `session synced:` row at the first line. The server comes
// from settings.json, one value every transcript of the agent repeats, so
// it is cut as a carried value, with its problem at that line.
func (p *Parser) parseTranscript(artifact *input.Artifact, agentDirectory, conversation string,
	options parserkit.Options, emit parserkit.Emit) {
	sessions := p.agentSessions(artifact, agentDirectory)
	serverSide := !p.localAgent.MatchString(agentDirectory)
	writer := &transcriptRows{parser: p, artifact: artifact, conversation: conversation, emit: emit}
	carried := parserkit.NewCarried(artifact)
	var problems parserkit.Problems
	for line, record := range parserkit.JSONLines(artifact.DiskPath, &problems) {
		writer.line = line
		writer.timestamp = timeutil.FromValue(record.Get("captured_at"))
		writer.project = sessionProject(sessions, writer.timestamp, p.Work)
		if serverSide {
			serverSide = false
			writer.put(timeline.TurnSystem, "session synced: server conversation "+conversation+
				" agent "+agentDirectory+" server "+carried.Cut("server", p.server(artifact, agentDirectory))+
				" (from .letta/transcripts path, first captured_at)", "")
			carried.Report(line, &problems)
		}
		writer.record(record, options)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, conversation, ""))
}
