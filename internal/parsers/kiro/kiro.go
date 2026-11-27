package kiro

import (
	"bytes"
	"errors"
	"io"
	"regexp"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
	parserkit.Base
	database, sessions, history, agent, subagent *parserkit.RelPattern
	conversationID, historyStart, denyRule       *regexp.Regexp
}

// New returns the parser.
func New() *Parser {
	return &Parser{
		Base: parserkit.NewBase("kiro", "kiro"),
		database: parserkit.MustRelPattern(`\A(?:\.local/share|Library/Application Support|AppData/Local)/` +
			`(?:amazon-q|kiro-cli)/data\.sqlite3\z`),
		sessions: parserkit.MustRelPattern(`\A\.kiro/sessions/`),
		// The chat prompt's history (CLI_BASH_HISTORY in the CLI's paths).
		history: parserkit.MustRelPattern(`\A\.aws/amazonq/\.cli_bash_history\z`),
		// Agent configurations, global and per project (AGENTS_DIR).
		agent: parserkit.MustRelPattern(`\A(?:\.aws/amazonq|\.amazonq)/cli-agents/[^/]+\.json\z`),
		// The delegate tool's per-agent task records (SUBAGENTS_DIR).
		subagent:       parserkit.MustRelPattern(`\A\.amazonq/\.subagents/[^/]+\.json\z`),
		conversationID: regexp.MustCompile(`"conversation_id"[ \t\n\r]*:[ \t\n\r]*"((?:[^"\\]|\\.)*)"`),
		historyStart:   regexp.MustCompile(`"history"[ \t\n\r]*:[ \t\n\r]*\[`),
		// A deny rule of the agent's toolsSettings, as the CLI words it; the
		// end may follow one line feed.
		denyRule: regexp.MustCompile(`(?s)\ATool use with (.+) was rejected because the arguments ` +
			`supplied were forbidden\n?\z`),
	}
}

// sniffExport reports whether a file starts like a /save export: an object
// with conversation_id in its first bytes. This is file selection, not a
// parse; a file that cannot be read is offered, since it may be an export,
// and its parse then reports it.
func sniffExport(path string) bool {
	file, err := parserkit.Open(path)
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

// Kinds returns the database, the history, the agent configurations, the
// subagent records and a conversation export: any other .json file outside
// .kiro/sessions whose start reads as one, tried last since it reads the
// file.
func (p *Parser) Kinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.PatternKind("data.sqlite3", p.database),
		parserkit.PatternKind(".cli_bash_history", p.history), parserkit.PatternKind("agent configuration", p.agent),
		parserkit.PatternKind("subagent record", p.subagent),
		{Name: "conversation export", Wants: func(artifact *input.Artifact) bool {
			rel := artifact.Rel
			return !p.sessions.MatchString(rel) && textlines.HasSuffixFoldASCII(rel, ".json") && sniffExport(artifact.DiskPath)
		}}}
}

// Wants reports whether the artifact is the database, the history, an
// agent or delegate file, or a .json that starts like a /save export.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Secondary reports whether the artifact is the prompt history, which
// repeats the conversations' prompts.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return p.history.MatchString(artifact.Rel)
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

// parseHistory emits one user row per history entry, with no time,
// session or project.
func (p *Parser) parseHistory(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	multiline := false
	for number, raw := range parserkit.Lines(artifact.DiskPath, &problems) {
		if !utf8.Valid(raw) {
			problems.AddRaw(number, "unparseable line (not valid UTF-8)", parserkit.LineText(raw))
			continue
		}
		line := parserkit.LineText(raw)
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
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// jsonDocument returns a settings file as a JSON object, or the row that
// replaces its rows when it cannot be read or is not one.
func (p *Parser) jsonDocument(artifact *input.Artifact, what string) (jsonvalue.Value, *timeline.Row) {
	var problems parserkit.Problems
	content, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		row := p.ErrorRows(artifact, problems, "", "")[0]
		return jsonvalue.Value{}, &row
	}
	document, err := parserkit.DecodeJSONBytes(artifact, content)
	if err == nil && document.IsObject() {
		return document, nil
	}
	row := p.SystemRow(artifact, parserkit.ProblemPrefix+what+" is not a JSON object")
	row.SourceLine = 1
	row.TimestampUTC = parserkit.FileMtime(artifact)
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
func (p *Parser) parseAgentConfig(artifact *input.Artifact, emit parserkit.Emit) {
	document, bad := p.jsonDocument(artifact, "agent configuration")
	if bad != nil {
		emit(*bad)
		return
	}
	timestamp, project := parserkit.FileMtime(artifact), homeProject(artifact)
	setting := func(key string, value jsonvalue.Value) {
		row := p.SystemRow(artifact, timeline.Compact("permission setting: "+key+"="+
			parserkit.SettingValue(value)+" (from "+artifact.Rel+", file mtime)"))
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
	carried := parserkit.NewCarried(artifact)
	for _, tool := range tools.Members() {
		if !tool.Value.IsObject() {
			setting("toolsSettings."+tool.Key, tool.Value)
			continue
		}
		// Each member's row repeats its tool's key.
		key := carried.Cut("tool key", tool.Key)
		for _, member := range tool.Value.Members() {
			setting("toolsSettings."+key+"."+member.Key, member.Value)
		}
	}
	var problems parserkit.Problems
	carried.Report(1, &problems)
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
}

// positiveTime returns the time of a positive epoch number, else "".
func positiveTime(value jsonvalue.Value) string {
	if number, ok := value.Float(); ok && value.IsNumber() && number > 0 {
		return timeutil.FromValue(value)
	}
	return ""
}

// delegateEnd returns the status row text of a delegate task that ended,
// with its exit code, summary and output, or false while it runs.
func delegateEnd(document jsonvalue.Value) (string, bool) {
	agent := parserkit.Text(document.Get("agent"))
	status, completed := parserkit.Text(document.Get("status")), document.Get("completed_at")
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

// parseDelegate emits a delegate task's "launched" row and, once it ended,
// its status row at completed_at, with the exit code, summary and output.
func (p *Parser) parseDelegate(artifact *input.Artifact, emit parserkit.Emit) {
	document, bad := p.jsonDocument(artifact, "delegate record")
	if bad != nil {
		emit(*bad)
		return
	}
	cwd := parserkit.Text(document.Get("cwd"))
	if cwd == "" || cwd == "Unknown" {
		cwd = homeProject(artifact)
	}
	agent := parserkit.Text(document.Get("agent"))
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
		" pid="+pid+trust+": "+parserkit.Text(document.Get("task")))
	if text, ended := delegateEnd(document); ended {
		event(positiveTime(document.Get("completed_at")), text)
	}
}

// parseExport emits the rows of a /save export.
func (p *Parser) parseExport(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) {
	var problems parserkit.Problems
	content, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	p.conversation(artifact, options, textlines.ValidUTF8(content), "", 0, emit)
}

// Parse emits the rows of one file.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
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

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
)
