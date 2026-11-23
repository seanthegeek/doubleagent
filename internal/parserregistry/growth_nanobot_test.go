package parserregistry

import (
	"encoding/base64"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The nanobot growth cases: the session files and database with their
// .workspace marker, the WebUI mirror and its sidebar index, the memory and
// CLI histories, the run records and the settings files.

// nanobotNamespace returns the sessions directory of namespace i.
func nanobotNamespace(i int) string {
	digits := strconv.Itoa(i)
	return ".nanobot/sessions/" + strings.Repeat("0", 32-len(digits)) + digits + "/"
}

// nanobotMetadata returns a session file's metadata line of the key with
// the model preset.
func nanobotMetadata(key, preset string) string {
	return `{"_type":"metadata","key":"` + key + `","created_at":"2026-01-01T00:00:00",` +
		`"updated_at":"2026-01-01T01:00:00","metadata":{"_nanobot_model_preset":"` + preset + `"}}`
}

// nanobotMessage returns session message i: a user prompt, an assistant
// reply calling a tool, or the tool's result, in turn.
func nanobotMessage(i int) string {
	stamp := `,"timestamp":"2026-01-01T00:00:` + strconv.Itoa(10+i%50) + `"`
	switch i % 3 {
	case 0:
		return `{"role":"user","content":"` + numbered("ask ", i) + `"` + stamp + "}"
	case 1:
		return `{"role":"assistant","content":"ok","tool_calls":[{"id":"` + numbered("c", i) + `","type":"function",` +
			`"function":{"name":"exec","arguments":"{\"command\":\"ls\"}"}}]` + stamp + "}"
	}
	return `{"role":"tool","name":"exec","tool_call_id":"` + numbered("c", i-1) + `","content":"out"` + stamp + "}"
}

// nanobotSession returns a session file: the metadata line, then n
// messages.
func nanobotSession(metadata string, n int) []byte {
	return append([]byte(metadata+"\n"), lines(n, nanobotMessage)...)
}

// nanobotGrowth returns the nanobot growth set.
func nanobotGrowth() growthSet {
	var cases []growthCase
	cases = append(cases, nanobotSessionCases()...)
	cases = append(cases, nanobotDatabaseCases()...)
	cases = append(cases, nanobotWebUICases()...)
	cases = append(cases, nanobotFileCases()...)
	cases = append(cases, nanobotSettingCases()...)
	return growthSet{parser: "nanobot", cases: cases}
}

// nanobotSessionCases returns the cases of the session JSONL files.
func nanobotSessionCases() []growthCase {
	namespace := nanobotNamespace(1)
	return []growthCase{
		{kind: "session jsonl", shape: shapeCarried + " by the .workspace marker", files: func(t *testing.T, n int) []growthFile {
			return []growthFile{text(namespace+".workspace", "/"+long("p")),
				{rel: namespace + "cli_direct.jsonl", data: nanobotSession(nanobotMetadata("cli:direct", "m"), n)}}
		}},
		{kind: "session jsonl", shape: shapeCarried + " by the session key", files: func(t *testing.T, n int) []growthFile {
			return one(".nanobot/sessions/s.jsonl", nanobotSession(nanobotMetadata(long("cli:"), "m"), n))
		}},
		{kind: "session jsonl", shape: shapeCarried + " by the model preset", files: func(t *testing.T, n int) []growthFile {
			return one(".nanobot/sessions/s.jsonl", nanobotSession(nanobotMetadata("cli:direct", long("preset")), n))
		}},
		{kind: "session jsonl", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			files := []growthFile{text(namespace+".workspace", "/p")}
			return append(files, manyFiles(n, func(i int) string { return namespace + numbered("s", i) + ".jsonl" },
				func(i int) string { return string(nanobotSession(nanobotMetadata(numbered("cli:", i), "m"), 3)) })...)
		}},
		{kind: "session jsonl", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			deep := `{"role":"assistant","content":` + nestedArrays(n, `{"type":"text","text":"x"}`) +
				`,"tool_calls":[` + joined(n, nanobotDamaged) + `]}`
			return one(namespace+".migration-conflicts/s.jsonl", append(lines(n, func(i int) string {
				return `{"role":`
			}), []byte(deep+"\n")...))
		}},
	}
}

// nanobotStore returns a sessions.sqlite3 of the sessions and messages
// rows.
func nanobotStore(t *testing.T, sessions, messages [][]any) []byte {
	t.Helper()
	return database(t, sqlitetest.Table{Name: "sessions",
		SQL: "CREATE TABLE sessions (key TEXT, created_at TEXT, updated_at TEXT, metadata TEXT)", Rows: sessions},
		sqlitetest.Table{Name: "messages", SQL: "CREATE TABLE messages (session_key TEXT, position INTEGER, record TEXT)",
			Rows: messages})
}

