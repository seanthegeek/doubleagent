package cursor

import (
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
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
// lists, in key order. A value that is not a JSON list is skipped, one
// that starts as a list and does not decode is a problem.
func readCloudAgents(store *store, problems *parser.Problems) []cloudAgent {
	table := store.table("ItemTable")
	if table == nil {
		return nil
	}
	var found []cloudAgent
	for _, row := range store.rows(substrPrefixed(table, cloudAgentsPrefix)) {
		if !lstripsToList(asText(row.value)) {
			continue
		}
		for _, agent := range decodeRecord(row.rowid, row.key, row.value, problems).Items() {
			if agent.IsObject() && !agent.Get("bcId").IsEmpty() {
				found = append(found, cloudAgent{rowid: row.rowid, agent: agent})
			}
		}
	}
	return found
}

// cloudModel returns a cloud agent's model (modelDetails.modelName, else
// the requested model) and the requested model's id.
func cloudModel(agent jsonvalue.Value) (string, string) {
	requested := jsonText(agent.Get("requestedModel").Get("modelId"))
	model := jsonText(agent.Get("modelDetails").Get("modelName"))
	if model == "" {
		model = requested
	}
	return model, requested
}

// cloud emits the rows of the cloud agent lists, the MCP authorisations
// and the repository list. An agent's origin row is left to
// conversation-search.db when that has a cloud-cache row for it.
func (i *items) cloud(emit parser.Emit) {
	if i.table == nil {
		return
	}
	cached := cloudCachedIDs(i.artifact)
	agents := readCloudAgents(i.store, i.problems)
	for _, found := range agents {
		i.cloudAgent(found, cached, emit)
	}
	i.mcpAuth(agents, emit)
	found, ok := keyedValue(i.store, i.table, githubReposKey)
	if !ok {
		return
	}
	for _, repo := range decodeRecord(found.rowid, githubReposKey, found.value, i.problems).Items() {
		if repo.IsObject() {
			target := jsonText(repo.Get("url"))
			if target == "" {
				target = orDash(jsonText(repo.Get("name")))
			}
			emit(i.system("source host repository: "+target+" type="+shown(repo.Get("type"))+" "+reposSource,
				i.mtime, found.rowid))
		}
	}
}

// cloudAgent emits the origin, sync, model, source host and "cloud agent:"
// rows of one cloud agent.
func (i *items) cloudAgent(found cloudAgent, cached map[string]bool, emit parser.Emit) {
	agent := found.agent
	ident := agent.Get("bcId").Text()
	repo := jsonText(agent.Get("repoUrl"))
	model, requested := cloudModel(agent)
	created := firstTruthy(agent.Get("createdAt"), agent.Get("rowUpdatedAtMs"))
	row := func(text string, when jsonvalue.Value) {
		made := i.owner.row(i.artifact, timeline.TurnSystem, text, timeutil.FromValue(when), found.rowid)
		made.SessionID, made.ProjectPath, made.Model = ident, repo, model
		emit(made)
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
	row(cloudText(agent), firstTruthy(agent.Get("rowUpdatedAtMs"), created))
}

// cloudText returns the "cloud agent:" text of a cloud agent, "-" for an
// absent value.
func cloudText(agent jsonvalue.Value) string {
	return "cloud agent: " + orDash(timeline.CompactValue(agent.Get("name"))) + " repo=" + shown(agent.Get("repoUrl")) +
		" status=" + shown(agent.Get("status")) + " workflowStatus=" + shown(agent.Get("workflowStatus")) +
		" archived=" + shown(agent.Get("isArchived")) + " killed=" + shown(agent.Get("isKilled")) +
		" vmStarted=" + shown(agent.Get("hasStartedVm")) + " " + cloudSource
}

// mcpAuth emits a "permission decision:" row per cloud agent MCP
// authorisation answer, in the agent's session, joined on its toolCallId,
// at the file's modification time.
func (i *items) mcpAuth(agents []cloudAgent, emit parser.Emit) {
	found, ok := keyedValue(i.store, i.table, mcpAuthKey)
	if !ok {
		return
	}
	document := decodeRecord(found.rowid, mcpAuthKey, found.value, i.problems)
	known := map[string]jsonvalue.Value{}
	for _, each := range agents {
		known[each.agent.Get("bcId").Text()] = each.agent
	}
	resolutions := []jsonvalue.Value{document}
	if document.IsArray() {
		resolutions = document.Items()
	}
	for _, resolution := range resolutions {
		if !resolution.IsObject() {
			continue
		}
		agentID, callID := jsonText(resolution.Get("agentId")), jsonText(resolution.Get("toolCallId"))
		row := i.system("permission decision: "+fieldOf(resolution.Get("userDecision"))+
			" mcp auth for cloud agent "+orDash(agentID)+" tool_call="+orDash(callID)+" "+mcpAuthSource,
			i.mtime, found.rowid)
		row.SessionID, row.ToolUseID = agentID, callID
		if agent, ok := known[agentID]; ok {
			row.ProjectPath = jsonText(agent.Get("repoUrl"))
			row.Model, _ = cloudModel(agent)
		}
		emit(row)
	}
}
