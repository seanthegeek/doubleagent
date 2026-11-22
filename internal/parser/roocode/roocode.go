// Package roocode reads Roo Code's tasks, their permission decisions, its
// standing settings and the CLI's prompt history.
//
// Roo Code forked Cline and keeps its per-task layout, read through package
// clinelegacy, under the extension's
// User/globalStorage/rooveterinaryinc.roo-cline/ and the CLI's
// .vscode-mock/global-storage/. Besides the task files it reads:
//
//   - tasks/<taskId>/history_item.json (the task's index entry; workspace is
//     the project path, mode gives a "permission mode:" row) and
//     tasks/_index.json (only entries whose history_item.json is gone);
//   - the global state (the extension's row of the editor's state.vscdb, or
//     the CLI's global-state.json): the standing permission keys and the
//     Roo Code Cloud settings cache as setting rows; the workspace state's
//     roo.retryQueue[], one "cloud upload queued:" row per request with
//     its bearer token through parser.SecretText, never its body;
//   - .roo/cli-settings.json, settings/mcp_settings.json and a project's
//     .roo/mcp.json (mcpServers.<name>.alwaysAllow only);
//   - .roo/cli-history.json, a secondary source repeating the tasks'
//     prompts: one user row per entry, with no time, task or project.
//
// A ui_messages.json tool ask marked isAnswered and an API history result
// opening with the JSON Roo writes for a denial or an approval with
// feedback give "permission decision:" rows. See research/roo-code.md.
package roocode

import (
	"path"
	"regexp"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/clinelegacy"
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
	itemPattern           = regexp.MustCompile(`(?i)^` + root + `/tasks/[^/]+/history_item\.json\z`)
	indexPattern          = regexp.MustCompile(`(?i)^` + root + `/tasks/_index\.json\z`)
	mcpPattern            = regexp.MustCompile(`(?i)^` + root + `/settings/mcp_settings\.json\z`)
	workspaceStatePattern = regexp.MustCompile(`^\.vscode-mock/workspace-storage/[^/]+/workspace-state\.json\z`)
	workspaceDatabase     = regexp.MustCompile(`(?:^|/)User/workspaceStorage/[^/]+/state\.vscdb\z`)
)

// Parser reads Roo Code's task files, editor state, settings and CLI
// history.
type Parser struct {
	*clinelegacy.TaskParser
}

// New returns the parser.
func New() *Parser {
	created := &Parser{}
	created.TaskParser = clinelegacy.NewTaskParser(parser.NewBase("roo-code", "roo-code"), root,
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
	return parser.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the CLI's files, the task files, the task history items
// and index, the MCP settings, the workspace state and the editors' state
// databases.
func (p *Parser) Kinds() []parser.Kind {
	kinds := []parser.Kind{parser.RelKind("cli-history.json", cliHistoryRel),
		parser.RelKind("cli-settings.json", cliSettingsRel), parser.RelKind("project mcp.json", projectMCPRel),
		parser.RelKind("global-state.json", globalStateRel)}
	return append(append(kinds, p.TaskKinds()...), parser.PatternKind("history_item.json", itemPattern),
		parser.PatternKind("tasks _index.json", indexPattern), parser.PatternKind("mcp_settings.json", mcpPattern),
		parser.PatternKind("workspace-state.json", workspaceStatePattern),
		parser.RelFuncKind("state.vscdb", vscodestate.IsStateDatabase),
		parser.PatternKind("workspace state.vscdb", workspaceDatabase))
}

// Secondary reports whether the artifact is the CLI's input history, which
// repeats task prompts.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return artifact.Rel == cliHistoryRel
}

// HistoryItem returns the task's history_item.json, else its
// tasks/_index.json entry, with the group of its id when the index repeats
// it, else null.
func (p *Parser) HistoryItem(task clinelegacy.Task) (jsonvalue.Value, *timeline.Group) {
	var ignored parser.Problems
	// The item is also parsed as its own artifact, whose parse reports its
	// repeated member names; a lookup from a task's parse notes none.
	if item, _ := parser.LoadJSON(nil, task.InDir(itemFile), &ignored); item.IsObject() {
		return item, nil
	}
	indexPath, found := task.InRoot("tasks/_index.json")
	if !found {
		return jsonvalue.NewNull(), nil
	}
	return p.CachedEntry(indexPath, "entries", task.TaskID), p.EntryGroup(indexPath, "entries", task.TaskID)
}

// Parse emits the rows of one Roo Code file.
func (p *Parser) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	rel := artifact.Rel
	switch {
	case p.TaskMatch(artifact):
		p.taskFile(artifact, options, emit)
	case itemPattern.MatchString(rel):
		p.historyItem(artifact, emit)
	case indexPattern.MatchString(rel):
		p.index(artifact, emit)
	case rel == cliHistoryRel:
		p.cliHistory(artifact, emit)
	case mcpPattern.MatchString(rel) || rel == projectMCPRel:
		p.mcp(artifact, emit)
	case rel == cliSettingsRel:
		p.cliSettings(artifact, emit)
	case rel == globalStateRel:
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
func (p *Parser) historyItem(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	item, _ := parser.LoadJSON(artifact, artifact.DiskPath, &problems)
	if item.IsObject() {
		p.historyRows(artifact, item, 1, nil, emit)
		return
	}
	if len(problems) == 0 {
		problems.Add(0, "history_item.json is not a JSON object")
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, path.Base(path.Dir(artifact.Rel)), ""))
}

// historyRows emits the task row of an index entry, then "permission
// mode: <mode>" from its mode, the slug that decides the tool groups the
// task may use; both are the rows of a new parent of group when it is set.
func (p *Parser) historyRows(artifact *input.Artifact, item jsonvalue.Value, position int,
	group *timeline.Group, emit parser.Emit) {
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
func (p *Parser) index(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	index, _ := parser.LoadJSON(artifact, artifact.DiskPath, &problems)
	if !index.Get("entries").IsArray() {
		if len(problems) == 0 {
			problems.Add(0, "_index.json has no entries array")
		}
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	for i, entry := range parser.EntriesOfKind(index.Get("entries"), "entries", jsonvalue.Object, 0, &problems) {
		taskID, named := parser.Component(parser.Text(entry.Get("id")))
		if item, ok := parser.Beside(artifact, taskID, itemFile); named && ok && parser.RegularFile(item) {
			continue
		}
		var group *timeline.Group
		if indexPath, shared := parser.Beside(artifact, "_index.json"); shared {
			group = p.EntryGroup(indexPath, "entries", entry.Get("id").Text())
		}
		p.historyRows(artifact, entry, i+1, group, emit)
	}
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parser.Parser          = (*Parser)(nil)
	_ parser.AgentReader     = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
)
