package zed

import (
	"cmp"
	"fmt"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// objectOrEmpty returns value when it is an object, else an empty object.
func objectOrEmpty(value pyjson.Value) pyjson.Value {
	if value.IsObject() {
		return value
	}
	return pyjson.NewObject()
}

// worktreeSnapshots returns the worktree snapshots of a project snapshot
// that are objects.
func worktreeSnapshots(snapshot pyjson.Value) []pyjson.Value {
	var worktrees []pyjson.Value
	for _, worktree := range parser.Elements(snapshot.Get("worktree_snapshots")) {
		if worktree.IsObject() {
			worktrees = append(worktrees, worktree)
		}
	}
	return worktrees
}

// branchAndRemote returns the git branch of a thread, the project's own
// worktree's else the first worktree's that has one, and the remote of the
// project's own snapshot, else of the first one; never another worktree's,
// whose remote is another repository's.
func branchAndRemote(worktrees []pyjson.Value, project string) (string, string) {
	firstBranch, ownBranch := "", ""
	var own []pyjson.Value
	for _, worktree := range worktrees {
		isOwn := isText(worktree.Get("worktree_path"), project)
		if isOwn {
			own = append(own, worktree)
		}
		gitState := worktree.Get("git_state")
		if current := gitState.Get("current_branch"); gitState.IsObject() && current.Truthy() {
			firstBranch = cmp.Or(firstBranch, current.Str())
			if isOwn {
				ownBranch = cmp.Or(ownBranch, current.Str())
			}
		}
	}
	if len(own) == 0 && len(worktrees) > 0 {
		own = worktrees[:1]
	}
	remote := ""
	if len(own) > 0 {
		if url := own[0].Get("git_state").Get("remote_url"); url.IsString() {
			remote = pytext.Strip(url.Text())
		}
	}
	return cmp.Or(ownBranch, firstBranch), remote
}

// threadParent returns the parent thread of a thread: the row's parent_id,
// else the blob's subagent_context.parent_thread_id, "" when neither is set.
func threadParent(record sqlitefile.Row, value pyjson.Value) string {
	if parent := record.Get("parent_id"); parent.Truthy() {
		return parent.Str()
	}
	return parser.Str(value.Get("subagent_context").Get("parent_thread_id"))
}

// threadState fills in the thread's project, branch, model and time, and
// emits its start row, its `git remote:` and `model:` rows and one
// `permission setting:` row per thread-scoped grant.
func (p *Parser) threadState(context *thread, record sqlitefile.Row, value pyjson.Value,
	emit parser.Emit) {
	snapshot := objectOrEmpty(value.Get("initial_project_snapshot"))
	worktrees := worktreeSnapshots(snapshot)
	if context.project == "" && len(worktrees) > 0 {
		context.project = parser.Str(worktrees[0].Get("worktree_path"))
	}
	branch, remote := branchAndRemote(worktrees, context.project)
	modelInfo := objectOrEmpty(value.Get("model"))
	var parts []string
	for _, part := range []pyjson.Value{modelInfo.Get("provider"), modelInfo.Get("model")} {
		if part.Truthy() {
			parts = append(parts, part.Str())
		}
	}
	context.model, context.branch = strings.Join(parts, "/"), branch
	if context.updated == "" {
		context.updated = timeutil.FromValue(value.Get("updated_at"))
	}
	start := timeutil.FirstNonEmpty(timeutil.FromValue(snapshot.Get("timestamp")),
		parser.ColumnTime(record.Get("created_at")), context.updated)
	emit(p.threadRow(context, timeline.TurnSystem, startText(record, value), start))
	if remote != "" {
		emit(p.threadRow(context, timeline.TurnSystem, timeline.GitRemotePrefix+remote, start))
	}
	if model := modelInfo.Get("model"); model.Truthy() {
		text := "model: " + model.Str()
		if provider := modelInfo.Get("provider"); provider.Truthy() {
			text += " provider=" + provider.Str()
		}
		emit(p.threadRow(context, timeline.TurnSystem, text, start))
	}
	for _, grant := range threadGrants(value) {
		text := fmt.Sprintf("permission setting: %s=%s (thread blob, as of updated_at)", grant.key,
			parser.SettingValue(grant.value))
		emit(p.threadRow(context, timeline.TurnSystem, text, ""))
	}
}

// startText returns the text of a thread's start row: its title, version
// and parent thread.
func startText(record sqlitefile.Row, value pyjson.Value) string {
	parts := []string{"thread start:"}
	title := parser.Str(value.Get("title"))
	if title == "" {
		title = columnStr(record.Get("summary"))
	}
	if title != "" {
		parts = append(parts, title)
	}
	if version := value.Get("version"); version.Truthy() {
		parts = append(parts, "version="+version.Str())
	}
	if parent := threadParent(record, value); parent != "" {
		parts = append(parts, "parent="+parent)
	}
	return strings.Join(parts, " ")
}

// threadGrants returns the thread-scoped grants: the agent profile, each
// granted write path and network host, and each sandbox escape flag that is
// set (DbSandboxGrants, db.rs:91-122 at a84689073d29).
func threadGrants(value pyjson.Value) []setting {
	var grants []setting
	if profile, ok := value.Lookup("profile"); ok && !profile.IsNull() && !isText(profile, "") {
		grants = append(grants, setting{"profile", profile})
	}
	sandbox := value.Get("sandbox_grants")
	if !sandbox.IsObject() {
		return grants
	}
	for _, key := range []string{"write_paths", "network_hosts"} {
		if entries := sandbox.Get(key); entries.IsArray() {
			for _, entry := range entries.Items() {
				grants = append(grants, setting{"sandbox_grants." + key, entry})
			}
		}
	}
	for _, key := range sandboxFlags() {
		if flag, ok := sandbox.Get(key).Bool(); ok && flag {
			grants = append(grants, setting{"sandbox_grants." + key, pyjson.NewBool(true)})
		}
	}
	return grants
}
