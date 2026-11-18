package geminicli

import (
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/reason"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// promptIDSeparator separates the session id from the prompt count in a
// prompt id, <session id>########<count>.
const promptIDSeparator = "########"

// fromName is the note of a row timed by the file's name.
const fromName = ", time in the file name)"

// where is the line, time, session and project of a system row.
type where struct {
	line                          int
	timestamp, sessionID, project string
}

// systemRow returns a system row of the artifact at a place.
func (p *Parser) systemRow(artifact *input.Artifact, place where, text string) timeline.Row {
	row := p.SystemRow(artifact, timeline.Compact(text))
	row.SourceLine, row.TimestampUTC = place.line, place.timestamp
	row.SessionID, row.ProjectPath = place.sessionID, place.project
	return row
}

// request is the method and query-less URL of a network request of the
// activity log.
type request struct {
	method, url string
}

// activityLog emits the rows of an activity log logs/session-<id>.jsonl: a
// console record as "console <type>: <content>", a network record naming a
// method as "network request:", one with a response as "network
// response:" and one with an error as "network error:"; a streamed chunk
// gives none. Header values and bodies are never read into a row.
func (p *Parser) activityLog(artifact *input.Artifact, match pathMatch, emit parser.Emit) {
	var problems parser.Problems
	project := projectOf(artifact, match, &problems)
	requests := map[string]request{}
	for line, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		payload, kind := record.Get("payload"), record.Get("type")
		if !payload.IsObject() || !parser.IsText(kind, "console") && !parser.IsText(kind, "network") {
			problems.AddRaw(line, "unparseable line (not an activity log record)",
				parser.JSONText(record))
			continue
		}
		text, ok := activityText(payload, parser.IsText(kind, "console"), requests)
		if !ok {
			continue
		}
		place := where{line, timeutil.FromValue(record.Get("timestamp")),
			parser.Text(record.Get("sessionId")), project}
		emit(p.systemRow(artifact, place, text))
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, match["session"], project))
}

// activityText returns the row text of one activity log record, or false
// for a record that gives no row, remembering each request it names.
func activityText(payload jsonvalue.Value, console bool, requests map[string]request) (string, bool) {
	requestID := parser.Text(payload.Get("id"))
	answered, known := requests[requestID]
	if !known {
		answered = request{"-", "-"}
	}
	switch value := payload.Get("response"); {
	case console:
		return "console " + orDash(payload.Get("type")) + ": " + parser.Text(payload.Get("content")), true
	case payload.Get("method").IsString():
		named := request{payload.Get("method").StringValue(), urlWithoutQuery(parser.Text(payload.Get("url")))}
		requests[requestID] = named
		return "network request: " + named.method + " " + named.url + " (request " + requestID + ")", true
	case value.IsObject():
		return "network response: " + answered.method + " " + answered.url + " status " +
			parser.SettingOrDash(value.Get("status")) + " in " + parser.SettingOrDash(value.Get("durationMs")) +
			" ms (request " + requestID + ")", true
	case !payload.Get("error").IsEmpty():
		return "network error: " + answered.method + " " + answered.url + ": " +
			payload.Get("error").Text() + " (request " + requestID + ")", true
	}
	return "", false
}

// jsonFile returns a whole-JSON file's value, or false after adding why it
// is none: "unparseable <label> (<reason>)" for text that is not JSON.
func jsonFile(artifact *input.Artifact, label string, problems *parser.Problems) (jsonvalue.Value,
	bool) {
	data, ok := parser.FileBytes(artifact.DiskPath, problems)
	if !ok {
		return jsonvalue.NewNull(), false
	}
	document, err := jsonvalue.Decode(data)
	if err != nil {
		problems.Add(0, "unparseable "+label+" ("+err.Error()+")")
		return jsonvalue.NewNull(), false
	}
	return document, true
}

// toolCheckpoint emits a "tool checkpoint:" row timed by the file's name,
// then the rows of its clientHistory items, which carry no time.
func (p *Parser) toolCheckpoint(artifact *input.Artifact, options parser.Options, match pathMatch,
	emit parser.Emit) {
	var problems parser.Problems
	project := projectOf(artifact, match, &problems)
	document, ok := jsonFile(artifact, "tool checkpoint", &problems)
	// A file holding null is no document: no row and no problem.
	if ok && !document.IsNull() && !document.IsObject() {
		problems.Add(0, "not a tool checkpoint: not a JSON object")
	}
	if !document.IsObject() {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
		return
	}
	promptID := parser.Text(document.Get("messageId"))
	sessionID := ""
	if strings.Contains(promptID, promptIDSeparator) {
		sessionID, _, _ = strings.Cut(promptID, promptIDSeparator)
	}
	toolCall := document.Get("toolCall")
	if !toolCall.IsObject() {
		toolCall = jsonvalue.NewNull()
	}
	text := "tool checkpoint: " + orDash(toolCall.Get("name")) + " " +
		orDash(toolCall.Get("args").Get("file_path")) + " commit " +
		orDash(document.Get("commitHash")) + " prompt " + orDash(document.Get("messageId")) +
		" (from " + artifact.Rel + fromName
	emit(p.systemRow(artifact, where{0, nameTime(match["time"]), sessionID, project}, text))
	p.itemRows(artifact, options, document.Get("clientHistory").Items(), project, sessionID,
		&problems, emit)
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, project))
}

