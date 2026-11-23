package growthtests

import (
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The Hermes growth cases: state.db and its snapshots, the session JSONL
// copies, the Responses store and the trajectory files, the prompt history
// and cron output, and the settings files.

// The tables of state.db the parser reads, in the columns it reads.
const (
	hermesSessionsSQL = "CREATE TABLE sessions (id TEXT, source TEXT, model TEXT, started_at REAL, cwd TEXT, " +
		"git_branch TEXT, title TEXT, origin_json TEXT, model_config TEXT, chat_id TEXT)"
	hermesMessagesSQL = "CREATE TABLE messages (id INTEGER, session_id TEXT, role TEXT, content TEXT, " +
		"tool_call_id TEXT, tool_calls TEXT, tool_name TEXT, timestamp REAL, display_kind TEXT)"
	hermesUsageSQL = "CREATE TABLE session_model_usage (session_id TEXT, model TEXT, billing_provider TEXT, " +
		"task TEXT, first_seen REAL)"
)

// hermesSession returns a sessions row of id in the cwd with the model.
func hermesSession(identifier, model, cwd string, started int) []any {
	return []any{identifier, "telegram", model, float64(1767000000 + started), cwd, "main", "title", `{"user_name":"u"}`,
		`{"yolo_mode":true}`, "chat"}
}

// hermesMessage returns a messages row of the session: a message of the
// role holding content, with the tool call id, tool calls and tool name.
func hermesMessage(i int, sessionID, role, content, callID, calls, toolName string) []any {
	return []any{int64(i + 1), sessionID, role, content, callID, calls, toolName, float64(1767000000 + i), nil}
}

// hermesState returns a state.db of the sessions, messages and usage rows
// and of the other tables.
func hermesState(t *testing.T, sessions, messages, usage [][]any, others ...sqlitetest.Table) []byte {
	t.Helper()
	tables := []sqlitetest.Table{{Name: "sessions", SQL: hermesSessionsSQL, Rows: sessions},
		{Name: "messages", SQL: hermesMessagesSQL, Rows: messages},
		{Name: "session_model_usage", SQL: hermesUsageSQL, Rows: usage}}
	return database(t, append(tables, others...)...)
}

// hermesChat returns n messages of session s: a user message and an
// assistant reply calling a tool, then the tool's result, in turn.
func hermesChat(n int) [][]any {
	return rows(n, func(i int) []any {
		switch i % 3 {
		case 0:
			return hermesMessage(i, "s", "user", numbered("ask ", i), "", "", "")
		case 1:
			return hermesMessage(i, "s", "assistant", "ok", "", `[{"id":"`+numbered("c", i)+
				`","function":{"name":"terminal","arguments":"{\"command\":\"ls\"}"}}]`, "")
		}
		return hermesMessage(i, "s", "tool", `{"output":"x","approval":"yes"}`, numbered("c", i-1), "", "")
	})
}

// hermesGrowth returns the hermes growth set.
func hermesGrowth() growthSet {
	var cases []growthCase
	cases = append(cases, hermesStateCases()...)
	cases = append(cases, hermesTableCases()...)
	cases = append(cases, hermesCopyCases()...)
	cases = append(cases, hermesResponseCases()...)
	cases = append(cases, hermesHistoryCases()...)
	cases = append(cases, hermesSettingCases()...)
	return growthSet{parser: "hermes", cases: cases}
}

// hermesStateCases returns the cases of state.db's sessions, messages and
// usage rows.
func hermesStateCases() []growthCase {
	const state = ".hermes/state.db"
	return []growthCase{
		{kind: "state.db", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(state, hermesState(t, rows(n, func(i int) []any { return hermesSession("s", "m", "/p", i) }),
				hermesChat(n), rows(n, func(i int) []any {
					return []any{"s", "model-" + "ab"[i%2:i%2+1], "openai", nil, float64(1767000000 + i)}
				})))
		}},
		{kind: "state.db", shape: shapeCarried + " by a session's project", files: func(t *testing.T, n int) []growthFile {
			return one(state, hermesState(t, [][]any{hermesSession("s", "m", "/"+long("p"), 0)}, hermesChat(n), nil))
		}},
		{kind: "state.db", shape: shapeCarried + " by a session's model", files: func(t *testing.T, n int) []growthFile {
			return one(state, hermesState(t, [][]any{hermesSession("s", long("model"), "/p", 0)}, hermesChat(n), nil))
		}},
		{kind: "state.db", shape: shapeCarried + " by a tool call's name", files: func(t *testing.T, n int) []growthFile {
			call := hermesMessage(0, "s", "assistant", "", "", `[{"id":"c","function":{"name":"`+long("tool")+
				`","arguments":"{}"}}]`, "")
			return one(state, hermesState(t, [][]any{hermesSession("s", "m", "/p", 0)},
				append([][]any{call}, rows(n, func(i int) []any {
					return hermesMessage(i+1, "s", "tool", "done", "c", "", "")
				})...), nil))
		}},
		{kind: "state.db", shape: shapeJoins, files: func(t *testing.T, n int) []growthFile {
			sessions := rows(n, func(i int) []any {
				return hermesSession("s", "m", "/p", i)
			})
			sessions[0][8] = `{"yolo_mode":"` + strings.Repeat("y", 1024*n) + `"}`
			return one(state, hermesState(t, sessions, rows(n, func(i int) []any {
				return []any{int64(i + 1), "s", "user", "[System: The active model for this chat has changed to " +
					numbered("m", i) + ". From this point forward, x", "", "", "", float64(1767000000 + i), "model_switch"}
			}), rows(n, func(i int) []any { return []any{"s", numbered("u", i), nil, nil, float64(1767000000 + i)} })))
		}},
		{kind: "state.db", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			deep := hermesMessage(0, "s", "assistant", "\x00json:"+nestedArrays(n, `{"type":"text","text":"x"}`), "",
				nestedArrays(n, "{}"), "")
			return one(state, hermesState(t, [][]any{hermesSession("s", "m", "/p", 0)},
				append([][]any{deep}, rows(n, func(i int) []any {
					return hermesMessage(i+1, "s", "assistant", "x", "", `[{"id":`, "")
				})...), nil))
		}},
	}
}

