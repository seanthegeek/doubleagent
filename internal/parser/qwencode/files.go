package qwencode

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// systemRow returns a system row of the artifact at a line and time.
func (p *Parser) systemRow(artifact *input.Artifact, line int, timestamp,
	text string) timeline.Row {
	row := p.SystemRow(artifact, timeline.Compact(text))
	row.SourceLine, row.TimestampUTC = line, timestamp
	return row
}

// readObject returns a JSON file's object, or false after adding its
// failure: the unreadable problem, or "<file> is not a JSON object".
func readObject(artifact *input.Artifact, problems *parser.Problems) (jsonvalue.Value, bool) {
	data, ok := parser.FileBytes(artifact.DiskPath, problems)
	if !ok {
		return jsonvalue.NewNull(), false
	}
	document, err := jsonvalue.Decode(data)
	if err != nil || !document.IsObject() {
		problems.Add(0, fileName(artifact)+" is not a JSON object")
		return jsonvalue.NewNull(), false
	}
	return document, true
}

// tokenUsage emits one "token usage:" row per record of a monthly token
// usage log, at its time. No delete touches the log, so it keeps every
// response of a deleted session.
func (p *Parser) tokenUsage(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	for line, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		var counts []string
		for _, pair := range [][2]string{{"input", "inputTokens"}, {"output", "outputTokens"},
			{"cached", "cachedTokens"}, {"thoughts", "thoughtsTokens"}, {"total", "totalTokens"}} {
			counts = append(counts, pair[0]+" "+parser.SettingOrDash(record.Get(pair[1])))
		}
		duration := "-"
		if value := record.Get("apiDurationMs"); isSet(value) {
			duration = value.Text() + "ms"
		}
		text := "token usage: model " + parser.SettingOrDash(record.Get("model")) + " auth " +
			parser.SettingOrDash(record.Get("authType")) + " source " + parser.SettingOrDash(record.Get("source")) + " tokens " +
			strings.Join(counts, " ") + " api " + duration + " (from " + fileName(artifact) + ")"
		row := p.systemRow(artifact, line, timeutil.FromValue(record.Get("timestamp")), text)
		row.SessionID = parser.Text(record.Get("sessionId"))
		// "unknown" is Qwen Code's placeholder for a response naming no model.
		if model := parser.Text(record.Get("model")); model != "unknown" {
			row.Model = model
		}
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// numbered is one record of a JSON lines file and its line.
type numbered struct {
	line   int
	record jsonvalue.Value
}

// clearTimes returns, for each message line of a group history, the
// recordedAt of the next clear of its chat's key; a line no clear follows
// is absent, and a clear without a time gives "".
func clearTimes(records []numbered) map[int]string {
	nextClear := map[string]string{}
	clearedAt := map[int]string{}
	for _, each := range slices.Backward(records) {
		key := parser.JSONText(each.record.Get("key"))
		if isText(each.record.Get("type"), "clear") {
			nextClear[key] = timeutil.FromValue(each.record.Get("recordedAt"))
		} else if cleared, ok := nextClear[key]; ok {
			clearedAt[each.line] = cleared
		}
	}
	return clearedAt
}

// groupHistory emits the rows of a channel's <channel>-group-history.jsonl,
// a secondary source: a message record as "group message: [<sender>] <text>
// (cleared <time>)" at the message's time, the suffix "(cleared, time not
// recorded)" when the next clear of its key has no time and "(waiting)"
// when none follows; a clear record as a "group history cleared:" row.
func (p *Parser) groupHistory(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	var records []numbered
	for line, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		records = append(records, numbered{line, record})
	}
	clearedAt := clearTimes(records)
	for _, each := range records {
		record, entry := each.record, each.record.Get("entry")
		var text, timestamp string
		switch {
		case isText(record.Get("type"), "message") && entry.IsObject():
			state := "waiting"
			if cleared, ok := clearedAt[each.line]; ok && cleared != "" {
				state = "cleared " + cleared
			} else if ok {
				state = "cleared, time not recorded"
			}
			sender := parser.Text(parser.FirstNonEmpty(entry.Get("senderName"), entry.Get("senderId")))
			text = "group message: [" + sender + "] " + parser.Text(entry.Get("text")) + " (" + state + ")"
			timestamp = timeutil.FromValue(entry.Get("timestamp"))
		case isText(record.Get("type"), "clear"):
			text = "group history cleared: " + cmp.Or(parser.Text(record.Get("key")), "-") + " (from " +
				fileName(artifact) + ", recordedAt)"
			timestamp = timeutil.FromValue(record.Get("recordedAt"))
		default:
			problems.AddRaw(each.line, "unparseable line (not a group history record)",
				parser.JSONText(record))
			continue
		}
		emit(p.systemRow(artifact, each.line, timestamp, text))
	}
	// The reader's bad lines and the records of no known type, in line order.
	slices.SortStableFunc(problems, func(left, right parser.Problem) int {
		return left.Line - right.Line
	})
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// summary emits the one "project summary" row of a .qwen/PROJECT_SUMMARY.md:
// the whole file at its modification time, in the project whose .qwen/
// holds it.
func (p *Parser) summary(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	project := artifact.Home.Original
	data, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
		return
	}
	row := p.systemRow(artifact, 1, parser.FileMtime(artifact),
		"project summary (from .qwen/PROJECT_SUMMARY.md, file mtime): "+textlines.ValidUTF8(data))
	row.ProjectPath = project
	emit(row)
}

