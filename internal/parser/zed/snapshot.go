package zed

import (
	"cmp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// objectOrEmpty returns value when it is an object, else an empty object.
func objectOrEmpty(value jsonvalue.Value) jsonvalue.Value {
	if value.IsObject() {
		return value
	}
	return jsonvalue.NewObject()
}

// worktreeSnapshots returns the worktree snapshots of a project snapshot
// that are objects.
func worktreeSnapshots(snapshot jsonvalue.Value) []jsonvalue.Value {
	var worktrees []jsonvalue.Value
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
func branchAndRemote(worktrees []jsonvalue.Value, project string) (string, string) {
	firstBranch, ownBranch := "", ""
	var own []jsonvalue.Value
	for _, worktree := range worktrees {
		isOwn := parser.IsText(worktree.Get("worktree_path"), project)
		if isOwn {
			own = append(own, worktree)
		}
		gitState := worktree.Get("git_state")
		if current := gitState.Get("current_branch"); gitState.IsObject() && !current.IsEmpty() {
			firstBranch = cmp.Or(firstBranch, current.Text())
			if isOwn {
				ownBranch = cmp.Or(ownBranch, current.Text())
			}
		}
	}
	if len(own) == 0 && len(worktrees) > 0 {
		own = worktrees[:1]
	}
	remote := ""
	if len(own) > 0 {
		if url := own[0].Get("git_state").Get("remote_url"); url.IsString() {
			remote = strings.TrimSpace(url.StringValue())
		}
	}
	return cmp.Or(ownBranch, firstBranch), remote
}

// threadParent returns the parent thread of a thread: the row's parent_id,
// else the blob's subagent_context.parent_thread_id, "" when neither is set.
func threadParent(record sqlitefile.Row, value jsonvalue.Value) string {
	if parent := record.Get("parent_id"); !parent.IsEmpty() {
		return parent.String()
	}
	return parser.Text(value.Get("subagent_context").Get("parent_thread_id"))
}

// threadState fills in the thread's project, branch, model and time, and
// emits its start row, its `git remote:` and `model:` rows and one
// `permission setting:` row per thread-scoped grant.
func (p *Parser) threadState(context *thread, record sqlitefile.Row, value jsonvalue.Value,
	emit parser.Emit) {
	snapshot := objectOrEmpty(value.Get("initial_project_snapshot"))
	worktrees := worktreeSnapshots(snapshot)
	if context.project == "" && len(worktrees) > 0 {
		context.project = parser.Text(worktrees[0].Get("worktree_path"))
	}
	branch, remote := branchAndRemote(worktrees, context.project)
	modelInfo := objectOrEmpty(value.Get("model"))
	var parts []string
	for _, part := range []jsonvalue.Value{modelInfo.Get("provider"), modelInfo.Get("model")} {
		if !part.IsEmpty() {
			parts = append(parts, part.Text())
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
	if model := modelInfo.Get("model"); !model.IsEmpty() {
		text := "model: " + model.Text()
		if provider := modelInfo.Get("provider"); !provider.IsEmpty() {
			text += " provider=" + provider.Text()
		}
		emit(p.threadRow(context, timeline.TurnSystem, text, start))
	}
	for _, grant := range threadGrants(value) {
		text := "permission setting: " + grant.key + "=" + parser.SettingValue(grant.value) +
			" (thread blob, as of updated_at)"
		emit(p.threadRow(context, timeline.TurnSystem, text, ""))
	}
}

// startText returns the text of a thread's start row: its title, version
// and parent thread.
func startText(record sqlitefile.Row, value jsonvalue.Value) string {
	parts := []string{"thread start:"}
	title := parser.Text(value.Get("title"))
	if title == "" {
		title = columnStr(record.Get("summary"))
	}
	if title != "" {
		parts = append(parts, title)
	}
	if version := value.Get("version"); !version.IsEmpty() {
		parts = append(parts, "version="+version.Text())
	}
	if parent := threadParent(record, value); parent != "" {
		parts = append(parts, "parent="+parent)
	}
	return strings.Join(parts, " ")
}

// threadGrants returns the thread-scoped grants: the agent profile, each
// granted write path and network host, and each sandbox escape flag that is
// set (DbSandboxGrants, db.rs:91-122 at a84689073d29).
func threadGrants(value jsonvalue.Value) []setting {
	var grants []setting
	if profile, ok := value.Lookup("profile"); ok && parser.Present(profile) {
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
			grants = append(grants, setting{"sandbox_grants." + key, jsonvalue.NewBool(true)})
		}
	}
	return grants
}
