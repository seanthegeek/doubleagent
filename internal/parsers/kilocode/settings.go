package kilocode

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// settingRow returns a system row at line 1 and timestamp for a standing
// setting, its text compacted.
func (p *Parser) settingRow(artifact *input.Artifact, timestamp, text string) timeline.Row {
	row := p.SystemRow(artifact, timeline.Compact(text))
	row.SourceLine = 1
	row.TimestampUTC = timestamp
	return row
}

// fileProblem returns the parser: row of a file that is not what the parser
// reads, at line 1 with the session id and no time.
func (p *Parser) fileProblem(artifact *input.Artifact, sessionID, message string) timeline.Row {
	var problems parserkit.Problems
	problems.Add(1, message)
	return p.ErrorRows(artifact, problems, sessionID, "")[0]
}

// config emits one "permission setting:" row per rule of a global config
// file's permission key, at the file's modification time:
// permission.<permission>.<pattern>=<action>, permission.<permission>=
// <action> for a rule given as one action, or permission=<action>. A file
// that does not decode as JSON with comments is one `parser:` row.
func (p *Parser) config(artifact *input.Artifact, emit parserkit.Emit) {
	timestamp := parserkit.FileMtime(artifact)
	var problems parserkit.Problems
	raw, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	document, err := parserkit.ParseJSONC(artifact, raw)
	switch {
	case err != nil:
		emit(p.fileProblem(artifact, "", artifact.Rel+" could not be decoded ("+err.Error()+")"))
		return
	case !document.IsObject():
		emit(p.fileProblem(artifact, "", artifact.Rel+" is not a JSON object"))
		return
	case !document.Has("permission"):
		return
	}
	source := "(from " + artifact.Rel + ", file mtime)"
	permission := document.Get("permission")
	if !permission.IsObject() {
		emit(p.settingRow(artifact, timestamp,
			"permission setting: permission="+parserkit.SettingValue(permission)+" "+source))
		return
	}
	carried := parserkit.NewCarried(artifact)
	for _, member := range permission.Members() {
		if !member.Value.IsObject() {
			emit(p.settingRow(artifact, timestamp, "permission setting: permission."+member.Key+"="+
				parserkit.SettingValue(member.Value)+" "+source))
			continue
		}
		// Each action's row repeats its permission's key.
		key := carried.Cut("permission key", member.Key)
		for _, action := range member.Value.Members() {
			emit(p.settingRow(artifact, timestamp, "permission setting: permission."+key+"."+
				action.Key+"="+parserkit.SettingValue(action.Value)+" "+source))
		}
	}
	carried.Report(1, &problems)
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// sandbox emits the "permission setting: sandbox.<key>=<value>" rows of a
// sandbox file at its modification time, with no session, since the file
// names are hashes: a row per key of a policy file, one sandbox.enabled row
// for a preference file (true or false). A file that is not the expected
// JSON is one `parser:` row.
func (p *Parser) sandbox(artifact *input.Artifact, emit parserkit.Emit) {
	timestamp := parserkit.FileMtime(artifact)
	source := "(from " + artifact.Rel + ", file mtime)"
	var problems parserkit.Problems
	document, _ := parserkit.LoadJSON(artifact, artifact.DiskPath, &problems)
	if problems.Damaged() {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	switch preference := sandboxPreferencePattern.MatchString(artifact.Rel); {
	case preference && document.IsBool():
		emit(p.settingRow(artifact, timestamp, "permission setting: sandbox.enabled="+
			parserkit.SettingValue(document)+" "+source))
		return
	case preference:
		emit(p.fileProblem(artifact, "", artifact.Rel+" is not true or false"))
		return
	case !document.IsObject():
		emit(p.fileProblem(artifact, "", artifact.Rel+" is not a JSON object"))
		return
	}
	for _, member := range document.Members() {
		emit(p.settingRow(artifact, timestamp, "permission setting: sandbox."+member.Key+"="+
			parserkit.SettingValue(member.Value)+" "+source))
	}
}

// retention emits the "sessions deleted by retention:" row of
// retention/state.json at its at: the counts of the last automatic
// deletion pass, "-" for an absent one, with cancelled= and
// reclaimedBytes= when present. A file that is not a JSON object is one
// `parser:` row.
func (p *Parser) retention(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	document, _ := parserkit.LoadJSON(artifact, artifact.DiskPath, &problems)
	if problems.Damaged() {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	if !document.IsObject() {
		emit(p.fileProblem(artifact, "", "retention/state.json is not a JSON object"))
		return
	}
	var parts []string
	for _, key := range []string{"deleted", "scanned", "skippedActive", "failed", "durationMs"} {
		value := "-"
		if count := document.Get(key); !count.IsNull() {
			value = parserkit.SettingValue(count)
		}
		parts = append(parts, key+"="+value)
	}
	for _, key := range []string{"cancelled", "reclaimedBytes"} {
		if value := document.Get(key); !value.IsNull() {
			parts = append(parts, key+"="+parserkit.SettingValue(value))
		}
	}
	emit(p.settingRow(artifact, timeutil.FromValue(document.Get("at")), "sessions deleted by retention: "+
		strings.Join(parts, " ")+" (from retention/state.json, at)"))
}

// share emits the "session synced:" row of a cloud session marker at the
// file's modification time, with the share link; a file
// that is not a JSON object gives one `parser:` row instead.
func (p *Parser) share(artifact *input.Artifact, sessionID string, emit parserkit.Emit) {
	var problems parserkit.Problems
	document, _ := parserkit.LoadJSON(artifact, artifact.DiskPath, &problems)
	if problems.Damaged() {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, ""))
		return
	}
	label := "storage/session_share/" + sessionID + ".json"
	rel := artifact.Rel[len(artifact.Rel)-len(label):]
	if !document.IsObject() {
		emit(p.fileProblem(artifact, sessionID, rel+" is not a JSON object"))
		return
	}
	row := p.SystemRow(artifact, "")
	row.SessionID, row.SourceLine = sessionID, 1
	row.TimestampUTC = parserkit.FileMtime(artifact)
	row.Text = timeline.Compact(timeline.SyncedPrefix + " kilo session " + parserkit.SettingOrDash(document.Get("id")) +
		" ingest " + parserkit.SettingOrDash(document.Get("ingestPath")) + " share " + parserkit.SettingOrDash(document.Get("url")) +
		" (from " + rel + ", file mtime)")
	emit(row)
}