// todos emits one "todo:" row per item of a session's todo file, at the
// file's time. The session is the file's sessionId, else its name;
// "default", the file a process without a session id writes, names none.
func (p *Parser) todos(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	name := parser.Stem(fileName(artifact))
	sessionID := name
	if name == "default" {
		sessionID = ""
	}
	document, ok := readObject(artifact, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, ""))
		return
	}
	if sessionID = parser.Text(document.Get("sessionId")); sessionID == "" {
		sessionID = name
	}
	if sessionID == "default" {
		sessionID = ""
	}
	mtime := parser.FileMtime(artifact)
	for i, item := range document.Get("todos").Items() {
		if !item.IsObject() {
			problems.Add(i+1, "todo "+strconv.Itoa(i+1)+" is not a JSON object")
			continue
		}
		var blocked []string
		for _, each := range item.Get("blockedBy").Items() {
			blocked = append(blocked, each.Text())
		}
		blockedBy := strings.Join(blocked, ",")
		if blockedBy == "" {
			blockedBy = "-"
		}
		text := "todo: [" + parser.SettingOrDash(item.Get("status")) + "] " + parser.Text(item.Get("content")) +
			" (id " + parser.SettingOrDash(item.Get("id")) + ", blocked by " + blockedBy + ", from todos/" +
			fileName(artifact) + ", file mtime)"
		row := p.systemRow(artifact, i+1, mtime, text)
		row.SessionID = sessionID
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, ""))
}

// groupNames returns the Web Shell's group names by id: each group with an
// id, by its name, else its id; a later group of the same id wins.
func groupNames(groups jsonvalue.Value) map[string]string {
	names := map[string]string{}
	for _, group := range groups.Items() {
		if groupID := group.Get("id"); group.IsObject() && !groupID.IsEmpty() {
			names[groupID.Text()] = parser.FirstNonEmpty(group.Get("name"), groupID).Text()
		}
	}
	return names
}

// organization emits one "session organization:" row per session of the
// Web Shell's grouping file: its group (by name when the file lists it),
// colour and pin time, at the entry's updatedAt.
func (p *Parser) organization(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	document, ok := readObject(artifact, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	names := groupNames(document.Get("groups"))
	for i, session := range document.Get("sessions").Members() {
		entry := session.Value
		if !entry.IsObject() {
			problems.Add(i+1, "session "+session.Key+" is not a JSON object")
			continue
		}
		group := "-"
		if groupID := entry.Get("groupId"); !groupID.IsEmpty() {
			group = groupID.Text()
			if name, ok := names[group]; ok {
				group = name
			}
		}
		pinned := parser.Text(entry.Get("pinnedAt"))
		if pinned == "" {
			pinned = "no"
		}
		text := "session organization: group " + group + " color " + parser.SettingOrDash(entry.Get("color")) +
			" pinned " + pinned + " (from session-organization.v1.json, updatedAt)"
		row := p.systemRow(artifact, i+1, timeutil.FromValue(entry.Get("updatedAt")), text)
		row.SessionID = session.Key
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}
