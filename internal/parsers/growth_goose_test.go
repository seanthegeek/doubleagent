package parsers

import (
	"slices"
	"strconv"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The Goose growth cases: sessions.db, whose messages take their session's
// working directory and their tool responses the name of the request of
// their id, the legacy session files, the request logs, the prompt
// history and the three standing files.

// The files of Goose, by path relative to the home.
const (
	gooseDatabase = ".local/share/goose/sessions/sessions.db"
	gooseLegacy   = ".local/share/goose/sessions/20260102_030405.jsonl"
	gooseLog      = ".local/state/goose/logs/llm_request.0.jsonl"
	gooseConfig   = ".config/goose/"
)

// gooseTables returns sessions.db holding the given sessions (id, name,
// working_dir, created_at, model_config_json) and messages (id, session_id,
// role, content_json, created_timestamp).
func gooseTables(t *testing.T, sessionRows, messageRows [][]any) []byte {
	t.Helper()
	return database(t, sqlitetest.Table{Name: "sessions", SQL: "CREATE TABLE sessions (id TEXT, name TEXT, " +
		"working_dir TEXT, created_at TEXT, model_config_json TEXT)", Rows: sessionRows},
		sqlitetest.Table{Name: "messages", SQL: "CREATE TABLE messages (id INTEGER, session_id TEXT, " +
			"role TEXT, content_json TEXT, created_timestamp INTEGER)", Rows: messageRows})
}

// gooseSession is one session of sessions.db at the working directory.
func gooseSession(sessionID, workingDir string) []any {
	return []any{sessionID, "name", workingDir, "2026-01-02 03:04:05", `{"model_name":"gpt"}`}
}

// gooseMessage is message i of the session holding content.
func gooseMessage(i int, sessionID, role, content string) []any {
	return []any{int64(i + 1), sessionID, role, content, int64(1767323045 + i)}
}

// gooseGrowth returns the goose growth set.
func gooseGrowth() growthSet {
	return growthSet{parser: "goose", cases: slices.Concat(gooseDatabaseCases(), gooseFileCases(),
		gooseStandingCases())}
}

// gooseDatabaseCases returns the cases of sessions.db.
func gooseDatabaseCases() []growthCase {
	return []growthCase{
		{kind: "sessions.db", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(gooseDatabase, gooseTables(t, [][]any{gooseSession("s", "/"+long("w"))},
				rows(n, func(i int) []any {
					return gooseMessage(i, "s", "user", `[{"type":"text","text":"`+numbered("hi", i)+`"}]`)
				})))
		}},
		{kind: "sessions.db", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			request := `[{"type":"toolRequest","id":"c","toolCall":{"status":"success","value":{"name":"` +
				long("tool") + `","arguments":{}}}}]`
			return one(gooseDatabase, gooseTables(t, [][]any{gooseSession("s", "/w")},
				append([][]any{gooseMessage(0, "s", "assistant", request)}, rows(n, func(i int) []any {
					return gooseMessage(i+1, "s", "user", `[{"type":"toolResponse","id":"c","toolResult":`+
						`{"status":"success","value":[{"type":"text","text":"ok"}]}}]`)
				})...)))
		}},
		{kind: "sessions.db", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			confirm := `[{"type":"actionRequired","data":{"actionType":"toolConfirmationResponse","id":"c",` +
				`"permission":"allow_once"}},{"type":"toolRequest","id":"c","toolCall":{"status":"success",` +
				`"value":{"name":"shell","arguments":{}}},"_meta":{"goose.executable":false}}]`
			return one(gooseDatabase, gooseTables(t, rows(n, func(i int) []any { return gooseSession("s", "/w") }),
				rows(n, func(i int) []any { return []any{int64(1), "s", "user", confirm, int64(1767323045)} })))
		}},
		// Each stored row of a repeated session id is a parent of the id's
		// group, with its own rows; each message is a member.
		{kind: "sessions.db", shape: shapeSharedID + " (stored rows in their own directories)",
			files: func(t *testing.T, n int) []growthFile {
				return one(gooseDatabase, gooseTables(t, rows(n, func(i int) []any {
					return gooseSession("s", "/"+numbered("w", i))
				}), rows(n, func(i int) []any {
					return gooseMessage(i, "s", "user", `[{"type":"text","text":"`+numbered("hi", i)+`"}]`)
				})))
			}},
		{kind: "sessions.db", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			content := `[{"type":"toolRequest","id":"c","toolCall":{"status":"success","value":{"name":"x",` +
				`"arguments":{"a":` + nestedArrays(n, "1") + `}}}},` + joined(n, strconv.Itoa) + `]`
			return one(gooseDatabase, gooseTables(t, [][]any{gooseSession("s", "/w")},
				[][]any{gooseMessage(0, "s", "assistant", content)}))
		}},
	}
}

