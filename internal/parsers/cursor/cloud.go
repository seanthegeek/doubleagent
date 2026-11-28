package cursor

import (
	"cmp"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The cloud agent keys and the texts of their rows. The agent list has one
// ItemTable key per Cursor account.
const (
	cloudAgentsPrefix = "cloudAgentRepository.agents."
	githubReposKey    = "cloudEnvironmentTemplate.githubRepos"
	cloudOrigin       = "session origin: remote cursor cloud agent"
	cloudSource       = "(from state.vscdb cloudAgentRepository.agents, rowUpdatedAtMs)"
	cloudSynced       = "session synced: cursor cloud agent %s (vendor copy; from state.vscdb" +
		" cloudAgentRepository.agents)"
	reposSource   = "(from state.vscdb cloudEnvironmentTemplate.githubRepos, file mtime)"
	cloudCache    = "cloud-cache"
	mcpAuthKey    = "cloudAgentRepository.mcpAuthResolution"
	mcpAuthSource = "(from state.vscdb cloudAgentRepository.mcpAuthResolution, file mtime)"
)

// cloudAgent is one element of a cloud agent list and the rowid of its
// list.
type cloudAgent struct {
	rowid int64
	agent jsonvalue.Value
}

// readCloudAgents returns every element with a bcId of the cloud agent
// lists, in key order. A value that does not decode, or is not a JSON
// array, is a problem.
func readCloudAgents(store *store, problems *parserkit.Problems) []cloudAgent {
	table := store.table("ItemTable")
	if table == nil {
		return nil
	}
	var found []cloudAgent
	for _, row := range store.rows(substrPrefixed(table, cloudAgentsPrefix)) {
		for _, agent := range objectEntries(store.artifact, row, problems) {
			if !agent.Get("bcId").IsEmpty() {
				found = append(found, cloudAgent{rowid: row.rowid, agent: agent})
			}
		}
	}
	return found
}

// cloudModel returns a cloud agent's model (modelDetails.modelName, else
// the requested model) and the requested model's id.
func cloudModel(agent jsonvalue.Value) (string, string) {
	requested := agent.Get("requestedModel").Get("modelId").Text()
	model := agent.Get("modelDetails").Get("modelName").Text()
	if model == "" {
		model = requested
	}
	return model, requested
}

// repeatNotice emits the parser: row of a stored row of a key ItemTable
// holds in n rows, which only damage or an edit writes, the key being
// UNIQUE, when n is more than one; each stored row gives its own rows.
func (i *items) repeatNotice(n int, found keyed, emit parserkit.Emit) {
	if n > 1 {
		emit(i.system(repeatedKeyText("ItemTable", n), i.mtime, found.rowid))
	}
}

// cloudRepeats returns the repeat of each bcId the agents hold more than
// once, its group the parser's own for the state database at path, which
// the database's parse and conversation-search.db's share, or a new one
// when path is "".
func (p *Parser) cloudRepeats(path string, agents []cloudAgent) map[string]*cloudRepeat {
	counts := map[string]int{}
	for _, found := range agents {
		counts[found.agent.Get("bcId").Text()]++
	}
	repeats := map[string]*cloudRepeat{}
	for ident, count := range counts {
		if count < 2 {
			continue
		}
		group := timeline.NewGroup()
		if path != "" {
			key := [2]string{path, ident}
			if p.cloudGroups == nil {
				p.cloudGroups = map[[2]string]*timeline.Group{}
			}
			if p.cloudGroups[key] == nil {
				p.cloudGroups[key] = group
			}
			group = p.cloudGroups[key]
		}
		repeats[ident] = &cloudRepeat{group: group, count: count}
	}
	return repeats
}

// cloudRepeat is a bcId the cloud agent lists hold in more than one entry,
// which Cursor is not known to write, and the group of its rows
// (timeline.Group): each entry is a parent, giving its own rows and a
// parser: row with its own repository and model, and the agent's MCP
// decisions and its rows in conversation-search.db are members, shown once
// with the repository and model of the entry the group goes with.
type cloudRepeat struct {
	group *timeline.Group
	count int
}

// cloudText returns the "cloud agent:" text of a cloud agent, "-" for an
// absent value.
func cloudText(artifact *input.Artifact, agent jsonvalue.Value) string {
	return "cloud agent: " + cmp.Or(timeline.CompactValue(agent.Get("name")), "-") + " repo=" +
		parserkit.SettingOrDash(agent.Get("repoUrl")) +
		" status=" + parserkit.SettingOrDash(agent.Get("status")) + " workflowStatus=" + parserkit.SettingOrDash(agent.Get("workflowStatus")) +
		" archived=" + parserkit.SettingOrDash(agent.Get("isArchived")) + " killed=" + parserkit.SettingOrDash(agent.Get("isKilled")) +
		" vmStarted=" + parserkit.SettingOrDash(agent.Get("hasStartedVm")) + " " + cloudSource
}

// cloudAgent emits the origin, sync, model, source host and "cloud agent:"
// rows of one cloud agent; an entry of a repeated bcId first gives the
// parser: row that makes it a parent of the bcId's group.
func (i *items) cloudAgent(found cloudAgent, cached map[string]bool, repeat *cloudRepeat, emit parserkit.Emit) {
	agent := found.agent
	ident := agent.Get("bcId").Text()
	repo := agent.Get("repoUrl").Text()
	model, requested := cloudModel(agent)
	created := firstOrLast(agent.Get("createdAt"), agent.Get("rowUpdatedAtMs"))
	var parent *timeline.Parent
	row := func(text string, when jsonvalue.Value) {
		made := i.owner.row(i.artifact, timeline.TurnSystem, text, timeutil.FromValue(when), found.rowid)
		made.SessionID, made.ProjectPath, made.Model = ident, repo, model
		if parent != nil {
			parent.Row(&made, timeline.Always)
		}
		emit(made)
	}
	if repeat != nil {
		notice := i.owner.row(i.artifact, timeline.TurnSystem, "", timeutil.FromValue(created), found.rowid)
		notice.SessionID, notice.ProjectPath, notice.Model = ident, repo, model
		parent = repeat.group.Parent(&notice)
		prefix := parserkit.ProblemPrefix + "cloudAgentRepository.agents holds " + strconv.Itoa(repeat.count) +
			" entries of this row's bcId; its decisions and rows in other files are shown once, with "
		parent.Texts(&notice, timeline.RowText{Text: prefix + "this row"},
			timeline.RowText{Text: prefix + "another of them"})
		emit(notice)
	}
	if !cached[ident] {
		row(cloudOrigin, created)
	}
	row(strings.Replace(cloudSynced, "%s", ident, 1), created)
	if model != "" {
		extra := ""
		if requested != "" && requested != model {
			extra = " requested=" + requested
		}
		row("model: "+model+extra, created)
	}
	if github := githubRepository(repo); github != "" {
		row("source host: github.com "+github+" (cloud agent)", created)
	}
	row(cloudText(i.artifact, agent), firstOrLast(agent.Get("rowUpdatedAtMs"), created))
}

// mcpAnswers emits the decisions of one stored row of the MCP
// authorisation key. A value that is neither an answer nor a list, and a
// list's entries that are not answers, are problems.
func (i *items) mcpAnswers(found keyed, known map[string]jsonvalue.Value, emit parserkit.Emit) {
	document := decodeRecord(i.artifact, found.rowid, mcpAuthKey, found.value, i.problems)
	// The value is one answer, or a list of them whose entries of another
	// kind are one problem.
	resolutions := []jsonvalue.Value{document}
	if !document.IsObject() {
		resolutions = nil
		for _, entry := range parserkit.EntriesOfKind(document, mcpAuthKey, jsonvalue.Object, int(found.rowid),
			i.problems) {
			resolutions = append(resolutions, entry)
		}
	}
	for _, resolution := range resolutions {
		agentID, callID := resolution.Get("agentId").Text(), resolution.Get("toolCallId").Text()
		row := i.system("permission decision: "+parserkit.SettingOrDash(resolution.Get("userDecision"))+
			" mcp auth for cloud agent "+cmp.Or(agentID, "-")+" tool_call="+cmp.Or(callID, "-")+" "+mcpAuthSource,
			i.mtime, found.rowid)
		row.SessionID, row.ToolUseID = agentID, callID
		if agent, ok := known[agentID]; ok {
			row.ProjectPath = agent.Get("repoUrl").Text()
			row.Model, _ = cloudModel(agent)
		}
		emit(row)
	}
}

// mcpAuth emits a "permission decision:" row per cloud agent MCP
// authorisation answer of each stored row of the key, in the agent's
// session, joined on its toolCallId, at the file's modification time.
func (i *items) mcpAuth(agents []cloudAgent, repeats map[string]*cloudRepeat, emit parserkit.Emit) {
	known := map[string]jsonvalue.Value{}
	for _, each := range agents {
		if ident := each.agent.Get("bcId").Text(); !known[ident].IsObject() {
			known[ident] = each.agent
		}
	}
	stored := keyedValues(i.store, i.table, mcpAuthKey)
	for _, found := range stored {
		i.repeatNotice(len(stored), found, emit)
		i.mcpAnswers(found, known, func(row timeline.Row) {
			if repeat := repeats[row.SessionID]; repeat != nil {
				repeat.group.Member(&row, timeline.TakeProject|timeline.TakeModel)
			}
			emit(row)
		})
	}
}

// cloud emits the rows of the cloud agent lists, the MCP authorisations
// and the repository list. An agent's origin row is left to
// conversation-search.db when that has a cloud-cache row for it. A
// repository list that is not a JSON array is a problem.
func (i *items) cloud(emit parserkit.Emit) {
	if i.table == nil {
		return
	}
	cached := cloudCachedIDs(i.artifact)
	agents := readCloudAgents(i.store, i.problems)
	repeats := i.owner.cloudRepeats(sharedState(i.artifact), agents)
	for _, found := range agents {
		i.cloudAgent(found, cached, repeats[found.agent.Get("bcId").Text()], emit)
	}
	i.mcpAuth(agents, repeats, emit)
	stored := keyedValues(i.store, i.table, githubReposKey)
	for _, found := range stored {
		i.repeatNotice(len(stored), found, emit)
		for _, repo := range objectEntries(i.artifact, found, i.problems) {
			target := repo.Get("url").Text()
			if target == "" {
				target = cmp.Or(repo.Get("name").Text(), "-")
			}
			emit(i.system("source host repository: "+target+" type="+parserkit.SettingOrDash(repo.Get("type"))+" "+
				reposSource, i.mtime, found.rowid))
		}
	}
}
