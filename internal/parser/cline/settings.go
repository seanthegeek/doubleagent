package cline

import (
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// settings emits the "permission setting:" rows of one standing settings
// file, all at its modification time.
type settings struct {
	owner    *Parser
	artifact *input.Artifact
	time     string
	emit     parser.Emit
}

// newSettings returns the writer of the setting rows of a file.
func (p *Parser) newSettings(artifact *input.Artifact, emit parser.Emit) *settings {
	return &settings{owner: p, artifact: artifact, time: parser.FileMtime(artifact), emit: emit}
}

// row returns a system row at line 1 and the file's time.
func (s *settings) row(text string) timeline.Row {
	row := s.owner.SystemRow(s.artifact, text)
	row.SourceLine = 1
	row.TimestampUTC = s.time
	return row
}

// setting emits "permission setting: <key>=<value> (from <rel>, file
// mtime)".
func (s *settings) setting(key string, value jsonvalue.Value) {
	s.emit(s.row(timeline.Compact("permission setting: " + key + "=" + parser.SettingValue(value) +
		" (from " + s.artifact.Rel + ", file mtime)")))
}

// list emits a setting row per entry of a list value.
func (s *settings) list(key string, value jsonvalue.Value) {
	for _, entry := range value.Items() {
		s.setting(key, entry)
	}
}

// jsonObject returns the file as a JSON object, or false after emitting
// the row that says why it is not one.
func (p *Parser) jsonObject(artifact *input.Artifact, what string, emit parser.Emit) (jsonvalue.Value, bool) {
	var problems parser.Problems
	document, _ := parser.LoadJSON(artifact.DiskPath, &problems)
	if document.IsObject() {
		return document, true
	}
	if len(problems) == 0 {
		problems.Add(0, what+" is not a JSON object")
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
	return jsonvalue.NewNull(), false
}

// globalState emits the standing settings and Cline Cloud rows of
// globalState.json: autoApprovalSettings and mode as permission settings,
// cloudTaskTarget as one row, and an origin and a sync row per
// cloudSessionStatuses entry. No other key is read. A cloudSessionStatuses
// that is not an object adds a problem.
func (p *Parser) globalState(artifact *input.Artifact, emit parser.Emit) {
	document, ok := p.jsonObject(artifact, "globalState.json", emit)
	if !ok {
		return
	}
	writer := p.newSettings(artifact, emit)
	auto := document.Get("autoApprovalSettings")
	for _, key := range []string{"enabled", "maxRequests"} {
		if value, found := auto.Lookup(key); found {
			writer.setting("autoApprovalSettings."+key, value)
		}
	}
	writer.list("autoApprovalSettings.favorites", auto.Get("favorites"))
	for _, member := range auto.Get("actions").Members() {
		writer.setting("autoApprovalSettings.actions."+member.Key, member.Value)
	}
	if mode := document.Get("mode"); parser.Present(mode) {
		writer.setting("mode", mode)
	}
	if target := document.Get("cloudTaskTarget"); target.IsObject() {
		emit(writer.row("cloud task target: target=" + parser.SettingOrDash(target.Get("target")) + " repository_id=" +
			parser.SettingOrDash(target.Get("repositoryId")) + " branch=" + parser.SettingOrDash(target.Get("branch")) +
			" (from " + artifact.Rel + ", file mtime)"))
	}
	source := artifact.Rel + " cloudSessionStatuses, observedAt"
	var problems parser.Problems
	for _, member := range parser.ObjectMember(document.Get("cloudSessionStatuses"), "cloudSessionStatuses", 0, &problems) {
		status := member.Value
		p.cloudRows(artifact, member.Key, timeutil.FromValue(status.Get("observedAt")),
			"status "+parser.SettingOrDash(status.Get("status")), source, 0, emit)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// cloudRows emits the "session origin:" and "session synced:" rows of a
// Cline Cloud session.
func (p *Parser) cloudRows(artifact *input.Artifact, sessionID, timestamp, detail, source string,
	line int, emit parser.Emit) {
	row := p.SystemRow(artifact, "session origin: remote cline-cloud session "+sessionID+" "+detail+
		" (from "+source+")")
	row.TimestampUTC = timestamp
	row.SessionID = sessionID
	row.SourceLine = line
	emit(row)
	row.Text = "session synced: cline-cloud holds session " + sessionID + ", no local transcript (from " +
		source + ")"
	emit(row)
}

// pendingStarts emits a pair of Cline Cloud rows per record of a
// pending-start file, {sessionId, account, endpoint}, at the file's
// modification time, the endpoint through parser.URLText.
func (p *Parser) pendingStarts(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	document, _ := parser.LoadJSON(artifact.DiskPath, &problems)
	if !document.IsArray() {
		if len(problems) == 0 {
			problems.Add(0, "cloud pending-start file is not a JSON array")
		}
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	timestamp := parser.FileMtime(artifact)
	for i, record := range document.Items() {
		position := i + 1
		if !record.Get("sessionId").IsString() {
			problems.Add(position, "pending-start record "+strconv.Itoa(position)+" has no sessionId")
			continue
		}
		detail := "sandbox start pending account=" + parser.SettingOrDash(record.Get("account")) + " endpoint=" +
			parser.URLText(artifact, parser.SettingOrDash(record.Get("endpoint")))
		p.cloudRows(artifact, record.Get("sessionId").StringValue(), timestamp, detail,
			artifact.Rel+", file mtime", position, emit)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// globalSettings emits a permission setting row per tool setting of
// settings/global-settings.json.
func (p *Parser) globalSettings(artifact *input.Artifact, emit parser.Emit) {
	document, ok := p.jsonObject(artifact, "global-settings.json", emit)
	if !ok {
		return
	}
	writer := p.newSettings(artifact, emit)
	for _, key := range []string{"toolAutoApprove", "planActMode"} {
		if value := document.Get(key); parser.Present(value) {
			writer.setting(key, value)
		}
	}
	for _, key := range []string{"disabledTools", "disabledPlugins", "disabledAgentPlugins"} {
		writer.list(key, document.Get(key))
	}
	for _, member := range document.Get("tools").Members() {
		if value, found := member.Value.Lookup("enabled"); found {
			writer.setting("tools."+member.Key+".enabled", value)
		}
	}
}

// mcpSettings emits each MCP server's autoApprove entries and disabled
// flag as settings. The server's command, env, headers and OAuth state are
// never read.
func (p *Parser) mcpSettings(artifact *input.Artifact, emit parser.Emit) {
	document, ok := p.jsonObject(artifact, "cline_mcp_settings.json", emit)
	if !ok {
		return
	}
	writer := p.newSettings(artifact, emit)
	for _, member := range document.Get("mcpServers").Members() {
		writer.list("mcpServers."+member.Key+".autoApprove", member.Value.Get("autoApprove"))
		if disabled := member.Value.Get("disabled"); disabled.IsBool() {
			writer.setting("mcpServers."+member.Key+".disabled", disabled)
		}
	}
}