// hermesTableCases returns the cases of state.db's deletion fences, relay
// peers and room events, and of its snapshots.
func hermesTableCases() []growthCase {
	const state = ".hermes/state.db"
	return []growthCase{
		{kind: "state.db", shape: shapeLargeShared + " by deletion fences", files: func(t *testing.T, n int) []growthFile {
			return one(state, hermesState(t, [][]any{hermesSession("s", "m", "/"+long("p"), 0)}, nil, nil,
				sqlitetest.Table{Name: "state_meta", SQL: "CREATE TABLE state_meta (key TEXT, value TEXT)",
					Rows: rows(n, func(i int) []any {
						return []any{"gateway.retired_session.v1.s", `{"retired_at":` + strconv.Itoa(1767000000+i) + `}`}
					})}))
		}},
		{kind: "state.db", shape: shapeLargeShared + " by room events", files: func(t *testing.T, n int) []growthFile {
			return one(state, hermesState(t, nil, nil, nil,
				sqlitetest.Table{Name: "hosted_rooms", SQL: "CREATE TABLE hosted_rooms (room_id TEXT, name TEXT)",
					Rows: [][]any{{"r", long("room")}}},
				sqlitetest.Table{Name: "hosted_room_events", SQL: "CREATE TABLE hosted_room_events (room_id TEXT, " +
					"seq INTEGER, event_id TEXT, kind TEXT, actor_json TEXT, payload_json TEXT, created_at REAL)",
					Rows: rows(n, func(i int) []any {
						return []any{"r", int64(i), numbered("e", i), "message.user", `{"kind":"user","id":"u"}`,
							`{"text":"hi"}`, float64(1767000000 + i)}
					})},
				sqlitetest.Table{Name: "conversation_generations", SQL: "CREATE TABLE conversation_generations " +
					"(source TEXT, session_key TEXT, generation INTEGER)", Rows: rows(n, func(i int) []any {
					return []any{"telegram", "k", int64(i)}
				})}))
		}},
		{kind: "state snapshot", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			snapshot := hermesState(t, [][]any{hermesSession("s", "m", "/p", 0)}, hermesChat(3), nil)
			files := make([]growthFile, 0, n)
			for i := range n {
				files = append(files, growthFile{rel: ".hermes/state-snapshots/20260101-0000" +
					strconv.Itoa(10+i%50) + "-" + strconv.Itoa(i) + "/state.db", data: snapshot})
			}
			return files
		}},
		{kind: "state snapshot", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".hermes/state-snapshots/20260101-000000/state.db",
				hermesState(t, [][]any{hermesSession("s", long("model"), "/"+long("p"), 0)}, hermesChat(n), nil))
		}},
	}
}

