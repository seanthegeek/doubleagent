package analyze

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The times of the Copilot Desktop stores below: the earlier stored row of
// a repeated key, the later, and the rows joined through the key.
const (
	copilotdesktopOld    = "2026-01-01T00:00:00.000Z"
	copilotdesktopNew    = "2026-06-01T00:00:00.000Z"
	copilotdesktopJoined = "2026-06-02T00:00:00.000Z"
)

// copilotdesktopColumns are the columns of the data.db tables the stores
// below fill, none of their keys declared unique, so a key can repeat.
func copilotdesktopColumns() map[string]string {
	return map[string]string{
		"projects":                  "id TEXT, name TEXT, container_kind TEXT, main_repo_path TEXT, default_branch TEXT, created_at TEXT",
		"worktrees":                 "path TEXT, branch TEXT, base_branch TEXT, project_id TEXT, created_at TEXT",
		"workspaces":                "id TEXT, project_id TEXT, name TEXT, workspace_type TEXT, branch TEXT, created_at TEXT",
		"workspace_session_aliases": "session_id TEXT, workspace_id TEXT, created_at TEXT",
		"sessions":                  "id TEXT, title TEXT, session_type TEXT, model TEXT, created_at TEXT",
	}
}

// copilotdesktopStore returns a data.db holding the rows given by table.
func copilotdesktopStore(t *testing.T, filled map[string][][]any) string {
	t.Helper()
	var tables []sqlitetest.Table
	for _, name := range []string{"projects", "worktrees", "workspaces", "workspace_session_aliases", "sessions"} {
		if content, ok := filled[name]; ok {
			tables = append(tables, sqlitetest.Table{Name: name,
				SQL: "CREATE TABLE " + name + " (" + copilotdesktopColumns()[name] + ")", Rows: content})
		}
	}
	data, err := sqlitetest.Build(tables...)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// copilotdesktopProject returns a projects row.
func copilotdesktopProject(projectID, name, path, created string) []any {
	return []any{projectID, name, "folder", path, "main", created}
}

// copilotdesktopSessionRow returns a sessions row of the joined time.
func copilotdesktopSessionRow(sessionID string) []any {
	return []any{sessionID, "fix it", "project", "m", copilotdesktopJoined}
}

// copilotdesktopRepeatedProject returns a data.db whose projects table
// holds the id p1 in two rows, /proj/a and, five months later, /proj/b,
// and a worktree, a workspace and a session of the project, joined a day
// after the later row.
func copilotdesktopRepeatedProject(t *testing.T) string {
	return copilotdesktopStore(t, map[string][][]any{
		"projects": {copilotdesktopProject("p1", "old", "/proj/a", copilotdesktopOld),
			copilotdesktopProject("p1", "new", "/proj/b", copilotdesktopNew)},
		"worktrees":                 {{"/wt", "b", "main", "p1", copilotdesktopJoined}},
		"workspaces":                {{"w1", "p1", "ws", "worktree", "b", copilotdesktopJoined}},
		"workspace_session_aliases": {{"s1", "w1", copilotdesktopJoined}},
		"sessions":                  {copilotdesktopSessionRow("s1")},
	})
}

// copilotdesktopRepeatedWorkspace returns a data.db whose workspaces table
// holds the id w1 in two rows, one of the project in /proj/a and, five
// months later, one of the project in /proj/b, and a session of the
// workspace a day after the later row.
func copilotdesktopRepeatedWorkspace(t *testing.T) string {
	return copilotdesktopStore(t, map[string][][]any{
		"projects": {copilotdesktopProject("pa", "a", "/proj/a", copilotdesktopOld),
			copilotdesktopProject("pb", "b", "/proj/b", copilotdesktopOld)},
		"workspaces": {{"w1", "pa", "old ws", "worktree", "b", copilotdesktopOld},
			{"w1", "pb", "new ws", "worktree", "b", copilotdesktopNew}},
		"workspace_session_aliases": {{"s1", "w1", copilotdesktopJoined}},
		"sessions":                  {copilotdesktopSessionRow("s1")},
	})
}

// copilotdesktopRepeatedAlias returns a data.db whose
// workspace_session_aliases table holds the session id s1 in two rows,
// one naming the workspace of /proj/a and, five months later, one naming
// that of /proj/b, and the session, a day after the later row.
func copilotdesktopRepeatedAlias(t *testing.T) string {
	return copilotdesktopStore(t, map[string][][]any{
		"projects": {copilotdesktopProject("pa", "a", "/proj/a", copilotdesktopOld),
			copilotdesktopProject("pb", "b", "/proj/b", copilotdesktopOld)},
		"workspaces": {{"wa", "pa", "ws a", "worktree", "b", copilotdesktopOld},
			{"wb", "pb", "ws b", "worktree", "b", copilotdesktopOld}},
		"workspace_session_aliases": {{"s1", "wa", copilotdesktopOld}, {"s1", "wb", copilotdesktopNew}},
		"sessions":                  {copilotdesktopSessionRow("s1")},
	})
}

// copilotdesktopRun runs timeline over a collection holding data.db with
// the options given, and returns each row as "time session project text".
func copilotdesktopRun(t *testing.T, database string, options ...string) []string {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))},
		[]collectedFile{{"alice", "/home/alice", "copilot-desktop", ".copilot/data.db", database}})
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline", "--agent", "copilot-desktop"}, options...), collection, output)
	if code, _, stderr := runCommand(env, args...); code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	return recordsOf(t, filepath.Join(output, TimelineFile), "timestamp_utc", "timestamp_utc", "session_id",
		"project_path", "text")
}

