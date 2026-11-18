package copilotcli

import (
	"cmp"
	"sort"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// readFlatYAML returns the top-level "key: value" scalars of a YAML file:
// null and ~ are empty and quoted values are unquoted. workspace.yaml holds
// only scalars, so a nested or multi-line value is skipped. This reads that
// one shape; it is not a YAML parser. A file that cannot be read gives
// nothing and its unreadable problem.
func readFlatYAML(path string, problems *parser.Problems) map[string]string {
	values := map[string]string{}
	data, ok := parser.FileBytes(path, problems)
	if !ok {
		return values
	}
	for _, line := range textlines.Split(textlines.ValidUTF8(data)) {
		if line == "" || strings.IndexByte(" \t#-", line[0]) >= 0 || !strings.Contains(line, ":") {
			continue
		}
		name, value, _ := strings.Cut(line, ":")
		name, value = strings.TrimRight(name, textlines.SpaceOrTab), strings.Trim(value, textlines.SpaceOrTab)
		switch {
		case value == "" || value == "null" || value == "~":
			values[name] = ""
		case strings.HasPrefix(value, `"`):
			if decoded, err := jsonvalue.DecodeString(value); err == nil {
				values[name] = decoded.Text()
			} else {
				values[name] = strings.Trim(value, `"`)
			}
		case len(value) > 1 && strings.HasPrefix(value, "'") && strings.HasSuffix(value, "'"):
			values[name] = strings.ReplaceAll(value[1:len(value)-1], "''", "'")
		default:
			values[name] = value
		}
	}
	return values
}

// readJSON returns the parsed file, or null when it cannot be read or
// parsed. A file that cannot be read adds its unreadable problem.
func readJSON(path string, problems *parser.Problems) jsonvalue.Value {
	data, ok := parser.FileBytes(path, problems)
	if !ok {
		return jsonvalue.NewNull()
	}
	value, err := jsonvalue.Decode(data)
	if err != nil {
		return jsonvalue.NewNull()
	}
	return value
}

// sidebarJoin returns session id -> working directory from the sidebar
// files of the Copilot home the artifact is in; empty when there are none.
// Links are not read.
func (p *Parser) sidebarJoin(artifact *input.Artifact) map[string]string {
	match := p.homePrefix.FindStringSubmatch(artifact.Rel)
	if match == nil {
		return nil
	}
	directory, ok := artifact.Home.PathOf(match[1] + "/sidebar-sessions-state")
	if !ok {
		return nil
	}
	if held, ok := p.sidebarCache[directory]; ok {
		return held
	}
	found := map[string]string{}
	p.sidebarCache[directory] = found
	listed, ok := parser.DirectoryAt(directory)
	if !ok {
		return found
	}
	entries, err := parser.ReadDir(directory)
	if err != nil {
		return found
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		path, named := listed.Child(name)
		if !named || !parser.RegularFile(path) {
			continue
		}
		// Each sidebar file is also an artifact parseSidebar reports.
		document := readJSON(path, &parser.Problems{})
		cwd, sessions := document.Get("cwd").Text(), document.Get("sessionIds")
		if cwd == "" || !sessions.IsArray() {
			continue
		}
		for _, session := range sessions.Items() {
			if _, set := found[session.Text()]; !set {
				found[session.Text()] = cwd
			}
		}
	}
	return found
}

// fallbackProject returns the session's working directory from its
// workspace.yaml, else from the sidebar files. The session's directory lies
// levels above the artifact's own directory.
func (p *Parser) fallbackProject(artifact *input.Artifact, levels int, sessionID string) string {
	path, found := parser.Above(artifact, levels, "workspace.yaml")
	if found && parser.RegularFile(path) {
		// The file is also an artifact parseWorkspace reads in full and reports.
		if cwd := readFlatYAML(path, &parser.Problems{})["cwd"]; cwd != "" {
			return cwd
		}
	}
	return p.sidebarJoin(artifact)[sessionID]
}

// parseWorkspace emits the rows of a session's workspace.yaml.
func (p *Parser) parseWorkspace(artifact *input.Artifact, sessionID string, emit parser.Emit) {
	var problems parser.Problems
	metadata := readFlatYAML(artifact.DiskPath, &problems)
	if len(problems) > 0 {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, ""))
		return
	}
	sessionID = cmp.Or(metadata["id"], sessionID)
	set := columns{timestamp: timeutil.FromText(metadata["created_at"]), sessionID: sessionID,
		project: cmp.Or(metadata["cwd"], p.sidebarJoin(artifact)[sessionID])}
	if metadata["name"] != "" {
		emit(p.row(artifact, 1, timeline.TurnSystem, "session name: "+metadata["name"], set))
	}
	emit(p.row(artifact, 1, timeline.TurnSystem, "session metadata: user_named="+
		cmp.Or(metadata["user_named"], "-")+" forks="+cmp.Or(metadata["fork_count"], "-")+" compactions="+
		cmp.Or(metadata["summary_count"], "-")+" client="+metadata["client_name"], set))
	// A session the app pre-created and never prompted (Copilot Desktop's pool
	// and probe sessions) has no events.jsonl; its cwd is the agent's, not a
	// project.
	events, found := parser.Beside(artifact, "events.jsonl")
	if _, err := osfile.Lstat(events); (!found || err != nil) && metadata["cwd"] != "" {
		emit(p.row(artifact, 1, timeline.TurnSystem, timeline.ScratchPrefix+" "+metadata["cwd"]+
			" (working directory of a session the app pre-created and never prompted: no events.jsonl)", set))
	}
	if metadata["mc_session_id"] != "" {
		set.timestamp = parser.FileMtime(artifact)
		emit(p.row(artifact, 1, timeline.TurnSystem, "session synced: remote session "+
			cmp.Or(metadata["mc_session_id"], "-")+" task "+cmp.Or(metadata["mc_task_id"], "-")+" last event "+
			cmp.Or(metadata["mc_last_event_id"], "-")+" remote_steerable="+cmp.Or(metadata["remote_steerable"], "-")+
			" (from workspace.yaml, file mtime)", set))
	}
}

