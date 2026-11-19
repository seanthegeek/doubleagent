package analyze

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/secrets"
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
// per agent, and keeps the records of the parses that gave them, which the
// output replaces in the project's object.
type projectTally struct {
	rows, sessions int
	agents         map[string]*agentTally
	secrets        []*secrets.Found
	known          map[*secrets.Found]bool
}

// addSecrets adds the records of the parses behind a row or session to the
// project's, each once.
func (t *projectTally) addSecrets(found ...*secrets.Found) {
	for _, each := range found {
		if each != nil && !t.known[each] {
			t.known[each] = true
			t.secrets = append(t.secrets, each)
		}
	}
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
		p.groups[key] = &projectTally{agents: map[string]*agentTally{}, known: map[*secrets.Found]bool{}}
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
		mergeOnDisk(inventory.onDisk[key], file.sharedFiles, file.git, agentList(file.agents.values))
	}
	for _, row := range rows {
		totals, tally := inventory.agentOf(row.ProjectPath, row.Agent)
		totals.addSecrets(row.Secrets)
		totals.rows++
		tally.rows++
		tally.turnTypes[row.TurnType]++
	}
	for _, session := range sessions {
		totals, tally := inventory.agentOf(session.ProjectPath, session.Agent)
		totals.addSecrets(session.Secrets...)
		totals.sessions++
		tally.sessions++
		key := timeline.ProjectKey(session.ProjectPath)
		inventory.marks[key] = append(inventory.marks[key], session)
	}
	for _, file := range files {
		for _, agent := range file.agents.values {
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
// display path and the records of the parses whose rows and sessions it
// counts.
type projectEntryJSON struct {
	path    string
	value   projectValueJSON
	secrets []*secrets.Found
}

// projectValueJSON is the object of a project path, or of the rows outside
// any project: the counts, the four marks and, for a project path only
// (the fields that are then set), its status, shared files, repository and
// recorded remotes.
type projectValueJSON struct {
	Status          string                    `json:"status,omitempty"`
	Rows            int                       `json:"rows"`
	Sessions        int                       `json:"sessions"`
	Agents          map[string]agentTallyJSON `json:"agents"`
	SharedFiles     *int64                    `json:"shared_files,omitempty"`
	Synced          string                    `json:"synced"`
	Relayed         string                    `json:"relayed"`
	Origin          string                    `json:"origin"`
	Recovered       string                    `json:"recovered"`
	Remotes         *[]remoteJSON             `json:"remotes,omitempty"`
	Head            *string                   `json:"head,omitempty"`
	RecordedRemotes *[]string                 `json:"recorded_remotes,omitempty"`
}

// agentTallyJSON is one agent of a project: its rows, sessions and turn
// types and, for a project path, its files and bytes.
type agentTallyJSON struct {
	Rows      int            `json:"rows"`
	Sessions  int            `json:"sessions"`
	TurnTypes map[string]int `json:"turn_types"`
	Files     *int64         `json:"files,omitempty"`
	Bytes     *int64         `json:"bytes,omitempty"`
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
	slices.SortFunc(keys, func(left, right string) int { return strings.Compare(p.names[left], p.names[right]) })
	out := make([]projectEntryJSON, len(keys))
	for i, key := range keys {
		out[i] = projectEntryJSON{p.names[key], p.entry(key, marks, remotes), p.groups[key].secrets}
	}
	return out
}

// entry returns the object of one project key.
func (p *projectInventory) entry(key string, marks []sessionMark,
	remotes map[string]map[string]bool) projectValueJSON {
	totals, path := p.groups[key], p.names[key]
	var disk *onDisk
	if path != "" {
		disk = p.onDisk[key]
		if disk == nil {
			disk = &onDisk{git: emptyGit()}
		}
	}
	labels := make([]string, len(marks))
	for i, mark := range marks {
		labels[i] = mark.label(p.marks[key], totals.agents)
	}
	value := newProjectValue(totals.rows, totals.sessions, agentTallies(totals.agents, disk), labels)
	if disk != nil {
		value.Status = "referenced"
		if p.collected[key] {
			value.Status = "collected"
		}
		git := emptyGit()
		if p.collected[key] {
			git = disk.git
		}
		remoteList, recorded := remotesJSON(git.remotes), list(sortedKeys(remotes[key]))
		value.SharedFiles, value.Remotes, value.Head = &disk.sharedFiles, &remoteList, &git.head
		value.RecordedRemotes = &recorded
	}
	return value
}

// newProjectValue returns a project object with its counts, its agents and
// the labels of the four marks in the order of projectMarks.
func newProjectValue(rows, sessions int, agents map[string]agentTallyJSON, labels []string) projectValueJSON {
	value := projectValueJSON{Rows: rows, Sessions: sessions, Agents: agents}
	fields := []*string{&value.Synced, &value.Relayed, &value.Origin, &value.Recovered}
	for i, label := range labels {
		*fields[i] = label
	}
	return value
}

// agentTallies returns the agents object of a project: per agent its rows,
// sessions and turn types and, with disk, its files and bytes.
func agentTallies(agents map[string]*agentTally, disk *onDisk) map[string]agentTallyJSON {
	out := make(map[string]agentTallyJSON, len(agents))
	for agent, tally := range agents {
		entry := agentTallyJSON{Rows: tally.rows, Sessions: tally.sessions, TurnTypes: tally.turnTypes}
		if disk != nil {
			files, _ := disk.agents.get(agent)
			entry.Files, entry.Bytes = &files.files, &files.bytes
		}
		out[agent] = entry
	}
	return out
}
