package vscode

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// cloudSessions emits one sync row per cloudSessions.json entry at the
// file's modification time; a file that cannot be read or is not a JSON
// object gives one problem row.
func (p *Parser) cloudSessions(artifact *input.Artifact, emit parser.Emit) {
	timestamp := parser.FileMtime(artifact)
	var problems parser.Problems
	data, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	document, err := jsonvalue.DecodeString(textlines.ValidUTF8(data))
	if err != nil || !document.IsObject() {
		why := ""
		if err != nil {
			why = " (" + err.Error() + ")"
		}
		row := p.systemRow(artifact, "", timestamp, "parser: cloudSessions.json is not a JSON object"+why)
		row.SourceLine = 1
		emit(row)
		return
	}
	for _, member := range document.Members() {
		entry := member.Value
		text := "session synced: remote session " + orDash(parser.Str(entry.Get("cloudSessionId"))) +
			" task " + orDash(parser.Str(entry.Get("cloudTaskId"))) +
			" (from User/globalStorage/github.copilot-chat/cloudSessions.json, file mtime)"
		row := p.systemRow(artifact, member.Key, timestamp, text)
		row.SourceLine = 1
		emit(row)
	}
}

// settings emits the standing permission and default model rows of a
// profile's settings.json at the file's modification time: a "permission
// setting:" row per auto-approve setting or entry and for the default
// permission level, and a "model setting:" row for the default model.
func (p *Parser) settings(artifact *input.Artifact, emit parser.Emit) {
	timestamp, source := parser.FileMtime(artifact), userRel(artifact.Rel)
	var problems parser.Problems
	data, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	document, err := parser.ParseJSONC(data)
	if err != nil || !document.IsObject() {
		why := ""
		if err != nil {
			why = " (" + err.Error() + ")"
		}
		emit(p.systemRow(artifact, "", "", "parser: "+source+" is not a JSON object"+why))
		return
	}
	for _, member := range document.Members() {
		var texts []string
		switch {
		case member.Key == defaultModelKey:
			texts = []string{"model setting: " + member.Key + "=" + parser.SettingValue(member.Value)}
		case settingsPermission.MatchString(member.Key):
			texts = settingTexts(member.Key, member.Value)
		}
		for _, text := range texts {
			row := p.systemRow(artifact, "", timestamp, text+" (from "+source+", file mtime)")
			row.SourceLine = 1
			emit(row)
		}
	}
}

// settingTexts returns the "permission setting:" texts of one
// settings.json key: one per entry of a non-empty object value
// ("<key>.<entry>=<value>"), else one ("<key>=<value>").
func settingTexts(key string, value jsonvalue.Value) []string {
	if !value.IsObject() || value.Len() == 0 {
		return []string{"permission setting: " + key + "=" + parser.SettingValue(value)}
	}
	texts := make([]string, 0, value.Len())
	for _, member := range value.Members() {
		texts = append(texts, "permission setting: "+key+"."+member.Key+"="+parser.SettingValue(member.Value))
	}
	return texts
}

// editing emits an "edit state:" row per file of a chat editing state's
// latest snapshot, at the file's modification time, with the folder's name
// as the session id and the workspace's folder as the project.
func (p *Parser) editing(artifact *input.Artifact, sessionID string, emit parser.Emit) {
	var problems parser.Problems
	project, _ := workspaceJSON(artifact, 2, &problems)
	document := jsonvalue.NewNull()
	if data, ok := parser.FileBytes(artifact.DiskPath, &problems); ok {
		decoded, err := jsonvalue.DecodeString(textlines.ValidUTF8(data))
		switch {
		case err != nil:
			problems.Add(0, "file is not valid JSON ("+err.Error()+")")
		case !decoded.IsObject():
			problems.Add(0, "state.json is not a JSON object")
		}
		document = decoded
	}
	timestamp := parser.FileMtime(artifact)
	for _, entry := range document.Get("recentSnapshot").Get("entries").Items() {
		if entry.IsObject() {
			row := p.systemRow(artifact, sessionID, timestamp, editStateText(entry))
			row.ProjectPath, row.SourceLine = project, 1
			emit(row)
		}
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, project))
}
