package analyze

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// detectedAgents returns the agents of the homes, but the shared project
// files no single agent owns, and of the project directories, in that
// order: shared project files reach every parser, so they are never an
// agent of their own.
func detectedAgents(detected detection) []agentFiles {
	var agents []agentFiles
	for _, home := range detected.homes {
		for _, agent := range home.agents {
			if agent.agent != input.ProjectAgent {
				agents = append(agents, agent)
			}
		}
	}
	for _, project := range detected.projects {
		agents = append(agents, project.agents...)
	}
	return agents
}

// agentLists returns the detected agents without a parser that are a parser
// backlog, those whose files hold nothing to parse, and those whose parser
// ran and found no rows (an agent --agent left out did not run).
func (r *timelineRun) agentLists(agents []agentFiles) (unparsed, collectedOnly, noRows []string) {
	nothing := parsers.NoTranscripts()
	for _, entry := range agents {
		switch {
		case !entry.parser && slices.Contains(nothing, entry.agent):
			collectedOnly = append(collectedOnly, entry.agent)
		case !entry.parser:
			unparsed = append(unparsed, entry.agent)
		case !slices.Contains(r.parsed, entry.agent) &&
			(len(r.options.Agents) == 0 || slices.Contains(r.options.Agents, entry.agent)):
			noRows = append(noRows, entry.agent)
		}
	}
	return sortedSet(unparsed), sortedSet(collectedOnly), sortedSet(noRows)
}

// sortedSet returns the distinct texts, sorted.
func sortedSet(texts []string) []string {
	unique := map[string]bool{}
	for _, text := range texts {
		unique[text] = true
	}
	return sortedKeys(unique)
}

// agentsSummary returns the text of the agents: line.
func agentsSummary(detected bool, parsed, unparsed, noRows, collectedOnly []string) string {
	names := func(agents []string) string {
		if len(agents) == 0 {
			return "none"
		}
		return strings.Join(sortedSet(agents), ", ")
	}
	if !detected && len(parsed) == 0 {
		return "none were found"
	}
	text := "parsed: " + names(parsed) + "; detected but not parsed: " + names(unparsed)
	if len(collectedOnly) > 0 {
		text += "; collected only: " + names(collectedOnly)
	}
	if len(noRows) > 0 {
		text += "; parser found no rows: " + names(noRows)
	}
	return text
}

// markCounts returns how many sessions are synced, relayed, remote and
// recovered.
func markCounts(sessions []timeline.SessionSummary) [4]int {
	var counts [4]int
	for _, session := range sessions {
		marks := sessionMarks(session)
		for i, marked := range marks {
			if marked {
				counts[i]++
			}
		}
	}
	return counts
}

// sessionMarks reports whether a session is synced, relayed, remote and
// recovered.
func sessionMarks(session timeline.SessionSummary) [4]bool {
	return [4]bool{session.Synced == "yes", session.Relayed == "yes", session.Origin == "remote",
		slices.Contains(recoveredValues(), session.Recovered)}
}

// markNames returns the keys and stdout words of the four session marks,
// in the order of markCounts.
func markNames() [4][2]string {
	return [4][2]string{{"synced_sessions", "synced"}, {"relayed_sessions", "relayed"},
		{"remote_sessions", "remote"}, {"recovered_sessions", "recovered"}}
}

// printTotals prints the agents, rows, sessions and inventory lines.
func (r *timelineRun) printTotals(w io.Writer, agents []agentFiles) {
	unparsed, collectedOnly, noRows := r.agentLists(agents)
	fmt.Fprint(w, "agents:    "+shown(agentsSummary(len(agents) > 0, r.parsed, unparsed, noRows,
		collectedOnly))+"\n")
	fmt.Fprintf(w, "rows:      %d in %s\n", len(r.rows), shown(r.outputPath(TimelineFile)))
	keys := make([][2]string, 0, len(r.counts))
	for key := range r.counts {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(left, right [2]string) int {
		if order := strings.Compare(left[0], right[0]); order != 0 {
			return order
		}
		return strings.Compare(left[1], right[1])
	})
	for _, key := range keys {
		fmt.Fprintf(w, "           %-14s %-12s %d\n", shown(key[0]), shown(key[1]), r.counts[key])
	}
	var marks []string
	for i, count := range markCounts(r.sessions) {
		if count > 0 {
			marks = append(marks, fmt.Sprintf("%d %s", count, markNames()[i][1]))
		}
	}
	marked := ""
	if len(marks) > 0 {
		marked = " (" + strings.Join(marks, ", ") + ")"
	}
	fmt.Fprintf(w, "sessions:  %d in %s%s\n", len(r.sessions), shown(r.outputPath(SessionsFile)), marked)
	fmt.Fprint(w, "inventory: "+shown(r.outputPath(InventoryFile))+
		" (homes, projects, scratch directories, counts per agent, problems)\n")
}

