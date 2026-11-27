package growthtests

import (
	"fmt"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The Copilot Desktop growth cases: the app's data.db and the checkpoint
// refs of its clones.

const (
	copilotdesktopRel  = ".copilot/data.db"
	copilotdesktopTime = "2026-10-01T09:00:00.000Z"
)

// copilotdesktopTables are the tables of data.db the cases fill, by name,
// with their columns.
func copilotdesktopTables() map[string]string {
	return map[string]string{
		"projects": "id TEXT, name TEXT, container_kind TEXT, main_repo_path TEXT, github_owner TEXT, " +
			"github_repo TEXT, default_branch TEXT, created_at TEXT",
		"workspaces":                "id TEXT, project_id TEXT, name TEXT, workspace_type TEXT, branch TEXT, created_at TEXT",
		"workspace_session_aliases": "session_id TEXT, workspace_id TEXT, created_at TEXT",
		"sessions": "id TEXT, title TEXT, session_type TEXT, model TEXT, auto_approve INTEGER, " +
			"permission_mode TEXT, remote_control_enabled INTEGER, archived_at TEXT, forked_from_session_id TEXT, " +
			"created_at TEXT",
		"worktrees":      "path TEXT, branch TEXT, base_branch TEXT, project_id TEXT, created_at TEXT",
		"activity_items": "session_id TEXT, activity_type TEXT, preview TEXT, created_at TEXT",
	}
}

// copilotdesktopDatabase returns a data.db holding the rows given by table
// name; a table given no rows is left out.
func copilotdesktopDatabase(t *testing.T, filled map[string][][]any) []byte {
	var tables []sqlitetest.Table
	for _, name := range []string{"projects", "workspaces", "workspace_session_aliases", "sessions", "worktrees",
		"activity_items"} {
		if content, ok := filled[name]; ok {
			tables = append(tables, sqlitetest.Table{Name: name,
				SQL: "CREATE TABLE " + name + " (" + copilotdesktopTables()[name] + ")", Rows: content})
		}
	}
	return database(t, tables...)
}

// copilotdesktopSession returns a sessions row.
func copilotdesktopSession(sessionID string) []any {
	return []any{sessionID, "fix the build", "interactive", "gpt-5", 0, "default", 0, nil, nil, copilotdesktopTime}
}

// copilotdesktopProject returns a projects row of the path.
func copilotdesktopProject(projectID, path string) []any {
	return []any{projectID, "proj", "local", path, nil, nil, "main", copilotdesktopTime}
}

// copilotdesktopJoinCases returns the cases of data.db where one project's
// path is joined to many rows.
func copilotdesktopJoinCases() []growthCase {
	return []growthCase{
		{kind: "data.db", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			return one(copilotdesktopRel, copilotdesktopDatabase(t, map[string][][]any{
				"projects":   {copilotdesktopProject("p1", "/"+long("repo"))},
				"workspaces": {{"w1", "p1", "main", "local", "main", copilotdesktopTime}},
				"workspace_session_aliases": rows(n, func(i int) []any {
					return []any{numbered("s", i), "w1", copilotdesktopTime}
				}),
				"sessions": rows(n, func(i int) []any { return copilotdesktopSession(numbered("s", i)) }),
			}))
		}},
		{kind: "data.db", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(copilotdesktopRel, copilotdesktopDatabase(t, map[string][][]any{
				"projects": {copilotdesktopProject("p1", "/"+long("repo"))},
				"worktrees": rows(n, func(i int) []any {
					return []any{numbered("/w/", i), numbered("b", i), "main", "p1", copilotdesktopTime}
				}),
			}))
		}},
	}
}

// copilotdesktopRepeatCases returns the cases of data.db that repeat ids
// and join keys, and of the checkpoint refs.
func copilotdesktopRepeatCases() []growthCase {
	return []growthCase{
		{kind: "data.db", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(copilotdesktopRel, copilotdesktopDatabase(t, map[string][][]any{
				"projects": rows(n, func(i int) []any { return copilotdesktopProject("p1", numbered("/p", i)) }),
				"workspaces": rows(n, func(i int) []any {
					return []any{"w1", "p1", "main", "local", "main", copilotdesktopTime}
				}),
				"workspace_session_aliases": rows(n, func(i int) []any { return []any{"s1", "w1", copilotdesktopTime} }),
				"sessions":                  rows(n, func(i int) []any { return copilotdesktopSession("s1") }),
			}))
		}},
		// Each link of a session's path to its project repeats, every stored
		// row of each naming another: the session rows are members of the
		// alias's group, the workspace rows parents of the workspace's.
		{kind: "data.db", shape: shapeSharedID + " (each link of a chain, every stored row its own)",
			files: func(t *testing.T, n int) []growthFile {
				return one(copilotdesktopRel, copilotdesktopDatabase(t, map[string][][]any{
					"projects": rows(n, func(i int) []any { return copilotdesktopProject("p1", numbered("/p", i)) }),
					"workspaces": rows(n, func(i int) []any {
						return []any{"w1", "p1", numbered("ws", i), "local", "main", copilotdesktopTime}
					}),
					"workspace_session_aliases": rows(n, func(i int) []any {
						return []any{"s1", numbered("w", i), copilotdesktopTime}
					}),
					"sessions": rows(n, func(i int) []any { return copilotdesktopSession("s1") }),
					"worktrees": rows(n, func(i int) []any {
						return []any{numbered("/w/", i), numbered("b", i), "main", "p1", copilotdesktopTime}
					}),
				}))
			}},
		{kind: "data.db", shape: shapeJoins, files: func(t *testing.T, n int) []growthFile {
			session := copilotdesktopSession("s1")
			session[1] = strings.Repeat("t", 1024*n)
			return one(copilotdesktopRel, copilotdesktopDatabase(t, map[string][][]any{
				"sessions": {session},
				"activity_items": rows(n, func(i int) []any {
					return []any{"s1", "message", numbered("preview ", i), copilotdesktopTime}
				}),
				"workspace_session_aliases": rows(n, func(i int) []any { return []any{"s1", "w1", copilotdesktopTime} }),
			}))
		}},
		{kind: "checkpoint ref", shape: shapeManyFiles, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string {
				return ".copilot/repos/proj/.git/refs/copilot/checkpoints/5e55a0a1-0000-4000-8000-000000000001/" +
					fmt.Sprintf("%020d", i) + "/e0000000-0000-4000-8000-000000000004"
			}, func(i int) string { return strings.Repeat("a", 40) + "\n" })
		}},
	}
}

// copilotdesktopGrowth returns the copilot-desktop growth set.
func copilotdesktopGrowth() growthSet {
	return growthSet{parser: "copilot-desktop", cases: append(copilotdesktopJoinCases(),
		copilotdesktopRepeatCases()...)}
}