// problemRows emits a "parser:" row per problem of a whole JSON file, at
// line 1 and a time, as the row of a file that is not a JSON object is.
func (p *Parser) problemRows(artifact *input.Artifact, problems parser.Problems, timestamp string,
	emit parser.Emit) {
	for _, problem := range problems {
		emit(p.problem(artifact, parser.ProblemPrefix+problem.Message, timestamp))
	}
}

// parseSnapshots emits the rows of a session's rewind snapshot index; a
// snapshots that is not a JSON array is a problem.
func (p *Parser) parseSnapshots(artifact *input.Artifact, sessionID string, emit parser.Emit) {
	var problems parser.Problems
	document := readJSON(artifact.DiskPath, &problems)
	if len(problems) > 0 {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, ""))
		return
	}
	if !document.IsObject() {
		emit(p.problem(artifact, "parser: rewind snapshot index is not a JSON object", ""))
		return
	}
	project := p.fallbackProject(artifact, 1, sessionID)
	var wrong parser.Problems
	for i, snapshot := range parser.ArrayMember(document.Get("snapshots"), "snapshots", 1, &wrong) {
		number := i + 1
		set := columns{timestamp: timeutil.FromValue(snapshot.Get("timestamp")), sessionID: sessionID,
			project: project}
		if message := snapshot.Get("userMessage"); parser.Present(message) {
			emit(p.row(artifact, number, timeline.TurnUser, message.Text(), set))
		}
		var paths []string
		for _, entry := range snapshot.Get("files").Members() {
			if path := entry.Value.Get("path"); !path.IsEmpty() {
				paths = append(paths, path.Text())
			}
		}
		line := "rewind snapshot: " + countText(snapshot.Get("fileCount")) + " files +" +
			countText(snapshot.Get("linesAdded")) + " -" + countText(snapshot.Get("linesRemoved"))
		if len(paths) > 0 {
			line += ": " + strings.Join(paths, ", ")
		}
		emit(p.row(artifact, number, timeline.TurnSystem, line, set))
	}
	p.problemRows(artifact, wrong, "", emit)
}

// countText returns a count as text, "0" when it is missing or empty.
func countText(value jsonvalue.Value) string {
	return cmp.Or(value.Text(), "0")
}

