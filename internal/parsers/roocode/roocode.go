package roocode

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers/clinelegacy"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/vscodestate"
)

// root matches the storage roots: the extension's global storage in any
// editor, and the CLI's.
const root = `(?:(?:.*/)?User/globalStorage/rooveterinaryinc\.roo-cline|\.vscode-mock/global-storage)`

// The files read, by path relative to the home.
const (
	cliHistoryRel  = ".roo/cli-history.json"
	cliSettingsRel = ".roo/cli-settings.json"
	projectMCPRel  = ".roo/mcp.json"
	globalStateRel = ".vscode-mock/global-storage/global-state.json"
)

// The patterns of the other files read.
var (
	mcpPattern            = parserkit.MustRelPattern(`^` + root + `/settings/mcp_settings\.json\z`)
	workspaceStatePattern = parserkit.MustRelPattern(`^\.vscode-mock/workspace-storage/[^/]+/workspace-state\.json\z`)
	workspaceDatabase     = parserkit.MustRelPattern(`(?:^|/)User/workspaceStorage/[^/]+/state\.vscdb\z`)
)

// Parser reads Roo Code's task files, editor state, settings and CLI
// history.
type Parser struct {
	*clinelegacy.TaskParser
}

// New returns the parser.
func New() *Parser {
	return &Parser{TaskParser: clinelegacy.NewTaskParser(parserkit.NewBase("roo-code", "roo-code"), root,
		"workspace", nil)}
}

// ReadsAgents names the editors whose state.vscdb holds the extension's
// global and workspace state.
func (p *Parser) ReadsAgents() []string {
	return vscodestate.EditorAgents()
}

// Kinds returns the CLI's files, the task files, the task history items
// and index, the MCP settings, the workspace state and the editors' state
// databases.
func (p *Parser) Kinds() []parserkit.Kind {
	kinds := []parserkit.Kind{parserkit.RelKind("cli-history.json", cliHistoryRel),
		parserkit.RelKind("cli-settings.json", cliSettingsRel), parserkit.RelKind("project mcp.json", projectMCPRel),
		parserkit.RelKind("global-state.json", globalStateRel)}
	kinds = append(append(kinds, p.TaskKinds()...), p.IndexKinds()...)
	return append(kinds, parserkit.PatternKind("mcp_settings.json", mcpPattern),
		parserkit.PatternKind("workspace-state.json", workspaceStatePattern),
		parserkit.RelFuncKind("state.vscdb", vscodestate.IsStateDatabase),
		parserkit.PatternKind("workspace state.vscdb", workspaceDatabase))
}

// Wants reports whether the artifact is a task, index, settings, state or
// CLI history file of Roo Code.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Secondary reports whether the artifact is the CLI's input history, which
// repeats task prompts.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return textlines.EqualFoldASCII(artifact.Rel, cliHistoryRel)
}

// Parse emits the rows of one Roo Code file.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	rel := artifact.Rel
	switch {
	case p.TaskMatch(artifact):
		p.taskFile(artifact, options, emit)
	case p.IndexMatch(artifact):
		p.ParseIndexFile(artifact, emit)
	case textlines.EqualFoldASCII(rel, cliHistoryRel):
		p.cliHistory(artifact, emit)
	case mcpPattern.MatchString(rel) || textlines.EqualFoldASCII(rel, projectMCPRel):
		p.mcp(artifact, emit)
	case textlines.EqualFoldASCII(rel, cliSettingsRel):
		p.cliSettings(artifact, emit)
	case textlines.EqualFoldASCII(rel, globalStateRel):
		if state, ok := p.stateFile(artifact, "global-state.json", emit); ok {
			p.globalStateRows(artifact, state, rel, emit)
		}
	case workspaceStatePattern.MatchString(rel):
		if state, ok := p.stateFile(artifact, "workspace-state.json", emit); ok {
			if problem := p.retryQueueRows(artifact, state, rel, emit); problem != "" {
				emit(p.ProblemRow(artifact, problem))
			}
		}
	case vscodestate.IsStateDatabase(rel) || workspaceDatabase.MatchString(rel):
		p.stateDatabase(artifact, emit)
	}
	return nil
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.AgentReader     = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
)