// copilotdesktopCase is one run over a store above and the rows it gives.
type copilotdesktopCase struct {
	name    string
	options []string
	want    []string
}

// copilotdesktopCheck runs each case over the database and compares the
// rows, and checks that the session row is shown once.
func copilotdesktopCheck(t *testing.T, database func(*testing.T) string, tests []copilotdesktopCase) {
	t.Helper()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := copilotdesktopRun(t, database(t), test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
			if count := strings.Count(strings.Join(rows, "\n"), "desktop session: fix it"); count > 1 {
				t.Errorf("the session row is shown %d times, want at most once", count)
			}
		})
	}
}

// copilotdesktopSessionText is the text of the stores' session row.
const copilotdesktopSessionText = "desktop session: fix it type=project model=m auto_approve= " +
	"permission_mode=- remote_control= archived=false"

func TestTheRowsOfARepeatedCopilotDesktopProjectGoWithTheFirstRowTheFilterKeeps(t *testing.T) {
	t.Parallel()
	const notice = "parser: projects holds 2 rows of this row's id; its worktrees, workspaces and sessions " +
		"are shown once, with "
	old, current := copilotdesktopOld+"  /proj/a ", copilotdesktopNew+"  /proj/b "
	oldRows := []string{old + "project added: old kind=folder path=/proj/a default_branch=main"}
	newRows := []string{current + "project added: new kind=folder path=/proj/b default_branch=main"}
	joined := func(path string) []string {
		return []string{copilotdesktopJoined + "  " + path + " worktree: /wt branch=b base=main",
			copilotdesktopJoined + "  " + path + " workspace: ws type=worktree branch=b",
			copilotdesktopJoined + " s1 " + path + " " + copilotdesktopSessionText}
	}
	copilotdesktopCheck(t, copilotdesktopRepeatedProject, []copilotdesktopCase{
		{"without a filter they go with the first row", nil, slices.Concat(oldRows,
			[]string{old + notice + "this row"}, newRows, []string{current + notice + "another of them"},
			joined("/proj/a"))},
		{"a window that keeps only the later row moves them to it", []string{"--since", "2026-03-01"},
			slices.Concat(newRows, []string{current + notice + "this row"}, joined("/proj/b"))},
		{"a window that keeps only the earlier row keeps them with it", []string{"--until", "2026-03-01"},
			slices.Concat(oldRows, []string{old + notice + "this row"})},
		{"a pattern only the later row matches moves them to it", []string{"--match", "new kind|fix it"},
			slices.Concat(newRows, joined("/proj/b")[2:])},
		{"a pattern only the earlier row matches keeps them with it", []string{"--match", "old kind|fix it"},
			slices.Concat(oldRows, joined("/proj/a")[2:])},
		{"a pattern neither row matches leaves them with the first", []string{"--match", "fix it"},
			joined("/proj/a")[2:]},
	})
}

func TestTheSessionsOfARepeatedCopilotDesktopWorkspaceGoWithTheFirstRowTheFilterKeeps(t *testing.T) {
	t.Parallel()
	const notice = "parser: workspaces holds 2 rows of this row's id; its sessions are shown once, with "
	old, current := copilotdesktopOld+"  /proj/a ", copilotdesktopNew+"  /proj/b "
	projects := []string{old + "project added: a kind=folder path=/proj/a default_branch=main",
		copilotdesktopOld + "  /proj/b project added: b kind=folder path=/proj/b default_branch=main"}
	oldRows := []string{old + "workspace: old ws type=worktree branch=b", old + notice + "this row"}
	session := func(path string) string {
		return copilotdesktopJoined + " s1 " + path + " " + copilotdesktopSessionText
	}
	copilotdesktopCheck(t, copilotdesktopRepeatedWorkspace, []copilotdesktopCase{
		{"without a filter it goes with the first row", nil, slices.Concat(projects, oldRows,
			[]string{current + "workspace: new ws type=worktree branch=b", current + notice + "another of them",
				session("/proj/a")})},
		{"a window that keeps only the later row moves it there", []string{"--since", "2026-03-01"},
			[]string{current + "workspace: new ws type=worktree branch=b", current + notice + "this row",
				session("/proj/b")}},
		{"a pattern only the later row matches moves it there", []string{"--match", "new ws|fix it"},
			[]string{current + "workspace: new ws type=worktree branch=b", session("/proj/b")}},
		{"a pattern only the earlier row matches keeps it with that row", []string{"--match", "old ws|fix it"},
			[]string{old + "workspace: old ws type=worktree branch=b", session("/proj/a")}},
		{"a pattern neither row matches leaves it with the first", []string{"--match", "fix it"},
			[]string{session("/proj/a")}},
	})
}

