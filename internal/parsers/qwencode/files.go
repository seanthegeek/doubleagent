package qwencode

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
func readObject(artifact *input.Artifact, problems *parserkit.Problems) (jsonvalue.Value, bool) {
	data, ok := parserkit.FileBytes(artifact.DiskPath, problems)
	if !ok {
		return jsonvalue.NewNull(), false
	}
	document, err := parserkit.DecodeJSONBytes(artifact, data)
	if err != nil || !document.IsObject() {
		problems.Add(0, fileName(artifact)+" is not a JSON object")
		return jsonvalue.NewNull(), false
	}
	return document, true
}

// tokenUsage emits one "token usage:" row per record of a monthly token
// usage log, at its time. No delete touches the log, so it keeps every
// response of a deleted session.
func (p *Parser) tokenUsage(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	for line, record := range parserkit.JSONLines(artifact.DiskPath, &problems) {
		var counts []string
		for _, pair := range [][2]string{{"input", "inputTokens"}, {"output", "outputTokens"},
			{"cached", "cachedTokens"}, {"thoughts", "thoughtsTokens"}, {"total", "totalTokens"}} {
			counts = append(counts, pair[0]+" "+parserkit.SettingOrDash(record.Get(pair[1])))
		}
		duration := "-"
		if value := record.Get("apiDurationMs"); parserkit.Present(value) {
			duration = value.Text() + "ms"
		}
		text := "token usage: model " + parserkit.SettingOrDash(record.Get("model")) + " auth " +
			parserkit.SettingOrDash(record.Get("authType")) + " source " + parserkit.SettingOrDash(record.Get("source")) + " tokens " +
			strings.Join(counts, " ") + " api " + duration + " (from " + fileName(artifact) + ")"
		row := p.systemRow(artifact, line, timeutil.FromValue(record.Get("timestamp")), text)
		row.SessionID = parserkit.Text(record.Get("sessionId"))
		// "unknown" is Qwen Code's placeholder for a response naming no model.
		if model := parserkit.Text(record.Get("model")); model != "unknown" {
			row.Model = model
		}
		emit(row)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
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
		key := parserkit.JSONText(each.record.Get("key"))
		if parserkit.IsText(each.record.Get("type"), "clear") {
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
func (p *Parser) groupHistory(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	var records []numbered
	for line, record := range parserkit.JSONLines(artifact.DiskPath, &problems) {
		records = append(records, numbered{line, record})
	}
	clearedAt := clearTimes(records)
	for _, each := range records {
		record, entry := each.record, each.record.Get("entry")
		var text, timestamp string
		switch {
		case parserkit.IsText(record.Get("type"), "message") && entry.IsObject():
			state := "waiting"
			if cleared, ok := clearedAt[each.line]; ok && cleared != "" {
				state = "cleared " + cleared
			} else if ok {
				state = "cleared, time not recorded"
			}
			sender := parserkit.Text(parserkit.FirstNonEmpty(entry.Get("senderName"), entry.Get("senderId")))
			text = "group message: [" + sender + "] " + parserkit.Text(entry.Get("text")) + " (" + state + ")"
			timestamp = timeutil.FromValue(entry.Get("timestamp"))
		case parserkit.IsText(record.Get("type"), "clear"):
			text = "group history cleared: " + cmp.Or(parserkit.Text(record.Get("key")), "-") + " (from " +
				fileName(artifact) + ", recordedAt)"
			timestamp = timeutil.FromValue(record.Get("recordedAt"))
		default:
			problems.AddRaw(each.line, "unparseable line (not a group history record)",
				parserkit.JSONText(record))
			continue
		}
		emit(p.systemRow(artifact, each.line, timestamp, text))
	}
	// The reader's bad lines and the records of no known type, in line order.
	slices.SortStableFunc(problems, func(left, right parserkit.Problem) int {
		return left.Line - right.Line
	})
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// summary emits the one "project summary" row of a .qwen/PROJECT_SUMMARY.md:
// the whole file at its modification time, in the project whose .qwen/
// holds it.
func (p *Parser) summary(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	project := artifact.Home.Original
	data, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
		return
	}
	row := p.systemRow(artifact, 1, parserkit.FileMtime(artifact),
		"project summary (from .qwen/PROJECT_SUMMARY.md, file mtime): "+textlines.ValidUTF8(data))
	row.ProjectPath = project
	emit(row)
}

// todos emits one "todo:" row per item of a session's todo file, at the
// file's time. The session is the file's sessionId, else its name;
// "default", the file a process without a session id writes, names none.
// A todos that is not an array adds a problem.
func (p *Parser) todos(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	name := parserkit.Stem(fileName(artifact))
	sessionID := name
	if name == "default" {
		sessionID = ""
	}
	document, ok := readObject(artifact, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, ""))
		return
	}
	if sessionID = parserkit.Text(document.Get("sessionId")); sessionID == "" {
		sessionID = name
	}
	if sessionID == "default" {
		sessionID = ""
	}
	mtime := parserkit.FileMtime(artifact)
	for i, item := range parserkit.ArrayMember(document.Get("todos"), "todos", 0, &problems) {
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
		text := "todo: [" + parserkit.SettingOrDash(item.Get("status")) + "] " + parserkit.Text(item.Get("content")) +
			" (id " + parserkit.SettingOrDash(item.Get("id")) + ", blocked by " + blockedBy + ", from todos/" +
			fileName(artifact) + ", file mtime)"
		row := p.systemRow(artifact, i+1, mtime, text)
		row.SessionID = sessionID
		emit(row)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, ""))
}

// groupNames returns the Web Shell's group names by id: each group with an
// id, by its name, else its id; a later group of the same id wins. Groups
// that are not objects add a problem.
func groupNames(groups jsonvalue.Value, problems *parserkit.Problems) map[string]string {
	names := map[string]string{}
	for _, group := range parserkit.EntriesOfKind(groups, "groups", jsonvalue.Object, 0, problems) {
		if groupID := group.Get("id"); !groupID.IsEmpty() {
			names[groupID.Text()] = parserkit.FirstNonEmpty(group.Get("name"), groupID).Text()
		}
	}
	return names
}

// organization emits one "session organization:" row per session of the
// Web Shell's grouping file: its group (by name when the file lists it),
// colour and pin time, at the entry's updatedAt. A sessions that is not an
// object adds a problem.
func (p *Parser) organization(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	document, ok := readObject(artifact, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	names := groupNames(document.Get("groups"), &problems)
	carried := parserkit.NewCarried(artifact)
	for i, session := range parserkit.ObjectMember(document.Get("sessions"), "sessions", 0, &problems) {
		entry := session.Value
		if !entry.IsObject() {
			problems.Add(i+1, "session "+session.Key+" is not a JSON object")
			continue
		}
		group := "-"
		if groupID := entry.Get("groupId"); !groupID.IsEmpty() {
			group = groupID.Text()
			if name, ok := names[group]; ok {
				// A group's name is written into each of its sessions' rows.
				group = carried.Cut("group name", name)
			}
		}
		pinned := parserkit.Text(entry.Get("pinnedAt"))
		if pinned == "" {
			pinned = "no"
		}
		text := "session organization: group " + group + " color " + parserkit.SettingOrDash(entry.Get("color")) +
			" pinned " + pinned + " (from session-organization.v1.json, updatedAt)"
		row := p.systemRow(artifact, i+1, timeutil.FromValue(entry.Get("updatedAt")), text)
		row.SessionID = session.Key
		emit(row)
	}
	carried.Report(0, &problems)
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}
