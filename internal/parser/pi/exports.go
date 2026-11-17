package pi

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The element of export-html/template.html that holds the session data as
// base64.
const (
	sessionDataOpen  = `<script id="session-data" type="application/json">`
	sessionDataClose = "</script>"
)

// parseHTMLExport emits an HTML export's `session exported:` row, its
// header's start row and the rows of every entry of every branch. A row's
// source line is the entry's 1-based position in the embedded entries.
func (p *Parser) parseHTMLExport(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	var problems parser.Problems
	data, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	sessionData, failure := htmlSessionData(data)
	header := sessionData.Get("header")
	if failure == "" && !(header.IsObject() && isText(header.Get("type"), "session")) {
		failure = "session data has no session header"
	}
	if failure != "" {
		problems.Add(0, artifact.Rel+" "+failure)
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	session := &Session{Header: header}
	entries := sessionData.Get("entries")
	if !entries.IsArray() {
		problems.Add(0, artifact.Rel+" session data has no entries array")
	}
	for i, record := range entries.Items() {
		if record.IsObject() {
			session.Entries = append(session.Entries, Entry{Line: i + 1, Record: record})
		} else {
			problems.AddRaw(i+1, notObjectRecord(i+1), parser.CompactJSON(record))
		}
	}
	sessionID, cwd := parser.Str(header.Get("id")), parser.Str(header.Get("cwd"))
	emit(p.systemRow(artifact, 0, parser.FileMtime(artifact), sessionID, cwd, fmt.Sprintf(
		"session exported: HTML /export of session %s, %d entries of every branch, leaf %s (from %s, "+
			"file mtime)", sessionID, len(session.Entries), parser.SettingOrDash(sessionData.Get("leafId")),
		artifact.Rel)))
	emit(p.systemRow(artifact, 0, timeutil.FromValue(header.Get("timestamp")), sessionID, cwd,
		SessionStartText(header)))
	p.copyRows(artifact, session, options, emit)
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, cwd))
}

// htmlSessionData returns the session data an HTML export embeds, or why it
// could not be read: the base64 text of the session-data element, UTF-8
// JSON once decoded, an object.
func htmlSessionData(data []byte) (pyjson.Value, string) {
	start := bytes.Index(data, []byte(sessionDataOpen))
	if start < 0 {
		return pyjson.NewNull(), "holds no session data"
	}
	start += len(sessionDataOpen)
	end := bytes.Index(data[start:], []byte(sessionDataClose))
	if end < 0 {
		return pyjson.NewNull(), "session data has no closing tag"
	}
	decoded, err := decodeBase64(bytes.Trim(data[start:start+end], jsonSpace))
	if err != nil {
		return pyjson.NewNull(), "session data is not base64 (" + err.Error() + ")"
	}
	if !utf8.Valid(decoded) {
		return pyjson.NewNull(), "session data is not JSON (not UTF-8 text)"
	}
	value, err := pyjson.DecodeString(string(decoded))
	if err != nil {
		return pyjson.NewNull(), "session data is not JSON (" + err.Error() + ")"
	}
	if !value.IsObject() {
		return pyjson.NewNull(), "session data is not a JSON object"
	}
	return value, ""
}

// decodeBase64 decodes standard base64 with its padding and nothing else:
// a line break inside the text is an error, though the decoder would skip
// it, so that only what pi writes is read.
func decodeBase64(text []byte) ([]byte, error) {
	if found := bytes.IndexAny(text, "\r\n"); found >= 0 {
		return nil, fmt.Errorf("invalid base64 data at byte %d", found)
	}
	decoded, err := base64.StdEncoding.DecodeString(string(text))
	if err != nil {
		var corrupt base64.CorruptInputError
		if errors.As(err, &corrupt) {
			return nil, fmt.Errorf("invalid base64 data at byte %d", int64(corrupt))
		}
		return nil, err
	}
	return decoded, nil
}

// parseJSONLExport emits a JSONL export's `session exported:` row and the
// rows of its branch. Its header's timestamp is the export time, so it
// gives no `session start:` row.
func (p *Parser) parseJSONLExport(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	session := ReadSession(artifact.DiskPath)
	if !session.HasHeader() {
		parser.EmitAll(emit, p.ErrorRows(artifact, session.Problems, "", ""))
		return
	}
	header := session.Header
	sessionID, cwd := parser.Str(header.Get("id")), parser.Str(header.Get("cwd"))
	emit(p.systemRow(artifact, session.HeaderLine, timeutil.FromValue(header.Get("timestamp")),
		sessionID, cwd, fmt.Sprintf("session exported: JSONL /export of the current branch of "+
			"session %s, %d entries (from %s, header timestamp)", sessionID, len(session.Entries),
			artifact.Rel)))
	p.copyRows(artifact, session, options, emit)
	parser.EmitAll(emit, p.ErrorRows(artifact, session.Problems, sessionID, cwd))
}

