package qwencode

import (
	"cmp"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// pullRequests emits one "source host:" row per pull request a session's
// .pr.json binds, its host and repository taken from the pull request's
// url, at its createdAt. A prs or issues that
// is not an array, or issues holding entries that are not objects, adds a
// problem.
func (p *Parser) pullRequests(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	name := fileName(artifact)
	sessionID := pullRequestsPattern.FindStringSubmatch(runtimeRel(artifact.Rel))[1]
	document, _ := readObject(artifact, &problems)
	for i, pullRequest := range parserkit.ArrayMember(document.Get("prs"), "prs", 0, &problems) {
		if !pullRequest.IsObject() {
			problems.Add(i+1, "pull request "+strconv.Itoa(i+1)+" is not a JSON object")
			continue
		}
		url := parserkit.Text(pullRequest.Get("url"))
		address := splitURL(url)
		var segments []string
		for segment := range strings.SplitSeq(address.path, "/") {
			if segment != "" {
				segments = append(segments, segment)
			}
		}
		repository := "-"
		if len(segments) >= 2 {
			repository = segments[0] + "/" + segments[1]
		}
		text := "source host: " + cmp.Or(address.hostname, "-") + " repository " + repository +
			" pull request " + cmp.Or(url, "-") + " state " +
			parserkit.SettingOrDash(pullRequest.Get("state")) + " source " + parserkit.SettingOrDash(pullRequest.Get("source")) +
			" issues " + cmp.Or(issueNumbers(pullRequest.Get("issues"), i+1, &problems), "-") +
			" (from " + name + ", createdAt)"
		row := p.systemRow(artifact, i+1, timeutil.FromValue(pullRequest.Get("createdAt")), text)
		row.SessionID = sessionID
		emit(row)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, ""))
}

// issueNumbers returns the numbers of a pull request's issues, joined by
// commas. An issues that is not an array, or that holds entries that are
// not objects, adds a problem at line.
func issueNumbers(issues jsonvalue.Value, line int, problems *parserkit.Problems) string {
	var numbers []string
	for _, issue := range parserkit.EntriesOfKind(issues, "issues", jsonvalue.Object, line, problems) {
		if number := issue.Get("number"); !number.IsNull() {
			numbers = append(numbers, number.Text())
		}
	}
	return strings.Join(numbers, ",")
}

