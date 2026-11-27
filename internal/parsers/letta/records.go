package letta

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers/pi"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// permissionKeys are the keys of a settings file's permissions object.
func permissionKeys() []string {
	return []string{"mode", "allow", "deny", "ask", "alwaysAsk", "additionalDirectories"}
}

// timeOrDash returns a record value as a timestamp, "-" when it is none.
func timeOrDash(value jsonvalue.Value) string {
	if timestamp := timeutil.FromValue(value); timestamp != "" {
		return timestamp
	}
	return "-"
}

// lineOne returns a system row of the artifact at line 1 with the given
// time, compacting the text.
func (p *Parser) lineOne(artifact *input.Artifact, timestamp, text string) timeline.Row {
	row := p.SystemRow(artifact, timeline.Compact(text))
	row.SourceLine = 1
	row.TimestampUTC = timestamp
	return row
}

// readDocument reads a JSON file of the artifact, emitting its unreadable
// row when it cannot be read. It returns the document, whether it is an
// object, and whether the file was read at all.
func (p *Parser) readDocument(artifact *input.Artifact, emit parserkit.Emit,
	project string) (jsonvalue.Value, bool, bool) {
	var problems parserkit.Problems
	document, isObject := readSettings(artifact, artifact.DiskPath, &problems)
	if len(problems) > 0 {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
		return document, false, false
	}
	return document, isObject, true
}

// plainEnvVariables returns the environment variables Letta Code reads that
// hold no credential, which a settings file's env shows as written: a
// server URL, directories and files, and switches (research/letta.md,
// "Relocation variables"). Any other variable an env sets, the user's own
// included, may be a key and is identified.
func plainEnvVariables() []string {
	return []string{"LETTA_BASE_URL", "LETTA_CODE_TELEM", "DO_NOT_TRACK", "LETTA_ENABLE_WINDOWS_AUTO_REFLECTION",
		"LETTA_MEMORY_DIR", "MEMORY_DIR", "LETTA_LISTENER_PERF_FILE", "LETTA_TUI_PERF_FILE",
		"LETTA_LOCAL_BACKEND_EXPERIMENTAL", "LETTA_HOME", "LETTA_LOCAL_BACKEND_DIR", "LETTA_TRANSCRIPT_ROOT",
		"LETTA_ARTIFACTS_DIR", "LETTA_MODS_DIR", "LETTA_EXTENSIONS_DIR", "LETTA_DEBUG_FILE", "XDG_CONFIG_HOME",
		"LETTA_DIR", "MEMGPT_CONFIG_PATH"}
}

// storedCredentials returns "<key>=<value>" of each env member and of the
// refreshToken of a settings file, joined by spaces, or "" when it holds
// none: the file the CLI writes the API key and refresh token into when the
// keychain cannot hold them. The refresh token and each env value go
// through parserkit.SecretText but those of the variables the CLI reads that
// hold no credential (plainEnvVariables). An env that is not an object adds
// a problem.
func storedCredentials(artifact *input.Artifact, document jsonvalue.Value, problems *parserkit.Problems) string {
	var parts []string
	for _, member := range parserkit.ObjectMember(document.Get("env"), "env", 0, problems) {
		value := parserkit.Text(member.Value)
		if value != "" && !slices.Contains(plainEnvVariables(), member.Key) {
			value = parserkit.SecretText(artifact, value)
		}
		if value != "" {
			parts = append(parts, "env."+member.Key+"="+value)
		}
	}
	if token := parserkit.Text(document.Get("refreshToken")); token != "" {
		parts = append(parts, "refreshToken="+parserkit.SecretText(artifact, token))
	}
	return strings.Join(parts, " ")
}

// syncRows writes, through put, one `session synced:` row per entry of a
// settings file's sessionsByServer members whose key does not start
// "local:", with the server's URL.
func syncRows(artifact *input.Artifact, servers []jsonvalue.Member, rel string, put func(sessionID, text string)) {
	for _, server := range servers {
		if strings.HasPrefix(server.Key, "local:") {
			continue
		}
		entry := server.Value
		put(parserkit.Text(entry.Get("conversationId")), "session synced: last session on "+
			server.Key+" agent "+parserkit.TextOr(entry.Get("agentId"), "-")+" conversation "+
			parserkit.TextOr(entry.Get("conversationId"), "-")+" (from "+rel+", file mtime)")
	}
}

