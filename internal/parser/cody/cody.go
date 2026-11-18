// Package cody reads Sourcegraph Cody's chat history, settings and model
// preferences.
//
// It reads the account-keyed chat history,
// {"<endpoint>-<username>": {"chat": {"<chatID>": transcript}}}, from the
// sourcegraph.cody-ai row of an editor's state.vscdb (its
// cody-local-chatHistory-v2 member) or the JetBrains plugin's
// Cody-nodejs/[Data/]JetBrains-globalState/cody-local-chatHistory-v2 file;
// the cody-model-preferences value (the row's member or the JetBrains
// file); the editor's User/settings.json (JSON with comments); and a
// project's .idea/cody_history.xml, the older plugin's history, a
// secondary source. The catalog gives the editor files to the editor, so
// the parser reads every editor agent's.
//
// No message has a time: every row takes the chat id, the creation time as
// Date.toUTCString() writes it, else lastInteractionTimestamp. A history
// that does not parse is walked chat by chat, and the XML is read as a
// stream, so the chats before the damage are still read. A MCP server's
// headers, env, url, command and args are never read. See research/cody.md.
package cody

import (
	"regexp"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/vscodestate"
)

// The extension's state row and its members that are read.
const (
	itemKey        = "sourcegraph.cody-ai"
	historyKey     = "cody-local-chatHistory-v2"
	preferencesKey = "cody-model-preferences"
)

// The files read, by path relative to the home.
var (
	jetBrainsHistory = regexp.MustCompile(
		`(?:^|/)Cody-nodejs/(?:[^/]+/)?JetBrains-globalState/cody-local-chatHistory-v2\z`)
	jetBrainsPreferences = regexp.MustCompile(
		`(?:^|/)Cody-nodejs/(?:[^/]+/)?JetBrains-globalState/cody-model-preferences\z`)
	settingsPattern = regexp.MustCompile(`(?:^|/)User/settings\.json\z`)
	legacyPattern   = regexp.MustCompile(`(?:^|/)\.idea/cody_history\.xml\z`)
)

// Parser reads Cody's chat history, model preferences, settings and the
// older JetBrains plugin's history.
type Parser struct {
	parser.Base
}

// New returns the parser.
func New() *Parser {
	return &Parser{Base: parser.NewBase("cody", "cody")}
}

// ReadsAgents names the editors whose state.vscdb and settings hold the
// extension's state.
func (p *Parser) ReadsAgents() []string {
	return vscodestate.EditorAgents()
}

// Wants reports whether the artifact is an editor state database or
// settings file, a JetBrains Cody file or the older plugin's history.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	rel := artifact.Rel
	return vscodestate.IsStateDatabase(rel) || jetBrainsHistory.MatchString(rel) ||
		jetBrainsPreferences.MatchString(rel) || settingsPattern.MatchString(rel) ||
		legacyPattern.MatchString(rel)
}

// Secondary reports whether the artifact is the older plugin's
// .idea/cody_history.xml.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return legacyPattern.MatchString(artifact.Rel)
}

// Parse emits the rows of one Cody file.
func (p *Parser) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	rel := artifact.Rel
	switch {
	case settingsPattern.MatchString(rel):
		p.settings(artifact, emit)
	case jetBrainsPreferences.MatchString(rel):
		p.preferencesFile(artifact, emit)
	case legacyPattern.MatchString(rel):
		p.legacy(artifact, emit)
	case jetBrainsHistory.MatchString(rel):
		p.jetBrains(artifact, options, emit)
	default:
		p.stateDatabase(artifact, options, emit)
	}
	return nil
}

// systemRow returns a system row of the artifact at a source line.
func (p *Parser) systemRow(artifact *input.Artifact, text string, line int) timeline.Row {
	row := p.SystemRow(artifact, text)
	row.SourceLine = line
	return row
}

// jetBrains emits the rows of the JetBrains plugin's history file, every
// one at line 1, then its problems.
func (p *Parser) jetBrains(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	var unreadable parser.Problems
	text, ok := parser.ReadText(artifact.DiskPath, &unreadable)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, unreadable, "", ""))
		return
	}
	var problems []string
	var recovered []recoveredError
	history := decodeHistory(text, &problems, &recovered)
	if !history.IsObject() {
		problems = append(problems, "chat history is not a JSON object")
	}
	p.historyRows(artifact, history, 1, options, emit)
	p.recoveredRows(artifact, recovered, emit)
	for _, problem := range problems {
		emit(p.systemRow(artifact, parser.ProblemPrefix+problem, 1))
	}
}

// stateDatabase emits the rows of the extension's row of a state.vscdb:
// its history, the history's damage, its model preferences, then the
// problems met, a value or a history that is not an object among them. A
// history is held to the JetBrains file's rule: written as JSON text, it
// must decode to an object, null included; a null member is no history.
func (p *Parser) stateDatabase(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	state, problems := vscodestate.ReadItem(artifact.DiskPath, itemKey)
	if state.IsNull() {
		for _, problem := range problems {
			emit(p.systemRow(artifact, parser.ProblemPrefix+problem, 0))
		}
		return
	}
	var recovered []recoveredError
	history := jsonvalue.NewNull()
	switch {
	case state.IsString(): // the value did not parse, and ReadItem said so
		history, _, recovered = recoverHistory(state.StringValue())
	case state.IsObject():
		history = state.Get(historyKey)
		fromText := history.IsString()
		if fromText {
			history = decodeHistory(history.StringValue(), &problems, &recovered)
		}
		if (fromText || !history.IsNull()) && !history.IsObject() {
			problems = append(problems, historyKey+" is not a JSON object")
		}
	default:
		problems = append(problems, itemKey+" value is not a JSON object")
	}
	p.historyRows(artifact, history, 0, options, emit)
	p.recoveredRows(artifact, recovered, emit)
	if state.IsObject() {
		p.preferenceRows(artifact, state.Get(preferencesKey), artifact.Rel+" "+preferencesKey, 0, emit)
	}
	for _, problem := range problems {
		emit(p.systemRow(artifact, parser.ProblemPrefix+problem, 0))
	}
}

// decodeHistory returns a history's text decoded, else what can be
// recovered from it, adding the problem and the damaged interactions.
func decodeHistory(text string, problems *[]string, recovered *[]recoveredError) jsonvalue.Value {
	history, err := jsonvalue.DecodeString(text)
	if err == nil {
		return history
	}
	history, problem, errors := recoverHistory(text)
	*problems = append(*problems, problem+" ("+err.Error()+")")
	*recovered = append(*recovered, errors...)
	return history
}

// recoveredRows emits a row per damaged interaction of a recovered
// history, under the session id of its chat.
func (p *Parser) recoveredRows(artifact *input.Artifact, recovered []recoveredError, emit parser.Emit) {
	for _, entry := range recovered {
		parser.EmitAll(emit, p.ErrorRows(artifact, parser.Problems{entry.problem}, entry.sessionID, ""))
	}
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parser.Parser          = (*Parser)(nil)
	_ parser.AgentReader     = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
)
