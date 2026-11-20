package nanobot

import (
	"fmt"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/aider"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// loadObject returns the JSON object of a whole-file artifact, or false
// and the `parser:` row that says why there is none; what names the file
// in that row. For config.json, which holds API keys, the reason goes
// through parser.ReaderErrorText, which returns it as written and records
// the reader's detail for the output to replace under --no-secrets.
func (p *Parser) loadObject(artifact *input.Artifact, what string) (jsonvalue.Value, timeline.Row, bool) {
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		return jsonvalue.Value{}, p.ProblemRow(artifact, problems[0].Message), false
	}
	value, err := jsonvalue.Decode(content)
	switch {
	case err != nil && what == "config.json":
		return jsonvalue.Value{}, p.ProblemRow(artifact, what+" did not parse: "+parser.ReaderErrorText(artifact, err)), false
	case err != nil:
		return jsonvalue.Value{}, p.ProblemRow(artifact, what+" did not parse: "+err.Error()), false
	case !value.IsObject():
		return jsonvalue.Value{}, p.ProblemRow(artifact, what+" is not a JSON object"), false
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
			"memory history #"+getOr(record, "cursor", "")+": "+
				parser.Text(record.Get("content")), parser.Text(record.Get("session_key")))
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
	carried := parser.NewCarried(artifact)
	// Each entry's row repeats the index's version.
	version := carried.Cut("index version", getOr(data, "version", ""))
	for i, entry := range entries.Items() {
		if !entry.IsObject() {
			problems.AddRaw(0, fmt.Sprintf("unparseable record %d (not a JSON object)", i+1),
				parser.JSONText(entry))
			continue
		}
		key := parser.Text(entry.Get("key"))
		parts := []string{"webui sidebar entry: key=" + key}
		for _, name := range []string{"title", "preview", "model_preset", "updated_at"} {
			if value := entry.Get(name); !value.IsEmpty() {
				parts = append(parts, name+"="+value.Text())
			}
		}
		parts = append(parts, "(from .webui_session_index.json version "+version+")")
		emit(p.settingRow(artifact, timeutil.FromValue(entry.Get("created_at")),
			strings.Join(parts, " "), key))
	}
	carried.Report(0, &problems)
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// parsePairing emits a row per approved chat sender, at the file's
// modification time, and per pending request, at its created_at; a pending
// request's pairing code is not shown. An approved or pending that is not
// an object, or an approved list that is not an array, adds a problem row.
func (p *Parser) parsePairing(artifact *input.Artifact, emit parser.Emit) {
	data, problem, ok := p.loadObject(artifact, "pairing.json")
	if !ok {
		emit(problem)
		return
	}
	timestamp := parser.FileMtime(artifact)
	var problems parser.Problems
	carried := parser.NewCarried(artifact)
	for _, member := range parser.ObjectMember(data.Get("approved"), "approved", 0, &problems) {
		channel := carried.Cut("approved channel", member.Key)
		for _, sender := range parser.ArrayMember(member.Value, "approved."+member.Key, 0, &problems) {
			emit(p.settingRow(artifact, timestamp, "permission setting: pairing.approved."+
				channel+"="+parser.SettingValue(sender)+" (from "+artifact.Rel+
				", file mtime)", ""))
		}
	}
	carried.Report(0, &problems)
	for _, info := range parser.MembersOfKind(data.Get("pending"), "pending", jsonvalue.Object, 0, &problems) {
		status := ""
		if value := info.Get("delivery_status"); !value.IsEmpty() {
			status = " delivery_status=" + value.Text()
		}
		sender := jsonvalue.NewString("")
		if value := info.Get("sender_id"); !value.IsEmpty() {
			sender = value
		}
		emit(p.settingRow(artifact, timeutil.FromValue(info.Get("created_at")), "permission setting: pairing.pending."+
			parser.Text(info.Get("channel"))+"="+parser.SettingValue(sender)+" expires_at="+
			timeutil.FromValue(info.Get("expires_at"))+status+" (from "+artifact.Rel+
			", created_at)", ""))
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
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
func firstKey(object jsonvalue.Value, names []string) (string, jsonvalue.Value, bool) {
	for _, name := range names {
		if value, ok := object.Lookup(name); ok && object.IsObject() {
			return name, value, true
		}
	}
	return "", jsonvalue.Value{}, false
}

// parseConfig emits one `permission setting:` row per standing setting of
// config.json, at its modification time: only the settings that grant or
// restrict the agent, named as the file spells them. A list gives one row
// per entry. Then one `stored credentials:` row gives the credentials the
// file holds (configCredentials), each through parser.SecretText, then a
// problem row per member of the wrong kind on the way to those rows.
func (p *Parser) parseConfig(artifact *input.Artifact, emit parser.Emit) {
	data, problem, ok := p.loadObject(artifact, "config.json")
	if !ok {
		emit(problem)
		return
	}
	var settings []jsonvalue.Member
	add := func(prefix string, object jsonvalue.Value, keys [][]string) {
		for _, names := range keys {
			if name, value, found := firstKey(object, names); found {
				settings = append(settings, jsonvalue.Member{Key: prefix + name, Value: value})
			}
		}
	}
	var problems parser.Problems
	carried := parser.NewCarried(artifact)
	tools := parser.ObjectOrEmptyMember(data.Get("tools"), "tools", 0, &problems)
	add("tools.", tools, configToolKeys())
	add("tools.exec.", parser.ObjectOrEmptyMember(tools.Get("exec"), "tools.exec", 0, &problems), configExecKeys())
	// channels holds settings beside the channel objects (sendProgress), so
	// a member that is not an object is not reported.
	for _, member := range parser.ObjectMember(data.Get("channels"), "channels", 0, &problems) {
		// Each allowFrom entry's row repeats the channel's name.
		add("channels."+carried.Cut("channel name", member.Key)+".", member.Value,
			[][]string{{"allowFrom", "allow_from"}})
	}
	carried.Report(0, &problems)
	timestamp := parser.FileMtime(artifact)
	for _, setting := range settings {
		entries := []jsonvalue.Value{setting.Value}
		if setting.Value.IsArray() {
			entries = setting.Value.Items()
		}
		for _, entry := range entries {
			emit(p.settingRow(artifact, timestamp, "permission setting: "+setting.Key+"="+
				parser.SettingValue(entry)+" (from "+artifact.Rel+", file mtime)", ""))
		}
	}
	var stored []string
	for _, credential := range configCredentials(data, &problems) {
		stored = append(stored, credential.Key+"="+parser.SecretText(artifact, parser.Text(credential.Value)))
	}
	if len(stored) > 0 {
		emit(p.settingRow(artifact, timestamp, "stored credentials: "+strings.Join(stored, " ")+" (from "+
			artifact.Rel+", file mtime)", ""))
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// configCredentials returns the credentials of config.json that are set,
// named as the file spells them: each provider's API key, each MCP
// server's env value and the Feishu and Linear channel secrets
// (research/nanobot.md, "Credentials"). A providers, MCP server list or
// MCP env that is not an object, and providers or servers that are not
// objects, add a problem.
func configCredentials(data jsonvalue.Value, problems *parser.Problems) []jsonvalue.Member {
	var found []jsonvalue.Member
	add := func(prefix string, object jsonvalue.Value, names []string) {
		if name, value, ok := firstKey(object, names); ok && parser.Text(value) != "" {
			found = append(found, jsonvalue.Member{Key: prefix + name, Value: value})
		}
	}
	for name, provider := range parser.MembersOfKind(data.Get("providers"), "providers", jsonvalue.Object, 0,
		problems) {
		add("providers."+name+".", provider, []string{"apiKey", "api_key"})
	}
	tools := data.Get("tools")
	if name, servers, ok := firstKey(tools, []string{"mcpServers", "mcp_servers"}); ok {
		for server, settings := range parser.MembersOfKind(servers, "tools."+name, jsonvalue.Object, 0, problems) {
			env := settings.Get("env")
			for _, variable := range parser.ObjectMember(env, "tools."+name+"."+server+".env", 0, problems) {
				add("tools."+name+"."+server+".env.", env, []string{variable.Key})
			}
		}
	}
	channels := data.Get("channels")
	for _, names := range [][]string{{"appSecret", "app_secret"}, {"verificationToken", "verification_token"}} {
		add("channels.feishu.", channels.Get("feishu"), names)
	}
	add("channels.linear.", channels.Get("linear"), []string{"clientSecret", "client_secret"})
	return found
}
