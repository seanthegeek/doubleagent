package parserregistry

import (
	"slices"
	"strconv"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The Zed growth cases: threads.db, whose rows hold each thread as JSON or
// as a zstd frame of it, a channel's db.sqlite with its sidebar threads,
// key-value store and archived worktrees joined to the threads they were
// archived from, and the user settings.

// The files of Zed, by path relative to the home.
const (
	zedThreads  = ".local/share/zed/threads/threads.db"
	zedSidebar  = ".local/share/zed/db/0-stable/db.sqlite"
	zedSettings = ".config/zed/settings.json"
)

// zedThreadTable returns threads.db holding the given rows: id, summary,
// updated_at, data_type, data, folder_paths.
func zedThreadTable(t *testing.T, threadRows [][]any) []byte {
	t.Helper()
	return database(t, sqlitetest.Table{Name: "threads", SQL: "CREATE TABLE threads (id TEXT, summary TEXT, " +
		"updated_at TEXT, data_type TEXT, data BLOB, folder_paths TEXT)", Rows: threadRows})
}

// zedThread returns a thread row of id holding the JSON thread as a zstd
// frame.
func zedThread(threadID, thread string) []any {
	return []any{threadID, "summary", "2026-01-02T03:04:05Z", "zstd", zstdFrame(17, []byte(thread)), "/w"}
}

// zedUser returns a User message holding text.
func zedUser(text string) string {
	return `{"User":{"id":"u","content":[{"Text":"` + text + `"}]}}`
}

// zedGrowth returns the zed growth set.
func zedGrowth() growthSet {
	return growthSet{parser: "zed", cases: slices.Concat(zedThreadCases(), zedSidebarCases(), zedSettingCases())}
}

// zedThreadCases returns the cases of threads.db.
func zedThreadCases() []growthCase {
	return []growthCase{
		{kind: "threads.db", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			thread := `{"version":"0.3.0","title":"t","model":{"provider":"p","model":"` + long("model") +
				`"},"messages":[` + joined(n, func(i int) string { return zedUser(numbered("hi", i)) }) + `]}`
			return one(zedThreads, zedThreadTable(t, [][]any{zedThread("t", thread)}))
		}},
		{kind: "threads.db", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			agent := `{"Agent":{"content":[` + joined(n, func(i int) string {
				return `{"ToolUse":{"id":"c","name":"read","input":{"path":"` + numbered("a", i) + `"}}}`
			}) + `],"tool_results":{"c":{"tool_use_id":"c","tool_name":"read","is_error":false,"content":"` +
				long("output") + `"}}}}`
			thread := `{"version":"0.3.0","messages":[` + zedUser("go") + "," + agent + `]}`
			return one(zedThreads, zedThreadTable(t, [][]any{zedThread("t", thread)}))
		}},
		{kind: "threads.db", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(zedThreads, zedThreadTable(t, rows(n, func(i int) []any {
				return zedThread("t", `{"version":"0.3.0","messages":[`+zedUser(numbered("hi", i))+`]}`)
			})))
		}},
		{kind: "threads.db", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			deep := `{"Agent":{"content":[{"ToolUse":{"id":"c","name":"x","input":` + nestedArrays(n, "1") + `}}]}}`
			thread := `{"version":"0.3.0","messages":[` + joined(n, strconv.Itoa) + "," + deep + `]}`
			return one(zedThreads, zedThreadTable(t, [][]any{zedThread("t", thread),
				{"bad", "s", "2026-01-02T03:04:06Z", "zstd", []byte("not zstd"), "/w"}}))
		}},
	}
}

