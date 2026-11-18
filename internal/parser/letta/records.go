package letta

import (
	"fmt"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/pi"
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
func (p *Parser) readDocument(artifact *input.Artifact, emit parser.Emit,
	project string) (jsonvalue.Value, bool, bool) {
	var problems parser.Problems
	document, isObject := readSettings(artifact.DiskPath, &problems)
	if len(problems) > 0 {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
		return document, false, false
	}
	return document, isObject, true
}

// parseSettings emits the sync and permission rows of a settings file, at
// its modification time: one sync row per sessionsByServer entry whose key
// does not start "local:" (not in the legacy file, which Letta reads only
// for its permissions), then one row per permissions entry. A
// sessionsByServer or permissions that is not an object adds a problem row.
func (p *Parser) parseSettings(artifact *input.Artifact, emit parser.Emit) {
	project := ""
	if artifact.Home != nil && artifact.Home.Project {
		project = artifact.Home.Original
	}
	document, isObject, read := p.readDocument(artifact, emit, project)
	if !read {
		return
	}
	timestamp := parser.FileMtime(artifact)
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
	var problems parser.Problems
	if artifact.Rel != legacySettingsRel {
		syncRows(parser.ObjectMember(document.Get("sessionsByServer"), "sessionsByServer", 0, &problems),
			artifact.Rel, put)
	}
	permissions := document.Get("permissions")
	parser.ObjectMember(permissions, "permissions", 0, &problems)
	for _, key := range permissionKeys() {
		for _, entry := range listOf(permissions.Get(key)) {
			put("", "permission setting: permissions."+key+"="+parser.SettingValue(entry)+
				" (from "+artifact.Rel+", file mtime)")
		}
	}
	for _, found := range problems {
		put("", parser.ProblemPrefix+found.Message)
	}
}

// syncRows writes, through put, one `session synced:` row per entry of a
// settings file's sessionsByServer members whose key does not start "local:".
func syncRows(servers []jsonvalue.Member, rel string, put func(sessionID, text string)) {
	for _, server := range servers {
		if strings.HasPrefix(server.Key, "local:") {
			continue
		}
		entry := server.Value
		put(parser.Text(entry.Get("conversationId")), "session synced: last session on "+
			server.Key+" agent "+parser.TextOr(entry.Get("agentId"), "-")+" conversation "+
			parser.TextOr(entry.Get("conversationId"), "-")+" (from "+rel+", file mtime)")
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

// parseRouting emits one `session relayed:` row per route of a channel's
// routing file, at its createdAt, else the file's modification time.
func (p *Parser) parseRouting(artifact *input.Artifact, channel string, emit parser.Emit) {
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
	mtime := parser.FileMtime(artifact)
	for _, route := range routes.Items() {
		if !route.IsObject() {
			continue
		}
		created := timeutil.FromValue(route.Get("createdAt"))
		source := "createdAt"
		if created == "" {
			source = "file mtime"
		}
		row := p.lineOne(artifact, timeutil.FirstNonEmpty(created, mtime), "session relayed: "+
			channel+" chat "+parser.SettingOrDash(route.Get("chatId"))+" type "+parser.SettingOrDash(route.Get("chatType"))+
			" thread "+parser.SettingOrDash(route.Get("threadId"))+" account "+parser.SettingOrDash(route.Get("accountId"))+
			" agent "+parser.SettingOrDash(route.Get("agentId"))+" enabled="+parser.SettingOrDash(route.Get("enabled"))+
			" outbound="+parser.SettingOrDash(route.Get("outboundEnabled"))+" detached="+
			parser.SettingOrDash(route.Get("detached"))+" updated="+timeOrDash(route.Get("updatedAt"))+
			" (from "+artifact.Rel+", "+source+")")
		row.SessionID = parser.Text(route.Get("conversationId"))
		emit(row)
	}
}

// parseCrons emits one `scheduled task:` row per task of crons.json, at its
// created_at, else the file's modification time. A tasks that is not an
// array adds a problem row.
func (p *Parser) parseCrons(artifact *input.Artifact, emit parser.Emit) {
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
	mtime := parser.FileMtime(artifact)
	var problems parser.Problems
	for _, task := range parser.ArrayMember(document.Get("tasks"), "tasks", 0, &problems) {
		if !task.IsObject() {
			continue
		}
		created := timeutil.FromValue(task.Get("created_at"))
		conversation := parser.Text(task.Get("conversation_id"))
		row := p.lineOne(artifact, timeutil.FirstNonEmpty(created, mtime), taskText(task, conversation,
			created != "", artifact.Rel))
		if conversation != "new" {
			row.SessionID = conversation
		}
		emit(row)
	}
	for _, found := range problems {
		emit(p.lineOne(artifact, mtime, parser.ProblemPrefix+found.Message))
	}
}

// taskText returns the `scheduled task:` row text of one task.
func taskText(task jsonvalue.Value, conversation string, created bool, rel string) string {
	field := func(key string) string { return parser.SettingOrDash(task.Get(key)) }
	source := "file mtime"
	if created {
		source = "created_at"
	}
	return "scheduled task: " + field("id") + " name=" + field("name") + " status=" +
		field("status") + " cron=" + field("cron") + " timezone=" + field("timezone") +
		" recurring=" + field("recurring") + " scheduled_for=" +
		timeOrDash(task.Get("scheduled_for")) + " agent=" + field("agent_id") + " conversation=" +
		parser.SettingOrDash(jsonvalue.NewString(conversation)) + " fire_count=" + field("fire_count") +
		" last_fired=" + timeOrDash(task.Get("last_fired_at")) + " last_run=" +
		timeOrDash(task.Get("last_run_at")) + " outcome=" + field("last_run_outcome") + " reason=" +
		field("last_run_reason") + " error=" + field("last_run_error") + " cancel_reason=" +
		field("cancel_reason") + " (from " + rel + ", " + source + ") description: " +
		field("description") + " prompt: " + field("prompt")
}

// parseRuns emits one `scheduled task run:` row per line of a task's run
// log, at its ts.
func (p *Parser) parseRuns(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	for line, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		field := func(key string) string { return parser.SettingOrDash(record.Get(key)) }
		row := p.SystemRow(artifact, timeline.Compact("scheduled task run: "+field("jobId")+
			" action="+field("action")+" status="+field("status")+" outcome="+
			field("outcome")+" reason="+field("reason")+" agent="+field("agentId")+
			" conversation="+field("conversationId")+" run="+field("runId")+
			" scheduled_for="+timeOrDash(record.Get("scheduledFor"))+" fired="+
			timeOrDash(record.Get("firedAt"))+" missed="+field("missedCount")+" summary="+
			field("summary")+" error="+field("error")+" (from "+artifact.Rel+", ts)"))
		row.SourceLine = line
		row.TimestampUTC = timeutil.FromValue(record.Get("ts"))
		row.SessionID = parser.Text(record.Get("conversationId"))
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// parseInterrupted emits the rows of an interrupted remote-listener turn,
// at the file's modification time (the record holds no time): one
// `interrupted turn:` row, then per results entry a `permission decision:`
// row (an approval, with approve) or a tool_result row (a tool return, with
// status), joined on tool_call_id. A results that is not an array adds a
// problem row.
func (p *Parser) parseInterrupted(artifact *input.Artifact, emit parser.Emit) {
	document, isObject, read := p.readDocument(artifact, emit, "")
	if !read {
		return
	}
	timestamp := parser.FileMtime(artifact)
	if !isObject {
		row := p.lineOne(artifact, timestamp, "")
		row.Text = "parser: " + artifact.Rel + " is not a JSON object"
		emit(row)
		return
	}
	sessionID, cwd := parser.Text(document.Get("conversationId")), parser.Text(document.Get("workingDirectory"))
	put := func(turn, toolUseID, text string) {
		row := p.lineOne(artifact, timestamp, text)
		row.TurnType, row.ToolUseID = turn, toolUseID
		row.SessionID, row.ProjectPath = sessionID, cwd
		emit(row)
	}
	var problems parser.Problems
	results := objects(parser.ArrayMember(document.Get("results"), "results", 0, &problems))
	put(timeline.TurnSystem, "", fmt.Sprintf("interrupted turn: agent %s conversation %s run %s "+
		"tool_calls=%s unstarted=%s results=%d working_directory=%s acting_user=%s (from %s, file mtime)",
		parser.SettingOrDash(document.Get("agentId")), parser.SettingOrDash(jsonvalue.NewString(sessionID)), parser.SettingOrDash(document.Get("runId")),
		joinedIDs(document.Get("toolCallIds")), joinedIDs(document.Get("unstartedToolCallIds")),
		len(results), parser.SettingOrDash(jsonvalue.NewString(cwd)), parser.SettingOrDash(document.Get("actingUserId")), artifact.Rel))
	for _, result := range results {
		callID := parser.Text(result.Get("tool_call_id"))
		if result.Has("approve") {
			put(timeline.TurnSystem, callID, interruptedDecision(result, artifact.Rel))
			continue
		}
		text := parser.TextOf(result.Get("tool_return"))
		if status := result.Get("status"); status.IsString() && status.StringValue() == "error" {
			text = "[error] " + text
		}
		put(timeline.TurnToolResult, callID, text)
	}
	for _, found := range problems {
		put(timeline.TurnSystem, "", parser.ProblemPrefix+found.Message)
	}
}

// objects returns the objects of a list, none for any other value.
func objects(list []jsonvalue.Value) []jsonvalue.Value {
	var out []jsonvalue.Value
	for _, item := range list {
		if item.IsObject() {
			out = append(out, item)
		}
	}
	return out
}

// interruptedDecision returns the `permission decision:` row text of an
// approval in an interrupted turn's results.
func interruptedDecision(result jsonvalue.Value, rel string) string {
	decision := "deny"
	if approved, isBool := result.Get("approve").Bool(); isBool && approved {
		decision = "approve"
	}
	return "permission decision: " + decision + " reason=" + parser.SettingOrDash(result.Get("reason")) +
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

// parseSessions emits one `letta session` row per sessions.jsonl line, then
// its `model:` row when the model is new or changed for the CLI session.
func (p *Parser) parseSessions(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	models := parser.Tracker{}
	for line, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		row := p.SystemRow(artifact, timeline.Compact(sessionText(record)))
		row.SourceLine = line
		row.TimestampUTC = timeutil.FromValue(record.Get("timestamp"))
		row.SessionID = parser.Text(record.Get("session_id"))
		row.ProjectPath = parser.Text(record.Get("project"))
		row.Model = parser.Text(record.Get("model"))
		emit(row)
		if text := pi.ModelText(models, row.SessionID, row.Model, parser.Text(record.Get("provider")),
			""); text != "" {
			row.Text = timeline.Compact(text)
			emit(row)
		}
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// sessionText returns the text of a sessions.jsonl line: the end of a CLI
// session when it has exit_reason or message_count, else its start.
func sessionText(record jsonvalue.Value) string {
	field := func(key string) string { return parser.Text(record.Get(key)) }
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
