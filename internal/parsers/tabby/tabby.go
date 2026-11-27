package tabby

import (
	"cmp"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
	vscodeServersKey  = "server.serverRecords"
)

// Parser reads Tabby's databases, event logs, config and command histories.
type Parser struct {
	parserkit.Base
	database, backup, events, config, intellij *parserkit.RelPattern
}

// New returns the parser.
func New() *Parser {
	return &Parser{
		Base:     parserkit.NewBase("tabby", "tabby"),
		database: parserkit.MustRelPattern(root + `ee/(?:dev-)?db\.sqlite\z`),
		backup:   parserkit.MustRelPattern(root + `ee/(?:dev-)?db\.backup-[^/]+\.sqlite\z`),
		events:   parserkit.MustRelPattern(root + `events/[0-9]{4}-[0-9]{2}-[0-9]{2}\.json\z`),
		config:   parserkit.MustRelPattern(root + `config\.toml\z`),
		// The IntelliJ plugin's history, in the IDE's options/ directory.
		intellij: parserkit.MustRelPattern(`(?:\A|/)JetBrains/[^/]+/options/intellij-tabby-command-history\.xml\z`),
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
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Kinds returns an editor's state database, among another agent's
// artifacts, and Tabby's own database, backups, event logs, config and
// JetBrains history.
func (p *Parser) Kinds() []parserkit.Kind {
	kinds := []parserkit.Kind{{Name: "editor state.vscdb", Wants: func(artifact *input.Artifact) bool {
		return !p.own(artifact) && vscodestate.IsStateDatabase(artifact.Rel)
	}}}
	for _, each := range []struct {
		name    string
		pattern *parserkit.RelPattern
	}{{"db.sqlite", p.database}, {"database backup", p.backup}, {"event log", p.events},
		{"config.toml", p.config}, {"IntelliJ command history", p.intellij}} {
		kinds = append(kinds, parserkit.Kind{Name: each.name, Wants: func(artifact *input.Artifact) bool {
			return p.own(artifact) && each.pattern.MatchString(artifact.Rel)
		}})
	}
	return kinds
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
func (p *Parser) Parse(artifact *input.Artifact, _ parserkit.Options, emit parserkit.Emit) error {
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

// parseConfig emits one row per configured model of config.toml, at the
// file's mtime: the model tables' names, kinds, endpoints and API keys.
func (p *Parser) parseConfig(artifact *input.Artifact, emit parserkit.Emit) {
	timestamp := parserkit.FileMtime(artifact)
	var problems parserkit.Problems
	content, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	document, err := tomldoc.Parse(textlines.ValidUTF8(content))
	if err != nil {
		emit(p.row(artifact, rowSpec{timestamp: timestamp, text: configErrorText(artifact, err)}))
		return
	}
	models := jsonvalue.NewNull()
	if value, ok := document.Get("model"); ok {
		models = tomldoc.JSONValue(value)
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
				emit(p.row(artifact, rowSpec{timestamp: timestamp, text: modelText(artifact, group, kind, body)}))
			}
		}
	}
}

// configErrorText returns the `parser:` text of a config.toml the TOML
// reader refused: "could not be read (<why>)" for a document over a size or
// depth limit, which quotes nothing from the file, else "is not valid TOML
// (<the reader's error>)" through parserkit.ReaderErrorText, since the
// reader's detail can quote part of api_key.
func configErrorText(artifact *input.Artifact, err error) string {
	var refusal *tomldoc.Error
	if errors.As(err, &refusal) && refusal.IsLimit() {
		return "parser: config.toml could not be read (" + err.Error() + ")"
	}
	return "parser: config.toml is not valid TOML (" + parserkit.ReaderErrorText(artifact, err) + ")"
}

// modelText returns the "model configured:" text of one
// [model.<group>.<kind>] table: model_name, kind, api_endpoint, api_key
// through parserkit.SecretText and supported_models
// for the http form, model_id for the local one.
func modelText(artifact *input.Artifact, group, kind string, body jsonvalue.Value) string {
	model, detail := body.Get("model_id"), ""
	if kind == "http" {
		model = body.Get("model_name")
		var parts []string
		if value := body.Get("kind"); !value.IsEmpty() {
			parts = append(parts, "kind="+value.Text())
		}
		if value := body.Get("api_endpoint"); !value.IsEmpty() {
			parts = append(parts, "endpoint="+value.Text())
		}
		if value := body.Get("api_key"); !value.IsEmpty() {
			parts = append(parts, "api_key="+parserkit.SecretText(artifact, value.Text()))
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
	emit parserkit.Emit) {
	emit(p.row(artifact, rowSpec{text: "inline edit command history: " +
		strconv.Itoa(len(commands)) + " commands, newest first (from " + source + ", no times)"}))
	for i, command := range commands {
		emit(p.row(artifact, rowSpec{line: i + 1, turnType: timeline.TurnUser, text: command}))
	}
}

// parseIntellijHistory emits the commands of the IntelliJ plugin's
// history: the value of every element under a "history" option of the
// plugin's component.
func (p *Parser) parseIntellijHistory(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	content, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
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

// parseVSCodeState emits a "tabby server:" row per server record of the
// VS Code extension's state in an editor's state.vscdb, its endpoint and
// its token through parserkit.SecretText, then the
// command history, then a row per problem, never echoing the value, which
// holds those tokens. A history that is not an array, and server records
// of the wrong kind, add a problem.
func (p *Parser) parseVSCodeState(artifact *input.Artifact, emit parserkit.Emit) {
	state, problems, repeats := vscodestate.ReadItem(artifact.DiskPath, vscodeItemKey)
	parserkit.NoteRepeats(artifact, repeats)
	switch {
	case state.IsObject():
		var found parserkit.Problems
		p.serverRows(artifact, state.Get(vscodeServersKey), emit, &found)
		var commands []string
		for _, entry := range parserkit.ArrayMember(state.Get(vscodeHistoryKey), vscodeHistoryKey, 0, &found) {
			if command := commandText(artifact, entry); command != "" {
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
func (p *Parser) parseEvents(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	for number, record := range parserkit.JSONLines(artifact.DiskPath, &problems) {
		owner := ""
		if user := record.Get("user"); parserkit.Present(user) {
			owner = "user=" + user.Text()
		}
		timestamp := timeutil.FromValue(record.Get("ts"))
		event := record.Get("event")
		if !event.IsObject() {
			emit(p.row(artifact, rowSpec{line: number, timestamp: timestamp,
				text: "unknown log entry: " + parserkit.JSONText(record)}))
			continue
		}
		p.eventRows(artifact, rowSpec{line: number, timestamp: timestamp}, owner, event, "", emit)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// eventRows emits the rows of one event, read from either store: a
// completion with its prompt and choices, a view, select or dismiss of a
// choice, or any other event as its JSON. spec gives the line and time. A
// completion, view, select or dismiss whose body is present but not an
// object is a problem row before the event's own row.
func (p *Parser) eventRows(artifact *input.Artifact, spec rowSpec, owner string, event jsonvalue.Value,
	kind string, emit parserkit.Emit) {
	if event.Len() != 1 {
		spec.text = parserkit.JoinNonEmpty(" | ", cmp.Or(kind, "event"), owner, parserkit.JSONText(event))
		emit(p.row(artifact, spec))
		return
	}
	member := event.Members()[0]
	tag, raw := member.Key, member.Value
	body := parserkit.ObjectOrEmpty(raw)
	if known := tag == "completion" || slices.Contains([]string{"view", "select", "dismiss"}, tag); known &&
		!raw.IsObject() && !raw.IsNull() {
		problem := spec
		problem.text = parserkit.ProblemPrefix + parserkit.WrongKind(tag, jsonvalue.Object)
		emit(p.row(artifact, problem))
	}
	spec.sessionID = parserkit.Text(body.Get("completion_id"))
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
		spec.text = parserkit.JoinNonEmpty(" | ", tag+" choice "+index, owner, parserkit.Labelled("kind=", body.Get("kind")),
			parserkit.Labelled("view=", body.Get("view_id")), elapsed)
		emit(p.row(artifact, spec))
	default:
		detail := ""
		if !body.IsEmpty() {
			detail = parserkit.JSONText(body)
		}
		spec.text = parserkit.JoinNonEmpty(" | ", tag, owner, detail)
		emit(p.row(artifact, spec))
	}
}

// completionRows emits a completion's system row, its prompt as a user row
// and each choice as an assistant row. A choices that is not an array adds
// a problem row after them.
func (p *Parser) completionRows(artifact *input.Artifact, spec rowSpec, owner string,
	body jsonvalue.Value, emit parserkit.Emit) {
	segments := body.Get("segments")
	if !segments.IsObject() {
		segments = jsonvalue.NewObject()
	}
	spec.projectPath = parserkit.Text(segments.Get("git_url"))
	spec.text = parserkit.JoinNonEmpty(" | ", "completion", owner, parserkit.Labelled("language=", body.Get("language")),
		parserkit.Labelled("file=", segments.Get("filepath")), parserkit.Labelled("client=", body.Get("user_agent")))
	emit(p.row(artifact, spec))
	spec.turnType, spec.text = timeline.TurnUser, parserkit.Text(body.Get("prompt"))
	emit(p.row(artifact, spec))
	var found parserkit.Problems
	for _, choice := range parserkit.EntriesOfKind(body.Get("choices"), "choices", jsonvalue.Object, spec.line, &found) {
		spec.turnType, spec.text = timeline.TurnAssistant, parserkit.Text(choice.Get("text"))
		emit(p.row(artifact, spec))
	}
	for _, problem := range found {
		emit(p.row(artifact, rowSpec{line: problem.Line, sessionID: spec.sessionID, timestamp: spec.timestamp,
			text: parserkit.ProblemPrefix + problem.Message}))
	}
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.AgentReader     = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
)

// serverRows emits one "tabby server:" row per member of the extension's
// server records, an object of endpoint to {token}, that holds a token.
// The rows have no time: the records hold none. Records that are not an
// object, or that hold a record that is not one, add a problem to found.
func (p *Parser) serverRows(artifact *input.Artifact, records jsonvalue.Value, emit parserkit.Emit,
	found *parserkit.Problems) {
	for endpoint, record := range parserkit.MembersOfKind(records, vscodeServersKey, jsonvalue.Object, 0, found) {
		if token := parserkit.Text(record.Get("token")); token != "" {
			emit(p.row(artifact, rowSpec{text: "tabby server: " + endpoint + " token=" +
				parserkit.SecretText(artifact, token) + " (from state.vscdb " + vscodeItemKey + " " + vscodeServersKey +
				", no times)"}))
		}
	}
}