// listOf returns a list's items, nothing for null, and any other value as
// a list of one.
func listOf(value jsonvalue.Value) []jsonvalue.Value {
	switch {
	case value.IsArray():
		return value.Items()
	case value.IsNull():
		return nil
	}
	return []jsonvalue.Value{value}
}

// parseSettings emits the sync and permission rows of a settings file, at
// its modification time: one sync row per sessionsByServer entry whose key
// does not start "local:" (not in the legacy file, which Letta reads only
// for its permissions), then one row per permissions entry, then one
// "stored credentials:" row of each env value and the refreshToken, each
// through parserkit.SecretText. A sessionsByServer or permissions that is not
// an object adds a problem row.
func (p *Parser) parseSettings(artifact *input.Artifact, emit parserkit.Emit) {
	project := ""
	if artifact.Home != nil && artifact.Home.Project {
		project = artifact.Home.Original
	}
	document, isObject, read := p.readDocument(artifact, emit, project)
	if !read {
		return
	}
	timestamp := parserkit.FileMtime(artifact)
	put := func(sessionID, text string) {
		row := p.lineOne(artifact, timestamp, text)
		row.SessionID, row.ProjectPath = sessionID, project
		emit(row)
	}
	if !isObject {
		row := p.lineOne(artifact, timestamp, "")
		row.Text, row.ProjectPath = "parser: "+artifact.Rel+" is not a JSON object", project
		emit(row)
		return
	}
	var problems parserkit.Problems
	if !textlines.EqualFoldASCII(artifact.Rel, legacySettingsRel) {
		syncRows(artifact, parserkit.ObjectMember(document.Get("sessionsByServer"), "sessionsByServer", 0, &problems),
			artifact.Rel, put)
	}
	permissions := document.Get("permissions")
	parserkit.ObjectMember(permissions, "permissions", 0, &problems)
	for _, key := range permissionKeys() {
		for _, entry := range listOf(permissions.Get(key)) {
			put("", "permission setting: permissions."+key+"="+parserkit.SettingValue(entry)+
				" (from "+artifact.Rel+", file mtime)")
		}
	}
	if stored := storedCredentials(artifact, document, &problems); stored != "" {
		put("", "stored credentials: "+stored+" (from "+artifact.Rel+", file mtime)")
	}
	for _, found := range problems {
		put("", parserkit.ProblemPrefix+found.Message)
	}
}

// parseRouting emits one `session relayed:` row per route of a channel's
// routing file, at its createdAt, else the file's modification time.
func (p *Parser) parseRouting(artifact *input.Artifact, channel string, emit parserkit.Emit) {
	document, _, read := p.readDocument(artifact, emit, "")
	if !read {
		return
	}
	routes := document.Get("routes")
	if !routes.IsArray() {
		row := p.lineOne(artifact, "", "")
		row.Text = "parser: " + artifact.Rel + " is not a JSON object with a routes list"
		emit(row)
		return
	}
	mtime := parserkit.FileMtime(artifact)
	var problems parserkit.Problems
	defer func() {
		for _, found := range problems {
			emit(p.lineOne(artifact, mtime, parserkit.ProblemPrefix+found.Message))
		}
	}()
	for _, route := range parserkit.EntriesOfKind(routes, "routes", jsonvalue.Object, 0, &problems) {
		created := timeutil.FromValue(route.Get("createdAt"))
		source := "createdAt"
		if created == "" {
			source = "file mtime"
		}
		row := p.lineOne(artifact, cmp.Or(created, mtime), "session relayed: "+
			channel+" chat "+parserkit.SettingOrDash(route.Get("chatId"))+" type "+parserkit.SettingOrDash(route.Get("chatType"))+
			" thread "+parserkit.SettingOrDash(route.Get("threadId"))+" account "+parserkit.SettingOrDash(route.Get("accountId"))+
			" agent "+parserkit.SettingOrDash(route.Get("agentId"))+" enabled="+parserkit.SettingOrDash(route.Get("enabled"))+
			" outbound="+parserkit.SettingOrDash(route.Get("outboundEnabled"))+" detached="+
			parserkit.SettingOrDash(route.Get("detached"))+" updated="+timeOrDash(route.Get("updatedAt"))+
			" (from "+artifact.Rel+", "+source+")")
		row.SessionID = parserkit.Text(route.Get("conversationId"))
		emit(row)
	}
}