// inventoryJSON is inventory.json, its keys in the documented order.
type inventoryJSON struct {
	Version            string                      `json:"doubleagent_version"`
	Input              string                      `json:"input"`
	Kind               string                      `json:"kind"`
	Host               string                      `json:"host"`
	Notes              []string                    `json:"notes"`
	Problems           []string                    `json:"problems"`
	Rows               int                         `json:"rows"`
	Sessions           int                         `json:"sessions"`
	Parsed             []string                    `json:"parsed"`
	DetectedNotParsed  []string                    `json:"detected_not_parsed"`
	NoRows             []string                    `json:"no_rows"`
	Homes              []inventoryHomeJSON         `json:"homes"`
	Projects           map[string]projectValueJSON `json:"projects"`
	OutsideProjects    projectValueJSON            `json:"outside_projects"`
	Deduplicated       map[string]int              `json:"deduplicated"`
	CollectedOnly      []string                    `json:"collected_only"`
	SyncedSessions     int                         `json:"synced_sessions"`
	RelayedSessions    int                         `json:"relayed_sessions"`
	RemoteSessions     int                         `json:"remote_sessions"`
	RecoveredSessions  int                         `json:"recovered_sessions"`
	ScratchDirectories map[string]projectValueJSON `json:"scratch_directories"`
}

// inventoryHomeJSON is one home of inventory.json: a home whose agents carry
// the counts of their sessions with each mark.
type inventoryHomeJSON struct {
	User     string               `json:"user"`
	Home     string               `json:"home"`
	Inferred bool                 `json:"inferred"`
	Agents   []inventoryAgentJSON `json:"agents"`
}

// inventoryAgentJSON is one agent of a home of inventory.json.
type inventoryAgentJSON struct {
	agentJSON
	SyncedSessions    int `json:"synced_sessions"`
	RelayedSessions   int `json:"relayed_sessions"`
	RemoteSessions    int `json:"remote_sessions"`
	RecoveredSessions int `json:"recovered_sessions"`
}

// inventory returns inventory.json's document. The projects and scratch
// directories are keyed by display path, which encoding/json writes in
// sorted order, the order entries gives them.
func (r *timelineRun) inventory(detected detection, agents []agentFiles) inventoryJSON {
	unparsed, collectedOnly, noRows := r.agentLists(agents)
	files := projectFiles(detected.projects, r.collection.Summary)
	marks := projectMarks(r.registry.Marks())
	projects := newProjectInventory(r.rows, r.sessions, collectedProjects(r.collection), files)
	entries := projects.entries(marks, recordedRemotes(r.rows, r.sessions))
	scratch := r.found.scratchKeys(r.collection, r.rows)
	counts := markCounts(r.sessions)
	return inventoryJSON{Version: r.env.Version, Input: detected.input, Kind: detected.kind,
		Host: detected.host, Notes: list(detected.notes), Problems: list(r.problems),
		Rows: len(r.rows), Sessions: len(r.sessions), Parsed: list(r.parsed),
		DetectedNotParsed: list(unparsed), NoRows: list(noRows),
		Homes: r.homesWithMarks(detected.homes), Projects: projectsWhere(entries, scratch, false),
		OutsideProjects: outsideProjects(entries, marks), Deduplicated: droppedCounts(r.dropped),
		CollectedOnly: list(collectedOnly), SyncedSessions: counts[0], RelayedSessions: counts[1],
		RemoteSessions: counts[2], RecoveredSessions: counts[3],
		ScratchDirectories: projectsWhere(entries, scratch, true)}
}

// homesWithMarks returns the homes array of inventory.json: each agent
// object ends with the counts of that user's sessions of the agent with
// each mark.
func (r *timelineRun) homesWithMarks(homes []homeEntry) []inventoryHomeJSON {
	counts := map[[2]string][4]int{}
	for _, session := range r.sessions {
		key := [2]string{session.User, session.Agent}
		total := counts[key]
		for i, marked := range sessionMarks(session) {
			if marked {
				total[i]++
			}
		}
		counts[key] = total
	}
	items := make([]inventoryHomeJSON, len(homes))
	for i, home := range homes {
		agents := make([]inventoryAgentJSON, len(home.agents))
		for j, agent := range agentsJSON(home.agents) {
			total := counts[[2]string{home.user, agent.Agent}]
			agents[j] = inventoryAgentJSON{agentJSON: agent, SyncedSessions: total[0],
				RelayedSessions: total[1], RemoteSessions: total[2], RecoveredSessions: total[3]}
		}
		items[i] = inventoryHomeJSON{User: home.user, Home: home.home, Inferred: home.inferred,
			Agents: agents}
	}
	return items
}

// projectsWhere returns the projects of the project paths that are scratch
// directories, or of those that are not; the empty path is in neither.
func projectsWhere(entries []projectEntryJSON, scratch map[string]bool,
	isScratch bool) map[string]projectValueJSON {
	projects := map[string]projectValueJSON{}
	for _, entry := range entries {
		if entry.path != "" && scratch[timeline.ProjectKey(entry.path)] == isScratch {
			projects[entry.path] = entry.value
		}
	}
	return projects
}

// outsideProjects returns the entry of the rows and sessions with no
// project path, or an empty one.
func outsideProjects(entries []projectEntryJSON, marks []sessionMark) projectValueJSON {
	for _, entry := range entries {
		if entry.path == "" {
			return entry.value
		}
	}
	return newProjectValue(0, 0, map[string]agentTallyJSON{}, make([]string, len(marks)))
}

// droppedCounts returns the deduplicated object: per agent with rows
// dropped, the count.
func droppedCounts(dropped map[string]int) map[string]int {
	counts := map[string]int{}
	for agent, count := range dropped {
		if count > 0 {
			counts[agent] = count
		}
	}
	return counts
}
