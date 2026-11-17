package analyze

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
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
	text := fmt.Sprintf("parsed: %s; detected but not parsed: %s", names(parsed), names(unparsed))
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
	fmt.Fprintf(w, "agents:    %s\n", agentsSummary(len(agents) > 0, r.parsed, unparsed, noRows,
		collectedOnly))
	fmt.Fprintf(w, "rows:      %d in %s\n", len(r.rows), r.outputPath(TimelineFile))
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
		fmt.Fprintf(w, "           %-14s %-12s %d\n", key[0], key[1], r.counts[key])
	}
	var marks []string
	for i, count := range markCounts(r.sessions) {
		if count > 0 {
			marks = append(marks, fmt.Sprintf("%d %s", count, markNames()[i][1]))
		}
	}
	shown := ""
	if len(marks) > 0 {
		shown = " (" + strings.Join(marks, ", ") + ")"
	}
	fmt.Fprintf(w, "sessions:  %d in %s%s\n", len(r.sessions), r.outputPath(SessionsFile), shown)
	fmt.Fprintf(w, "inventory: %s (homes, projects, scratch directories, counts per agent, problems)\n",
		r.outputPath(InventoryFile))
}

// inventory returns the text of inventory.json: indented, ASCII only, and
// without a final newline.
func (r *timelineRun) inventory(detected detection, agents []agentFiles) string {
	unparsed, collectedOnly, noRows := r.agentLists(agents)
	files := projectFiles(detected.projects, r.collection.Summary)
	marks := projectMarks(r.registry.Marks())
	projects := newProjectInventory(r.rows, r.sessions, collectedProjects(r.collection), files)
	entries := projects.entries(marks, recordedRemotes(r.rows, r.sessions))
	scratch := r.found.scratchKeys(r.collection, r.rows)
	members := []jsonvalue.Member{member("doubleagent_version", jsonvalue.NewString(r.env.Version))}
	members = append(members, detectionMembers(detected)[:4]...)
	members = append(members,
		member("problems", stringArray(r.problems)),
		member("rows", jsonvalue.NewInt(int64(len(r.rows)))),
		member("sessions", jsonvalue.NewInt(int64(len(r.sessions)))),
		member("parsed", stringArray(r.parsed)),
		member("detected_not_parsed", stringArray(unparsed)),
		member("no_rows", stringArray(noRows)),
		member("homes", r.homesWithMarks(detected.homes)),
		member("projects", projectsWhere(entries, scratch, false)),
		member("outside_projects", outsideProjects(entries, marks)),
		member("deduplicated", droppedJSON(r.dropped)),
		member("collected_only", stringArray(collectedOnly)))
	for i, count := range markCounts(r.sessions) {
		members = append(members, member(markNames()[i][0], jsonvalue.NewInt(int64(count))))
	}
	members = append(members, member("scratch_directories", projectsWhere(entries, scratch, true)))
	return jsonvalue.NewObject(members...).IndentedJSON("  ")
}

// homesWithMarks returns the homes array of inventory.json: each agent
// object ends with the counts of that user's sessions of the agent with
// each mark.
func (r *timelineRun) homesWithMarks(homes []homeEntry) jsonvalue.Value {
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
	items := make([]jsonvalue.Value, len(homes))
	for i, home := range homes {
		items[i] = jsonvalue.NewObject(homeMembers(home, func(agent string) []jsonvalue.Member {
			total := counts[[2]string{home.user, agent}]
			members := make([]jsonvalue.Member, 0, 4)
			for j, names := range markNames() {
				members = append(members, member(names[0], jsonvalue.NewInt(int64(total[j]))))
			}
			return members
		})...)
	}
	return jsonvalue.NewArray(items...)
}

// projectsWhere returns the projects object of the project paths that are
// scratch directories, or of those that are not; the empty path is in
// neither.
func projectsWhere(entries []projectEntryJSON, scratch map[string]bool, isScratch bool) jsonvalue.Value {
	var members []jsonvalue.Member
	for _, entry := range entries {
		if entry.path != "" && scratch[timeline.ProjectKey(entry.path)] == isScratch {
			members = append(members, member(entry.path, entry.value))
		}
	}
	return jsonvalue.NewObject(members...)
}

// outsideProjects returns the entry of the rows and sessions with no
// project path, or an empty one.
func outsideProjects(entries []projectEntryJSON, marks []sessionMark) jsonvalue.Value {
	for _, entry := range entries {
		if entry.path == "" {
			return entry.value
		}
	}
	members := []jsonvalue.Member{member("rows", jsonvalue.NewInt(0)), member("sessions", jsonvalue.NewInt(0)),
		member("agents", jsonvalue.NewObject())}
	for _, mark := range marks {
		members = append(members, member(mark.key, jsonvalue.NewString("")))
	}
	return jsonvalue.NewObject(members...)
}

// droppedJSON returns the deduplicated object: per agent with rows
// dropped, sorted, the count.
func droppedJSON(dropped map[string]int) jsonvalue.Value {
	var members []jsonvalue.Member
	for _, agent := range sortedKeys(dropped) {
		if dropped[agent] > 0 {
			members = append(members, member(agent, jsonvalue.NewInt(int64(dropped[agent]))))
		}
	}
	return jsonvalue.NewObject(members...)
}
