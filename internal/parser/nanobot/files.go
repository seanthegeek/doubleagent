package nanobot

import (
	"fmt"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/aider"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// loadObject returns the JSON object of a whole-file artifact, or false
// and the `parser:` row that says why there is none; what names the file
// in that row.
func (p *Parser) loadObject(artifact *input.Artifact, what string) (pyjson.Value, timeline.Row, bool) {
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		return pyjson.Value{}, p.ProblemRow(artifact, problems[0].Message), false
	}
	value, err := pyjson.Decode(content)
	switch {
	case err != nil:
		return pyjson.Value{}, p.ProblemRow(artifact, what+" did not parse: "+err.Error()), false
	case !value.IsObject():
		return pyjson.Value{}, p.ProblemRow(artifact, what+" is not a JSON object"), false
	}
	return value, timeline.Row{}, true
}

// settingRow returns a system row at timestamp with the text compacted.
func (p *Parser) settingRow(artifact *input.Artifact, timestamp, text, sessionID string) timeline.Row {
	row := p.SystemRow(artifact, timeline.Compact(text))
	row.TimestampUTC = timestamp
	row.SessionID = sessionID
	return row
}

// parseHistory emits one `memory history` row per line of
// memory/history.jsonl.
func (p *Parser) parseHistory(artifact *input.Artifact, emit parser.Emit) {
	project := ""
	if match := p.files.originalMemory.FindStringSubmatch(artifact.Original); match != nil {
		project = match[1]
	}
	var problems parser.Problems
	for line, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		row := p.settingRow(artifact, timeutil.FromValue(record.Get("timestamp")),
			fmt.Sprintf("memory history #%s: %s", getOr(record, "cursor", ""),
				parser.Str(record.Get("content"))), parser.Str(record.Get("session_key")))
		row.SourceLine = line
		row.ProjectPath = project
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
}