// taskText returns the `scheduled task:` row text of one task.
func taskText(task jsonvalue.Value, conversation string, created bool, rel string) string {
	field := func(key string) string { return parserkit.SettingOrDash(task.Get(key)) }
	source := "file mtime"
	if created {
		source = "created_at"
	}
	return "scheduled task: " + field("id") + " name=" + field("name") + " status=" +
		field("status") + " cron=" + field("cron") + " timezone=" + field("timezone") +
		" recurring=" + field("recurring") + " scheduled_for=" +
		timeOrDash(task.Get("scheduled_for")) + " agent=" + field("agent_id") + " conversation=" +
		parserkit.SettingOrDash(jsonvalue.NewString(conversation)) + " fire_count=" + field("fire_count") +
		" last_fired=" + timeOrDash(task.Get("last_fired_at")) + " last_run=" +
		timeOrDash(task.Get("last_run_at")) + " outcome=" + field("last_run_outcome") + " reason=" +
		field("last_run_reason") + " error=" + field("last_run_error") + " cancel_reason=" +
		field("cancel_reason") + " (from " + rel + ", " + source + ") description: " +
		field("description") + " prompt: " + field("prompt")
}

// parseCrons emits one `scheduled task:` row per task of crons.json, at its
// created_at, else the file's modification time. A tasks that is not an
// array adds a problem row.
func (p *Parser) parseCrons(artifact *input.Artifact, emit parserkit.Emit) {
	document, isObject, read := p.readDocument(artifact, emit, "")
	if !read {
		return
	}
	if !isObject {
		row := p.lineOne(artifact, "", "")
		row.Text = "parser: " + artifact.Rel + " is not a JSON object"
		emit(row)
		return
	}
	mtime := parserkit.FileMtime(artifact)
	var problems parserkit.Problems
	for _, task := range parserkit.EntriesOfKind(document.Get("tasks"), "tasks", jsonvalue.Object, 0, &problems) {
		created := timeutil.FromValue(task.Get("created_at"))
		conversation := parserkit.Text(task.Get("conversation_id"))
		row := p.lineOne(artifact, cmp.Or(created, mtime), taskText(task, conversation,
			created != "", artifact.Rel))
		if conversation != "new" {
			row.SessionID = conversation
		}
		emit(row)
	}
	for _, found := range problems {
		emit(p.lineOne(artifact, mtime, parserkit.ProblemPrefix+found.Message))
	}
}

