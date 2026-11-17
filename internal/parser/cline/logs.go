package cline

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/claudecode"
	"github.com/seanthegeek/doubleagent/internal/parser/clinelegacy"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// hookKeys are the hook audit keys named on a "hook audit:" row, each with
// the name the row gives it.
func hookKeys() [][2]string {
	return [][2]string{{"clineVersion", "cline_version"}, {"userId", "user"}, {"agent_id", "agent"},
		{"parent_agent_id", "parent_agent"}, {"reason", "reason"}, {"pid", "pid"},
		{"source", "source"}}
}

// hooksLog emits one row per line of logs/hooks.jsonl: the transcript event
// a line repeats, else a "hook audit:" row.
func (p *Parser) hooksLog(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	for line, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		row := p.BaseRow(artifact)
		row.SourceLine = line
		row.TimestampUTC = timeutil.FromValue(clinelegacy.FirstTruthy(record.Get("ts"), record.Get("timestamp")))
		row.SessionID = parser.Str(clinelegacy.FirstTruthy(record.Get("sessionContext").Get("rootSessionId"),
			record.Get("taskId"), record.Get("sessionId")))
		if roots := record.Get("workspaceRoots"); roots.Len() > 0 && roots.IsArray() &&
			roots.Index(0).IsString() {
			row.ProjectPath = roots.Index(0).StringValue()
		}
		hook := parser.Str(record.Get("hookName"))
		if !hookEvent(&row, hook, record) {
			row.TurnType = timeline.TurnSystem
			row.Text = timeline.Compact("hook audit: " + orDash(hook) + auditDetail(record))
		}
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// auditDetail returns the " name=value" fields of a hook audit line and its
// error's name and message.
func auditDetail(record jsonvalue.Value) string {
	detail := ""
	for _, pair := range hookKeys() {
		if value := record.Get(pair[0]); clinelegacy.Present(value) {
			detail += " " + pair[1] + "=" + parser.SettingValue(value)
		}
	}
	if failure := record.Get("error"); failure.IsObject() && failure.Len() > 0 {
		detail += " error=" + dash(failure.Get("name")) + ": " + dash(failure.Get("message"))
	}
	return detail
}

// orDash returns text, or "-" when it is empty.
func orDash(text string) string {
	if text == "" {
		return "-"
	}
	return text
}

// hookEvent fills row as the transcript event a hook line repeats, a
// prompt, a tool call, a tool result or the final reply, and reports
// whether the line is one.
func hookEvent(row *timeline.Row, hook string, record jsonvalue.Value) bool {
	switch hook {
	case "prompt_submit":
		prompt := record.Get("userPromptSubmit").Get("prompt")
		if !prompt.IsString() || strings.TrimSpace(prompt.StringValue()) == "" {
			return false
		}
		row.TurnType, row.Text = timeline.TurnUser, timeline.Compact(prompt.StringValue())
	case "tool_call":
		call := record.Get("tool_call")
		row.TurnType = timeline.TurnToolUse
		row.ToolName = parser.Str(call.Get("name"))
		row.ToolUseID = parser.Str(call.Get("id"))
		row.Text = timeline.Compact(claudecode.ToolSummary(row.ToolName, call.Get("input")))
	case "tool_result":
		toolResult(row, record.Get("tool_result"), record.Get("postToolUse"))
	case "agent_end":
		text := record.Get("turn").Get("outputText")
		if !text.IsString() || strings.TrimSpace(text.StringValue()) == "" {
			return false
		}
		row.TurnType, row.Text = timeline.TurnAssistant, timeline.Compact(text.StringValue())
	default:
		return false
	}
	return true
}

// toolResult fills row as a tool result: the error, else the post-use
// hook's result text, else the output.
func toolResult(row *timeline.Row, result, post jsonvalue.Value) {
	row.TurnType = timeline.TurnToolResult
	row.ToolName = parser.Str(clinelegacy.FirstTruthy(result.Get("name"), post.Get("toolName")))
	row.ToolUseID = parser.Str(result.Get("id"))
	output := result.Get("output")
	switch {
	case !result.Get("error").IsEmpty():
		row.Text = timeline.Compact("[error] " + result.Get("error").Text())
	case post.Get("result").IsString():
		row.Text = timeline.Compact(post.Get("result").StringValue())
	case output.IsString() || output.IsArray():
		row.Text = timeline.Compact(parser.TextOf(output))
	default:
		row.Text = timeline.Compact(parser.CompactJSON(output))
	}
}

// inputHistory emits one user row per prompt of the CLI's input history.
// Every line carries the time of the file's last rewrite, so only the
// newest prompt gets its own time, and none does when a line is bad.
func (p *Parser) inputHistory(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	type entry struct {
		line      int
		prompt    string
		timestamp jsonvalue.Value
	}
	var entries []entry
	for line, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		if prompt := record.Get("prompt"); prompt.IsString() && strings.TrimSpace(prompt.StringValue()) != "" {
			entries = append(entries, entry{line: line, prompt: prompt.StringValue(), timestamp: record.Get("ts")})
		}
	}
	for k, each := range entries {
		row := p.BaseRow(artifact)
		row.SourceLine = each.line
		row.TurnType = timeline.TurnUser
		if k == len(entries)-1 && len(problems) == 0 && clinelegacy.IsNumber(each.timestamp) {
			row.TimestampUTC = timeutil.FromValue(each.timestamp)
		}
		row.Text = timeline.Compact(each.prompt)
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// taskHistory emits the "task:" row of each entry of state/taskHistory.json.
func (p *Parser) taskHistory(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	for position, item := range parser.JSONArray(artifact.DiskPath, &problems) {
		if item.IsObject() {
			emit(p.HistoryRow(artifact, item, position))
		}
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// syncQueue emits one "enterprise prompt upload <status>" row per queued
// task file of the extension's cache/sync-queue.json, prefixed "session
// synced: " only when its status is synced. The items' data and lastError
// are never read.
func (p *Parser) syncQueue(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	document, _ := parser.LoadJSON(artifact.DiskPath, &problems)
	items := document.Get("items")
	if !items.IsObject() {
		if len(problems) == 0 {
			problems.Add(0, "sync-queue.json is not a JSON object with an items object")
		}
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	for i, member := range items.Members() {
		item := member.Value
		if !item.IsObject() {
			problems.Add(i+1, "sync-queue item "+member.Key+" is not a JSON object")
			continue
		}
		row := p.SystemRow(artifact, "enterprise prompt upload "+dash(item.Get("status"))+" key "+
			dash(item.Get("key"))+" retries "+dash(item.Get("retryCount"))+
			" (from saoudrizwan.claude-dev/cache/sync-queue.json, item timestamp)")
		row.SourceLine = i + 1
		row.SessionID = parser.Str(item.Get("taskId"))
		if row.SessionID != "" {
			history := p.HistoryItem(clinelegacy.Task{Artifact: artifact, TaskID: row.SessionID,
				RootLevels: 1})
			row.ProjectPath = parser.Str(history.Get(p.ProjectKey()))
		}
		row.TimestampUTC = timeutil.FromValue(item.Get("timestamp"))
		if isText(item.Get("status"), "synced") {
			row.Text = "session synced: " + row.Text
		}
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}
