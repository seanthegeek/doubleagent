// Package tabby reads Tabby's server stores: chat threads, pages,
// completion events, the model config and the editors' inline-edit command
// histories.
//
// Tabby is a self-hosted server, so its stores sit under the server
// account's home: the user column is that account, and the person who
// chatted is named in each thread's system row. Paths start at .tabby/, or
// at the server root itself when the collection base is a container
// volume's _data. Files read:
//
//   - ee/db.sqlite or ee/dev-db.sqlite, and the ee/*db.backup-<date>.sqlite
//     copies made before each migration, a secondary source that can hold
//     threads since deleted. Only the columns of tableColumns are read, each
//     when the table has it; the secret-bearing columns never are.
//   - events/<date>.json, one log entry per line (user, ts, event), a
//     secondary source: the server writes each event here and to
//     user_events.
//   - config.toml: one "model configured:" row per [model.<group>.<http|local>]
//     table at the file's time; api_key is never read.
//   - The IntelliJ plugin's options/intellij-tabby-command-history.xml and
//     the VS Code extension's edit.recentlyCommand in the editor's
//     state.vscdb, secondary sources with no times.
//
// The session id is the thread id, page:<id> or the completion_id; the
// project path is the first attached code[].git_url (Tabby records no local
// directory) or the event's segments.git_url. Integrations and their cached
// repositories give "source host:" rows. See research/tabby.md.
package tabby

import (
	"cmp"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
	"github.com/seanthegeek/doubleagent/internal/tomldoc"
	"github.com/seanthegeek/doubleagent/internal/vscodestate"
)

// root is what a server path starts with: .tabby/ under a user's home, or
// nothing when the collection base is a container volume holding the
// server root itself.
const root = `\A(?:\.tabby/)?`

// The IntelliJ plugin's history component, and the VS Code extension's
// state key and its history field.
const (
	intellijComponent = "com.tabbyml.intellijtabby.inlineChat.CommandHistory"
	vscodeItemKey     = "TabbyML.vscode-tabby"
	vscodeHistoryKey  = "edit.recentlyCommand"
)

// Parser reads Tabby's databases, event logs, config and command histories.
type Parser struct {
	parser.Base
	database, backup, events, config, intellij *regexp.Regexp
}

// New returns the parser.
func New() *Parser {
	return &Parser{
		Base:     parser.NewBase("tabby", "tabby"),
		database: regexp.MustCompile(root + `ee/(?:dev-)?db\.sqlite\z`),
		backup:   regexp.MustCompile(root + `ee/(?:dev-)?db\.backup-[^/]+\.sqlite\z`),
		events:   regexp.MustCompile(root + `events/[0-9]{4}-[0-9]{2}-[0-9]{2}\.json\z`),
		config:   regexp.MustCompile(root + `config\.toml\z`),
		// The IntelliJ plugin's history, in the IDE's options/ directory.
		intellij: regexp.MustCompile(`(?:\A|/)JetBrains/[^/]+/options/intellij-tabby-command-history\.xml\z`),
	}
}

// ReadsAgents names the editors: the VS Code extension's history is a row
// of the editor's state.vscdb, which the catalog attributes to the editor.
func (p *Parser) ReadsAgents() []string {
	return vscodestate.EditorAgents()
}

// own reports whether the artifact is Tabby's own rather than an editor's;
// one with no agent counts as own.
func (p *Parser) own(artifact *input.Artifact) bool {
	return artifact.Agent == "" || artifact.Agent == p.Agent()
}

// Wants reports whether the artifact is a Tabby database, event log,
// config or history, or an editor's state database.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	rel := artifact.Rel
	if !p.own(artifact) {
		return vscodestate.IsStateDatabase(rel)
	}
	return p.database.MatchString(rel) || p.backup.MatchString(rel) ||
		p.events.MatchString(rel) || p.config.MatchString(rel) || p.intellij.MatchString(rel)
}

