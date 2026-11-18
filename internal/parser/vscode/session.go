package vscode

import (
	"path"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// sessionRows builds the rows of one chat session: each takes the
// session's id and project.
type sessionRows struct {
	owner     *Parser
	artifact  *input.Artifact
	sessionID string
	project   string
	emit      parser.Emit
	tracked   parser.Tracker // one file is one session, so the keys need no session id
}

// row returns a row of the session at a time and source line.
func (s *sessionRows) row(turnType, text string, timestamp jsonvalue.Value, line int, model string) timeline.Row {
	row := s.owner.BaseRow(s.artifact)
	row.SessionID = s.sessionID
	row.ProjectPath = s.project
	row.TimestampUTC = timeutil.FromValue(timestamp)
	row.SourceLine = line
	row.Model = model
	row.TurnType = turnType
	row.Text = timeline.Compact(text)
	return row
}

// session emits the rows of a chat session file: a .jsonl mutation log
// replayed, or a .json state. A state that is not an object, or requests
// that is not an array, adds a problem.
func (p *Parser) session(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	lines := requestLines{}
	var problems parser.Problems
	state := jsonvalue.NewNull()
	if strings.HasSuffix(artifact.Rel, ".jsonl") {
		state = replay(artifact.DiskPath, lines, &problems, p.Work)
	} else if data, ok := parser.FileBytes(artifact.DiskPath, &problems); ok {
		decoded, err := jsonvalue.DecodeString(textlines.ValidUTF8(data))
		if err != nil {
			problems.Add(0, "file is not valid JSON ("+err.Error()+")")
		}
		state = decoded
	}
	if !state.IsObject() {
		if !state.IsNull() {
			problems.Add(lines[sessionStart], parser.WrongKind(path.Base(artifact.Rel), jsonvalue.Object))
		}
		state = jsonvalue.NewObject()
	}
	sessionID := parser.Text(state.Get("sessionId"))
	if sessionID == "" {
		sessionID = path.Base(artifact.Rel)
		if dot := strings.LastIndex(sessionID, "."); dot >= 0 {
			sessionID = sessionID[:dot]
		}
	}
	folder, workspaceFile := p.workspaceFolder(artifact, &problems)
	project := uriToPath(parser.Text(state.Get("workingDirectory")))
	if project == "" {
		project = folder
	}
	rows := &sessionRows{owner: p, artifact: artifact, sessionID: sessionID, project: project, emit: emit,
		tracked: parser.Tracker{}}
	if state.Len() > 0 {
		emit(rows.row(timeline.TurnSystem, sessionStartText(state, workspaceFile), state.Get("creationDate"),
			lines[sessionStart], ""))
	}
	for i, request := range parser.ArrayMember(state.Get("requests"), "requests", lines[sessionStart], &problems) {
		if request.IsObject() {
			rows.request(request, lines[int64(i)], options)
		}
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, project))
}

// sessionStartText returns the text of a session's first row.
func sessionStartText(state jsonvalue.Value, workspaceFile string) string {
	parts := []string{"session start:"}
	for _, field := range []struct{ label, key string }{
		{"responder", "responderUsername"}, {"location", "initialLocation"}, {"title", "customTitle"},
	} {
		if value := state.Get(field.key); !value.IsEmpty() {
			parts = append(parts, field.label+"="+value.Text())
		}
	}
	if workspaceFile != "" {
		parts = append(parts, "workspace="+workspaceFile)
	}
	return strings.Join(parts, " ")
}

// workspaceFolder returns the folder and workspace file of the
// workspace.json beside a workspace's chatSessions directory, "" for each
// when the session is not under one.
func (p *Parser) workspaceFolder(artifact *input.Artifact, problems *parser.Problems) (string, string) {
	parts := strings.Split(artifact.Rel, "/")
	if len(parts) < 4 || parts[len(parts)-2] != "chatSessions" || parts[len(parts)-4] != "workspaceStorage" {
		return "", ""
	}
	return p.workspaceJSON(artifact, 1, problems)
}

// workspaceState names a workspace.json as read: its path, modification
// time in nanoseconds and size.
type workspaceState struct {
	path        string
	mtime, size int64
}

// workspaceFile is a workspace.json read once: its folder and workspace
// file, and the problems reading it added.
type workspaceFile struct {
	folder, workspace string
	problems          parser.Problems
}

// workspaceJSON returns the folder and workspace file, as paths, of the
// workspace.json in the directory levels above the artifact's; "" for each
// when there is none. One that cannot be read, does not decode or is not a
// JSON object adds a whole-file problem. The file is read once per state
// for the parser's life, since every chat session and state database of
// the workspace looks it up, and its problems are added for each lookup.
func (p *Parser) workspaceJSON(artifact *input.Artifact, levels int, problems *parser.Problems) (string, string) {
	file, ok := parser.Above(artifact, levels, "workspace.json")
	if !ok {
		return "", ""
	}
	info, err := osfile.Lstat(file)
	if err != nil {
		return "", ""
	}
	key := workspaceState{path: file, mtime: info.ModTime().UnixNano(), size: info.Size()}
	read, cached := p.workspaces[key]
	if !cached {
		p.Work.Add("workspace.json reads", 1)
		read = &workspaceFile{}
		read.folder, read.workspace = readWorkspace(file, &read.problems)
		if p.workspaces == nil {
			p.workspaces = map[workspaceState]*workspaceFile{}
		}
		p.workspaces[key] = read
	}
	*problems = append(*problems, read.problems...)
	return read.folder, read.workspace
}

// readWorkspace returns the folder and workspace file of the workspace.json
// at file, as workspaceJSON describes.
func readWorkspace(file string, problems *parser.Problems) (string, string) {
	if !parser.RegularFile(file) {
		return "", ""
	}
	content, ok := parser.FileBytes(file, problems)
	if !ok {
		return "", ""
	}
	data, err := jsonvalue.DecodeString(textlines.ValidUTF8(content))
	switch {
	case err != nil:
		problems.Add(0, "workspace.json not read ("+err.Error()+")")
		return "", ""
	case !data.IsObject():
		problems.Add(0, "workspace.json is not a JSON object")
		return "", ""
	}
	return uriToPath(parser.Text(data.Get("folder"))),
		uriToPath(parser.Text(data.Get("workspace")))
}

// change emits a system row when a tracked state of the session changes.
func (s *sessionRows) change(label, key, value string, timestamp jsonvalue.Value, line int, model string) {
	if text := s.tracked.Change(label, key, value); text != "" {
		s.emit(s.row(timeline.TurnSystem, text, timestamp, line, model))
	}
}

// modelText returns the model and its options, sorted by name, as the
// tracked model state.
func modelText(model string, config jsonvalue.Value) string {
	if model == "" || !config.IsObject() || config.Len() == 0 {
		return model
	}
	keys := config.Keys()
	slices.Sort(keys)
	pairs := make([]string, len(keys))
	for i, key := range keys {
		pairs[i] = key + "=" + parser.SettingValue(config.Get(key))
	}
	return model + " " + strings.Join(pairs, " ")
}

// shownSetting returns a value as a tracked setting, "" when it is false.
func shownSetting(value jsonvalue.Value) string {
	if value.IsEmpty() {
		return ""
	}
	return parser.SettingValue(value)
}
