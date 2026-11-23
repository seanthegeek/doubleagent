package growthtests

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The Crush growth cases: a project's crush.db, whose messages hold their
// parts as a JSON array and whose sessions, files and reads name a session
// by id, the projects list, crush.json and crushrc.

// crushDatabase is where a project's crush.db sits under the home.
const crushDatabase = "p/.crush/crush.db"

// crushSessions returns the sessions table holding the given rows: id,
// title, parent_session_id, created_at.
func crushSessions(sessionRows [][]any) sqlitetest.Table {
	return sqlitetest.Table{Name: "sessions", SQL: "CREATE TABLE sessions (id TEXT, title TEXT, " +
		"parent_session_id TEXT, created_at INTEGER)", Rows: sessionRows}
}

// crushMessages returns the messages table holding the given rows: id,
// session_id, role, parts, model, provider, created_at.
func crushMessages(messageRows [][]any) sqlitetest.Table {
	return sqlitetest.Table{Name: "messages", SQL: "CREATE TABLE messages (id TEXT, session_id TEXT, " +
		"role TEXT, parts TEXT, model TEXT, provider TEXT, created_at INTEGER)", Rows: messageRows}
}

// crushText returns a parts array of one text part.
func crushText(text string) string {
	return `[{"type":"text","data":{"text":"` + text + `"}}]`
}

// crushGrowth returns the crush growth set.
func crushGrowth() growthSet {
	return growthSet{parser: "crush", cases: slices.Concat(crushDatabaseCases(), crushSettingCases())}
}

// crushDatabaseCases returns the cases of crush.db.
func crushDatabaseCases() []growthCase {
	return []growthCase{
		{kind: "crush.db", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			parts := "[" + joined(n, func(i int) string {
				return `{"type":"text","data":{"text":"` + numbered("t", i) + `"}}`
			}) + "]"
			return one(crushDatabase, database(t, crushSessions([][]any{{"s", "title", nil, int64(1767000000)}}),
				crushMessages([][]any{{"m", "s", long("role"), parts, "gpt", "openai", int64(1767000001)}})))
		}},
		{kind: "crush.db", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(crushDatabase, database(t,
				crushSessions(rows(n, func(i int) []any { return []any{"s", numbered("title", i), "gone", int64(i)} })),
				crushMessages(rows(n, func(i int) []any {
					return []any{"m", "s", "user", crushText(numbered("hi", i)), numbered("model", i%2), "openai",
						int64(1767000000 + i)}
				}))))
		}},
		{kind: "crush.db", shape: shapeJoins, files: func(t *testing.T, n int) []growthFile {
			files := sqlitetest.Table{Name: "files", SQL: "CREATE TABLE files (id TEXT, session_id TEXT, path TEXT, " +
				"content TEXT, version INTEGER, created_at INTEGER)", Rows: rows(n, func(i int) []any {
				return []any{numbered("f", i), "s", "/p/a.go", strings.Repeat("c", 1024), int64(i), int64(i)}
			})}
			reads := sqlitetest.Table{Name: "read_files", SQL: "CREATE TABLE read_files (session_id TEXT, " +
				"path TEXT, read_at INTEGER)", Rows: rows(n, func(i int) []any { return []any{"s", "/p/a.go", int64(i)} })}
			return one(crushDatabase, database(t, crushSessions([][]any{{"s", "t", nil, int64(1)}}),
				crushMessages(rows(n, func(i int) []any {
					parts := `[{"type":"tool_call","data":{"id":"c","name":"view","input":"{}"}},` +
						`{"type":"tool_result","data":{"tool_call_id":"c","name":"view","content":"ok"}}]`
					return []any{numbered("m", i), "s", "assistant", parts, "gpt", "openai", int64(i)}
				})), files, reads))
		}},
		{kind: "crush.db", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			deep := `[{"type":"tool_call","data":{"id":"c","name":"x","input":` + nestedArrays(n, "1") + `}},` +
				joined(n, func(i int) string { return strconv.Itoa(i) }) + "]"
			return one(crushDatabase, database(t, crushSessions(nil), crushMessages([][]any{
				{"m", "s", "assistant", deep, "gpt", "openai", int64(1)}})))
		}},
	}
}

// crushSettingCases returns the cases of projects.json, crush.json and
// crushrc.
func crushSettingCases() []growthCase {
	return []growthCase{
		{kind: "projects.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".local/share/crush/projects.json", []byte(`{"projects":[`+joined(n, func(i int) string {
				return `{"path":"/p","data_dir":".crush","last_accessed":"2026-01-02T03:04:05Z"}`
			})+`]}`))
		}},
		{kind: "projects.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(".local/share/crush/projects.json", []byte(`{"projects":[`+joined(n, strconv.Itoa)+
				`,{"path":"/p","data_dir":`+nestedArrays(n, `"x"`)+`}]}`))
		}},
		{kind: "crush.json", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".config/crush/crush.json", []byte(`{"mcp":{"`+long("server")+`":{"disabled_tools":[`+
				joined(n, func(i int) string { return `"` + numbered("tool", i) + `"` })+`]}}}`))
		}},
		{kind: "crush.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(".config/crush/crush.json", []byte(`{"mcp":{`+joined(n, func(i int) string {
				return `"` + numbered("s", i) + `":` + strconv.Itoa(i)
			})+`},"permissions":{"allowed_tools":[`+nestedArrays(n, `"x"`)+`]}}`))
		}},
		{kind: "crushrc", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".config/crush/crushrc", []byte("permissions allow "+strings.ReplaceAll(joined(n, func(i int) string {
				return numbered("tool", i)
			}), ",", " ")+"\n"))
		}},
		{kind: "crushrc", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".config/crush/crushrc", lines(n, func(i int) string {
				return "permissions allow 'x" + numbered("t", i)
			}))
		}},
	}
}
