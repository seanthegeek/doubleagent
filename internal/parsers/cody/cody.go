package cody

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
	jetBrainsHistory = parserkit.MustRelPattern(
		`(?:^|/)Cody-nodejs/(?:[^/]+/)?JetBrains-globalState/cody-local-chatHistory-v2\z`)
	jetBrainsPreferences = parserkit.MustRelPattern(
		`(?:^|/)Cody-nodejs/(?:[^/]+/)?JetBrains-globalState/cody-model-preferences\z`)
	settingsPattern = parserkit.MustRelPattern(`(?:^|/)User/settings\.json\z`)
	legacyPattern   = parserkit.MustRelPattern(`(?:^|/)\.idea/cody_history\.xml\z`)
)

// Parser reads Cody's chat history, model preferences, settings and the
// older JetBrains plugin's history.
type Parser struct {
	parserkit.Base
}

// New returns the parser.
func New() *Parser {
	return &Parser{Base: parserkit.NewBase("cody", "cody")}
}

// ReadsAgents names the editors whose state.vscdb and settings hold the
// extension's state.
func (p *Parser) ReadsAgents() []string {
	return vscodestate.EditorAgents()
}

// Wants reports whether the artifact is an editor state database or
// settings file, a JetBrains Cody file or the older plugin's history.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the editors' state databases, the JetBrains chat history
// and model preferences, the editor settings and the old JetBrains history.
func (p *Parser) Kinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.RelFuncKind("state.vscdb", vscodestate.IsStateDatabase),
		parserkit.PatternKind("JetBrains chat history", jetBrainsHistory),
		parserkit.PatternKind("JetBrains model preferences", jetBrainsPreferences),
		parserkit.PatternKind("settings.json", settingsPattern), parserkit.PatternKind("cody_history.xml", legacyPattern)}
}

// Secondary reports whether the artifact is the older plugin's
// .idea/cody_history.xml.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return legacyPattern.MatchString(artifact.Rel)
}

// Parse emits the rows of one Cody file.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
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
func (p *Parser) jetBrains(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) {
	var unreadable parserkit.Problems
	text, ok := parserkit.ReadText(artifact.DiskPath, &unreadable)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, unreadable, "", ""))
		return
	}
	var problems []string
	var recovered []recoveredError
	history := decodeHistory(artifact, text, &problems, &recovered)
	if !history.IsObject() {
		problems = append(problems, parserkit.WrongKind("chat history", jsonvalue.Object))
	}
	p.historyRows(artifact, history, 1, options, emit, &problems)
	p.recoveredRows(artifact, recovered, emit)
	for _, problem := range problems {
		emit(p.systemRow(artifact, parserkit.ProblemPrefix+problem, 1))
	}
}

// stateDatabase emits the rows of the extension's row of a state.vscdb:
// its history, the history's damage, its model preferences, then the
// problems met, a value, a history or model preferences that is not an
// object among them. A history is held to the JetBrains file's rule:
// written as JSON text, it must decode to an object, null included; a null
// member is no history.
func (p *Parser) stateDatabase(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) {
	state, problems, repeats := vscodestate.ReadItem(artifact.DiskPath, itemKey)
	parserkit.NoteRepeats(artifact, repeats)
	if state.IsNull() {
		for _, problem := range problems {
			emit(p.systemRow(artifact, parserkit.ProblemPrefix+problem, 0))
		}
		return
	}
	var recovered []recoveredError
	history := jsonvalue.NewNull()
	switch {
	case state.IsString(): // the value did not parse, and ReadItem said so
		history, _, recovered = recoverHistory(artifact, state.StringValue())
	case state.IsObject():
		history = state.Get(historyKey)
		fromText := history.IsString()
		if fromText {
			history = decodeHistory(artifact, history.StringValue(), &problems, &recovered)
		}
		if (fromText || !history.IsNull()) && !history.IsObject() {
			problems = append(problems, parserkit.WrongKind(historyKey, jsonvalue.Object))
		}
	default:
		problems = append(problems, itemKey+" value is not a JSON object")
	}
	p.historyRows(artifact, history, 0, options, emit, &problems)
	p.recoveredRows(artifact, recovered, emit)
	if state.IsObject() {
		var found parserkit.Problems
		p.preferenceRows(artifact, state.Get(preferencesKey), artifact.Rel+" "+preferencesKey, 0, emit, &found)
		addProblems(&problems, found)
	}
	for _, problem := range problems {
		emit(p.systemRow(artifact, parserkit.ProblemPrefix+problem, 0))
	}
}

// decodeHistory returns a history's text decoded, else what can be
// recovered from it, adding the problem and the damaged interactions.
func decodeHistory(artifact *input.Artifact, text string, problems *[]string, recovered *[]recoveredError) jsonvalue.Value {
	history, err := parserkit.DecodeJSON(artifact, text)
	if err == nil {
		return history
	}
	history, problem, errors := recoverHistory(artifact, text)
	*problems = append(*problems, problem+" ("+err.Error()+")")
	*recovered = append(*recovered, errors...)
	return history
}

// recoveredRows emits a row per damaged interaction of a recovered
// history, under the session id of its chat.
func (p *Parser) recoveredRows(artifact *input.Artifact, recovered []recoveredError, emit parserkit.Emit) {
	for _, entry := range recovered {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, parserkit.Problems{entry.problem}, entry.sessionID, ""))
	}
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.AgentReader     = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
)
