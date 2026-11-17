package zed

import (
	"fmt"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// notText is the reason a key-value entry whose value is not text or a
// blob does not decode.
const notText = "the value is not text"

// decodeColumn decodes a key-value entry's value as JSON, or returns why it
// does not decode: the value is not text, or the JSON reader's message.
func decodeColumn(value sqlitefile.Value) (jsonvalue.Value, string) {
	if value.Type != sqlitefile.Text && value.Type != sqlitefile.Blob {
		return jsonvalue.Value{}, notText
	}
	decoded, err := jsonvalue.Decode(value.Bytes)
	if err != nil {
		return jsonvalue.Value{}, err.Error()
	}
	return decoded, ""
}

// draft emits `unsent draft: <text>` for an agent_draft_prompts entry, at
// the draft's sidebar updated_at, or at the file's modification time, saying
// so, when the draft has no sidebar row.
func (p *Parser) draft(artifact *input.Artifact, row sqlitefile.Row, store *sidebarStore,
	problems *parser.Problems, emit parser.Emit) {
	key, value := row.Get("key").String(), row.Get("value")
	blocks, failure := decodeColumn(value)
	if failure != "" {
		label := fmt.Sprintf("unparseable record %d (%s %s: %s)", row.Rowid, draftsNamespace, key,
			failure)
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
	text = fmt.Sprintf("unsent draft: %s (from %s %s, file mtime)", text, artifact.Rel,
		draftsNamespace)
	emit(p.stateRow(artifact, text, parser.FileMtime(artifact), row.Rowid, "", ""))
}

// createdSeconds returns a created_git_worktrees record's creation time in
// seconds, created_at_seconds plus created_at_subsec_nanos, or why the
// record has none.
func createdSeconds(record jsonvalue.Value) (float64, string) {
	if !record.IsObject() {
		return 0, "the record is not a JSON object"
	}
	seconds, ok := record.Lookup("created_at_seconds")
	if !ok {
		return 0, "no created_at_seconds field"
	}
	whole, wholeOK := pythonNumber(seconds)
	nanos, nanosOK := 0.0, true
	if fraction, found := record.Lookup("created_at_subsec_nanos"); found {
		nanos, nanosOK = pythonNumber(fraction)
	}
	if !wholeOK || !nanosOK {
		return 0, "the creation time is not a number"
	}
	return whole + nanos/1e9, ""
}

// pythonNumber returns a JSON number, or a boolean as Python's 0 or 1, as a
// float.
func pythonNumber(value jsonvalue.Value) (float64, bool) {
	if flag, ok := value.Bool(); ok {
		if flag {
			return 1, true
		}
		return 0, true
	}
	return value.Float()
}

// createdWorktree emits `scratch directory:` for a created_git_worktrees
// entry, at its creation time. The project path is the worktree; the main
// repository comes from the sidebar or the archived worktrees.
func (p *Parser) createdWorktree(artifact *input.Artifact, row sqlitefile.Row,
	store *sidebarStore, problems *parser.Problems, emit parser.Emit) {
	value := row.Get("value")
	host, path, _ := strings.Cut(row.Get("key").String(), "\n")
	record, failure := decodeColumn(value)
	seconds := 0.0
	if failure == "" {
		seconds, failure = createdSeconds(record)
	}
	if failure != "" {
		label := fmt.Sprintf("unparseable record %d (%s: %s)", row.Rowid, worktreesNamespace, failure)
		problems.AddRaw(int(row.Rowid), label, value.String())
		return
	}
	main := store.mainRepository(path)
	if main != "" {
		main = " of " + main
	}
	if host != "" && host != "local" {
		host = " host=" + host
	} else {
		host = ""
	}
	text := fmt.Sprintf("scratch directory: %s (Zed-created git worktree%s%s) (from %s %s,"+
		" created_at)", path, main, host, artifact.Rel, worktreesNamespace)
	emit(p.stateRow(artifact, text, timeutil.FromEpoch(seconds), row.Rowid, "", path))
}

// mainRepository returns the main repository of a Zed-created worktree:
// from the first sidebar thread with main worktrees whose folders hold the
// path, else from the first archived worktree at the path, else "".
func (s *sidebarStore) mainRepository(path string) string {
	for _, entry := range s.sidebar {
		folders := columnStr(entry.Get("folder_paths"))
		if !entry.Get("main_worktree_paths").IsEmpty() && containsLine(folders, path) {
			if main := firstFolder(entry.Get("main_worktree_paths")); main != "" {
				return main
			}
			break
		}
	}
	for _, archive := range s.archived {
		if worktree := archive.Get("worktree_path"); worktree.Type == sqlitefile.Text &&
			worktree.Text() == path {
			return archive.Get("main_repo_path").String()
		}
	}
	return ""
}

// containsLine reports whether one of the newline-separated lines of text
// is line.
func containsLine(text, line string) bool {
	for _, each := range strings.Split(text, "\n") {
		if each == line {
			return true
		}
	}
	return false
}

// archivedWorktree returns the row of one archived worktree and one thread
// linked to it.
func (p *Parser) archivedWorktree(artifact *input.Artifact, row archivedRow,
	store *sidebarStore) timeline.Row {
	sessionID := ""
	if !row.thread.IsEmpty() {
		if side, ok := store.byKey[threadKey(row.thread)]; ok {
			sessionID = columnStr(side.Get("session_id"))
		}
	}
	text := fmt.Sprintf("archived worktree: %s of %s branch=%s staged=%s unstaged=%s original=%s"+
		" ref=refs/archived-worktrees/%s (from %s archived_git_worktrees, file mtime)",
		columnStr(row.Get("worktree_path")), columnStr(row.Get("main_repo_path")),
		orDash(columnStr(row.Get("branch_name"))), orDash(columnStr(row.Get("staged_commit_hash"))),
		orDash(columnStr(row.Get("unstaged_commit_hash"))),
		orDash(columnStr(row.Get("original_commit_hash"))), row.Get("id").String(), artifact.Rel)
	return p.stateRow(artifact, text, parser.FileMtime(artifact), row.Rowid, sessionID,
		columnStr(row.Get("main_repo_path")))
}