// gooseFileCases returns the cases of the legacy sessions, the request
// logs and the prompt history.
func gooseFileCases() []growthCase {
	return []growthCase{
		{kind: "legacy session", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			header := `{"id":"s","working_dir":"/` + long("w") + `","description":"d","created_at":"2026-01-02"}`
			return one(gooseLegacy, append([]byte(header+"\n"), lines(n, func(i int) string {
				return `{"role":"user","created":1767323045,"content":[{"type":"text","text":"` + numbered("hi", i) + `"}]}`
			})...))
		}},
		{kind: "legacy session", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(gooseLegacy, append(lines(n, func(i int) string { return `{"role":"user","content":` }),
				[]byte(`{"role":"user","content":[{"type":"text","text":`+nestedArrays(n, `"x"`)+`}]}`+"\n")...))
		}},
		{kind: "llm request log", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			request := `{"model_config":{"model_name":"` + long("model") + `"},"input":{"messages":[]}}`
			return one(gooseLog, append([]byte(request+"\n"), lines(n, func(i int) string {
				return `{"data":{"id":"` + numbered("m", i) + `","role":"assistant","content":"` + numbered("a", i) + `"}}`
			})...))
		}},
		{kind: "llm request log", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			request := `{"model_config":{"model_name":"gpt"},"input":{"messages":[{"role":"user","content":"hi"}]}}`
			return one(gooseLog, append([]byte(request+"\n"), lines(n, func(i int) string {
				return `{"data":{"id":"m","role":"assistant","content":[{"type":"text","text":"` + numbered("w", i) + ` "}]}}`
			})...))
		}},
		{kind: "llm request log", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(gooseLog, append(lines(n, func(i int) string { return `{"data":` + strconv.Itoa(i) + `}` }),
				[]byte(`{"data":{"id":"m","content":[{"type":"text","text":`+nestedArrays(n, `"x"`)+`}]}}`+"\n")...))
		}},
		{kind: "history.txt", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".config/goose/history.txt", append([]byte("#V2\n"), lines(n, func(i int) string {
				return `same prompt\nsecond line\\`
			})...))
		}},
	}
}

// gooseStandingCases returns the cases of permission.yaml,
// roaming_trust.json and schedule.json.
func gooseStandingCases() []growthCase {
	return []growthCase{
		{kind: "permission.yaml", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(gooseConfig+"permission.yaml", append([]byte(long("user")+":\n  always_allow:\n"),
				lines(n, func(i int) string { return "  - " + numbered("tool", i) })...))
		}},
		{kind: "permission.yaml", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(gooseConfig+"permission.yaml", lines(n, func(i int) string { return numbered("bad line ", i) }))
		}},
		{kind: "roaming_trust.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(gooseConfig+"roaming_trust.json", []byte(`{"allowed":[`+joined(n, func(i int) string {
				return `"peer"`
			})+`],"revoked_keys":[`+nestedArrays(n, `"k"`)+`]}`))
		}},
		{kind: "schedule.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".local/share/goose/schedule.json", []byte(`[`+joined(n, func(i int) string {
				return `{"id":"job","cron":"0 * * * *","source":"/r.yaml","parameters":{"a":` + strconv.Itoa(i) + `}}`
			})+`]`))
		}},
		{kind: "schedule.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".local/share/goose/schedule.json", []byte(`[`+joined(n, strconv.Itoa)+
				`,{"id":"job","parameters":`+nestedArrays(n, "1")+`}]`))
		}},
	}
}
