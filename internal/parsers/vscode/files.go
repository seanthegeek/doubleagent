package vscode

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// cloudSessions emits one sync row per cloudSessions.json entry at the
// file's modification time; a file that cannot be read or is not a JSON
// object gives one problem row.
func (p *Parser) cloudSessions(artifact *input.Artifact, emit parserkit.Emit) {
	timestamp := parserkit.FileMtime(artifact)
	var problems parserkit.Problems
	data, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	document, err := parserkit.DecodeJSON(artifact, textlines.ValidUTF8(data))
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
		text := "session synced: remote session " + parserkit.TextOr(entry.Get("cloudSessionId"), "-") +
			" task " + parserkit.TextOr(entry.Get("cloudTaskId"), "-") +
			" (from User/globalStorage/github.copilot-chat/cloudSessions.json, file mtime)"
		row := p.systemRow(artifact, member.Key, timestamp, text)
		row.SourceLine = 1
		emit(row)
	}
}

// settingTexts returns the "permission setting:" texts of one
// settings.json key: one per entry of a non-empty object value
// ("<key>.<entry>=<value>"), else one ("<key>=<value>").
func settingTexts(key string, value jsonvalue.Value) []string {
	if !value.IsObject() || value.Len() == 0 {
		return []string{"permission setting: " + key + "=" + parserkit.SettingValue(value)}
	}
	texts := make([]string, 0, value.Len())
	for _, member := range value.Members() {
		texts = append(texts, "permission setting: "+key+"."+member.Key+"="+parserkit.SettingValue(member.Value))
	}
	return texts
}

// settings emits the standing permission and default model rows of a
// profile's settings.json at the file's modification time: a "permission
// setting:" row per auto-approve setting or entry and for the default
// permission level, and a "model setting:" row for the default model.
func (p *Parser) settings(artifact *input.Artifact, emit parserkit.Emit) {
	timestamp, source := parserkit.FileMtime(artifact), userRel(artifact.Rel)
	var problems parserkit.Problems
	data, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	document, err := parserkit.ParseJSONC(artifact, data)
	if err != nil || !document.IsObject() {
		why := ""
		if err != nil {
			why = " (" + err.Error() + ")"
		}
		emit(p.systemRow(artifact, "", "", "parser: "+source+" is not a JSON object"+why))
		return
	}
	carried := parserkit.NewCarried(artifact)
	for _, member := range document.Members() {
		var texts []string
		switch {
		case member.Key == defaultModelKey:
			texts = []string{"model setting: " + member.Key + "=" + parserkit.SettingValue(member.Value)}
		case settingsPermission.MatchString(member.Key):
			texts = settingTexts(carried.Cut("settings key", member.Key), member.Value)
		}
		for _, text := range texts {
			row := p.systemRow(artifact, "", timestamp, text+" (from "+source+", file mtime)")
			row.SourceLine = 1
			emit(row)
		}
	}
	carried.Report(1, &problems)
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// editing emits an "edit state:" row per file of a chat editing state's
// latest snapshot, at the file's modification time, with the folder's name
// as the session id and the workspace's folder as the project. A
// recentSnapshot or its entries of the wrong kind adds a problem.
func (p *Parser) editing(artifact *input.Artifact, sessionID string, emit parserkit.Emit) {
	var problems parserkit.Problems
	project, _ := p.workspaceJSON(artifact, 2, &problems)
	document := jsonvalue.NewNull()
	if data, ok := parserkit.FileBytes(artifact.DiskPath, &problems); ok {
		decoded, err := parserkit.DecodeJSON(artifact, textlines.ValidUTF8(data))
		switch {
		case err != nil:
			problems.Add(0, "file is not valid JSON ("+err.Error()+")")
		case !decoded.IsObject():
			problems.Add(0, "state.json is not a JSON object")
		}
		document = decoded
	}
	timestamp := parserkit.FileMtime(artifact)
	snapshot := document.Get("recentSnapshot")
	if !snapshot.IsObject() && !snapshot.IsNull() {
		problems.Add(0, parserkit.WrongKind("recentSnapshot", jsonvalue.Object))
	}
	for _, entry := range parserkit.EntriesOfKind(snapshot.Get("entries"), "recentSnapshot.entries", jsonvalue.Object, 0,
		&problems) {
		row := p.systemRow(artifact, sessionID, timestamp, editStateText(entry))
		row.ProjectPath, row.SourceLine = project, 1
		emit(row)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, project))
}