// parseRuns emits one `scheduled task run:` row per line of a task's run
// log, at its ts.
func (p *Parser) parseRuns(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	for line, record := range parserkit.JSONLines(artifact.DiskPath, &problems) {
		field := func(key string) string { return parserkit.SettingOrDash(record.Get(key)) }
		row := p.SystemRow(artifact, timeline.Compact("scheduled task run: "+field("jobId")+
			" action="+field("action")+" status="+field("status")+" outcome="+
			field("outcome")+" reason="+field("reason")+" agent="+field("agentId")+
			" conversation="+field("conversationId")+" run="+field("runId")+
			" scheduled_for="+timeOrDash(record.Get("scheduledFor"))+" fired="+
			timeOrDash(record.Get("firedAt"))+" missed="+field("missedCount")+" summary="+
			field("summary")+" error="+field("error")+" (from "+artifact.Rel+", ts)"))
		row.SourceLine = line
		row.TimestampUTC = timeutil.FromValue(record.Get("ts"))
		row.SessionID = parserkit.Text(record.Get("conversationId"))
		emit(row)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// interruptedDecision returns the `permission decision:` row text of an
// approval in an interrupted turn's results.
func interruptedDecision(result jsonvalue.Value, rel string) string {
	decision := "deny"
	if approved, isBool := result.Get("approve").Bool(); isBool && approved {
		decision = "approve"
	}
	return "permission decision: " + decision + " reason=" + parserkit.SettingOrDash(result.Get("reason")) +
		" (from " + rel + ", interrupted turn, file mtime)"
}

// joinedIDs returns a list of ids joined by commas, each as its row text, or "-" for an empty list or a value that is not a list.
func joinedIDs(value jsonvalue.Value) string {
	var items []string
	for _, item := range value.Items() {
		items = append(items, item.Text())
	}
	if joined := strings.Join(items, ","); joined != "" {
		return joined
	}
	return "-"
}

// parseInterrupted emits the rows of an interrupted remote-listener turn,
// at the file's modification time (the record holds no time): one
// `interrupted turn:` row, then per results entry a `permission decision:`
// row (an approval, with approve) or a tool_result row (a tool return, with
// status), joined on tool_call_id. A results that is not an array adds a
// problem row.
func (p *Parser) parseInterrupted(artifact *input.Artifact, emit parserkit.Emit) {
	document, isObject, read := p.readDocument(artifact, emit, "")
	if !read {
		return
	}
	timestamp := parserkit.FileMtime(artifact)
	if !isObject {
		row := p.lineOne(artifact, timestamp, "")
		row.Text = "parser: " + artifact.Rel + " is not a JSON object"
		emit(row)
		return
	}
	sessionID, cwd := parserkit.Text(document.Get("conversationId")), parserkit.Text(document.Get("workingDirectory"))
	put := func(turn, toolUseID, text string) {
		row := p.lineOne(artifact, timestamp, text)
		row.TurnType, row.ToolUseID = turn, toolUseID
		row.SessionID, row.ProjectPath = sessionID, cwd
		emit(row)
	}
	var problems parserkit.Problems
	results := parserkit.ListOfKind(document.Get("results"), "results", jsonvalue.Object, 0, &problems)
	put(timeline.TurnSystem, "", fmt.Sprintf("interrupted turn: agent %s conversation %s run %s "+
		"tool_calls=%s unstarted=%s results=%d working_directory=%s acting_user=%s (from %s, file mtime)",
		parserkit.SettingOrDash(document.Get("agentId")), parserkit.SettingOrDash(jsonvalue.NewString(sessionID)), parserkit.SettingOrDash(document.Get("runId")),
		joinedIDs(document.Get("toolCallIds")), joinedIDs(document.Get("unstartedToolCallIds")),
		len(results), parserkit.SettingOrDash(jsonvalue.NewString(cwd)), parserkit.SettingOrDash(document.Get("actingUserId")), artifact.Rel))
	for _, result := range results {
		callID := parserkit.Text(result.Get("tool_call_id"))
		if result.Has("approve") {
			put(timeline.TurnSystem, callID, interruptedDecision(result, artifact.Rel))
			continue
		}
		text := parserkit.TextOf(result.Get("tool_return"))
		if status := result.Get("status"); status.IsString() && status.StringValue() == "error" {
			text = "[error] " + text
		}
		put(timeline.TurnToolResult, callID, text)
	}
	for _, found := range problems {
		put(timeline.TurnSystem, "", parserkit.ProblemPrefix+found.Message)
	}
}

// sessionText returns the text of a sessions.jsonl line: the end of a CLI
// session when it has exit_reason or message_count, else its start.
func sessionText(record jsonvalue.Value) string {
	field := func(key string) string { return parserkit.Text(record.Get(key)) }
	present := func(value jsonvalue.Value, key string) string {
		if item, ok := value.Lookup(key); ok {
			return item.Text()
		}
		return ""
	}
	if record.Has("exit_reason") || record.Has("message_count") {
		duration := record.Get("duration")
		return "letta session end: agent=" + field("agent_id") + " exit_reason=" + field("exit_reason") +
			" messages=" + present(record, "message_count") + " tool_calls=" +
			present(record, "tool_call_count") + " wall_ms=" + present(duration, "wall_ms")
	}
	return "letta session start: agent=" + field("agent_id") + " provider=" + field("provider") +
		" model=" + field("model") + " project=" + field("project")
}

// parseSessions emits one `letta session` row per sessions.jsonl line, then
// its `model:` row when the model is new or changed for the CLI session.
func (p *Parser) parseSessions(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	models := parserkit.Tracker{}
	for line, record := range parserkit.JSONLines(artifact.DiskPath, &problems) {
		row := p.SystemRow(artifact, timeline.Compact(sessionText(record)))
		row.SourceLine = line
		row.TimestampUTC = timeutil.FromValue(record.Get("timestamp"))
		row.SessionID = parserkit.Text(record.Get("session_id"))
		row.ProjectPath = parserkit.Text(record.Get("project"))
		row.Model = parserkit.Text(record.Get("model"))
		emit(row)
		if text := pi.ModelText(models, row.SessionID, row.Model, parserkit.Text(record.Get("provider")),
			""); text != "" {
			row.Text = timeline.Compact(text)
			emit(row)
		}
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}
