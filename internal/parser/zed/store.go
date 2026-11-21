package zed

import (
	"cmp"
	"errors"
	"fmt"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// errNotText is why a key-value entry whose value is not text or a blob
// does not decode.
var errNotText = errors.New("the value is not text")

// decodeColumn decodes a key-value entry's value as JSON, or returns an
// error saying why it does not decode: the value is not text, or the JSON
// reader's error.
func decodeColumn(value sqlitefile.Value) (jsonvalue.Value, error) {
	if value.Type != sqlitefile.Text && value.Type != sqlitefile.Blob {
		return jsonvalue.Value{}, errNotText
	}
	return jsonvalue.Decode(value.Bytes)
}

// draft emits `unsent draft: <text>` for an agent_draft_prompts entry, at
// the draft's sidebar updated_at, or at the file's modification time, saying
// so, when the draft has no sidebar row.
func (p *Parser) draft(artifact *input.Artifact, row sqlitefile.Row, store *sidebarStore,
	problems *parser.Problems, emit parser.Emit) {
	key, value := row.Get("key").String(), row.Get("value")
	blocks, err := decodeColumn(value)
	if err != nil {
		label := fmt.Sprintf("unparseable record %d (%s %s: %v)", row.Rowid, draftsNamespace, key, err)
		problems.AddRaw(int(row.Rowid), label, value.String())
		return
	}
	text := acpBlocksText(blocks)
	if strings.TrimSpace(text) == "" {
		return
	}
	if side, ok := store.byKey[key]; ok {
		emit(p.stateRow(artifact, "unsent draft: "+text, parser.ColumnTime(side.Get("updated_at")),
			row.Rowid, "", firstFolder(side.Get("folder_paths"))))
		return
	}
	text = "unsent draft: " + text + " (from " + artifact.Rel + " " + draftsNamespace +
		", file mtime)"
	emit(p.stateRow(artifact, text, parser.FileMtime(artifact), row.Rowid, "", ""))
}

// createdSeconds returns a created_git_worktrees record's creation time in
// seconds, created_at_seconds plus created_at_subsec_nanos, or an error
// saying why the record has none.
func createdSeconds(record jsonvalue.Value) (float64, error) {
	if !record.IsObject() {
		return 0, errors.New("the record is not a JSON object")
	}
	seconds, ok := record.Lookup("created_at_seconds")
	if !ok {
		return 0, errors.New("no created_at_seconds field")
	}
	whole, wholeOK := seconds.Float()
	nanos, nanosOK := 0.0, true
	if fraction, found := record.Lookup("created_at_subsec_nanos"); found {
		nanos, nanosOK = fraction.Float()
	}
	if !wholeOK || !nanosOK {
		return 0, errors.New("the creation time is not a number")
	}
	return whole + nanos/1e9, nil
}

// createdWorktree emits `scratch directory:` for a created_git_worktrees
// entry, at its creation time. The project path is the worktree; the main
// repository comes from the sidebar or the archived worktrees.
func (p *Parser) createdWorktree(artifact *input.Artifact, row sqlitefile.Row,
	store *sidebarStore, problems *parser.Problems, emit parser.Emit) {
	value := row.Get("value")
	host, path, _ := strings.Cut(row.Get("key").String(), "\n")
	record, err := decodeColumn(value)
	seconds := 0.0
	if err == nil {
		seconds, err = createdSeconds(record)
	}
	if err != nil {
		label := fmt.Sprintf("unparseable record %d (%s: %v)", row.Rowid, worktreesNamespace, err)
		problems.AddRaw(int(row.Rowid), label, value.String())
		return
	}
	main := store.carried.Cut("main repository", store.mainRepository(path))
	if main != "" {
		main = " of " + main
	}
	if host != "" && host != "local" {
		host = " host=" + host
	} else {
		host = ""
	}
	text := "scratch directory: " + path + " (Zed-created git worktree" + main + host +
		") (from " + artifact.Rel + " " + worktreesNamespace + ", created_at)"
	emit(p.stateRow(artifact, text, timeutil.FromEpoch(seconds), row.Rowid, "", path))
}

// mainRepository returns the main repository of a Zed-created worktree:
// from the first sidebar thread with main worktrees whose folders hold the
// path, else from the first archived worktree at the path, else "". The
// index of both is built once per store.
func (s *sidebarStore) mainRepository(path string) string {
	if s.repositories == nil {
		s.repositories = s.indexRepositories()
	}
	if main, ok := s.repositories.sidebar[path]; ok && main != "" {
		return main
	}
	return s.repositories.archived[path]
}

// repositoryIndex holds, by worktree path, what mainRepository looks up:
// the first main worktree of the first sidebar thread with main worktrees
// whose folders hold the path ("" when it has none), and the main
// repository of the first archived worktree at the path.
type repositoryIndex struct {
	sidebar, archived map[string]string
}

// indexRepositories builds the store's repository index, counting the
// folders and archived worktrees it reads in the store's work.
func (s *sidebarStore) indexRepositories() *repositoryIndex {
	index := &repositoryIndex{sidebar: map[string]string{}, archived: map[string]string{}}
	for _, entry := range s.sidebar {
		if entry.Get("main_worktree_paths").IsEmpty() {
			continue
		}
		main := firstFolder(entry.Get("main_worktree_paths"))
		for _, folder := range strings.Split(parser.NonEmptyColumn(entry.Get("folder_paths")), "\n") {
			s.work.Add("sidebar folders indexed", 1)
			if _, held := index.sidebar[folder]; !held {
				index.sidebar[folder] = main
			}
		}
	}
	for _, archive := range s.archived {
		s.work.Add("archived worktrees indexed", 1)
		worktree := archive.Get("worktree_path")
		if _, held := index.archived[worktree.Text()]; worktree.Type == sqlitefile.Text && !held {
			index.archived[worktree.Text()] = archive.Get("main_repo_path").String()
		}
	}
	return index
}

// archivedWorktree emits the row of one archived worktree and one thread
// linked to it. The join copies the worktree's columns onto the row of each
// linked thread, so the store's carried cuts them. Of an id the table
// repeats, each stored row's own row, unlinked, is a parent, shown unless
// the group goes with it, and a link's row takes the text, project and
// source line of the stored row the group goes with ("Rows of a repeated
// id" in analyzer/docs/output.md).
func (p *Parser) archivedWorktree(artifact *input.Artifact, row archivedRow, store *sidebarStore,
	emit parser.Emit) {
	sessionID := ""
	if !row.thread.IsEmpty() {
		if side, ok := store.byKey[threadKey(row.thread)]; ok {
			sessionID = parser.NonEmptyColumn(side.Get("session_id"))
		}
	}
	if row.link {
		linked := p.stateRow(artifact, "", parser.FileMtime(artifact), row.Rowid, sessionID, "")
		row.group.Member(&linked, timeline.TakeText|timeline.TakeProject|timeline.TakeSourceLine)
		emit(linked)
		return
	}
	column := func(name string) string {
		return store.carried.Cut("archived_git_worktrees."+name, parser.NonEmptyColumn(row.Get(name)))
	}
	text := "archived worktree: " + column("worktree_path") + " of " + column("main_repo_path") + " branch=" +
		cmp.Or(column("branch_name"), "-") + " staged=" + cmp.Or(column("staged_commit_hash"), "-") +
		" unstaged=" + cmp.Or(column("unstaged_commit_hash"), "-") + " original=" +
		cmp.Or(column("original_commit_hash"), "-") + " ref=refs/archived-worktrees/" +
		store.carried.Cut("archived_git_worktrees.id", row.Get("id").String()) + " (from " + artifact.Rel +
		" archived_git_worktrees, file mtime)"
	own := p.stateRow(artifact, text, parser.FileMtime(artifact), row.Rowid, sessionID,
		parser.NonEmptyColumn(row.Get("main_repo_path")))
	if row.group != nil {
		row.group.Parent(&own).Row(&own, timeline.WhenNotChosen)
	}
	emit(own)
}