// zedSidebarTables returns the sidebar tables of a db.sqlite: the sidebar
// threads (thread_id, session_id, title, updated_at, folder_paths,
// main_worktree_paths), the key-value store (namespace, key, value), the
// archived worktrees (id, worktree_path, main_repo_path, branch_name) and
// their links (thread_id, archived_worktree_id).
func zedSidebarTables(t *testing.T, sidebar, store, archived, links [][]any) []byte {
	t.Helper()
	return database(t, sqlitetest.Table{Name: "sidebar_threads", SQL: "CREATE TABLE sidebar_threads (thread_id BLOB, " +
		"session_id TEXT, title TEXT, updated_at TEXT, folder_paths TEXT, main_worktree_paths TEXT)", Rows: sidebar},
		sqlitetest.Table{Name: "scoped_kv_store", SQL: "CREATE TABLE scoped_kv_store (namespace TEXT, key TEXT, " +
			"value TEXT)", Rows: store},
		sqlitetest.Table{Name: "archived_git_worktrees", SQL: "CREATE TABLE archived_git_worktrees (id INTEGER, " +
			"worktree_path TEXT, main_repo_path TEXT, branch_name TEXT)", Rows: archived},
		sqlitetest.Table{Name: "thread_archived_worktrees", SQL: "CREATE TABLE thread_archived_worktrees " +
			"(thread_id TEXT, archived_worktree_id INTEGER)", Rows: links})
}

// zedSidebarCases returns the cases of a channel's db.sqlite.
func zedSidebarCases() []growthCase {
	return []growthCase{
		{kind: "workspace db.sqlite", shape: shapeJoins, files: func(t *testing.T, n int) []growthFile {
			sidebar := rows(n, func(i int) []any {
				return []any{numbered("t", i), numbered("s", i), "thread", "2026-01-02T03:04:05Z", "/w", nil}
			})
			links := rows(n, func(i int) []any { return []any{numbered("t", i), int64(1)} })
			return one(zedSidebar, zedSidebarTables(t, sidebar, nil, [][]any{{int64(1), "/w/tree", "/w", "main"}}, links))
		}},
		{kind: "workspace db.sqlite", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			folders := string(lines(n, func(i int) string { return numbered("/w/tree", i) }))
			sidebar := [][]any{{"t", "s", "thread", "2026-01-02T03:04:05Z", folders, "/" + long("main")}}
			store := rows(n, func(i int) []any {
				return []any{"created_git_worktrees", "local\n" + numbered("/w/tree", i),
					`{"created_at_seconds":` + strconv.Itoa(1767323045+i) + `}`}
			})
			return one(zedSidebar, zedSidebarTables(t, sidebar, store, nil, nil))
		}},
		{kind: "workspace db.sqlite", shape: shapeSharedID, n: 128, files: func(t *testing.T, n int) []growthFile {
			archived := rows(n, func(i int) []any { return []any{int64(1), numbered("/w/copy", i), "/w", nil} })
			links := rows(n, func(i int) []any { return []any{numbered("t", i), int64(1)} })
			return one(zedSidebar, zedSidebarTables(t, nil, nil, archived, links))
		}},
		{kind: "workspace db.sqlite", shape: shapeSharedID, n: 128, files: func(t *testing.T, n int) []growthFile {
			sidebar := rows(n, func(i int) []any {
				return []any{"t", numbered("s", i), "thread", "2026-01-02T03:04:05Z", numbered("/w", i), nil}
			})
			store := rows(n, func(i int) []any {
				return []any{"agent_draft_prompts", "t", `[{"type":"text","text":"` + numbered("draft", i) + `"}]`}
			})
			archived := rows(n, func(i int) []any { return []any{int64(i), numbered("/w/tree", i), "/w", nil} })
			links := rows(n, func(i int) []any { return []any{"t", int64(i)} })
			return one(zedSidebar, zedSidebarTables(t, sidebar, store, archived, links))
		}},
	}
}

// zedSettingCases returns the cases of the settings files.
func zedSettingCases() []growthCase {
	return []growthCase{
		{kind: "settings.json", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(zedSettings, []byte(`{"agent":{"tool_permissions":{"`+long("tool")+`":{`+
				joined(n, func(i int) string { return `"` + numbered("key", i) + `":"allow"` })+`}}}}`))
		}},
		{kind: "settings.json", shape: shapeNested, n: 256,
			files: func(t *testing.T, n int) []growthFile {
				return one(zedSettings, []byte(`// comment
{"agent":{"tool_permissions":`+nestedObjects(n, "level", `"allow"`)+`},"agent_servers":{`+
					joined(n, func(i int) string { return `"` + numbered("s", i) + `":` + strconv.Itoa(i) })+`}}`))
			}},
	}
}