// Secondary reports whether the artifact is a backup database, an event
// log or a command history.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	rel := artifact.Rel
	if !p.own(artifact) {
		return vscodestate.IsStateDatabase(rel)
	}
	return p.backup.MatchString(rel) || p.events.MatchString(rel) || p.intellij.MatchString(rel)
}

// Parse emits the rows of one file.
func (p *Parser) Parse(artifact *input.Artifact, _ parser.Options, emit parser.Emit) error {
	switch rel := artifact.Rel; {
	case !p.own(artifact):
		p.parseVSCodeState(artifact, emit)
	case p.events.MatchString(rel):
		p.parseEvents(artifact, emit)
	case p.config.MatchString(rel):
		p.parseConfig(artifact, emit)
	case p.intellij.MatchString(rel):
		p.parseIntellijHistory(artifact, emit)
	default:
		p.parseDatabase(artifact, p.backup.MatchString(rel), emit)
	}
	return nil
}

// rowSpec is what a row of the parser carries besides the artifact's.
type rowSpec struct {
	line                              int
	sessionID, projectPath, timestamp string
	turnType, text                    string
}

// row returns a row of the artifact with the compacted text, a system row
// unless the spec names another turn type.
func (p *Parser) row(artifact *input.Artifact, spec rowSpec) timeline.Row {
	row := p.BaseRow(artifact)
	row.SourceLine = spec.line
	row.SessionID = spec.sessionID
	row.ProjectPath = spec.projectPath
	row.TimestampUTC = spec.timestamp
	row.TurnType = spec.turnType
	if row.TurnType == "" {
		row.TurnType = timeline.TurnSystem
	}
	row.Text = timeline.Compact(spec.text)
	return row
}

// tomlValue returns a TOML value as the JSON value of the same shape,
// a date or time as its text.
func tomlValue(value tomldoc.Value) jsonvalue.Value {
	switch value.Kind() {
	case tomldoc.KindInteger:
		return jsonvalue.NewInt(value.Int())
	case tomldoc.KindFloat:
		return jsonvalue.NewFloat(value.Float())
	case tomldoc.KindBoolean:
		return jsonvalue.NewBool(value.Bool())
	case tomldoc.KindArray:
		items := make([]jsonvalue.Value, 0, len(value.Items()))
		for _, item := range value.Items() {
			items = append(items, tomlValue(item))
		}
		return jsonvalue.NewArray(items...)
	case tomldoc.KindTable:
		var members []jsonvalue.Member
		for _, entry := range value.Table().Entries() {
			members = append(members, jsonvalue.Member{Key: entry.Key, Value: tomlValue(entry.Value)})
		}
		return jsonvalue.NewObject(members...)
	}
	return jsonvalue.NewString(value.Text())
}

// parseConfig emits one row per configured model of config.toml, at the
// file's mtime. Only the model tables' names, kinds and endpoints are read;
// api_key never reaches a row.
func (p *Parser) parseConfig(artifact *input.Artifact, emit parser.Emit) {
	timestamp := parser.FileMtime(artifact)
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	document, err := tomldoc.Parse(textlines.ValidUTF8(content))
	if err != nil {
		emit(p.row(artifact, rowSpec{timestamp: timestamp,
			text: "parser: config.toml is not valid TOML (" + tomlPosition(err) + ")"}))
		return
	}
	models := jsonvalue.NewNull()
	if value, ok := document.Get("model"); ok {
		models = tomlValue(value)
	}
	if !models.IsObject() {
		return
	}
	for _, group := range []string{"completion", "chat", "embedding"} {
		config := models.Get(group)
		if !config.IsObject() {
			continue
		}
		for _, kind := range []string{"http", "local"} {
			if body := config.Get(kind); body.IsObject() {
				emit(p.row(artifact, rowSpec{timestamp: timestamp, text: modelText(group, kind, body)}))
			}
		}
	}
}

// tomlPositionPattern matches the start of a TOML error that says where
// the document breaks the rules.
var tomlPositionPattern = regexp.MustCompile(`\Ainvalid TOML at line [0-9]+, column [0-9]+`)