// nanobotRecords returns n messages rows of the key.
func nanobotRecords(key string, n int) [][]any {
	return rows(n, func(i int) []any { return []any{key, int64(i), nanobotMessage(i)} })
}

// nanobotDatabaseCases returns the cases of the session database.
func nanobotDatabaseCases() []growthCase {
	namespace := nanobotNamespace(1)
	store := namespace + "sessions.sqlite3"
	return []growthCase{
		{kind: "sessions.sqlite3", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(store, nanobotStore(t, rows(n, func(i int) []any {
				return []any{"cli:direct", "2026-01-01T00:00:00", "2026-01-01T01:00:00", `{"last_channel":"cli"}`}
			}), nanobotRecords("cli:direct", n)))
		}},
		{kind: "sessions.sqlite3", shape: shapeJoins, files: func(t *testing.T, n int) []growthFile {
			metadata := `{"title":"` + strings.Repeat("t", 1024*n) + `"}`
			return one(store, nanobotStore(t, [][]any{{"telegram:42", "2026-01-01T00:00:00", nil, metadata}},
				nanobotRecords("telegram:42", n)))
		}},
		{kind: "sessions.sqlite3", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{text(namespace+".workspace", "/"+long("p")), {rel: store, data: nanobotStore(t,
				[][]any{{"cli:direct", "2026-01-01T00:00:00", nil, "{}"}}, nanobotRecords("cli:direct", n))}}
		}},
		{kind: "sessions.sqlite3", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(store, nanobotStore(t, [][]any{{"cli:direct", nil, nil, nestedObjects(n, "a", "1")}},
				append(rows(n, func(i int) []any { return []any{"cli:direct", int64(i), "[7]"} }),
					[]any{"cli:direct", int64(n), `{"role":"user","content":` + nestedArrays(n, `"x"`) + `}`})))
		}},
	}
}

// nanobotChatFile returns the name of the session file of a WebUI chat in
// its namespace: the base64url of its session id.
func nanobotChatFile(chat string) string {
	return base64.RawURLEncoding.EncodeToString([]byte("websocket:"+chat)) + ".jsonl"
}

// nanobotWebUIRecord returns WebUI record i of the chat: a prompt or a
// reply, in turn.
func nanobotWebUIRecord(chat string, i int) string {
	event := `"event":"user","text":"` + numbered("ask ", i) + `"`
	if i%2 == 1 {
		event = `"event":"message","text":"ok","tool_events":[{"phase":"start","name":"exec","call_id":"c",` +
			`"arguments":{"command":"ls"}}]`
	}
	return `{` + event + `,"chat_id":"` + chat + `","created_at_ms":` + strconv.Itoa(1767000000000+i) + `}`
}

// nanobotUnlisted returns the n session namespace directories of the
// WebUI case of unlisted directories.
func nanobotUnlisted(n int) []string {
	rels := make([]string, 0, n)
	for i := range n {
		rels = append(rels, strings.TrimSuffix(nanobotNamespace(i), "/"))
	}
	return rels
}

// nanobotWebUICases returns the cases of the WebUI mirror and its sidebar
// index.
func nanobotWebUICases() []growthCase {
	return []growthCase{
		{kind: "WebUI mirror", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			files := make([]growthFile, 0, 3*n)
			for i := range n {
				chat := numbered("chat", i)
				files = append(files, text(nanobotNamespace(i)+nanobotChatFile(chat), nanobotMetadata("websocket:"+chat, "m")),
					text(nanobotNamespace(i)+".workspace", numbered("/p", i)),
					text(".nanobot/webui/websocket_"+chat+".jsonl", nanobotWebUIRecord(chat, 0)+"\n"))
			}
			return files
		}},
		{kind: "WebUI mirror", shape: shapeUnlisted + " (session namespaces)", n: 16,
			files: func(t *testing.T, n int) []growthFile {
				files := make([]growthFile, 0, 2*n)
				for i := range n {
					chat := numbered("chat", i)
					files = append(files, text(nanobotNamespace(i)+".workspace", numbered("/p", i)),
						text(".nanobot/webui/websocket_"+chat+".jsonl", nanobotWebUIRecord(chat, 0)+"\n"))
				}
				return files
			}, unlisted: func(n int) []string {
				return nanobotUnlisted(n)
			}},
		{kind: "WebUI mirror", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			namespace := nanobotNamespace(1)
			return []growthFile{text(namespace+nanobotChatFile("c"), nanobotMetadata("websocket:c", "m")),
				text(namespace+".workspace", "/"+long("p")),
				{rel: ".nanobot/webui/websocket_c.segments/000001.jsonl", data: lines(n, func(i int) string {
					return nanobotWebUIRecord("c", i)
				})}}
		}},
		{kind: "WebUI mirror", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".nanobot/webui/websocket_c.jsonl", append(lines(n, func(i int) string {
				return `{"event":"message","chat_id":"c","tool_events":[7]}`
			}), []byte(`{"event":"file_edit","chat_id":"c","edits":`+nestedArrays(n, "{}")+"}\n")...))
		}},
		{kind: "sidebar index", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(nanobotNamespace(1)+".webui_session_index.json", []byte(`{"version":"`+long("v")+`","sessions":[`+
				joined(n, func(i int) string {
					return `{"key":"websocket:` + numbered("c", i) + `","title":"t","created_at":1767000000}`
				})+"]}"))
		}},
		{kind: "sidebar index", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(nanobotNamespace(1)+".webui_session_index.json", []byte(`{"version":1,"sessions":[`+
				joined(n, nanobotDamaged)+`,{"key":"k","title":`+nestedArrays(n, `"x"`)+`}]}`))
		}},
	}
}

