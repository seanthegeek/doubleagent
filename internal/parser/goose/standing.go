package goose

import (
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// standingRow returns a system row of a standing file at a line and time.
func (p *Parser) standingRow(artifact *input.Artifact, line int, timestamp, text string) timeline.Row {
	row := p.row(artifact, line, "", "", "", timestamp)
	row.TurnType = timeline.TurnSystem
	row.Text = timeline.Compact(text)
	return row
}

// unquote returns a YAML scalar as the serialiser writes it: plain,
// single-quoted with a doubled single quote for a quote, or double-quoted
// with JSON's escapes.
func unquote(value string) string {
	if len(value) >= 2 && value[0] == '\'' && value[len(value)-1] == '\'' {
		return strings.ReplaceAll(value[1:len(value)-1], "''", "'")
	}
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		decoded, err := jsonvalue.DecodeString(value)
		if err != nil || !decoded.IsString() {
			return value[1 : len(value)-1]
		}
		return decoded.StringValue()
	}
	return value
}

// permissionEntry is one tool of permission.yaml: its line, category,
// level and tool name.
type permissionEntry struct {
	line                  int
	category, level, tool string
}

// permissionEntries returns each tool of permission.yaml. The file is a
// map from category (user, smart_approve) to always_allow, ask_before and
// never_allow, each a block list of tool names or []; only that nesting is
// read, and any other line is a bad-line problem with its raw text.
func permissionEntries(path string, problems *parser.Problems) []permissionEntry {
	var entries []permissionEntry
	category, level := "", ""
	for number, raw := range parser.Lines(path, problems) {
		text := textlines.WithoutTerminator(textlines.ValidUTF8(raw))
		body := strings.Trim(text, textlines.SpaceOrTab)
		if body == "" || strings.HasPrefix(body, "#") || body == "---" || body == "{}" {
			continue
		}
		indent := len(text) - len(strings.TrimLeft(text, " "))
		switch {
		case strings.HasPrefix(body, "- ") || body == "-":
			tool := unquote(strings.Trim(body[1:], textlines.SpaceOrTab))
			if category != "" && level != "" && tool != "" {
				entries = append(entries, permissionEntry{number, category, level, tool})
				continue
			}
		case strings.HasSuffix(body, ":") && indent == 0:
			category, level = unquote(strings.Trim(body[:len(body)-1], textlines.SpaceOrTab)), ""
			continue
		case indent > 0 && category != "":
			key, value, found := strings.Cut(body, ":")
			key = strings.Trim(key, textlines.SpaceOrTab)
			value = strings.Trim(value, textlines.SpaceOrTab)
			if found && (value == "" || value == "[]") && key != "" {
				level = unquote(key)
				continue
			}
		}
		problems.AddRaw(number, "unparseable line (not a category, level or list item)",
			parser.LineText(raw))
	}
	return entries
}

// parsePermissionYAML emits one "permission setting:" row per tool of
// permission.yaml, then its bad lines.
func (p *Parser) parsePermissionYAML(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	timestamp := parser.FileMtime(artifact)
	carried := parser.NewCarried(artifact)
	for _, entry := range permissionEntries(artifact.DiskPath, &problems) {
		// A category and a level line stand before each of their tools.
		emit(p.standingRow(artifact, entry.line, timestamp, "permission setting: "+
			carried.Cut("permission category", entry.category)+"."+carried.Cut("permission level", entry.level)+
			"="+entry.tool+" (from "+artifact.Rel+", file mtime)"))
	}
	carried.Report(0, &problems)
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// readJSONFile returns a whole-file JSON store, or false after emitting
// the rows of a file that cannot be read; message says why a readable one
// is not JSON.
func (p *Parser) readJSONFile(artifact *input.Artifact, emit parser.Emit) (jsonvalue.Value, string, bool) {
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return jsonvalue.Value{}, "", false
	}
	document, err := jsonvalue.Decode(content)
	if err != nil {
		return jsonvalue.Value{}, artifact.Rel + " is not JSON (" + err.Error() + ")", true
	}
	return document, "", true
}

// parseRoamingTrust emits one "permission setting:" row per allowed peer
// and revoked key of roaming_trust.json; an allowed or revoked_keys that is
// not an array adds a problem.
func (p *Parser) parseRoamingTrust(artifact *input.Artifact, emit parser.Emit) {
	document, message, ok := p.readJSONFile(artifact, emit)
	if !ok {
		return
	}
	timestamp := parser.FileMtime(artifact)
	if message == "" && !document.IsObject() {
		message = artifact.Rel + " is not a JSON object"
	}
	if message != "" {
		emit(p.standingRow(artifact, 0, timestamp, parser.ProblemPrefix+message))
		return
	}
	var problems parser.Problems
	for _, key := range []string{"allowed", "revoked_keys"} {
		for _, peer := range parser.ArrayMember(document.Get(key), key, 0, &problems) {
			emit(p.standingRow(artifact, 1, timestamp, "permission setting: roaming_trust."+key+"="+
				parser.SettingValue(peer)+" (from "+artifact.Rel+", file mtime)"))
		}
	}
	for _, found := range problems {
		emit(p.standingRow(artifact, found.Line, timestamp, parser.ProblemPrefix+found.Message))
	}
}

// parseSchedule emits one "scheduled task:" row per job of schedule.json,
// then a bad-record row per entry that is not an object.
func (p *Parser) parseSchedule(artifact *input.Artifact, emit parser.Emit) {
	document, message, ok := p.readJSONFile(artifact, emit)
	if !ok {
		return
	}
	timestamp := parser.FileMtime(artifact)
	if message == "" && !document.IsArray() {
		message = artifact.Rel + " is not a JSON array"
	}
	if message != "" {
		emit(p.standingRow(artifact, 0, timestamp, parser.ProblemPrefix+message))
		return
	}
	var problems parser.Problems
	for i, job := range document.Items() {
		if !job.IsObject() {
			problems.AddRaw(0, "unparseable record "+strconv.Itoa(i+1)+" (not a JSON object)",
				parser.JSONText(job))
			continue
		}
		emit(p.standingRow(artifact, 1, timestamp, scheduleText(job, artifact.Rel)))
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// scheduleText returns the "scheduled task:" text of one job; "-" stands
// for an absent value, and parameters are added only when the job has any.
func scheduleText(job jsonvalue.Value, rel string) string {
	value := func(key string) string { return parser.SettingOrDash(job.Get(key)) }
	flag := func(key string) string {
		return parser.SettingValue(jsonvalue.NewBool(!job.Get(key).IsEmpty()))
	}
	parameters := ""
	if stored := job.Get("parameters"); !stored.IsEmpty() {
		parameters = " parameters=" + parser.JSONText(stored)
	}
	return "scheduled task: " + value("id") + " " + value("cron") + " paused=" + flag("paused") +
		" running=" + flag("currently_running") + " source=" + value("source") + " last_run=" +
		timeutil.FirstNonEmpty(timeutil.FromValue(job.Get("last_run")), value("last_run")) +
		" current_session=" + value("current_session_id") + " recipe_base_dir=" +
		value("recipe_base_dir") + parameters + " (from " + rel + ", file mtime)"
}
