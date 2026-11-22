package cursor

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// project is a conversation's project path, a local path or a cloud
// agent's repository URL as written, and its git branch.
type project struct {
	path   string
	branch string
}

// stateIndex is what the side files of a home need from its state.vscdb:
// the project of each composer (its first stored rows') and cloud agent,
// the model of each cloud agent and the group of each composer id the
// file repeats (repeat.go), kept for one state of the file and its log.
type stateIndex struct {
	key      [5]int64
	path     string
	projects map[string]project
	models   map[string]string
	groups   map[string]*timeline.Group
	cloud    map[string]*cloudRepeat
}

// fileState returns the size and modification time of a file, -1 for each
// when it cannot be looked at.
func fileState(path string) (int64, int64) {
	info, err := osfile.Lstat(path)
	if err != nil {
		return -1, -1
	}
	return info.Size(), info.ModTime().UnixNano()
}

// homeState returns the IDE's state.vscdb in the artifact's home, "" when
// there is none. It is a path of the home, not beside the artifact.
func homeState(artifact *input.Artifact) string {
	if artifact.Home == nil {
		return ""
	}
	for _, parent := range ideParents() {
		if path, ok := artifact.Home.PathOf(parent + "/User/globalStorage/state.vscdb"); ok &&
			parser.RegularFile(path) {
			return path
		}
	}
	return ""
}

// homeIndex returns the projects and models of the home's state.vscdb,
// read once per state of that file and its log; empty when it is missing
// or cannot be read (its own parse reports why).
func (p *Parser) homeIndex(artifact *input.Artifact) *stateIndex {
	path := homeState(artifact)
	if path == "" {
		return &stateIndex{projects: map[string]project{}, models: map[string]string{}}
	}
	size, mtime := fileState(path)
	var logSize, logMtime int64
	if log, ok := parser.Sidecar(path, "-wal"); ok {
		logSize, logMtime = fileState(log)
	}
	key := [5]int64{size, mtime, logSize, logMtime, 1}
	if p.index != nil && p.index.path == path && p.index.key == key {
		return p.index
	}
	p.index = &stateIndex{key: key, path: path, projects: map[string]project{}, models: map[string]string{},
		groups: map[string]*timeline.Group{}}
	database, err := parser.OpenDatabase(path)
	if err != nil {
		return p.index
	}
	defer database.Close()
	store := newStore(nil, database, p.Work)
	found := readComposers(store)
	p.assignGroups(path, found)
	for _, current := range found.list {
		projectPath, branch := current.project()
		p.index.projects[current.ident] = project{path: projectPath, branch: branch}
		if current.group != nil {
			p.index.groups[current.ident] = current.group
		}
	}
	var ignored parser.Problems
	agents := readCloudAgents(store, &ignored)
	p.index.cloud = p.cloudRepeats(path, agents)
	for _, found := range agents {
		ident := found.agent.Get("bcId").Text()
		if _, seen := p.index.projects[ident]; !seen {
			p.index.projects[ident] = project{path: found.agent.Get("repoUrl").Text()}
		}
		if _, seen := p.index.models[ident]; !seen {
			p.index.models[ident], _ = cloudModel(found.agent)
		}
	}
	if store.err != nil {
		p.index.projects, p.index.models, p.index.groups = map[string]project{}, map[string]string{},
			map[string]*timeline.Group{}
		p.index.cloud = nil
	}
	return p.index
}

// cloudCachedIDs returns the ids of the cloud-cache rows of the
// conversation-search.db beside the state.vscdb artifact, whose parse
// gives their origin rows; none when it is missing or cannot be read.
func cloudCachedIDs(artifact *input.Artifact) map[string]bool {
	ids := map[string]bool{}
	path, ok := parser.Beside(artifact, "conversation-search.db")
	if !ok {
		return ids
	}
	database, err := parser.OpenDatabase(path)
	if err != nil {
		return ids
	}
	defer database.Close()
	store := newStore(nil, database, parser.Work{})
	table := store.table("conversations")
	if table == nil {
		return ids
	}
	rows, err := table.Select("id").Where(func(row sqlitefile.Row) bool {
		return row.Equals("source", sqlitefile.TextValue(cloudCache))
	}).All()
	if err != nil {
		return map[string]bool{}
	}
	for _, row := range rows {
		ids[asText(row.Get("id"))] = true
	}
	return ids
}
