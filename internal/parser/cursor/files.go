package cursor

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/reason"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// transcript emits the rows of an agent transcript: prompts at their time
// tag, replies and tool calls without a time. A message that is not a JSON
// object, or content that is neither text nor a JSON array, is a problem.
func (p *Parser) transcript(artifact *input.Artifact, emit parser.Emit) {
	sessionID := transcriptPattern.FindStringSubmatch(artifact.Rel)[1]
	index := p.homeIndex(artifact)
	found := index.projects[sessionID]
	row := func(turnType, text, when string, line int) timeline.Row {
		made := p.row(artifact, turnType, text, when, int64(line))
		made.SessionID, made.ProjectPath, made.GitBranch = sessionID, found.path, found.branch
		index.member(sessionID, &made, timeline.TakeProject|timeline.TakeBranch)
		return made
	}
	var problems parser.Problems
	for number, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		role := record.Get("role")
		if !parser.IsText(role, "user") && !parser.IsText(role, "assistant") {
			continue
		}
		message := record.Get("message")
		parser.ObjectMember(message, "message", number, &problems)
		content := message.Get("content")
		if content.IsString() {
			content = jsonvalue.NewArray(jsonvalue.NewObject(jsonvalue.Member{Key: "type", Value: jsonvalue.NewString("text")},
				jsonvalue.Member{Key: "text", Value: content}))
		}
		for _, block := range parser.EntriesOfKind(content, "message.content", jsonvalue.Object, number, &problems) {
			switch {
			case parser.IsText(block.Get("type"), "text") && block.Get("text").IsString():
				text := block.Get("text").StringValue()
				if role.StringValue() == "user" {
					query, when := unwrapQuery(text)
					if query = timeline.Compact(query); query != "" {
						emit(row(timeline.TurnUser, query, when, number))
					}
				} else if text = timeline.Compact(text); text != "" {
					emit(row(timeline.TurnAssistant, text, "", number))
				}
			case parser.IsText(block.Get("type"), "tool_use"):
				made := row(timeline.TurnToolUse, parser.JSONText(block.Get("input")), "", number)
				made.ToolName = parser.Text(block.Get("name"))
				emit(made)
			}
		}
	}
	for _, made := range p.ErrorRows(artifact, problems, sessionID, found.path) {
		index.member(sessionID, &made, timeline.TakeProject)
		emit(made)
	}
}

// readDocument reads a JSON settings or attempt file, emitting the row
// that says why it cannot be used and returning false. A file that cannot
// be read gives its problem with the project.
func (p *Parser) readDocument(artifact *input.Artifact, project string, emit parser.Emit) (jsonvalue.Value, bool) {
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
		return jsonvalue.Value{}, false
	}
	document, err := jsonvalue.DecodeString(textlines.ValidUTF8(content))
	switch {
	case err != nil:
		emit(p.row(artifact, timeline.TurnSystem, "parser: unreadable "+artifact.Rel+": "+reason.For(err), "", 0))
		return jsonvalue.Value{}, false
	case !document.IsObject():
		emit(p.row(artifact, timeline.TurnSystem, "parser: "+artifact.Rel+" is not a JSON object", "", 0))
		return jsonvalue.Value{}, false
	}
	return document, true
}

// attempt emits the "mcp oauth attempt:" row of an MCP OAuth attempt file,
// at createdAtMs, else the file's modification time, saying which. The
// client id is an identifier, not a secret.
func (p *Parser) attempt(artifact *input.Artifact, emit parser.Emit) {
	document, ok := p.readDocument(artifact, "", emit)
	if !ok {
		return
	}
	source := "mcp-oauth-attempts/" + attemptPattern.FindStringSubmatch(artifact.Rel)[1]
	created := timeutil.FromValue(document.Get("createdAtMs"))
	if !parser.Present(document.Get("createdAtMs")) {
		created, source = parser.FileMtime(artifact), source+", file mtime"
	}
	text := "mcp oauth attempt: server=" + fieldOf(document.Get("serverUrl")) + " client_id=" +
		fieldOf(document.Get("clientInformation").Get("client_id")) + " workspace=" +
		fieldOf(document.Get("owner").Get("workspaceId")) + " (from " + source + ")"
	emit(p.systemAt(artifact, text, created, 1))
}

// settingsFile emits a "permission setting:" row per permission setting
// of .cursor/cli-config.json or .cursor/permissions.json, then the
// problems of cli-config.json members of the wrong kind.
func (p *Parser) settingsFile(artifact *input.Artifact, emit parser.Emit) {
	project := ""
	if artifact.Home != nil && artifact.Home.Project {
		project = artifact.Home.Original
	}
	document, ok := p.readDocument(artifact, project, emit)
	if !ok {
		return
	}
	var problems parser.Problems
	settings := permissionsSettings(document)
	if cliConfigPattern.MatchString(artifact.Rel) {
		settings = cliConfigSettings(document, &problems)
	}
	mtime := parser.FileMtime(artifact)
	for _, setting := range settings {
		row := p.systemAt(artifact, timeline.Compact("permission setting: "+setting+" (from "+artifact.Rel+
			", file mtime)"), mtime, 1)
		row.ProjectPath = project
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
}

// listSettings returns "<label>=<value>" per item of an array.
func listSettings(label string, list jsonvalue.Value) []string {
	var out []string
	for _, entry := range list.Items() {
		out = append(out, label+"="+parser.SettingValue(entry))
	}
	return out
}

// cliConfigSettings returns "<key>=<value>" per setting of
// .cursor/cli-config.json that governs permissions. The CLI writes the
// file, so a permissions, permissions.allow or permissions.deny of the
// wrong kind adds a whole-file problem.
func cliConfigSettings(document jsonvalue.Value, problems *parser.Problems) []string {
	var out []string
	if value, ok := document.Lookup("approvalMode"); ok {
		out = append(out, "approvalMode="+parser.SettingValue(value))
	}
	permissions := document.Get("permissions")
	parser.ObjectMember(permissions, "permissions", 0, problems)
	for _, key := range []string{"allow", "deny"} {
		label := "permissions." + key
		parser.ArrayMember(permissions.Get(key), label, 0, problems)
		out = append(out, listSettings(label, permissions.Get(key))...)
	}
	if value, ok := document.Get("sandbox").Lookup("mode"); ok {
		out = append(out, "sandbox.mode="+parser.SettingValue(value))
	}
	return out
}

// permissionsSettings returns "<key>=<value>" per entry of a
// permissions.json.
func permissionsSettings(document jsonvalue.Value) []string {
	allowlists := document.Get("allowlists")
	out := listSettings("allowlists.mcp", allowlists.Get("mcp"))
	out = append(out, listSettings("allowlists.terminal", allowlists.Get("terminal"))...)
	if value, ok := document.Lookup("approvalMode"); ok {
		out = append(out, "approvalMode="+parser.SettingValue(value))
	}
	out = append(out, listSettings("allowInstructions", document.Get("allowInstructions"))...)
	return append(out, listSettings("blockInstructions", document.Get("blockInstructions"))...)
}