// nanobotFileCases returns the cases of the memory and CLI histories and
// the run records.
func nanobotFileCases() []growthCase {
	return []growthCase{
		{kind: "memory history.jsonl", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".nanobot/workspace/memory/history.jsonl", lines(n, func(i int) string {
				return `{"cursor":1,"session_key":"cli:direct","content":"note","timestamp":"2026-01-01T00:00:00"}`
			}))
		}},
		{kind: "memory history.jsonl", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".nanobot/memory/history.jsonl", append(lines(n, func(i int) string { return "{" }),
				[]byte(`{"cursor":1,"content":`+nestedArrays(n, `"x"`)+"}\n")...))
		}},
		{kind: "cli_history", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".nanobot/history/cli_history", append([]byte("\n# 2026-01-01 00:00:00.000000\n"),
				lines(n, func(i int) string { return "+" + numbered(strings.Repeat("l", 1000), i) })...))
		}},
		{kind: "cli_history", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".nanobot/history/cli_history", lines(n, func(i int) string {
				return "# 2026-01-01 00:00:00.000000\n+" + numbered("ask ", i) + "\ndamaged"
			}))
		}},
		{kind: "run record", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string {
				return ".nanobot/workspace/" + []string{"cron", "triggers"}[i%2] + "/runs/" + numbered("r", i) + ".json"
			}, func(i int) string {
				return `{"job_name":"j","job_id":"1","run_id":"cron:1:1767000000:x","status":"ok",` +
					`"updated_at_ms":1767000000000,"rendered_prompt":"p","response":"r","trigger_name":"t",` +
					`"content":"c","session_key":"cli:direct"}`
			})
		}},
		{kind: "run record", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".nanobot/cron/runs/r.json", []byte(`{"job_name":`+nestedArrays(n, `"j"`)+
				`,"prompt_vars":`+nestedObjects(n, "message", `"m"`)+`,"status":"ok","error":[`+joined(n, nanobotDamaged)+`]}`))
		}},
	}
}

// nanobotSettingCases returns the cases of config.json and pairing.json.
func nanobotSettingCases() []growthCase {
	return []growthCase{
		{kind: "config.json", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".nanobot/config.json", []byte(`{"channels":{"`+long("channel")+`":{"allowFrom":[`+
				joined(n, func(i int) string { return `"` + numbered("u", i) + `"` })+`]}}}`))
		}},
		{kind: "config.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".nanobot/config.json", []byte(`{"tools":{"exec":{"allowPatterns":`+nestedArrays(n, `"x"`)+
				`},"mcpServers":{`+joined(n, func(i int) string { return `"` + numbered("s", i) + `":` + nanobotDamaged(i) })+
				`}},"providers":{"openai":{"apiKey":"k"}}}`))
		}},
		{kind: "pairing.json", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".nanobot/pairing.json", []byte(`{"approved":{"`+long("channel")+`":[`+joined(n, func(i int) string {
				return `"` + numbered("u", i) + `"`
			})+`]}}`))
		}},
		{kind: "pairing.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".nanobot/pairing.json", []byte(`{"approved":{"slack":`+nestedArrays(n, `"u"`)+`},"pending":{`+
				joined(n, func(i int) string { return `"` + numbered("p", i) + `":` + nanobotDamaged(i) })+`}}`))
		}},
	}
}

// nanobotDamaged returns damaged record i: a string of a kilobyte where an object
// belongs, long enough that the work of reading it outweighs the buffers a
// parse reuses from one run to the next.
func nanobotDamaged(i int) string {
	return `"` + numbered(strings.Repeat("x", 1000), i) + `"`
}