func TestTheSessionOfARepeatedCopilotDesktopAliasGoesWithTheFirstRowTheFilterKeeps(t *testing.T) {
	t.Parallel()
	const notice = "parser: workspace_session_aliases holds 2 rows of this row's session_id; its session's " +
		"rows are shown once, with "
	stored := []string{copilotdesktopOld + "  /proj/a project added: a kind=folder path=/proj/a default_branch=main",
		copilotdesktopOld + "  /proj/b project added: b kind=folder path=/proj/b default_branch=main",
		copilotdesktopOld + "  /proj/a workspace: ws a type=worktree branch=b",
		copilotdesktopOld + "  /proj/b workspace: ws b type=worktree branch=b"}
	old, current := copilotdesktopOld+" s1 /proj/a "+notice, copilotdesktopNew+" s1 /proj/b "+notice
	session := func(path string) string {
		return copilotdesktopJoined + " s1 " + path + " " + copilotdesktopSessionText
	}
	copilotdesktopCheck(t, copilotdesktopRepeatedAlias, []copilotdesktopCase{
		{"without a filter it goes with the first row", nil, slices.Concat(stored,
			[]string{old + "this row", current + "another of them", session("/proj/a")})},
		{"a window that keeps only the later row moves it there", []string{"--since", "2026-03-01"},
			[]string{current + "this row", session("/proj/b")}},
		{"a window that keeps only the earlier row keeps it with that row", []string{"--until", "2026-03-01"},
			slices.Concat(stored, []string{old + "this row"})},
		{"a pattern neither row matches leaves it with the first", []string{"--match", "fix it"},
			[]string{session("/proj/a")}},
	})
}

// copilotdesktopRepeatedChain returns a data.db whose projects table holds
// the id p1 in two rows, /proj/a and, five months later, /proj/b, and
// whose workspaces table holds the id w1 of that project in two rows, and
// a session of the workspace, all three joined a day after the later
// project row.
func copilotdesktopRepeatedChain(t *testing.T) string {
	return copilotdesktopStore(t, map[string][][]any{
		"projects": {copilotdesktopProject("p1", "old", "/proj/a", copilotdesktopOld),
			copilotdesktopProject("p1", "new", "/proj/b", copilotdesktopNew)},
		"workspaces": {{"w1", "p1", "ws one", "worktree", "b", copilotdesktopJoined},
			{"w1", "p1", "ws two", "worktree", "b", copilotdesktopJoined}},
		"workspace_session_aliases": {{"s1", "w1", copilotdesktopJoined}},
		"sessions":                  {copilotdesktopSessionRow("s1")},
	})
}

// A row holds one place in one group, so a session whose workspace id
// and the workspace's project id both repeat is a member of the
// workspace's group alone, and the stored workspace rows, the group's
// parents, keep the path of the project's first stored row, whichever
// project row the filter keeps.
func TestASessionThroughTwoRepeatedKeysFollowsTheWorkspaceGroupAlone(t *testing.T) {
	const notice = "parser: workspaces holds 2 rows of this row's id; its sessions are shown once, with "
	joined := copilotdesktopJoined + "  /proj/a "
	copilotdesktopCheck(t, copilotdesktopRepeatedChain, []copilotdesktopCase{
		{"with a window that keeps only the later project row", []string{"--since", "2026-03-01"}, []string{
			copilotdesktopNew + "  /proj/b project added: new kind=folder path=/proj/b default_branch=main",
			copilotdesktopNew + "  /proj/b parser: projects holds 2 rows of this row's id; its worktrees, " +
				"workspaces and sessions are shown once, with this row",
			joined + "workspace: ws one type=worktree branch=b", joined + notice + "this row",
			joined + "workspace: ws two type=worktree branch=b", joined + notice + "another of them",
			copilotdesktopJoined + " s1 /proj/a " + copilotdesktopSessionText}},
	})
}
