package twinny

import (
	"path"
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// logHeader matches the first line of a log entry: its host-local time,
// level and message. The digits are ASCII only.
var logHeader = regexp.MustCompile(
	`^([0-9]{4}-[0-9][0-9]-[0-9][0-9] [0-9][0-9]:[0-9][0-9]:[0-9][0-9]\.[0-9]{3}) ` +
		`\[(trace|debug|info|warning|error)\] (.*)\z`)

// fileChangeText returns the "file changed:" text of one changed file of a
// reply: "file changed: <status> <path> +<added> -<removed> reply=<id>",
// with " undone=true" and " noUndo=true" when the entry sets them.
func fileChangeText(changeID, entry jsonvalue.Value) string {
	field := func(key string) string {
		value, ok := entry.Lookup(key)
		if !ok {
			return ""
		}
		return parser.SettingValue(value)
	}
	text := "file changed: " + field("status") + " " + field("path") + " +" + field("added") +
		" -" + field("removed") + " reply="
	if !changeID.IsNull() {
		text += parser.SettingValue(changeID)
	}
	for _, flag := range []string{"undone", "noUndo"} {
		if set, ok := entry.Get(flag).Bool(); ok && set {
			text += " " + flag + "=true"
		}
	}
	return text
}

// changes emits the "file changed:" rows of one agent-changes record; a
// files member that is not an array adds a problem.
func (p *Parser) changes(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	raw, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	record, err := jsonvalue.DecodeString(textlines.ValidUTF8(raw))
	switch {
	case err != nil:
		problems.Add(0, "agent-changes record is not valid JSON ("+err.Error()+")")
	case !record.IsObject():
		problems.Add(0, "agent-changes record is not a JSON object")
	}
	if len(problems) > 0 {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	root := ""
	if value := record.Get("root"); value.IsString() {
		root = value.StringValue()
	}
	timestamp := timeutil.FromValue(record.Get("createdAt"))
	for _, entry := range parser.ArrayMember(record.Get("files"), "files", 0, &problems) {
		if !entry.IsObject() {
			continue
		}
		row := p.systemRow(artifact, timestamp, fileChangeText(record.Get("id"), entry))
		row.ProjectPath = root
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// logEntry is one entry of a log file: its header line's number, time,
// level and message, and its continuation lines.
type logEntry struct {
	line                 int
	when, level, message string
	more                 []string
}

// log emits one system row per entry of a Twinny log channel file, then a
// row per line that is neither a header nor a continuation.
func (p *Parser) log(artifact *input.Artifact, emit parser.Emit) {
	name := path.Base(artifact.Rel)
	var problems parser.Problems
	var entries []*logEntry
	var open *logEntry // the entry continuation lines join; none after a bad line
	for number, raw := range parser.Lines(artifact.DiskPath, &problems) {
		line := parser.LineText(raw)
		if match := logHeader.FindStringSubmatch(line); match != nil {
			open = &logEntry{line: number, when: match[1], level: match[2], message: match[3]}
			entries = append(entries, open)
		} else if open != nil && (line == "" || line[0] == ' ' || line[0] == '\t') {
			open.more = append(open.more, strings.TrimPrefix(line, "    "))
		} else if line != "" {
			problems.AddRaw(number, "unparseable line (no log line header)", line)
			open = nil
		}
	}
	for _, entry := range entries {
		head := "log " + entry.level + ": " + entry.message + " (from " + name + ", log line time, host-local)"
		row := p.systemRow(artifact, timeutil.FromText(entry.when), strings.Join(append([]string{head}, entry.more...), "\n"))
		row.SourceLine = entry.line
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}