// parseHistory emits one user row per prompt of command-history-state.json;
// a commandHistory that is not a JSON array is a problem.
func (p *Parser) parseHistory(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	document := readJSON(artifact.DiskPath, &problems)
	if len(problems) > 0 {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	if !document.IsObject() {
		emit(p.problem(artifact, "parser: command history is not a JSON object", ""))
		return
	}
	var wrong parser.Problems
	for i, entry := range parser.ArrayMember(document.Get("commandHistory"), "commandHistory", 1, &wrong) {
		if entry.IsString() && strings.TrimSpace(entry.StringValue()) != "" {
			emit(p.row(artifact, i+1, timeline.TurnUser, entry.StringValue(), columns{}))
		}
	}
	p.problemRows(artifact, wrong, "", emit)
}

// settingRow returns the "permission setting:" row of one standing grant.
func (p *Parser) settingRow(artifact *input.Artifact, timestamp, key string, value jsonvalue.Value,
	project string) timeline.Row {
	return p.row(artifact, 1, timeline.TurnSystem, "permission setting: "+key+"="+
		parser.SettingValue(value)+" (from "+artifact.Rel+", file mtime)",
		columns{timestamp: timestamp, project: project})
}

// parsePermissions emits one row per standing grant of
// permissions-config.json; a locations, location or grant list of the
// wrong kind is a problem.
func (p *Parser) parsePermissions(artifact *input.Artifact, emit parser.Emit) {
	timestamp := parser.FileMtime(artifact)
	var problems parser.Problems
	document := readJSON(artifact.DiskPath, &problems)
	if len(problems) > 0 {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	if !document.IsObject() {
		emit(p.problem(artifact, "parser: permissions-config.json is not a JSON object", timestamp))
		return
	}
	var wrong parser.Problems
	for _, location := range parser.ObjectMember(document.Get("locations"), "locations", 1, &wrong) {
		member := "locations." + location.Key
		parser.ObjectMember(location.Value, member, 1, &wrong)
		for _, key := range []string{"tool_approvals", "allowed_directories"} {
			for _, item := range parser.ArrayMember(location.Value.Get(key), member+"."+key, 1, &wrong) {
				emit(p.settingRow(artifact, timestamp, member+"."+key, item, location.Key))
			}
		}
	}
	p.problemRows(artifact, wrong, timestamp, emit)
}

// parseSettings emits one row per allowed or denied URL of settings.json,
// which is JSON with comments; a list that is not a JSON array is a
// problem.
func (p *Parser) parseSettings(artifact *input.Artifact, emit parser.Emit) {
	timestamp := parser.FileMtime(artifact)
	var problems parser.Problems
	data, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	document, err := jsonvalue.DecodeString(parser.StripJSONC(textlines.ValidUTF8(data)))
	if err != nil || !document.IsObject() {
		emit(p.problem(artifact, "parser: settings.json is not a JSON object", timestamp))
		return
	}
	var wrong parser.Problems
	for _, key := range []string{"allowedUrls", "deniedUrls"} {
		for _, item := range parser.ArrayMember(document.Get(key), key, 1, &wrong) {
			emit(p.settingRow(artifact, timestamp, key, item, ""))
		}
	}
	p.problemRows(artifact, wrong, timestamp, emit)
}

// parseLog emits a "signed in:" row per sign-in line of a process log.
func (p *Parser) parseLog(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	data, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	lines := textlines.Split(textlines.ValidUTF8(data))
	sessions := map[string]bool{}
	for _, line := range lines {
		if match := p.workspaceLine.FindStringSubmatch(line); match != nil {
			sessions[match[1]] = true
		}
	}
	sessionID := ""
	if len(sessions) == 1 {
		for only := range sessions {
			sessionID = only
		}
	}
	for i, line := range lines {
		match := p.signedIn.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		timestamp := ""
		if stamp := p.logTime.FindStringSubmatch(line); stamp != nil {
			timestamp = timeutil.FromText(stamp[1])
		}
		emit(p.row(artifact, i+1, timeline.TurnSystem, "signed in: "+match[1]+" as "+match[2],
			columns{timestamp: timestamp, sessionID: sessionID}))
	}
}

// parseSidebar emits the problem row of a sidebar file that cannot be
// read or is not a JSON object, else nothing: a sidebar file gives no rows
// of its own, since sidebarJoin reads it as a join, and one it cannot read
// is a join lost.
func (p *Parser) parseSidebar(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	document := readJSON(artifact.DiskPath, &problems)
	if len(problems) > 0 {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	if !document.IsObject() {
		emit(p.problem(artifact, "parser: sidebar file is not a JSON object", ""))
	}
}