// parseCLIHistory emits one user row per entry of history/cli_history,
// with no session or project.
func (p *Parser) parseCLIHistory(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	for _, entry := range aider.ReadInputHistory(artifact.DiskPath, &problems) {
		row := p.BaseRow(artifact)
		row.SourceLine = entry.Line
		row.TimestampUTC = timeutil.FromText(entry.Timestamp)
		row.TurnType = timeline.TurnUser
		row.Text = timeline.Compact(entry.Text)
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// parseSidebar emits one `webui sidebar entry:` row per cached entry of
// .webui_session_index.json, at its created_at.
func (p *Parser) parseSidebar(artifact *input.Artifact, emit parser.Emit) {
	data, problem, ok := p.loadObject(artifact, ".webui_session_index.json")
	if !ok {
		emit(problem)
		return
	}
	entries := data.Get("sessions")
	if !entries.IsArray() {
		emit(p.ProblemRow(artifact, ".webui_session_index.json has no sessions list"))
		return
	}
	var problems parser.Problems
	for i, entry := range entries.Items() {
		if !entry.IsObject() {
			problems.AddRaw(0, fmt.Sprintf("unparseable record %d (not a JSON object)", i+1),
				parser.CompactJSON(entry))
			continue
		}
		key := parser.Str(entry.Get("key"))
		parts := []string{"webui sidebar entry: key=" + key}
		for _, name := range []string{"title", "preview", "model_preset", "updated_at"} {
			if value := entry.Get(name); value.Truthy() {
				parts = append(parts, name+"="+value.Str())
			}
		}
		parts = append(parts, "(from .webui_session_index.json version "+getOr(data, "version", "")+")")
		emit(p.settingRow(artifact, timeutil.FromValue(entry.Get("created_at")),
			strings.Join(parts, " "), key))
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// parsePairing emits a row per approved chat sender, at the file's
// modification time, and per pending request, at its created_at; a pending
// request's pairing code is not shown.
func (p *Parser) parsePairing(artifact *input.Artifact, emit parser.Emit) {
	data, problem, ok := p.loadObject(artifact, "pairing.json")
	if !ok {
		emit(problem)
		return
	}
	timestamp := parser.FileMtime(artifact)
	for _, member := range objectMembers(data.Get("approved")) {
		if !member.Value.IsArray() {
			continue
		}
		for _, sender := range member.Value.Items() {
			emit(p.settingRow(artifact, timestamp, fmt.Sprintf(
				"permission setting: pairing.approved.%s=%s (from %s, file mtime)", member.Key,
				parser.SettingValue(sender), artifact.Rel), ""))
		}
	}
	for _, member := range objectMembers(data.Get("pending")) {
		info := member.Value
		if !info.IsObject() {
			continue
		}
		status := ""
		if value := info.Get("delivery_status"); value.Truthy() {
			status = " delivery_status=" + value.Str()
		}
		sender := pyjson.NewString("")
		if value := info.Get("sender_id"); value.Truthy() {
			sender = value
		}
		emit(p.settingRow(artifact, timeutil.FromValue(info.Get("created_at")), fmt.Sprintf(
			"permission setting: pairing.pending.%s=%s expires_at=%s%s (from %s, created_at)",
			parser.Str(info.Get("channel")), parser.SettingValue(sender),
			timeutil.FromValue(info.Get("expires_at")), status, artifact.Rel), ""))
	}
}

// objectMembers returns the members of an object, none for anything else.
func objectMembers(value pyjson.Value) []pyjson.Member {
	if !value.IsObject() {
		return nil
	}
	return value.Members()
}

// configToolKeys are the tools settings of config.json that grant or
// restrict what the agent may do: each list is the spellings nanobot
// accepts for one key (camelCase, as it saves them, snake_case and any
// legacy alias), the row naming the key as the file spells it.
func configToolKeys() [][]string {
	return [][]string{
		{"restrictToWorkspace", "restrict_to_workspace"},
		{"ssrfWhitelist", "ssrf_whitelist"},
		{"webuiAllowLocalServiceAccess", "webui_allow_local_service_access",
			"allowLocalPreviewAccess", "allow_local_preview_access"},
		{"webuiAllowRemotePackageInstall", "webui_allow_remote_package_install"},
	}
}

// configExecKeys are the tools.exec settings read, in the same form.
func configExecKeys() [][]string {
	return [][]string{{"enable"}, {"sandbox"}, {"allowPatterns", "allow_patterns"},
		{"denyPatterns", "deny_patterns"}, {"sandboxRoBinds", "sandbox_ro_binds"},
		{"sandboxRwBinds", "sandbox_rw_binds"}, {"allowedEnvKeys", "allowed_env_keys"}}
}

// firstKey returns the first of names the object holds, with its value.
func firstKey(object pyjson.Value, names []string) (string, pyjson.Value, bool) {
	for _, name := range names {
		if value, ok := object.Lookup(name); ok && object.IsObject() {
			return name, value, true
		}
	}
	return "", pyjson.Value{}, false
}

// parseConfig emits one `permission setting:` row per standing setting of
// config.json, at its modification time: only the settings that grant or
// restrict the agent, named as the file spells them, since the file holds
// API keys. A list gives one row per entry.
func (p *Parser) parseConfig(artifact *input.Artifact, emit parser.Emit) {
	data, problem, ok := p.loadObject(artifact, "config.json")
	if !ok {
		emit(problem)
		return
	}
	var settings []pyjson.Member
	add := func(prefix string, object pyjson.Value, keys [][]string) {
		for _, names := range keys {
			if name, value, found := firstKey(object, names); found {
				settings = append(settings, pyjson.Member{Key: prefix + name, Value: value})
			}
		}
	}
	tools := data.Get("tools")
	add("tools.", tools, configToolKeys())
	add("tools.exec.", tools.Get("exec"), configExecKeys())
	for _, member := range objectMembers(data.Get("channels")) {
		add("channels."+member.Key+".", member.Value, [][]string{{"allowFrom", "allow_from"}})
	}
	timestamp := parser.FileMtime(artifact)
	for _, setting := range settings {
		entries := []pyjson.Value{setting.Value}
		if setting.Value.IsArray() {
			entries = setting.Value.Items()
		}
		for _, entry := range entries {
			emit(p.settingRow(artifact, timestamp, fmt.Sprintf("permission setting: %s=%s (from %s,"+
				" file mtime)", setting.Key, parser.SettingValue(entry), artifact.Rel), ""))
		}
	}
}