// tomlPosition returns where config.toml breaks the TOML rules, "invalid
// TOML at line <n>, column <n>", without the reader's detail, which can
// quote part of api_key; an error without a position is "invalid TOML".
func tomlPosition(err error) string {
	if position := tomlPositionPattern.FindString(err.Error()); position != "" {
		return position
	}
	return "invalid TOML"
}

// modelText returns the "model configured:" text of one
// [model.<group>.<kind>] table: model_name, kind, api_endpoint (without
// the credentials a URL may carry) and supported_models for the http form,
// model_id for the local one.
func modelText(group, kind string, body jsonvalue.Value) string {
	model, detail := body.Get("model_id"), ""
	if kind == "http" {
		model = body.Get("model_name")
		var parts []string
		if value := body.Get("kind"); !value.IsEmpty() {
			parts = append(parts, "kind="+value.Text())
		}
		if value := body.Get("api_endpoint"); !value.IsEmpty() {
			parts = append(parts, "endpoint="+parser.WithoutCredentials(value.Text()))
		}
		if supported := body.Get("supported_models"); supported.IsArray() && supported.Len() > 0 {
			var names []string
			for _, name := range supported.Items() {
				names = append(names, name.Text())
			}
			parts = append(parts, "supported_models="+strings.Join(names, ","))
		}
		if len(parts) > 0 {
			detail = " " + strings.Join(parts, " ")
		}
	}
	shown := "-"
	if !model.IsEmpty() {
		shown = model.Text()
	}
	return "model configured: " + group + " " + shown + " via " + kind + detail +
		" (from config.toml [model." + group + "." + kind + "], file mtime)"
}

// historyRows emits a leading system row, then one user row per command,
// newest first. The rows have no time: the histories record none.
func (p *Parser) historyRows(artifact *input.Artifact, source string, commands []string,
	emit parser.Emit) {
	emit(p.row(artifact, rowSpec{text: "inline edit command history: " +
		strconv.Itoa(len(commands)) + " commands, newest first (from " + source + ", no times)"}))
	for i, command := range commands {
		emit(p.row(artifact, rowSpec{line: i + 1, turnType: timeline.TurnUser, text: command}))
	}
}

// parseIntellijHistory emits the commands of the IntelliJ plugin's
// history: the value of every element under a "history" option of the
// plugin's component.
func (p *Parser) parseIntellijHistory(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	document, err := parseXML(content)
	if err != nil {
		emit(p.row(artifact, rowSpec{text: "parser: not a readable XML document (" + err.Error() + ")"}))
		return
	}
	commands := historyCommands(document, p.Work)
	if len(commands) > 0 {
		p.historyRows(artifact, "intellij-tabby-command-history.xml", commands, emit)
	}
}

// parseVSCodeState emits the VS Code extension's command history from an
// editor's state.vscdb, then a row per problem, never echoing the value:
// the same row holds the extension's endpoint tokens. A history that is
// not an array adds a problem.
func (p *Parser) parseVSCodeState(artifact *input.Artifact, emit parser.Emit) {
	state, problems := vscodestate.ReadItem(artifact.DiskPath, vscodeItemKey)
	switch {
	case state.IsObject():
		var commands []string
		var found parser.Problems
		for _, entry := range parser.ArrayMember(state.Get(vscodeHistoryKey), vscodeHistoryKey, 0, &found) {
			if command := commandText(entry); command != "" {
				commands = append(commands, command)
			}
		}
		if len(commands) > 0 {
			p.historyRows(artifact, "state.vscdb "+vscodeItemKey+" "+vscodeHistoryKey, commands, emit)
		}
		for _, problem := range found {
			problems = append(problems, problem.Message)
		}
	case !state.IsNull() && len(problems) == 0:
		problems = []string{vscodeItemKey + " value is not a JSON object"}
	}
	for _, problem := range problems {
		emit(p.row(artifact, rowSpec{text: "parser: " + vscodestate.ShownProblem(problem)}))
	}
}

