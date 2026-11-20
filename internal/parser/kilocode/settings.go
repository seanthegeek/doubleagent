package kilocode

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// settingRow returns a system row at line 1 and timestamp for a standing
// setting or a problem with the whole file, its text compacted.
func (p *Parser) settingRow(artifact *input.Artifact, timestamp, text string) timeline.Row {
	row := p.SystemRow(artifact, timeline.Compact(text))
	row.SourceLine = 1
	row.TimestampUTC = timestamp
	return row
}

// readJSON returns the file at path decoded as JSON and true, or false
// after adding its unreadable problem; text that is not JSON decodes to
// null.
func readJSON(path string, problems *parser.Problems) (jsonvalue.Value, bool) {
	data, ok := parser.FileBytes(path, problems)
	if !ok {
		return jsonvalue.NewNull(), false
	}
	value, err := jsonvalue.Decode(data)
	if err != nil {
		return jsonvalue.NewNull(), true
	}
	return value, true
}

// config emits one "permission setting:" row per rule of a global config
// file's permission key, at the file's modification time:
// permission.<permission>.<pattern>=<action>, permission.<permission>=
// <action> for a rule given as one action, or permission=<action>. A file
// that does not decode as JSON with comments is one `parser:` row.
func (p *Parser) config(artifact *input.Artifact, emit parser.Emit) {
	timestamp := parser.FileMtime(artifact)
	var problems parser.Problems
	raw, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	document, err := parser.ParseJSONC(raw)
	switch {
	case err != nil:
		emit(p.settingRow(artifact, timestamp,
			"parser: "+artifact.Rel+" could not be decoded ("+err.Error()+")"))
		return
	case !document.IsObject():
		emit(p.settingRow(artifact, timestamp, "parser: "+artifact.Rel+" is not a JSON object"))
		return
	case !document.Has("permission"):
		return
	}
	source := "(from " + artifact.Rel + ", file mtime)"
	permission := document.Get("permission")
	if !permission.IsObject() {
		emit(p.settingRow(artifact, timestamp,
			"permission setting: permission="+parser.SettingValue(permission)+" "+source))
		return
	}
	carried := parser.NewCarried(artifact)
	for _, member := range permission.Members() {
		if !member.Value.IsObject() {
			emit(p.settingRow(artifact, timestamp, "permission setting: permission."+member.Key+"="+
				parser.SettingValue(member.Value)+" "+source))
			continue
		}
		// Each action's row repeats its permission's key.
		key := carried.Cut("permission key", member.Key)
		for _, action := range member.Value.Members() {
			emit(p.settingRow(artifact, timestamp, "permission setting: permission."+key+"."+
				action.Key+"="+parser.SettingValue(action.Value)+" "+source))
		}
	}
	carried.Report(1, &problems)
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// sandbox emits the "permission setting: sandbox.<key>=<value>" rows of a
// sandbox file at its modification time, with no session, since the file
// names are hashes: a row per key of a policy file, one sandbox.enabled row
// for a preference file (true or false). A file that is not the expected
// JSON is one `parser:` row.
func (p *Parser) sandbox(artifact *input.Artifact, emit parser.Emit) {
	timestamp := parser.FileMtime(artifact)
	source := "(from " + artifact.Rel + ", file mtime)"
	var problems parser.Problems
	document, ok := readJSON(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	if sandboxPreferencePattern.MatchString(artifact.Rel) {
		text := "parser: " + artifact.Rel + " is not true or false"
		if document.IsBool() {
			text = "permission setting: sandbox.enabled=" + parser.SettingValue(document) + " " + source
		}
		emit(p.settingRow(artifact, timestamp, text))
		return
	}
	if !document.IsObject() {
		emit(p.settingRow(artifact, timestamp, "parser: "+artifact.Rel+" is not a JSON object"))
		return
	}
	for _, member := range document.Members() {
		emit(p.settingRow(artifact, timestamp, "permission setting: sandbox."+member.Key+"="+
			parser.SettingValue(member.Value)+" "+source))
	}
}

// retention emits the "sessions deleted by retention:" row of
// retention/state.json at its at: the counts of the last automatic
// deletion pass, "-" for an absent one, with cancelled= and
// reclaimedBytes= when present. A file that is not a JSON object is one
// `parser:` row.
func (p *Parser) retention(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	document, ok := readJSON(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	if !document.IsObject() {
		emit(p.settingRow(artifact, parser.FileMtime(artifact),
			"parser: retention/state.json is not a JSON object"))
		return
	}
	var parts []string
	for _, key := range []string{"deleted", "scanned", "skippedActive", "failed", "durationMs"} {
		value := "-"
		if count := document.Get(key); !count.IsNull() {
			value = parser.SettingValue(count)
		}
		parts = append(parts, key+"="+value)
	}
	for _, key := range []string{"cancelled", "reclaimedBytes"} {
		if value := document.Get(key); !value.IsNull() {
			parts = append(parts, key+"="+parser.SettingValue(value))
		}
	}
	emit(p.settingRow(artifact, timeutil.FromValue(document.Get("at")), "sessions deleted by retention: "+
		strings.Join(parts, " ")+" (from retention/state.json, at)"))
}

// share emits the "session synced:" row of a cloud session marker at the
// file's modification time, with the share link; a file
// that is not a JSON object gives one `parser:` row instead.
func (p *Parser) share(artifact *input.Artifact, sessionID string, emit parser.Emit) {
	var problems parser.Problems
	document, ok := readJSON(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, ""))
		return
	}
	row := p.SystemRow(artifact, "")
	row.SessionID, row.SourceLine = sessionID, 1
	row.TimestampUTC = parser.FileMtime(artifact)
	rel := "storage/session_share/" + sessionID + ".json"
	if !document.IsObject() {
		row.Text = "parser: " + rel + " is not a JSON object"
		emit(row)
		return
	}
	row.Text = timeline.Compact(timeline.SyncedPrefix + " kilo session " + parser.SettingOrDash(document.Get("id")) +
		" ingest " + parser.SettingOrDash(document.Get("ingestPath")) + " share " + parser.SettingOrDash(document.Get("url")) +
		" (from " + rel + ", file mtime)")
	emit(row)
}
