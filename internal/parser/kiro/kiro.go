// Package kiro reads the conversations of the Amazon Q Developer CLI, the
// open-source predecessor of the Kiro CLI.
//
// It reads, relative to the home: data.sqlite3 under .local/share,
// Library/Application Support or AppData/Local, in amazon-q/ or kiro-cli/
// (the conversations table, one serialised ConversationState per working
// directory, and the legacy Fig shell history table; auth_kv and state hold
// tokens and are never read); /save exports, a .json whose top-level object
// has conversation_id and history; .aws/amazonq/.cli_bash_history, the
// prompt's line-editor history (a secondary source); the agent
// configurations {.aws/amazonq,.amazonq}/cli-agents/<name>.json; and the
// delegate tool's .amazonq/.subagents/<agent>.json. Kiro CLI's
// .kiro/sessions/ files are not read.
//
// A ConversationState cut mid-write is salvaged entry by entry. Entries
// without a time of their own take the previous row's; the entries only a
// checkpoint snapshot or the tangent-mode copy still holds follow the
// history's rows, marked recovered. See research/kiro.md.
package kiro

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The first line of a history file that escapes multi-line entries, the
// agent the delegate tool runs with every tool trusted, and how much of a
// .json file is read to tell whether it is a /save export.
const (
	historyV2        = "#V2"
	defaultAgentName = "q_cli_default"
	sniffBytes       = 4096
)

// Parser reads the Amazon Q Developer CLI's files.
type Parser struct {
	parser.Base
	database, sessions, history, agent, subagent *regexp.Regexp
	conversationID, historyStart, denyRule       *regexp.Regexp
}

// New returns the parser.
func New() *Parser {
	return &Parser{
		Base: parser.NewBase("kiro", "kiro"),
		database: regexp.MustCompile(`\A(?:\.local/share|Library/Application Support|AppData/Local)/` +
			`(?:amazon-q|kiro-cli)/data\.sqlite3\z`),
		sessions: regexp.MustCompile(`\A\.kiro/sessions/`),
		// The chat prompt's history (CLI_BASH_HISTORY in the CLI's paths).
		history: regexp.MustCompile(`\A\.aws/amazonq/\.cli_bash_history\z`),
		// Agent configurations, global and per project (AGENTS_DIR).
		agent: regexp.MustCompile(`\A(?:\.aws/amazonq|\.amazonq)/cli-agents/[^/]+\.json\z`),
		// The delegate tool's per-agent task records (SUBAGENTS_DIR).
		subagent:       regexp.MustCompile(`\A\.amazonq/\.subagents/[^/]+\.json\z`),
		conversationID: regexp.MustCompile(`"conversation_id"[ \t\n\r]*:[ \t\n\r]*"((?:[^"\\]|\\.)*)"`),
		historyStart:   regexp.MustCompile(`"history"[ \t\n\r]*:[ \t\n\r]*\[`),
		// A deny rule of the agent's toolsSettings, as the CLI words it; the
		// end may follow one line feed.
		denyRule: regexp.MustCompile(`(?s)\ATool use with (.+) was rejected because the arguments ` +
			`supplied were forbidden\n?\z`),
	}
}

// Wants reports whether the artifact is the database, the history, an
// agent or delegate file, or a .json that starts like a /save export.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	rel := artifact.Rel
	switch {
	case p.database.MatchString(rel), p.history.MatchString(rel), p.agent.MatchString(rel),
		p.subagent.MatchString(rel):
		return true
	case p.sessions.MatchString(rel) || !strings.HasSuffix(rel, ".json"):
		return false
	}
	return sniffExport(artifact.DiskPath)
}

// sniffExport reports whether a file starts like a /save export: an object
// with conversation_id in its first bytes. This is file selection, not a
// parse; a file that cannot be read is offered, since it may be an export,
// and its parse then reports it.
func sniffExport(path string) bool {
	file, err := parser.Open(path)
	if err != nil {
		return true
	}
	defer file.Close()
	head := make([]byte, sniffBytes)
	n, err := io.ReadFull(file, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}
	head = head[:n]
	trimmed := bytes.TrimLeft(head, textlines.ASCIIWhitespace)
	return len(trimmed) > 0 && trimmed[0] == '{' && bytes.Contains(head, []byte(`"conversation_id"`))
}

// Secondary reports whether the artifact is the prompt history, which
// repeats the conversations' prompts.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return p.history.MatchString(artifact.Rel)
}

// Parse emits the rows of one file.
func (p *Parser) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	switch rel := artifact.Rel; {
	case p.database.MatchString(rel):
		p.parseDatabase(artifact, options, emit)
	case p.history.MatchString(rel):
		p.parseHistory(artifact, emit)
	case p.agent.MatchString(rel):
		p.parseAgentConfig(artifact, emit)
	case p.subagent.MatchString(rel):
		p.parseDelegate(artifact, emit)
	default:
		p.parseExport(artifact, options, emit)
	}
	return nil
}