// parseEvents emits the rows of one events/<date>.json log.
func (p *Parser) parseEvents(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	for number, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		owner := ""
		if user := record.Get("user"); parser.Present(user) {
			owner = "user=" + user.Text()
		}
		timestamp := timeutil.FromValue(record.Get("ts"))
		event := record.Get("event")
		if !event.IsObject() {
			emit(p.row(artifact, rowSpec{line: number, timestamp: timestamp,
				text: "unknown log entry: " + parser.JSONText(record)}))
			continue
		}
		p.eventRows(artifact, rowSpec{line: number, timestamp: timestamp}, owner, event, "", emit)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// eventRows emits the rows of one event, read from either store: a
// completion with its prompt and choices, a view, select or dismiss of a
// choice, or any other event as its JSON. spec gives the line and time. A
// completion, view, select or dismiss whose body is present but not an
// object is a problem row before the event's own row.
func (p *Parser) eventRows(artifact *input.Artifact, spec rowSpec, owner string, event jsonvalue.Value,
	kind string, emit parser.Emit) {
	if event.Len() != 1 {
		spec.text = parser.JoinNonEmpty(" | ", cmp.Or(kind, "event"), owner, parser.JSONText(event))
		emit(p.row(artifact, spec))
		return
	}
	member := event.Members()[0]
	tag, raw := member.Key, member.Value
	body := parser.ObjectOrEmpty(raw)
	if known := tag == "completion" || slices.Contains([]string{"view", "select", "dismiss"}, tag); known &&
		!raw.IsObject() && !raw.IsNull() {
		problem := spec
		problem.text = parser.ProblemPrefix + parser.WrongKind(tag, jsonvalue.Object)
		emit(p.row(artifact, problem))
	}
	spec.sessionID = parser.Text(body.Get("completion_id"))
	switch {
	case tag == "completion":
		p.completionRows(artifact, spec, owner, body, emit)
	case slices.Contains([]string{"view", "select", "dismiss"}, tag):
		index := ""
		if value, ok := body.Lookup("choice_index"); ok {
			index = value.Text()
		}
		elapsed := ""
		if value := body.Get("elapsed"); !value.IsNull() {
			elapsed = "elapsed=" + value.Text() + "ms"
		}
		spec.text = parser.JoinNonEmpty(" | ", tag+" choice "+index, owner, parser.Labelled("kind=", body.Get("kind")),
			parser.Labelled("view=", body.Get("view_id")), elapsed)
		emit(p.row(artifact, spec))
	default:
		detail := ""
		if !body.IsEmpty() {
			detail = parser.JSONText(body)
		}
		spec.text = parser.JoinNonEmpty(" | ", tag, owner, detail)
		emit(p.row(artifact, spec))
	}
}

// completionRows emits a completion's system row, its prompt as a user row
// and each choice as an assistant row. A choices that is not an array adds
// a problem row after them.
func (p *Parser) completionRows(artifact *input.Artifact, spec rowSpec, owner string,
	body jsonvalue.Value, emit parser.Emit) {
	segments := body.Get("segments")
	if !segments.IsObject() {
		segments = jsonvalue.NewObject()
	}
	spec.projectPath = parser.Text(segments.Get("git_url"))
	spec.text = parser.JoinNonEmpty(" | ", "completion", owner, parser.Labelled("language=", body.Get("language")),
		parser.Labelled("file=", segments.Get("filepath")), parser.Labelled("client=", body.Get("user_agent")))
	emit(p.row(artifact, spec))
	spec.turnType, spec.text = timeline.TurnUser, parser.Text(body.Get("prompt"))
	emit(p.row(artifact, spec))
	var found parser.Problems
	for _, choice := range parser.EntriesOfKind(body.Get("choices"), "choices", jsonvalue.Object, spec.line, &found) {
		spec.turnType, spec.text = timeline.TurnAssistant, parser.Text(choice.Get("text"))
		emit(p.row(artifact, spec))
	}
	for _, problem := range found {
		emit(p.row(artifact, rowSpec{line: problem.Line, sessionID: spec.sessionID, timestamp: spec.timestamp,
			text: parser.ProblemPrefix + problem.Message}))
	}
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parser.Parser          = (*Parser)(nil)
	_ parser.AgentReader     = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
)
