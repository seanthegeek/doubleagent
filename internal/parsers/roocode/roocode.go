package roocode

import (
	"path"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers/clinelegacy"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
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
	itemFile       = "history_item.json"
)

// The patterns of the other files read.
var (
	itemPattern           = parserkit.MustRelPattern(`^` + root + `/tasks/[^/]+/history_item\.json\z`)
	indexPattern          = parserkit.MustRelPattern(`^` + root + `/tasks/_index\.json\z`)
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
	created := &Parser{}
	created.TaskParser = clinelegacy.NewTaskParser(parserkit.NewBase("roo-code", "roo-code"), root,
		"workspace", created)
	return created
}

// ReadsAgents names the editors whose state.vscdb holds the extension's
// global and workspace state.
func (p *Parser) ReadsAgents() []string {
	return vscodestate.EditorAgents()
}

// Wants reports whether the artifact is a task, index, settings, state or
// CLI history file of Roo Code.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the CLI's files, the task files, the task history items
// and index, the MCP settings, the workspace state and the editors' state
// databases.
func (p *Parser) Kinds() []parserkit.Kind {
	kinds := []parserkit.Kind{parserkit.RelKind("cli-history.json", cliHistoryRel),
		parserkit.RelKind("cli-settings.json", cliSettingsRel), parserkit.RelKind("project mcp.json", projectMCPRel),
		parserkit.RelKind("global-state.json", globalStateRel)}
	return append(append(kinds, p.TaskKinds()...), parserkit.PatternKind("history_item.json", itemPattern),
		parserkit.PatternKind("tasks _index.json", indexPattern), parserkit.PatternKind("mcp_settings.json", mcpPattern),
		parserkit.PatternKind("workspace-state.json", workspaceStatePattern),
		parserkit.RelFuncKind("state.vscdb", vscodestate.IsStateDatabase),
		parserkit.PatternKind("workspace state.vscdb", workspaceDatabase))
}

// Secondary reports whether the artifact is the CLI's input history, which
// repeats task prompts.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return textlines.EqualFoldASCII(artifact.Rel, cliHistoryRel)
}

// HistoryItem returns the task's history_item.json, else its
// tasks/_index.json entry, with the group of its id when the index repeats
// it, else null.
func (p *Parser) HistoryItem(task clinelegacy.Task) (jsonvalue.Value, *timeline.Group) {
	var ignored parserkit.Problems
	// The item is also parsed as its own artifact, whose parse reports its
	// repeated member names; a lookup from a task's parse notes none.
	if item, _ := parserkit.LoadJSON(nil, task.InDir(itemFile), &ignored); item.IsObject() {
		return item, nil
	}
	indexPath, found := task.InRoot("tasks/_index.json")
	if !found {
		return jsonvalue.NewNull(), nil
	}
	return p.CachedEntry(indexPath, "entries", task.TaskID), p.EntryGroup(indexPath, "entries", task.TaskID)
}

// Parse emits the rows of one Roo Code file.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	rel := artifact.Rel
	switch {
	case p.TaskMatch(artifact):
		p.taskFile(artifact, options, emit)
	case itemPattern.MatchString(rel):
		p.historyItem(artifact, emit)
	case indexPattern.MatchString(rel):
		p.index(artifact, emit)
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
				emit(p.badFile(artifact, problem))
			}
		}
	case vscodestate.IsStateDatabase(rel) || workspaceDatabase.MatchString(rel):
		p.stateDatabase(artifact, emit)
	}
	return nil
}

// historyItem emits the task row of a history_item.json.
func (p *Parser) historyItem(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	item, _ := parserkit.LoadJSON(artifact, artifact.DiskPath, &problems)
	if item.IsObject() {
		p.historyRows(artifact, item, 1, nil, emit)
		return
	}
	if len(problems) == 0 {
		problems.Add(0, "history_item.json is not a JSON object")
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, path.Base(path.Dir(artifact.Rel)), ""))
}

// historyRows emits the task row of an index entry, then "permission
// mode: <mode>" from its mode, the slug that decides the tool groups the
// task may use; both are the rows of a new parent of group when it is set.
func (p *Parser) historyRows(artifact *input.Artifact, item jsonvalue.Value, position int,
	group *timeline.Group, emit parserkit.Emit) {
	row := p.HistoryRow(artifact, item, position)
	if group != nil {
		group.Parent(&row)
	}
	emit(row)
	if mode := item.Get("mode"); mode.IsString() && mode.StringValue() != "" {
		emit(systemLike(row, "permission mode: "+mode.StringValue()))
	}
}

// index emits the task rows of the tasks/_index.json entries whose
// history_item.json is gone; an id that is not a plain file name has none.
// The rows of each entry of an id the index repeats are a parent of the
// id's group, which the task's files share (EntryGroup).
func (p *Parser) index(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	index, _ := parserkit.LoadJSON(artifact, artifact.DiskPath, &problems)
	if !index.Get("entries").IsArray() {
		if len(problems) == 0 {
			problems.Add(0, "_index.json has no entries array")
		}
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	for i, entry := range parserkit.EntriesOfKind(index.Get("entries"), "entries", jsonvalue.Object, 0, &problems) {
		taskID, named := parserkit.Component(parserkit.Text(entry.Get("id")))
		if item, ok := parserkit.Beside(artifact, taskID, itemFile); named && ok && parserkit.RegularFile(item) {
			continue
		}
		var group *timeline.Group
		if indexPath, shared := parserkit.Beside(artifact, "_index.json"); shared {
			group = p.EntryGroup(indexPath, "entries", entry.Get("id").Text())
		}
		p.historyRows(artifact, entry, i+1, group, emit)
	}
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.AgentReader     = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
)
