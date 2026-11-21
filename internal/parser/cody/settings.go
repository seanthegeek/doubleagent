package cody

import (
	"bytes"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// mcpKey is the settings key of the MCP servers.
const mcpKey = "cody.mcpServers"

// permissionKeys are the flat settings.json keys whose leaves are
// permission settings.
func permissionKeys() []string {
	return []string{"cody.agentic.context.experimentalOptions", "cody.agentic.context.experimentalShell",
		"cody.chat.agenticContext"}
}

// setting is one permission entry of a settings file.
type setting struct {
	key   string
	value jsonvalue.Value
}

// leaves appends (dotted key, value) per leaf of a settings value; an
// empty object is a leaf. The key written before each member is cut by
// carried.
func leaves(out []setting, prefix string, value jsonvalue.Value, carried *parser.Carried) []setting {
	if !value.IsObject() || value.Len() == 0 {
		return append(out, setting{prefix, value})
	}
	for _, member := range value.Members() {
		out = leaves(out, carried.Cut("settings key", prefix)+"."+member.Key, member.Value, carried)
	}
	return out
}

// settingsEntries returns the permission entries of an editor's
// settings.json: every leaf of the permission keys, and disabled and
// disabledTools of each MCP server, never its headers, env, url or command.
func settingsEntries(document jsonvalue.Value, carried *parser.Carried) []setting {
	var out []setting
	for _, key := range permissionKeys() {
		if value, ok := document.Lookup(key); ok {
			out = leaves(out, key, value, carried)
		}
	}
	for _, server := range document.Get(mcpKey).Members() {
		if !server.Value.IsObject() {
			continue
		}
		for _, field := range []string{"disabled", "disabledTools"} {
			if value, ok := server.Value.Lookup(field); ok {
				out = append(out, setting{mcpKey + "." + server.Key + "." + field, value})
			}
		}
	}
	return out
}

// settings emits the "permission setting:" rows of an editor's
// User/settings.json, at the file's modification time.
func (p *Parser) settings(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	raw, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	document, err := parser.ParseJSONC(artifact, raw)
	if err != nil || !document.IsObject() {
		// The file is the editor's; it concerns Cody only when it names Cody.
		if bytes.Contains(raw, []byte(`"cody.`)) {
			emit(p.systemRow(artifact, parser.ProblemPrefix+parser.WrongKind("settings.json", jsonvalue.Object), 1))
		}
		return
	}
	timestamp := parser.FileMtime(artifact)
	carried := parser.NewCarried(artifact)
	for _, entry := range settingsEntries(document, carried) {
		text := "permission setting: " + entry.key + "=" + parser.SettingValue(entry.value) +
			" (from " + artifact.Rel + ", file mtime)"
		row := p.systemRow(artifact, timeline.Compact(text), 1)
		row.TimestampUTC = timestamp
		emit(row)
	}
	carried.Report(1, &problems)
	for _, problem := range problems {
		emit(p.systemRow(artifact, parser.ProblemPrefix+problem.Message, 1))
	}
}

// preferencesFile emits the "model setting:" rows of the JetBrains
// plugin's cody-model-preferences file.
func (p *Parser) preferencesFile(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	text, ok := parser.ReadText(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	preferences, err := parser.DecodeJSON(artifact, text)
	if err != nil || !preferences.IsObject() {
		emit(p.systemRow(artifact, parser.ProblemPrefix+parser.WrongKind(preferencesKey, jsonvalue.Object), 1))
		return
	}
	var found parser.Problems
	p.preferenceRows(artifact, preferences, artifact.Rel, 1, emit, &found)
	for _, problem := range found {
		emit(p.systemRow(artifact, parser.ProblemPrefix+problem.Message, 1))
	}
}

// preferenceRows emits a "model setting:" row per model the preferences
// name, the user's selected entries before the server's defaults, at the
// file's modification time, with the endpoint.
// Preferences that are not an object, and endpoints of another kind, add a
// problem to problems, at line; a selected or defaults that is not an
// object, or that holds models of another kind, adds one however many
// endpoints hold it so.
func (p *Parser) preferenceRows(artifact *input.Artifact, preferences jsonvalue.Value, source string, line int,
	emit parser.Emit, problems *parser.Problems) {
	timestamp := parser.FileMtime(artifact)
	var tally parser.EntryTally
	defer tally.Report(line, problems)
	carried := parser.NewCarried(artifact)
	defer carried.Report(line, problems)
	for key, value := range parser.MembersOfKind(preferences, preferencesKey, jsonvalue.Object, line, problems) {
		// Each model's row repeats its endpoint.
		endpoint := jsonvalue.Member{Key: carried.Cut("endpoint", key), Value: value}
		for _, kind := range []string{"selected", "defaults"} {
			for name, model := range tally.MembersOfKind(endpoint.Value.Get(kind), kind, jsonvalue.String) {
				usage := jsonvalue.Member{Key: name, Value: model}
				if model.StringValue() == "" {
					continue
				}
				row := p.systemRow(artifact, "model setting: "+kind+"."+usage.Key+"="+model.StringValue()+
					" endpoint="+endpoint.Key+" (from "+source+", file mtime)", line)
				row.TimestampUTC = timestamp
				row.Model = model.StringValue()
				emit(row)
			}
		}
	}
}