// parseHistory emits one user row per history entry, with no time,
// session or project.
func (p *Parser) parseHistory(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	multiline := false
	for number, raw := range parser.Lines(artifact.DiskPath, &problems) {
		if !utf8.Valid(raw) {
			problems.AddRaw(number, "unparseable line (not valid UTF-8)", parser.LineText(raw))
			continue
		}
		line := parser.LineText(raw)
		if number == 1 && line == historyV2 {
			multiline = true
			continue
		}
		if line == "" {
			continue
		}
		if multiline {
			line = unescapeHistory(line)
		}
		row := p.BaseRow(artifact)
		row.SourceLine = number
		row.TurnType = timeline.TurnUser
		row.Text = timeline.Compact(line)
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// unescapeHistory returns one entry of a "#V2" history file, where \n is a
// line feed and \\ a backslash. A line with any other escape, or a
// trailing backslash, is kept exactly as written, as the line editor's
// loader keeps it.
func unescapeHistory(line string) string {
	out := make([]byte, 0, len(line))
	for i := 0; i < len(line); i++ {
		if line[i] != '\\' {
			out = append(out, line[i])
			continue
		}
		next := byte(0)
		if i+1 < len(line) {
			next = line[i+1]
		}
		switch next {
		case 'n':
			out = append(out, '\n')
		case '\\':
			out = append(out, '\\')
		default:
			return line
		}
		i++
	}
	return string(out)
}

// jsonDocument returns a settings file as a JSON object, or the row that
// replaces its rows when it cannot be read or is not one.
func (p *Parser) jsonDocument(artifact *input.Artifact, what string) (jsonvalue.Value, *timeline.Row) {
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		row := p.ErrorRows(artifact, problems, "", "")[0]
		return jsonvalue.Value{}, &row
	}
	document, err := jsonvalue.Decode(content)
	if err == nil && document.IsObject() {
		return document, nil
	}
	row := p.SystemRow(artifact, parser.ProblemPrefix+what+" is not a JSON object")
	row.SourceLine = 1
	row.TimestampUTC = parser.FileMtime(artifact)
	return jsonvalue.Value{}, &row
}

// homeProject returns the home's path when the home is a project, else "".
func homeProject(artifact *input.Artifact) string {
	if artifact.Home != nil && artifact.Home.Project {
		return artifact.Home.Original
	}
	return ""
}

// parseAgentConfig emits one "permission setting:" row per allowedTools
// entry and toolsSettings key.
func (p *Parser) parseAgentConfig(artifact *input.Artifact, emit parser.Emit) {
	document, bad := p.jsonDocument(artifact, "agent configuration")
	if bad != nil {
		emit(*bad)
		return
	}
	timestamp, project := parser.FileMtime(artifact), homeProject(artifact)
	setting := func(key string, value jsonvalue.Value) {
		row := p.SystemRow(artifact, timeline.Compact("permission setting: "+key+"="+
			parser.SettingValue(value)+" (from "+artifact.Rel+", file mtime)"))
		row.SourceLine = 1
		row.TimestampUTC = timestamp
		row.ProjectPath = project
		emit(row)
	}
	if allowed := document.Get("allowedTools"); allowed.IsArray() {
		for _, entry := range allowed.Items() {
			setting("allowedTools", entry)
		}
	}
	tools := document.Get("toolsSettings")
	if !tools.IsObject() {
		return
	}
	for _, tool := range tools.Members() {
		if !tool.Value.IsObject() {
			setting("toolsSettings."+tool.Key, tool.Value)
			continue
		}
		for _, member := range tool.Value.Members() {
			setting("toolsSettings."+tool.Key+"."+member.Key, member.Value)
		}
	}
}

// positiveTime returns the time of a positive epoch number, else "".
func positiveTime(value jsonvalue.Value) string {
	if number, ok := value.Float(); ok && value.IsNumber() && number > 0 {
		return timeutil.FromValue(value)
	}
	return ""
}

// parseDelegate emits a delegate task's "launched" row and, once it ended,
// its status row at completed_at, with the exit code, summary and output.
func (p *Parser) parseDelegate(artifact *input.Artifact, emit parser.Emit) {
	document, bad := p.jsonDocument(artifact, "delegate record")
	if bad != nil {
		emit(*bad)
		return
	}
	cwd := parser.Text(document.Get("cwd"))
	if cwd == "" || cwd == "Unknown" {
		cwd = homeProject(artifact)
	}
	agent := parser.Text(document.Get("agent"))
	event := func(timestamp, text string) {
		row := p.SystemRow(artifact, timeline.Compact(text))
		row.SourceLine = 1
		row.TimestampUTC = timestamp
		row.ProjectPath = cwd
		emit(row)
	}
	pid, trust := "", ""
	if value, ok := document.Lookup("pid"); ok {
		pid = value.Text()
	}
	if agent == defaultAgentName {
		trust = " trust_all_tools=true"
	}
	event(positiveTime(document.Get("launched_at")), "delegate task launched: agent="+agent+
		" pid="+pid+trust+": "+parser.Text(document.Get("task")))
	if text, ended := delegateEnd(document); ended {
		event(positiveTime(document.Get("completed_at")), text)
	}
}

// delegateEnd returns the status row text of a delegate task that ended,
// with its exit code, summary and output, or false while it runs.
func delegateEnd(document jsonvalue.Value) (string, bool) {
	agent := parser.Text(document.Get("agent"))
	status, completed := parser.Text(document.Get("status")), document.Get("completed_at")
	if completed.IsNull() && (status == "" || status == "running") {
		return "", false
	}
	exitCode := ""
	if value := document.Get("exit_code"); !value.IsNull() {
		exitCode = value.Text()
	}
	if status == "" {
		status = "ended"
	}
	text := "delegate task " + status + ": agent=" + agent + " exit_code=" + exitCode
	if summary := document.Get("summary"); !summary.IsEmpty() {
		text += "\nsummary: " + summary.Text()
	}
	if output := document.Get("output"); !output.IsEmpty() {
		text += "\noutput: " + output.Text()
	}
	return text, true
}

// parseExport emits the rows of a /save export.
func (p *Parser) parseExport(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	p.conversation(artifact, options, textlines.ValidUTF8(content), "", 0, emit)
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parser.Parser          = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
)
