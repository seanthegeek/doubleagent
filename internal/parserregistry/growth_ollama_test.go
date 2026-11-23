package parserregistry

import (
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The Ollama growth cases: the desktop app's database, the REPL history,
// the launcher's config and the Claude Desktop profile it writes.

const (
	ollamaDatabaseRel = "Library/Application Support/Ollama/db.sqlite"
	ollamaProfileRel  = "Library/Application Support/Claude-3p/configLibrary/" +
		"00000000-0000-4000-8000-000000000114.json"
	ollamaTime = "2026-01-02T03:04:05.000-05:00"
)

// ollamaDatabase returns a db.sqlite holding the messages, tool calls and
// attachments given, each row in the column order of its table. The ids
// are not keys, so a case can repeat one as damage would.
func ollamaDatabase(t *testing.T, messages, calls, attachments [][]any) []byte {
	return database(t,
		sqlitetest.Table{Name: "messages", SQL: "CREATE TABLE messages (id INTEGER, chat_id TEXT, role TEXT, " +
			"content TEXT, thinking TEXT, model_name TEXT, created_at TIMESTAMP)", Rows: messages},
		sqlitetest.Table{Name: "tool_calls", SQL: "CREATE TABLE tool_calls (id INTEGER, message_id INTEGER, " +
			"function_name TEXT, function_arguments TEXT)", Rows: calls},
		sqlitetest.Table{Name: "attachments", SQL: "CREATE TABLE attachments (id INTEGER, message_id INTEGER, " +
			"filename TEXT, data BLOB)", Rows: attachments})
}

// ollamaMessage returns a messages row of the chat.
func ollamaMessage(messageID int, chat, role, content, model string) []any {
	return []any{messageID, chat, role, content, "", model, ollamaTime}
}

// ollamaCall returns a tool_calls row of the message.
func ollamaCall(callID, message int, name string) []any {
	return []any{callID, message, name, `{"query":"q` + strconv.Itoa(callID) + `"}`}
}

// ollamaGrowth returns the ollama growth set.
func ollamaGrowth() growthSet {
	return growthSet{parser: "ollama", cases: append(ollamaDatabaseCases(), ollamaFileCases()...)}
}

// ollamaDatabaseCases returns the cases of db.sqlite.
func ollamaDatabaseCases() []growthCase {
	return []growthCase{
		{kind: "database", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			messages := [][]any{ollamaMessage(1, long("chat"), "assistant", "calling", "m")}
			return one(ollamaDatabaseRel, ollamaDatabase(t, messages,
				rows(n, func(i int) []any { return ollamaCall(i+1, 1, "web_search") }), nil))
		}},
		{kind: "database", shape: shapeCarried + " (a repeated message id's model)", files: func(t *testing.T,
			n int) []growthFile {
			messages := [][]any{ollamaMessage(1, "c", "assistant", "a", long("m")),
				ollamaMessage(1, "c", "assistant", "b", long("n"))}
			return one(ollamaDatabaseRel, ollamaDatabase(t, messages,
				rows(n, func(i int) []any { return ollamaCall(i+1, 1, "web_search") }), nil))
		}},
		{kind: "database", shape: shapeJoins + " in the chat id", files: func(t *testing.T, n int) []growthFile {
			messages := [][]any{ollamaMessage(1, strings.Repeat("c", 1024*n), "assistant", "calling", "m")}
			return one(ollamaDatabaseRel, ollamaDatabase(t, messages,
				rows(n, func(i int) []any { return ollamaCall(i+1, 1, "web_search") }), nil))
		}},
		{kind: "database", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			messages := append([][]any{ollamaMessage(1, "c", "assistant", "", "m")},
				rows(n, func(i int) []any { return ollamaMessage(i+2, "c", "tool", "result", "") })...)
			return one(ollamaDatabaseRel, ollamaDatabase(t, messages, [][]any{ollamaCall(1, 1, long("tool"))}, nil))
		}},
		{kind: "database", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			messages := rows(n, func(i int) []any { return ollamaMessage(1, "c", "assistant", "hi", "m") })
			messages = append(messages, rows(n, func(i int) []any { return ollamaMessage(2, "c", "user", "u", "") })...)
			attachments := rows(n, func(i int) []any { return []any{i, 2, numbered("f", i), []byte("data")} })
			return one(ollamaDatabaseRel, ollamaDatabase(t, messages,
				rows(n, func(i int) []any { return ollamaCall(i, 1, "web_fetch") }), attachments))
		}},
		{kind: "database", shape: shapeSharedID + " with no content of their own", files: func(t *testing.T, n int) []growthFile {
			messages := rows(n, func(i int) []any {
				return ollamaMessage(1, "c", "assistant", "", []string{"m1", "m2"}[i%2])
			})
			messages = append(messages, ollamaMessage(2, "c", "assistant", "next", "m3"))
			return one(ollamaDatabaseRel, ollamaDatabase(t, messages,
				rows(n, func(i int) []any { return ollamaCall(i, 1, "web_fetch") }), nil))
		}},
		{kind: "database", shape: shapeJoins, files: func(t *testing.T, n int) []growthFile {
			messages := [][]any{ollamaMessage(1, "c", "assistant", strings.Repeat("a", 1024*n), "m"),
				ollamaMessage(2, "c", "user", strings.Repeat("u", 1024*n), "")}
			attachments := rows(n, func(i int) []any { return []any{i, 2, "same.txt", []byte("data")} })
			return one(ollamaDatabaseRel, ollamaDatabase(t, messages,
				rows(n, func(i int) []any { return ollamaCall(1, 1, "web_fetch") }), attachments))
		}},
		{kind: "database", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			messages := []any{1, "c", "assistant", "x", "", "m", ollamaTime}
			calls := rows(n, func(i int) []any { return []any{i, 1, "tool", `{"query":`} })
			calls = append(calls, []any{n, 1, "tool", nestedObjects(n, "a", `"x"`)})
			return one(ollamaDatabaseRel, ollamaDatabase(t, [][]any{messages}, calls, nil))
		}},
	}
}

// ollamaFileCases returns the cases of the history, config and profile.
func ollamaFileCases() []growthCase {
	return []growthCase{
		{kind: "history", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".ollama/history", lines(n, func(i int) string {
				if i%2 == 0 {
					return "bad \xff" + strconv.Itoa(i)
				}
				return "/set " + strconv.Itoa(i)
			}))
		}},
		{kind: "history", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".ollama/history", append([]byte(long("prompt")+"\n"),
				lines(n, func(i int) string { return numbered("p", i) })...))
		}},
		{kind: "config", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".ollama/config.json", []byte(`{"model":"`+long("model")+`","integrations":{`+
				joined(n, func(i int) string { return `"` + numbered("tool", i) + `":{"automode":true}` })+`}}`))
		}},
		{kind: "config", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".ollama/config.json", []byte(`{"integrations":{`+joined(n, func(i int) string {
				return `"` + numbered("bad", i) + `":` + strconv.Itoa(i)
			})+`,"deep":{"automode":`+nestedArrays(n, "true")+`}}}`))
		}},
		{kind: "profile", shape: shapeNested, agent: "claude-desktop", files: func(t *testing.T, n int) []growthFile {
			return one(ollamaProfileRel, []byte(`{"other":`+nestedArrays(n, "1")+`,"autoModeEnabled":`+
				nestedObjects(n, "a", "true")+`}`))
		}},
	}
}