// copyRows emits the entry rows of an export's session, as copies of its
// session file's. No `model:` row is given: its place depends on which
// entries a branch holds, and the session file gives it; each row keeps
// its model.
func (p *Parser) copyRows(artifact *input.Artifact, session *Session, options parser.Options,
	emit parser.Emit) {
	header := session.Header
	sessionID, cwd := parser.Str(header.Get("id")), parser.Str(header.Get("cwd"))
	entryRows(p.Base, artifact, session, options, sessionID, cwd, nil, func(row timeline.Row) {
		if !(row.TurnType == timeline.TurnSystem && strings.HasPrefix(row.Text, "model: ")) {
			emit(row)
		}
	})
}

// parseBugZip emits a bug report zip's `bug report:` row, its crash and
// summary rows and its session's rows, which carry that member's line
// numbers.
func (p *Parser) parseBugZip(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	var problems parser.Problems
	members := bugZipMembers(artifact, &problems)
	report := bugJSONMember(artifact.Rel, members, "report.json", &problems)
	diagnostics := bugJSONMember(artifact.Rel, members, "diagnostics.json", &problems)
	reportSession := report.Get("session")
	sessionID := parser.Str(either(reportSession.Get("id"), diagnostics.Get("sessionId")))
	cwd := parser.Str(reportSession.Get("cwd"))
	created := timeutil.FromValue(report.Get("createdAt"))
	source := func(member, what string) string {
		return "(from " + artifact.Rel + " " + member + ", " + what + ")"
	}
	if report.Len() > 0 || diagnostics.Len() > 0 {
		emit(p.systemRow(artifact, 0, created, sessionID, cwd,
			bugReportRowText(report, diagnostics, source("report.json", "createdAt"))))
	}
	for _, record := range diagnostics.Get("crashes").Items() {
		if record.IsObject() {
			text, crashSession := p.crashText(record, source("diagnostics.json", "record timestamp"))
			emit(p.systemRow(artifact, 0, timeutil.FromValue(record.Get("timestamp")), crashSession,
				parser.Str(record.Get("cwd")), text))
		}
	}
	if summary, ok := members["summary.md"]; ok {
		emit(p.systemRow(artifact, 0, created, sessionID, cwd, "bug report summary: "+
			parser.SettingOrDash(report.Get("id"))+" "+source("summary.md", "report.json createdAt")+
			"\n"+pytext.DecodeUTF8(summary)))
	}
	if data, ok := members["session.jsonl"]; ok {
		session := ReadSessionBytes(data)
		problems = append(problems, session.Problems...)
		if session.HasHeader() {
			if identifier := session.Header.Get("id"); identifier.Truthy() {
				sessionID = identifier.Str()
			}
			if path := session.Header.Get("cwd"); path.Truthy() {
				cwd = path.Str()
			}
			p.copyRows(artifact, session, options, emit)
		}
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, cwd))
}

// either returns first when Python counts it true, else second: Python's
// `first or second`.
func either(first, second pyjson.Value) pyjson.Value {
	if first.Truthy() {
		return first
	}
	return second
}

// bugJSONMember returns a JSON member of a bug report zip as an object,
// null when the member is absent; a member that is not a UTF-8 JSON object
// adds a problem and gives null.
func bugJSONMember(rel string, members map[string][]byte, name string,
	problems *parser.Problems) pyjson.Value {
	data, ok := members[name]
	if !ok {
		return pyjson.NewNull()
	}
	if utf8.Valid(data) {
		if value, err := pyjson.DecodeString(string(data)); err == nil && value.IsObject() {
			return value
		}
	}
	problems.Add(0, rel+" member "+name+" is not a JSON object")
	return pyjson.NewNull()
}

// bugReportRowText returns the `bug report:` row text of a zip's
// report.json and diagnostics.json: a first line naming the report, its
// session and environment, then one line per failed assistant turn. The
// hint and the redacted settings are not written.
func bugReportRowText(report, diagnostics pyjson.Value, source string) string {
	dash := parser.SettingOrDash
	session, environment, model := report.Get("session"), report.Get("environment"), report.Get("model")
	text := fmt.Sprintf("bug report: %s session=%s transcript=%s summary=%s messages=%s entries=%s "+
		"version=%s platform=%s/%s model=%s/%s %s", dash(report.Get("id")),
		dash(either(session.Get("id"), diagnostics.Get("sessionId"))), dash(session.Get("included")),
		dash(session.Get("summaryIncluded")), dash(session.Get("messageCount")),
		dash(diagnostics.Get("entryCount")), dash(environment.Get("version")),
		dash(environment.Get("platform")), dash(environment.Get("arch")),
		dash(model.Get("provider")), dash(model.Get("id")), source)
	for _, turn := range diagnostics.Get("assistant").Items() {
		if turn.IsObject() {
			text += fmt.Sprintf("\nfailed assistant turn %s at %s: stopReason=%s %s",
				dash(turn.Get("entryId")), dash(turn.Get("timestamp")), dash(turn.Get("stopReason")),
				dash(turn.Get("errorMessage")))
		}
	}
	return text
}
