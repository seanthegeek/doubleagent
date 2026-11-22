package parsers

import (
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The ChatGPT desktop growth cases: the conversation catalog of
// codex-dev.db, which the catalog gives to codex-cli.

// chatgptCatalog returns a codex-dev.db whose local_thread_catalog holds
// the rows, each thread_id, display_title, source_created_at,
// source_updated_at, conversation_origin, host_id and source_kind.
func chatgptCatalog(t *testing.T, catalog [][]any) []byte {
	return database(t, sqlitetest.Table{Name: "local_thread_catalog", SQL: "CREATE TABLE local_thread_catalog " +
		"(thread_id TEXT, display_title TEXT, source_created_at REAL, source_updated_at REAL, " +
		"conversation_origin TEXT, host_id TEXT, source_kind TEXT)", Rows: catalog})
}

// chatgptdesktopGrowth returns the chatgpt-desktop growth set.
func chatgptdesktopGrowth() growthSet {
	const rel = ".codex/sqlite/codex-dev.db"
	return growthSet{parser: "chatgpt-desktop", cases: []growthCase{
		{kind: "app database", shape: shapeSharedID, agent: "codex-cli", files: func(t *testing.T, n int) []growthFile {
			return one(rel, chatgptCatalog(t, rows(n, func(i int) []any {
				return []any{"6a0b-same", numbered("title ", i), 1.79e9, 1.79e9 + float64(i), "chatgpt", "h", "chatgpt"}
			})))
		}},
		{kind: "app database", shape: shapeJoins, agent: "codex-cli", files: func(t *testing.T, n int) []growthFile {
			body := strings.Repeat("t", 1024*n)
			return one(rel, chatgptCatalog(t, append(rows(n, func(i int) []any {
				return []any{numbered("codex-", i), nil, nil, nil, nil, nil, "vscode"}
			}), []any{"6a0b", body, 1.79e9, nil, "chatgpt", body, "chatgpt"})))
		}},
		{kind: "app database", shape: shapeCarried, agent: "codex-cli", files: func(t *testing.T, n int) []growthFile {
			return one(rel, chatgptCatalog(t, append([][]any{{long("thread"), "t", 1.79e9, nil, "chatgpt", "h",
				"chatgpt"}}, rows(n, func(i int) []any {
				return []any{numbered("6a0b-", i), "t", 1.79e9, nil, nil, nil, "chatgpt"}
			})...)))
		}},
	}}
}