// audit emits one "source host: github" row per publication record of the
// GitHub channel. The actor is the GitHub login the reply answered, where
// recorded, and the comment's URL is shown; the
// reply's body is recorded only as its hash and length, left out.
func (p *Parser) audit(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	for line, record := range parserkit.JSONLines(artifact.DiskPath, &problems) {
		failure := ""
		if !record.Get("failurePhase").IsEmpty() || !record.Get("failureError").IsEmpty() {
			failure = " failure " + parserkit.SettingOrDash(record.Get("failurePhase")) + ": " +
				parserkit.SettingOrDash(record.Get("failureError"))
		}
		text := "source host: github repository " + parserkit.SettingOrDash(record.Get("repository")) + " actor " +
			parserkit.SettingOrDash(record.Get("actor")) + " publication " + parserkit.SettingOrDash(record.Get("outcome")) + " trigger " +
			parserkit.SettingOrDash(record.Get("triggerKind")) + " number " + parserkit.SettingOrDash(record.Get("number")) + " comment " +
			parserkit.SettingOrDash(record.Get("commentUrl")) + failure + " (from " + fileName(artifact) + ", at)"
		row := p.systemRow(artifact, line, timeutil.FromValue(record.Get("at")), text)
		row.SessionID = parserkit.Text(record.Get("sessionId"))
		emit(row)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// channelRoutes emits one "session relayed:" row per entry of a channel
// routes file, named in the row text as name, at the entry's startedAt or
// else the file's modification time.
func (p *Parser) channelRoutes(artifact *input.Artifact, name string, emit parserkit.Emit) {
	var problems parserkit.Problems
	data, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	document, err := parserkit.DecodeJSONBytes(artifact, data)
	if err != nil || !document.IsObject() {
		problems.Add(0, name+" is not a JSON object")
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	mtime := ""
	for i, member := range document.Members() {
		entry := member.Value
		if !entry.IsObject() {
			problems.Add(i+1, "entry "+member.Key+" is not a JSON object")
			continue
		}
		target := entry.Get("target")
		row := p.SystemRow(artifact, "")
		row.SourceLine = i + 1
		row.SessionID = parserkit.Text(entry.Get("sessionId"))
		row.ProjectPath = parserkit.Text(entry.Get("cwd"))
		row.TimestampUTC = timeutil.FromValue(entry.Get("startedAt"))
		when := "startedAt"
		if row.TimestampUTC == "" {
			if mtime == "" {
				mtime = parserkit.FileMtime(artifact)
			}
			row.TimestampUTC, when = mtime, "file mtime"
		}
		row.Text = timeline.RelayedPrefix + " channel " + parserkit.SettingOrDash(target.Get("channelName")) + " chat " +
			parserkit.SettingOrDash(target.Get("chatId")) + " sender " + parserkit.SettingOrDash(target.Get("senderId")) + " thread " +
			parserkit.SettingOrDash(target.Get("threadId")) + " (from " + name + ", " + when + ")"
		emit(row)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// settings emits one "permission setting:" row per permissions.allow, ask
// and deny rule and one for tools.approvalMode of a settings.json, read as
// JSON with comments as Qwen Code reads it. No other key is read, since the
// file can hold provider keys and channel tokens. A permissions that is not
// an object, or a list of it that is not an array, adds a problem.
func (p *Parser) settings(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	data, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	timestamp, project := parserkit.FileMtime(artifact), ""
	if artifact.Home.Project {
		project = artifact.Home.Original
	}
	document, err := parserkit.ParseJSONC(artifact, data)
	if err != nil || !document.IsObject() {
		problems.Add(0, artifact.Rel+" is not a JSON object")
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
		return
	}
	var keys []string
	var values []jsonvalue.Value
	permissions := document.Get("permissions")
	if !permissions.IsObject() && !permissions.IsNull() {
		problems.Add(0, parserkit.WrongKind("permissions", jsonvalue.Object))
	}
	for _, list := range []string{"allow", "ask", "deny"} {
		member := "permissions." + list
		for _, entry := range parserkit.ArrayMember(permissions.Get(list), member, 0, &problems) {
			keys, values = append(keys, member), append(values, entry)
		}
	}
	if mode := document.Get("tools").Get("approvalMode"); !mode.IsNull() {
		keys, values = append(keys, "tools.approvalMode"), append(values, mode)
	}
	for i, key := range keys {
		row := p.systemRow(artifact, 1, timestamp, "permission setting: "+key+"="+
			parserkit.SettingValue(values[i])+" (from "+artifact.Rel+", file mtime)")
		row.ProjectPath = project
		emit(row)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
}

// usage emits one "session usage:" row per line of a usage_record.jsonl,
// at its startTime. The file survives the session's delete.
func (p *Parser) usage(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	for line, record := range parserkit.JSONLines(artifact.DiskPath, &problems) {
		var counts []string
		for _, tool := range record.Get("tools").Get("byName").Members() {
			count := tool.Value
			if count.IsObject() {
				count = count.Get("count")
			}
			counts = append(counts, tool.Key+"="+count.Text())
		}
		duration := "-"
		if value := record.Get("durationMs"); parserkit.Present(value) {
			duration = value.Text() + "ms"
		}
		models := "-"
		if record.Get("models").Len() > 0 && record.Get("models").IsObject() {
			models = strings.Join(record.Get("models").Keys(), ",")
		}
		project := parserkit.Text(record.Get("project"))
		row := p.systemRow(artifact, line, timeutil.FromValue(record.Get("startTime")),
			"session usage: project "+cmp.Or(project, "-")+" duration "+duration+" models "+models+
				" tools "+cmp.Or(strings.Join(counts, ","), "-")+" (from usage_record.jsonl, startTime)")
		row.SessionID, row.ProjectPath = parserkit.Text(record.Get("sessionId")), project
		emit(row)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}