// hermesLine returns a session JSONL line: a message of the role holding
// content, with the tool call id and the tool calls.
func hermesLine(role, content, callID, calls string) string {
	line := `{"role":"` + role + `","content":"` + content + `","timestamp":1767000000`
	if callID != "" {
		line += `,"tool_call_id":"` + callID + `"`
	}
	if calls != "" {
		line += `,"tool_calls":` + calls
	}
	return line + "}"
}

// hermesCopyCases returns the cases of the session JSONL copies.
func hermesCopyCases() []growthCase {
	const copied = ".hermes/sessions/s.jsonl"
	return []growthCase{
		{kind: "session jsonl", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			first := hermesLine("assistant", "", "", `[{"id":"c","function":{"name":"`+long("tool")+`"}}]`)
			return one(copied, append([]byte(first+"\n"), lines(n, func(i int) string {
				return hermesLine("tool", "done", "c", "")
			})...))
		}},
		{kind: "session jsonl", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(copied, lines(n, func(i int) string {
				return hermesLine("assistant", "x", "", `[{"id":"c","function":{"name":"terminal","arguments":"ls"}}]`)
			}))
		}},
		{kind: "session jsonl", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(copied, append(lines(n, func(i int) string { return `{"role":"user","content":` }),
				[]byte(`{"role":"user","content":`+nestedArrays(n, `"x"`)+`}`+"\n")...))
		}},
		{kind: "session jsonl", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return ".hermes/sessions/" + numbered("s", i) + ".jsonl" },
				func(i int) string { return hermesLine("user", "hi", "", "") + "\n" })
		}},
	}
}

// hermesResponses returns a response_store.db of the responses rows and
// the conversations rows.
func hermesResponses(t *testing.T, responses, conversations [][]any) []byte {
	t.Helper()
	return database(t, sqlitetest.Table{Name: "responses", SQL: "CREATE TABLE responses (response_id TEXT, data TEXT)",
		Rows: responses}, sqlitetest.Table{Name: "conversations",
		SQL: "CREATE TABLE conversations (name TEXT, response_id TEXT)", Rows: conversations})
}

// hermesResponse returns a responses.data document of session s holding
// the history.
func hermesResponse(i int, history string) string {
	return `{"session_id":"s","response":{"status":"completed","model":"m","created_at":` +
		strconv.Itoa(1767000000+i) + `},"conversation_history":[` + history + `]}`
}

// hermesResponseCases returns the cases of the Responses store and the
// trajectory files.
func hermesResponseCases() []growthCase {
	const store = ".hermes/response_store.db"
	return []growthCase{
		{kind: "response_store.db", shape: shapeLargeShared, n: 32, files: func(t *testing.T, n int) []growthFile {
			first := `{"role":"user","content":"` + long("ask") + `"}`
			return one(store, hermesResponses(t, rows(n, func(i int) []any {
				return []any{numbered("r", i), hermesResponse(i, first+`,{"role":"assistant","content":"`+
					numbered("a", i)+`"}`)}
			}), nil))
		}},
		{kind: "response_store.db", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(store, hermesResponses(t, rows(n, func(i int) []any {
				return []any{"r", hermesResponse(i, `{"role":"user","content":"`+numbered("q", i)+`"}`)}
			}), [][]any{{long("name"), "r"}}))
		}},
		{kind: "response_store.db", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(store, hermesResponses(t, append(rows(n, func(i int) []any {
				return []any{numbered("r", i), `{"session_id":`}
			}), []any{"deep", hermesResponse(0, nestedArrays(n, "1")+`,{"role":"user","content":`+
				nestedArrays(n, `"x"`)+`}`)}), nil))
		}},
		{kind: "trajectory samples", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one("trajectory_samples.jsonl", []byte(`{"model":"`+long("model")+`","completed":true,`+
				`"timestamp":"2026-01-01T00:00:00","conversations":[`+joined(n, func(i int) string {
				return `{"from":"human","value":"` + numbered("q", i) + `"}`
			})+"]}\n"))
		}},
		{kind: "trajectory samples", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one("failed_trajectories.jsonl", append(lines(n, func(i int) string { return `{"conversations":7}` }),
				[]byte(`{"model":"m","conversations":[{"from":"gpt","value":`+nestedArrays(n, `"x"`)+`}]}`+"\n")...))
		}},
	}
}

