package analyze

import (
	"slices"

	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// sessionMark is one mark inventory.json summarises per project: its key,
// the session's value of it, the value every session needs for the full
// label, the values a session counts with, the full, some and none labels,
// and the agents whose parser records it.
type sessionMark struct {
	key    string
	value  func(timeline.SessionSummary) string
	full   string
	hits   []string
	labels [3]string
	agents map[string]bool
}

// projectMarks returns the marks in key order, each with its recording
// agents from marks.
func projectMarks(marks timeline.MarkAgents) []sessionMark {
	return []sessionMark{
		{"synced", func(session timeline.SessionSummary) string { return session.Synced }, "yes",
			[]string{"yes"}, [3]string{"yes", "partial", "no"}, marks.Synced},
		{"relayed", func(session timeline.SessionSummary) string { return session.Relayed }, "yes",
			[]string{"yes"}, [3]string{"yes", "partial", "no"}, marks.Relayed},
		{"origin", func(session timeline.SessionSummary) string { return session.Origin }, "remote",
			[]string{"remote"}, [3]string{"remote", "mixed", "local"}, marks.Origin},
		{"recovered", func(session timeline.SessionSummary) string { return session.Recovered }, "yes",
			recoveredValues(), [3]string{"yes", "partial", "no"}, marks.Secondary},
	}
}

// recoveredValues are the recovered values of a session that secondary
// sources kept rows of.
func recoveredValues() []string {
	return []string{"yes", "partial"}
}

// label returns a project's mark from its sessions: the full label when
// every session has the full value, the some label when some count, the
// none label when none does and one of the project's agents records the
// mark, else "", as for a project with no session.
func (m sessionMark) label(sessions []timeline.SessionSummary, agents map[string]*agentTally) string {
	if len(sessions) == 0 {
		return ""
	}
	all, some := true, false
	for _, session := range sessions {
		value := m.value(session)
		all = all && value == m.full
		some = some || slices.Contains(m.hits, value)
	}
	switch {
	case all:
		return m.labels[0]
	case some:
		return m.labels[1]
	}
	for agent := range agents {
		if m.agents[agent] {
			return m.labels[2]
		}
	}
	return ""
}

// agentTally counts one agent's rows, sessions and turn types in one
// project.
type agentTally struct {
	rows, sessions int
	turnTypes      map[string]int
}

// projectTally counts the rows and sessions of one project, in total and
// per agent.
type projectTally struct {
	rows, sessions int
	agents         map[string]*agentTally
}

// projectInventory groups rows, sessions and files by project key.
type projectInventory struct {
	groups    map[string]*projectTally
	names     map[string]string
	collected map[string]bool
	onDisk    map[string]*onDisk
	marks     map[string][]timeline.SessionSummary
}

// group returns the tally of a path's project, naming the project by the
// first spelling met.
func (p *projectInventory) group(path string) *projectTally {
	key := timeline.ProjectKey(path)
	if _, ok := p.names[key]; !ok {
		p.names[key] = path
	}
	if p.groups[key] == nil {
		p.groups[key] = &projectTally{agents: map[string]*agentTally{}}
	}
	return p.groups[key]
}

// agentOf returns a path's project tally and the tally of one agent in it.
func (p *projectInventory) agentOf(path, agent string) (*projectTally, *agentTally) {
	totals := p.group(path)
	if totals.agents[agent] == nil {
		totals.agents[agent] = &agentTally{turnTypes: map[string]int{}}
	}
	return totals, totals.agents[agent]
}

// newProjectInventory counts the rows and sessions per project. Collected
// paths are met first, so they name their projects; then rows, sessions
// and the paths of the files.
func newProjectInventory(rows []timeline.Row, sessions []timeline.SessionSummary,
	collected []string, files []diskEntry) *projectInventory {
	inventory := &projectInventory{groups: map[string]*projectTally{}, names: map[string]string{},
		collected: map[string]bool{}, onDisk: map[string]*onDisk{},
		marks: map[string][]timeline.SessionSummary{}}
	for _, path := range collected {
		if path != "" {
			inventory.group(path)
			inventory.collected[timeline.ProjectKey(path)] = true
		}
	}
	for _, file := range files {
		key := timeline.ProjectKey(file.path)
		if inventory.onDisk[key] == nil {
			inventory.onDisk[key] = &onDisk{git: emptyGit()}
		}
		mergeOnDisk(inventory.onDisk[key], file.sharedFiles, file.git, agentList(file.agents))
	}
	for _, row := range rows {
		totals, tally := inventory.agentOf(row.ProjectPath, row.Agent)
		totals.rows++
		tally.rows++
		tally.turnTypes[row.TurnType]++
	}
	for _, session := range sessions {
		totals, tally := inventory.agentOf(session.ProjectPath, session.Agent)
		totals.sessions++
		tally.sessions++
		key := timeline.ProjectKey(session.ProjectPath)
		inventory.marks[key] = append(inventory.marks[key], session)
	}
	for _, file := range files {
		for _, agent := range file.agents {
			if file.path != "" {
				inventory.agentOf(file.path, agent.agent)
			}
		}
	}
	return inventory
}

// agentList returns file counts as agent entries, for merging.
func agentList(counts []fileCounts) []agentFiles {
	agents := make([]agentFiles, len(counts))
	for i, each := range counts {
		agents[i] = agentFiles{fileCounts: each}
	}
	return agents
}

// projectEntryJSON is one project's object of inventory.json, with its
// display path.
type projectEntryJSON struct {
	path  string
	value pyjson.Value
}

// entries returns each project's object in order of its display path: the
// counts, the marks and, for a project path (not the empty one), its
// status, files, shared files, repository and recorded remotes.
func (p *projectInventory) entries(marks []sessionMark,
	remotes map[string]map[string]bool) []projectEntryJSON {
	keys := make([]string, 0, len(p.groups))
	for key := range p.groups {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(left, right string) int { return compareText(p.names[left], p.names[right]) })
	out := make([]projectEntryJSON, len(keys))
	for i, key := range keys {
		out[i] = projectEntryJSON{p.names[key], p.entry(key, marks, remotes)}
	}
	return out
}

// entry returns the object of one project key.
func (p *projectInventory) entry(key string, marks []sessionMark,
	remotes map[string]map[string]bool) pyjson.Value {
	totals, path := p.groups[key], p.names[key]
	var disk *onDisk
	if path != "" {
		disk = p.onDisk[key]
		if disk == nil {
			disk = &onDisk{git: emptyGit()}
		}
	}
	members := []pyjson.Member{}
	if disk != nil {
		status := "referenced"
		if p.collected[key] {
			status = "collected"
		}
		members = append(members, member("status", pyjson.NewString(status)))
	}
	members = append(members, member("rows", pyjson.NewInt(int64(totals.rows))),
		member("sessions", pyjson.NewInt(int64(totals.sessions))),
		member("agents", agentTallies(totals.agents, disk)))
	if disk != nil {
		members = append(members, member("shared_files", pyjson.NewInt(disk.sharedFiles)))
	}
	for _, mark := range marks {
		members = append(members, member(mark.key, pyjson.NewString(mark.label(p.marks[key], totals.agents))))
	}
	if disk != nil {
		git := emptyGit()
		if p.collected[key] {
			git = disk.git
		}
		members = append(members, member("remotes", remotesJSON(git.remotes)),
			member("head", pyjson.NewString(git.head)),
			member("recorded_remotes", stringArray(sortedKeys(remotes[key]))))
	}
	return pyjson.NewObject(members...)
}

// agentTallies returns the agents object of a project: per agent, sorted,
// its rows, sessions and turn types and, with disk, its files and bytes.
func agentTallies(agents map[string]*agentTally, disk *onDisk) pyjson.Value {
	var members []pyjson.Member
	for _, agent := range sortedKeys(agents) {
		tally := agents[agent]
		var turnTypes []pyjson.Member
		for _, turnType := range sortedKeys(tally.turnTypes) {
			turnTypes = append(turnTypes, member(turnType, pyjson.NewInt(int64(tally.turnTypes[turnType]))))
		}
		fields := []pyjson.Member{member("rows", pyjson.NewInt(int64(tally.rows))),
			member("sessions", pyjson.NewInt(int64(tally.sessions))),
			member("turn_types", pyjson.NewObject(turnTypes...))}
		if disk != nil {
			var files fileCounts
			if index := slices.IndexFunc(disk.agents, func(each fileCounts) bool { return each.agent == agent }); index >= 0 {
				files = disk.agents[index]
			}
			fields = append(fields, member("files", pyjson.NewInt(files.files)),
				member("bytes", pyjson.NewInt(files.bytes)))
		}
		members = append(members, member(agent, pyjson.NewObject(fields...)))
	}
	return pyjson.NewObject(members...)
}

// compareText orders two texts by code point, as Python compares strings.
func compareText(left, right string) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	}
	return 0
}
