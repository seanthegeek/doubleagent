package pearai

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers/clinelegacy"
	"github.com/seanthegeek/doubleagent/internal/parsers/continuedev"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/vscodestate"
)

// root matches the agent extension's global storage in the editor.
const root = `(?:.*/)?User/globalStorage/pearai\.pearai-roo-cline`

// extensionID is the agent extension's id, its state.vscdb row's key.
const extensionID = "pearai.pearai-roo-cline"

// The files read, by path relative to the home. The log is the editor's
// output channel file of the chat extension's PearAI channel (pearai-app
// src/vs/workbench/api/common/extHostOutput.ts:121, :160-170).
var (
	sessionPattern = parserkit.MustRelPattern(`^(?:.*/)?\.pearai/sessions/([^/]+)\.json\z`)
	logPattern     = parserkit.MustRelPattern(
		`^(?:.*/)?logs/[^/]+/window[0-9]+/exthost/output_logging_[^/]+/[0-9]+-PearAI\.log\z`)
	devDataPattern = parserkit.MustRelPattern(`^(?:.*/)?\.pearai/dev_data/(chat|tokens_generated)\.jsonl\z`)
	statePattern   = parserkit.MustRelPattern(`^(?:.*/)?User/globalStorage/state\.vscdb\z`)
)

// Parser reads PearAI's chat sessions, agent tasks, editor state and chat
// logs.
type Parser struct {
	*clinelegacy.TaskParser
	indexes *continuedev.Indexes
}

// New returns the parser.
func New() *Parser {
	created := &Parser{indexes: continuedev.NewIndexes()}
	created.TaskParser = clinelegacy.NewTaskParser(parserkit.NewBase("pearai", "pearai"), root,
		"workspace", created)
	return created
}

// Kinds returns the sessions, the task files, the logs, the two dev_data
// logs and the state database.
func (p *Parser) Kinds() []parserkit.Kind {
	kinds := append([]parserkit.Kind{parserkit.PatternKind("session", sessionPattern)}, p.TaskKinds()...)
	kinds = append(kinds, parserkit.PatternKind("log", logPattern))
	for _, name := range []string{"chat", "tokens_generated"} {
		kinds = append(kinds, parserkit.RelFuncKind("dev_data "+name+".jsonl", func(rel string) bool {
			match := devDataPattern.FindStringSubmatch(rel)
			return match != nil && textlines.EqualFoldASCII(match[1], name)
		}))
	}
	return append(kinds, parserkit.PatternKind("state.vscdb", statePattern))
}

// Wants reports whether the artifact is a session (the index included),
// task, log, dev_data or state file.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Secondary reports whether the artifact is the output channel log or
// dev_data/chat.jsonl.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	match := devDataPattern.FindStringSubmatch(artifact.Rel)
	return logPattern.MatchString(artifact.Rel) || (match != nil && textlines.EqualFoldASCII(match[1], "chat"))
}

// Parse emits the rows of one PearAI file.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	rel := artifact.Rel
	session := sessionPattern.FindStringSubmatch(rel)
	devData := devDataPattern.FindStringSubmatch(rel)
	switch {
	case session != nil && textlines.EqualFoldASCII(session[1]+".json", continuedev.IndexName):
		parserkit.EmitAll(emit, p.ErrorRows(artifact, p.indexes.Errors(artifact.DiskPath), "", ""))
		if index, ok := parserkit.Beside(artifact, continuedev.IndexName); ok {
			parserkit.EmitAll(emit, p.indexes.Notices(index, p.BaseRow(artifact)))
		}
	case session != nil:
		p.session(artifact, session[1], emit)
	case p.TaskMatch(artifact):
		p.taskFile(artifact, options, emit)
	case logPattern.MatchString(rel):
		p.outputLog(artifact, emit)
	case devData != nil && textlines.EqualFoldASCII(devData[1], "chat"):
		p.devChat(artifact, emit)
	case devData != nil:
		p.tokens(artifact, emit)
	case statePattern.MatchString(rel):
		p.state(artifact, emit)
	}
	return nil
}

// globalState returns the reader of the extension's global state from a
// state.vscdb, for clinelegacy's cache. The repeated member names it returns
// are reported by the parse of the database itself, not a task file's lookup.
func globalState(artifact *input.Artifact) clinelegacy.Source {
	return func(path string, readerError func(error) string) (jsonvalue.Value, []string, jsonvalue.Repeats) {
		value, problems, repeats := vscodestate.ReadItem(artifact.Home.Listing, path, extensionID, readerError)
		return value, problems, repeats
	}
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
	_ clinelegacy.APIRower      = (*Parser)(nil)
	_ clinelegacy.Family        = (*Parser)(nil)
)
