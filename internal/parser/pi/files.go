package pi

import (
	"fmt"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The lines of the debug log and the terminal UI crash log the parser
// recognises.
const (
	debugMarker    = "=== Agent messages (JSONL) ==="
	debugHeader    = "Debug output at "
	tuiCrashHeader = "Crash at "
	tuiCrashMarker = "=== All rendered lines ==="
)

// jsonSpace is the whitespace JSON allows, which a blank line holds.
const jsonSpace = " \t\n\r"

// jsonObject returns a whole-file JSON object, without a leading byte-order
// mark as pi drops it. A file that cannot be read adds its unreadable
// problem; any other failure adds "<rel> is not a JSON object". A path that
// is not a regular file gives false and no problem.
func jsonObject(artifact *input.Artifact, problems *parser.Problems) (jsonvalue.Value, bool) {
	text, ok := parser.ReadText(artifact.DiskPath, problems)
	if !ok {
		return jsonvalue.NewNull(), false
	}
	document, err := jsonvalue.DecodeString(text)
	if err != nil || !document.IsObject() {
		problems.Add(0, artifact.Rel+" is not a JSON object")
		return jsonvalue.NewNull(), false
	}
	return document, true
}

// parseTrust emits one standing row per project path of trust.json, at the
// file's modification time.
func (p *Parser) parseTrust(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	document, ok := jsonObject(artifact, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	timestamp := parser.FileMtime(artifact)
	for _, member := range document.Members() {
		emit(p.systemRow(artifact, 1, timestamp, "", member.Key,
			"permission setting: "+member.Key+"="+parser.SettingValue(member.Value)+
				" (from "+artifact.Rel+", file mtime)"))
	}
}

// parseSettings emits the defaultProjectTrust standing row of
// settings.json, at the file's modification time. An empty file is no
// settings to pi, and gives nothing.
func (p *Parser) parseSettings(artifact *input.Artifact, emit parser.Emit) {
	if info, err := osfile.Lstat(artifact.DiskPath); err == nil && info.Size() == 0 {
		return
	}
	var problems parser.Problems
	document, ok := jsonObject(artifact, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	value := document.Get("defaultProjectTrust")
	if !parser.Present(value) {
		return
	}
	emit(p.systemRow(artifact, 1, parser.FileMtime(artifact), "", "",
		"permission setting: defaultProjectTrust="+parser.SettingValue(value)+
			" (from "+artifact.Rel+", file mtime)"))
}

// parseDebugLog emits the snapshot row of pi-debug.log and the rows of the
// messages after its JSONL marker, with no session id: the log does not
// name its session. The screen lines before the marker are not read. A log
// that cannot be opened is its one unreadable row, with no snapshot row.
func (p *Parser) parseDebugLog(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	session := &Session{}
	written, inMessages, readAny := "", false, false
	for number, raw := range parser.Lines(artifact.DiskPath, &session.Problems) {
		readAny = true
		line := parser.LineText(raw)
		if number == 1 && strings.HasPrefix(line, debugHeader) {
			written = timeutil.FromText(strings.TrimSpace(line[len(debugHeader):]))
		}
		if !inMessages {
			inMessages = line == debugMarker
			continue
		}
		if strings.Trim(line, jsonSpace) != "" {
			session.addDebugMessage(number, line)
		}
	}
	if !readAny && len(session.Problems) > 0 {
		parser.EmitAll(emit, p.ErrorRows(artifact, session.Problems, "", ""))
		return
	}
	emit(p.systemRow(artifact, 1, written, "", "", fmt.Sprintf("debug snapshot: /debug wrote %d "+
		"message(s) of the open session, which the log does not name (from %s, Debug output at line)",
		len(session.Entries), artifact.Rel)))
	entryRows(p.Base, artifact, session, options, "", "", nil, emit)
	parser.EmitAll(emit, p.ErrorRows(artifact, session.Problems, "", ""))
}

// addDebugMessage adds one message line of the debug log as a message
// entry, or its problem when it is not a JSON object.
func (s *Session) addDebugMessage(number int, line string) {
	message, err := jsonvalue.DecodeString(line)
	switch {
	case err != nil:
		s.Problems.AddRaw(number, parser.UnparseableLine(err), line)
	case !message.IsObject():
		s.Problems.AddRaw(number, "unparseable line (not a JSON object)", line)
	default:
		s.Entries = append(s.Entries, Entry{Line: number, Record: jsonvalue.NewObject(
			jsonvalue.Member{Key: "type", Value: jsonvalue.NewString("message")},
			jsonvalue.Member{Key: "message", Value: message})})
	}
}

// parseCrashes emits one row per crash record of crashes.json, at its own
// time.
func (p *Parser) parseCrashes(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	for index, record := range parser.JSONArray(artifact.DiskPath, &problems) {
		if !record.IsObject() {
			problems.AddRaw(index, notObjectRecord(index), parser.JSONText(record))
			continue
		}
		text, sessionID := p.crashText(record, "(from "+artifact.Rel+", record timestamp)")
		emit(p.systemRow(artifact, index, timeutil.FromValue(record.Get("timestamp")), sessionID,
			parser.Text(record.Get("cwd")), text))
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// notObjectRecord returns the message of an array element at a 1-based
// position that is not an object.
func notObjectRecord(position int) string {
	return fmt.Sprintf("unparseable record %d (not a JSON object)", position)
}

// crashText returns a crash record's `crash:` row text, ending with source,
// and the session id its sessionFile names.
func (p *Parser) crashText(record jsonvalue.Value, source string) (string, string) {
	sessionFile := parser.Text(record.Get("sessionFile"))
	sessionID := ""
	if match := p.sessionName.FindStringSubmatch(strings.ReplaceAll(sessionFile, `\`, "/")); match != nil {
		sessionID = match[1]
	}
	field := func(key string) string { return parser.SettingOrDash(record.Get(key)) }
	text := "crash: " + field("kind") + " " + field("message") + " version=" + field("version") +
		" sessionFile=" + field("sessionFile") + " " + source
	if stack := record.Get("stack"); !stack.IsEmpty() {
		text += "\n" + stack.Text()
	}
	return text, sessionID
}

// parseMeta emits the creation row of an experimental durable session,
// whose transcript in session.sqlite is not read.
func (p *Parser) parseMeta(artifact *input.Artifact, sessionID string, emit parser.Emit) {
	var problems parser.Problems
	meta, ok := jsonObject(artifact, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, ""))
		return
	}
	cwd := parser.Text(meta.Get("cwd"))
	emit(p.systemRow(artifact, 1, timeutil.FromValue(meta.Get("createdAt")), sessionID, cwd,
		"experimental durable session created: cwd="+cwd+" (transcript in session.sqlite, not parsed)"))
}

// tuiCrash is one record of the terminal UI crash log while it is read.
type tuiCrash struct {
	line     int
	time     string
	notes    []string
	inScreen bool
	screen   []string
}

// parseTUICrash emits one `tui crash:` row per "Crash at" record, its
// rendered screen lines as text. The terminal UI rewrites the file at each
// crash, so it normally holds one record.
func (p *Parser) parseTUICrash(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	var crashes []*tuiCrash
	for number, raw := range parser.Lines(artifact.DiskPath, &problems) {
		line := parser.LineText(raw)
		if rest, ok := strings.CutPrefix(line, tuiCrashHeader); ok {
			crashes = append(crashes, &tuiCrash{line: number, time: rest})
			continue
		}
		if message := addCrashLine(crashes, line); message != "" {
			problems.AddRaw(number, message, line)
		}
	}
	for _, crash := range crashes {
		screen := crash.screen
		for len(screen) > 0 && strings.Trim(screen[len(screen)-1], jsonSpace) == "" {
			screen = screen[:len(screen)-1]
		}
		notes := strings.Join(crash.notes, "; ")
		if notes == "" {
			notes = "-"
		}
		text := "tui crash: " + notes + " (from " + artifact.Rel + ", Crash at line)"
		if len(screen) > 0 {
			text += "\n" + strings.Join(screen, "\n")
		}
		emit(p.systemRow(artifact, crash.line, timeutil.FromText(strings.Trim(crash.time, jsonSpace)),
			"", "", text))
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// addCrashLine adds a line after a "Crash at" line to the last record: a
// screen line after the marker, a note before it. It returns the problem
// message of a line that belongs nowhere, "" otherwise.
func addCrashLine(crashes []*tuiCrash, line string) string {
	blank := strings.Trim(line, jsonSpace) == ""
	if len(crashes) == 0 {
		if blank {
			return ""
		}
		return "unparseable line (before the first Crash at line)"
	}
	crash := crashes[len(crashes)-1]
	switch {
	case crash.inScreen:
		crash.screen = append(crash.screen, line)
	case line == tuiCrashMarker:
		crash.inScreen = true
	case strings.HasPrefix(line, "Terminal width: ") || strings.HasPrefix(line, "Line "):
		crash.notes = append(crash.notes, line)
	case !blank:
		return "unparseable line (not a crash header line)"
	}
	return ""
}