// hermesHistoryCases returns the cases of the prompt history and the cron
// output.
func hermesHistoryCases() []growthCase {
	return []growthCase{
		{kind: ".hermes_history", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".hermes/.hermes_history", append([]byte("\n# 2026-01-01 00:00:00.000000\n"),
				lines(n, func(i int) string { return "+" + numbered(strings.Repeat("l", 1000), i) })...))
		}},
		{kind: ".hermes_history", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".hermes/.hermes_history", lines(n, func(i int) string {
				return "\n# 2026-01-01 00:00:00.000000\n+/" + numbered("cmd ", i) + "\ndamaged"
			}))
		}},
		{kind: "cron output", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string {
				return ".hermes/cron/output/job/2026-01-01_00-00-" + strconv.Itoa(10+i%50) + "-" + strconv.Itoa(i) + ".md"
			}, func(i int) string {
				return "# Cron\n\n**Prompt Characters:** 3\n## Prompt\n\nask\n\n**Response Characters:** 2\n" +
					"## Response\n\nok\n"
			})
		}},
		{kind: "cron output", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			document := strings.Repeat("**Prompt Characters:** 99999\n## Prompt\n\n"+strings.Repeat("p", 1000)+"\n", n) +
				strings.Repeat("**Response Characters:** 9\n## Response\n\n", n)
			return one(".hermes/cron/output/job/2026-01-01_00-00-00.md", []byte(document))
		}},
	}
}

// hermesSettingCases returns the cases of the pending pastes, config.yaml,
// the pairing approvals and the desktop app's connections.
func hermesSettingCases() []growthCase {
	return []growthCase{
		{kind: "pending pastes", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".hermes/pastes/pending.json", []byte("["+joined(n, func(i int) string {
				return `{"url":"https://paste.rs/` + strings.Repeat("a", 1000) + `","expire_at":` + strconv.Itoa(1767000000+i) + `}`
			})+"]"))
		}},
		{kind: "pending pastes", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".hermes/pastes/pending.json", []byte("["+joined(n, func(i int) string { return `{"x":1}` })+
				`,{"url":`+nestedArrays(n, `"u"`)+`}]`))
		}},
		{kind: "config.yaml", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".hermes/config.yaml", append([]byte("model: m\ncommand_allowlist:\n"),
				lines(n, func(i int) string { return "  - 'rm -rf " + strings.Repeat("x", 1000) + "' # same" })...))
		}},
		{kind: "config.yaml", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".hermes/config.yaml", append(append([]byte("command_allowlist:\n"),
				lines(n, func(i int) string { return "  - " + numbered(strings.Repeat("l", 1000), i) })...),
				[]byte("  - "+strings.Repeat("[", n)+"\n")...))
		}},
		{kind: "pairing approvals", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".hermes/pairing/telegram-approved.json", []byte("{"+joined(n, func(i int) string {
				return `"42":{"user_name":"` + strings.Repeat("u", 1000) + `","approved_at":` + strconv.Itoa(1767000000+i) + `}`
			})+"}"))
		}},
		{kind: "pairing approvals", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".hermes/platforms/pairing/slack-approved.json", []byte("{"+joined(n, func(i int) string {
				return `"` + numbered("u", i) + `":` + hermesDamaged(i)
			})+`,"deep":{"user_name":`+nestedArrays(n, `"x"`)+`}}`))
		}},
		{kind: "connections", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".config/Hermes/connections.json", []byte(`{"connections":[`+joined(n, func(i int) string {
				return `{"kind":"ssh","label":"l","host":"h","user":"u","port":22,` +
					`"token":{"encoding":"plain","value":"t"},"headers":{"encoding":"plain","value":"{\"a\":\"b\"}"}}`
			})+`],"quarantined":[`+joined(n, func(i int) string { return `{"reason":"r"}` })+"]}"))
		}},
		{kind: "connections", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".config/Hermes/connections.json", []byte(`{"connections":[`+joined(n, hermesDamaged)+`,{"kind":"ssh","url":`+nestedArrays(n, `"u"`)+`}]}`))
		}},
	}
}

// hermesDamaged returns damaged record i: a string of a kilobyte where an object
// belongs, long enough that the work of reading it outweighs the buffers a
// parse reuses from one run to the next.
func hermesDamaged(i int) string {
	return `"` + numbered(strings.Repeat("x", 1000), i) + `"`
}