// memoryFile emits one "memory file:" row holding the file's text, at its
// modification time; a file that is not UTF-8 text is a problem.
func (p *Parser) memoryFile(artifact *input.Artifact, match pathMatch, emit parser.Emit) {
	var problems parser.Problems
	project := projectOf(artifact, match, &problems)
	if data, ok := parser.FileBytes(artifact.DiskPath, &problems); ok {
		if utf8.Valid(data) {
			text := "memory file: " + match["name"] + " " + fromFile(artifact) + "\n" + string(data)
			emit(standingRow(p.Base, artifact, parser.FileMtime(artifact), project, text))
		} else {
			problems.Add(0, match["name"]+" is not UTF-8 text")
		}
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
}

// clipboardImage emits one "clipboard image:" row naming the file and its
// size, never its content, timed by the file's name.
func (p *Parser) clipboardImage(artifact *input.Artifact, match pathMatch, emit parser.Emit) {
	var problems parser.Problems
	project := projectOf(artifact, match, &problems)
	size := artifact.Manifest.Get("size")
	sizeText := size.Text()
	if !size.IsInt() {
		if info, err := osfile.Lstat(artifact.DiskPath); err != nil {
			problems.Add(0, reason.Unreadable(artifact.DiskPath, err))
			sizeText = "-"
		} else {
			sizeText = strconv.FormatInt(info.Size(), 10)
		}
	}
	name := artifact.Rel[strings.LastIndex(artifact.Rel, "/")+1:]
	text := "clipboard image: " + name + " " + sizeText + " bytes (from " + artifact.Rel + fromName
	emit(standingRow(p.Base, artifact, timeutil.FromText(match["ms"]), project, text))
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
}

// errorReport emits an "error report:" row timed by the file's name, then
// the rows of the request's history items, which carry no time. The
// context is the Content list sent, or an object whose requestContents
// holds it.
func (p *Parser) errorReport(artifact *input.Artifact, options parser.Options, match pathMatch,
	emit parser.Emit) {
	var problems parser.Problems
	project := projectOf(artifact, match, &problems)
	document, ok := jsonFile(artifact, "error report", &problems)
	report := document.Get("error")
	if ok && !document.IsNull() && !report.IsObject() {
		problems.Add(0, "not an error report: no error object")
	}
	if !report.IsObject() {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
		return
	}
	text := "error report: " + match["type"] + ": " + parser.Text(report.Get("message")) +
		" (from " + artifact.Rel + fromName
	if stack := report.Get("stack"); stack.IsString() && stack.StringValue() != "" {
		text += "\n" + stack.StringValue()
	}
	emit(p.systemRow(artifact, where{0, nameTime(match["time"]), "", project}, text))
	context := document.Get("context")
	if context.IsObject() {
		context = context.Get("requestContents")
	}
	p.itemRows(artifact, options, context.Items(), project, "", &problems, emit)
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
}

// logs emits one user row per record of a logs.json, a secondary source.
func (p *Parser) logs(artifact *input.Artifact, match pathMatch, emit parser.Emit) {
	var problems parser.Problems
	project := resolveProject(inBase(artifact, match), match["slug"], "", &problems)
	if data, ok := parser.FileBytes(artifact.DiskPath, &problems); ok {
		p.logRows(artifact, project, data, emit)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
}

// logRows emits one user row per object of a logs.json array, numbered by
// its 1-based index, or the row saying the file is not an array.
func (p *Parser) logRows(artifact *input.Artifact, project string, data []byte, emit parser.Emit) {
	place := where{project: project}
	records, err := jsonvalue.Decode(data)
	switch {
	case err != nil:
		emit(p.systemRow(artifact, place, "parser: unparseable logs.json: "+err.Error()))
		return
	case !records.IsArray():
		emit(p.systemRow(artifact, place, "parser: logs.json is not a JSON array"))
		return
	}
	for i, record := range records.Items() {
		if !record.IsObject() {
			continue
		}
		row := p.BaseRow(artifact)
		row.SourceLine = i + 1
		row.TimestampUTC = timeutil.FromValue(record.Get("timestamp"))
		row.SessionID = parser.Text(record.Get("sessionId"))
		row.ProjectPath = project
		row.TurnType = timeline.TurnUser
		row.Text = timeline.CompactValue(record.Get("message"))
		emit(row)
	}
}
