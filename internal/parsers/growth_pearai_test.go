package parsers

import (
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The PearAI growth cases: the Continue-style sessions with their index
// and dev data, the Roo-style tasks with the editor's global state, and
// the output log.

// The files the cases write.
const (
	pearaiStorage  = ".config/PearAI/User/globalStorage/"
	pearaiState    = pearaiStorage + "state.vscdb"
	pearaiTasks    = pearaiStorage + "pearai.pearai-roo-cline/tasks/"
	pearaiSessions = ".pearai/sessions/"
	pearaiLog      = ".config/PearAI/logs/20260101T000000/window1/exthost/output_logging_20260101T000000/1-PearAI.log"
)

// pearaiDatabase returns a state.vscdb whose extension item holds value.
func pearaiDatabase(t *testing.T, value string) []byte {
	t.Helper()
	return database(t, sqlitetest.Table{Name: "ItemTable", SQL: "CREATE TABLE ItemTable (key TEXT, value BLOB)",
		Rows: [][]any{{"pearai.pearai-roo-cline", value}}})
}

// pearaiHistoryItem returns a taskHistory entry of the task in the
// workspace.
func pearaiHistoryItem(taskID, workspace string) string {
	return `{"id":"` + taskID + `","ts":1767000000000,"task":"do it","workspace":"` + workspace +
		`","modelId":"m","tokensIn":1,"tokensOut":2}`
}

// pearaiUIMessage returns ui_messages.json entry i: the task, a tool ask,
// or a reply, in turn.
func pearaiUIMessage(i int) string {
	stamp := `"ts":` + strconv.Itoa(1767000000000+i)
	switch i % 3 {
	case 0:
		return `{` + stamp + `,"type":"say","say":"text","text":"` + numbered("ask ", i) + `"}`
	case 1:
		return `{` + stamp + `,"type":"ask","ask":"tool","text":"{\"tool\":\"readFile\",\"path\":\"a.go\"}"}`
	}
	return `{` + stamp + `,"type":"say","say":"text","text":"ok"}`
}

// pearaiSession returns a session file of the id in the workspace holding
// n history items: a prompt and a reply, in turn.
func pearaiSession(identifier, workspace string, n int) string {
	return `{"sessionId":"` + identifier + `","title":"t","workspaceDirectory":"` + workspace + `","history":[` +
		joined(n, func(i int) string {
			if i%2 == 0 {
				return `{"message":{"role":"user","content":"` + numbered("ask ", i) + `"},` +
					`"contextItems":[{"name":"a.go"}]}`
			}
			return `{"message":{"role":"assistant","content":"ok"},"promptLogs":[{"completionOptions":{"model":"m"}}]}`
		}) + "]}"
}

// pearaiGrowth returns the pearai growth set.
func pearaiGrowth() growthSet {
	var cases []growthCase
	cases = append(cases, pearaiSessionCases()...)
	cases = append(cases, pearaiTaskCases()...)
	cases = append(cases, pearaiAPICases()...)
	cases = append(cases, pearaiLogCases()...)
	return growthSet{parser: "pearai", cases: cases}
}

// pearaiSessionCases returns the cases of the sessions, their index and
// the dev data.
func pearaiSessionCases() []growthCase {
	return []growthCase{
		{kind: "session", shape: shapeCarried + " by the workspace", files: func(t *testing.T, n int) []growthFile {
			return one(pearaiSessions+"s.json", []byte(pearaiSession("s", "/"+long("w"), n)))
		}},
		{kind: "session", shape: shapeCarried + " by the session id", files: func(t *testing.T, n int) []growthFile {
			return one(pearaiSessions+"s.json", []byte(pearaiSession(long("s"), "/w", n)))
		}},
		{kind: "session", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			files := []growthFile{text(pearaiSessions+"sessions.json", "["+joined(n, func(i int) string {
				return `{"sessionId":"` + numbered("s", i) + `","title":"t","dateCreated":"1767000000000",` +
					`"workspaceDirectory":"/w","integrationType":"chat"}`
			})+"]")}
			return append(files, manyFiles(n, func(i int) string { return pearaiSessions + numbered("s", i) + ".json" },
				func(i int) string { return `{"sessionId":"` + numbered("s", i) + `","history":[]}` })...)
		}},
		{kind: "session", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(pearaiSessions+"s.json", []byte(`{"sessionId":"s","history":[`+joined(n, func(i int) string {
				return `{"message":{"role":"user","content":"x"},"promptLogs":7}`
			})+`,{"message":{"role":"user","content":`+nestedArrays(n, `{"type":"text","text":"x"}`)+`}}]}`))
		}},
		{kind: "dev_data chat.jsonl", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{text(pearaiSessions+"sessions.json", `[{"sessionId":"s","workspaceDirectory":"/`+
				long("w")+`"}]`), {rel: ".pearai/dev_data/chat.jsonl", data: lines(n, func(i int) string {
				return `{"sessionId":"s","prompt":"<user>\nhi\n","completion":"ok","completionOptions":{"model":"m"}}`
			})}}
		}},
		{kind: "dev_data chat.jsonl", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".pearai/dev_data/chat.jsonl", append(lines(n, func(i int) string { return `{"prompt":` }),
				[]byte(`{"prompt":"x","completion":`+nestedArrays(n, `"y"`)+`,"feedback":true}`+"\n")...))
		}},
		{kind: "dev_data tokens_generated.jsonl", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".pearai/dev_data/tokens_generated.jsonl", lines(n, func(i int) string {
				return `{"model":"m","provider":"p","promptTokens":1,"generatedTokens":2}`
			}))
		}},
		{kind: "dev_data tokens_generated.jsonl", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".pearai/dev_data/tokens_generated.jsonl", append(lines(n, func(i int) string { return "[" }),
				[]byte(`{"model":`+nestedArrays(n, `"m"`)+"}\n")...))
		}},
	}
}

// pearaiTaskCases returns the cases of the task UI messages, the task
// metadata and the global state they look up.
func pearaiTaskCases() []growthCase {
	return []growthCase{
		{kind: "task ui_messages.json", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{{rel: pearaiState, data: pearaiDatabase(t, `{"taskHistory":[`+
				pearaiHistoryItem("t1", "/"+long("w"))+"]}")},
				text(pearaiTasks+"t1/ui_messages.json", "["+joined(n, pearaiUIMessage)+"]")}
		}},
		{kind: "task ui_messages.json", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			files := []growthFile{{rel: pearaiState, data: pearaiDatabase(t, `{"taskHistory":[`+joined(n, func(i int) string {
				return pearaiHistoryItem(numbered("t", i), "/w")
			})+"]}")}}
			return append(files, manyFiles(n, func(i int) string { return pearaiTasks + numbered("t", i) + "/ui_messages.json" },
				func(i int) string { return "[" + joined(3, pearaiUIMessage) + "]" })...)
		}},
		{kind: "task ui_messages.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(pearaiTasks+"t1/ui_messages.json", []byte("["+joined(n, pearaiDamaged)+
				`,{"ts":1,"type":"say","say":"text","text":`+nestedArrays(n, `"x"`)+`}]`))
		}},
		{kind: "task task_metadata.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(pearaiTasks+"t1/task_metadata.json", []byte(`{"model_usage":[`+joined(n, func(i int) string {
				return `{"ts":1767000000000,"model_id":"` + []string{"a", "b"}[i%2] + `","mode":"code"}`
			})+`],"files_in_context":[`+joined(n, func(i int) string {
				return `{"path":"a.go","record_state":"active","record_source":"read_tool","cline_read_date":1767000000000}`
			})+"]}"))
		}},
		{kind: "task task_metadata.json", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{{rel: pearaiState, data: pearaiDatabase(t, `{"taskHistory":[`+
				pearaiHistoryItem("t1", "/"+long("w"))+"]}")},
				text(pearaiTasks+"t1/task_metadata.json", `{"environment_history":[`+joined(n, func(i int) string {
					return `{"ts":` + strconv.Itoa(1767000000000+i) + `,"os_name":"linux"}`
				})+"]}")}
		}},
		{kind: "state.vscdb", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(pearaiState, pearaiDatabase(t, `{"taskHistory":[`+joined(n, func(i int) string {
				return pearaiHistoryItem("t1", "/w")
			})+`],"allowedCommands":["ls"],"alwaysAllowWrite":true}`))
		}},
		{kind: "state.vscdb", shape: shapeSharedID + " and the task's many messages", files: func(t *testing.T,
			n int) []growthFile {
			return []growthFile{{rel: pearaiState, data: pearaiDatabase(t, `{"taskHistory":[`+joined(n, func(i int) string {
				return pearaiHistoryItem("t1", numbered("/w", i))
			})+"]}")}, text(pearaiTasks+"t1/ui_messages.json", "["+joined(n, pearaiUIMessage)+"]")}
		}},
		{kind: "state.vscdb", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(pearaiState, pearaiDatabase(t, `{"taskHistory":[`+joined(n, pearaiDamaged)+
				`,{"id":"t","task":`+nestedArrays(n, `"x"`)+`,"creatorModeConfig":`+nestedObjects(n, "a", "1")+
				`}],"allowedCommands":`+nestedArrays(n, `"ls"`)+"}"))
		}},
	}
}

// pearaiAPICases returns the cases of the API histories, alone and beside
// the UI messages whose asks their permission rows look up.
func pearaiAPICases() []growthCase {
	return []growthCase{
		{kind: "task api_conversation_history.json", shape: shapeJoins, files: func(t *testing.T, n int) []growthFile {
			usage := `{"model_usage":[` + joined(n, func(i int) string {
				return `{"ts":` + strconv.Itoa(1767000000000+2*i) + `,"model_id":"` + numbered("m", i) + `"}`
			}) + "]}"
			return []growthFile{text(pearaiTasks+"t1/task_metadata.json", usage),
				text(pearaiTasks+"t1/api_conversation_history.json", "["+joined(n, func(i int) string {
					return `{"role":"assistant","ts":` + strconv.Itoa(1767000000001+2*i) +
						`,"content":[{"type":"text","text":"ok <read_file><path>a.go</path></read_file>"}]}`
				})+"]")}
		}},
		{kind: "task api_conversation_history.json", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{text(pearaiTasks+"t1/ui_messages.json", "["+joined(n, pearaiUIMessage)+"]"),
				text(pearaiTasks+"t1/api_conversation_history.json", "["+joined(n, func(i int) string {
					return `{"role":"user","ts":` + strconv.Itoa(1767000000000+i) + `,"content":[{"type":"text",` +
						`"text":"[read_file for 'a.go'] Result:"},{"type":"text","text":"The user denied this operation."}]}`
				})+"]")}
		}},
		{kind: "task claude_messages.json", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return pearaiTasks + numbered("t", i) + "/claude_messages.json" },
				func(i int) string {
					return `[{"role":"user","content":"hi"},{"role":"assistant","content":[{"type":"text","text":"ok"}]}]`
				})
		}},
		{kind: "task claude_messages.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(pearaiTasks+"t1/claude_messages.json", []byte("["+joined(n, pearaiDamaged)+
				`,{"role":"user","content":`+nestedArrays(n, `{"type":"text","text":"x"}`)+`}]`))
		}},
	}
}

// pearaiLogRecord returns one prompt write and its completion of the
// output log, the settings naming the model.
func pearaiLogRecord(model string, i int) string {
	separator := strings.Repeat("=", 74) + "\n" + strings.Repeat("=", 74) + "\n"
	return "\n" + separator + "Settings:\nmodel: " + model + "\n\n" + strings.Repeat("#", 44) + "\n\n<user>\n" +
		numbered("ask ", i) + "\n" + "\n" + separator + "Completion:\nok\n"
}

// pearaiLogCases returns the cases of the output log.
func pearaiLogCases() []growthCase {
	return []growthCase{
		{kind: "log", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			first := pearaiLogRecord(long("model"), 0)
			separator := strings.Repeat("=", 74) + "\n" + strings.Repeat("=", 74) + "\n"
			return one(pearaiLog, []byte(first+strings.Repeat("\n"+separator+"Completion:\nok\n", n)))
		}},
		{kind: "log", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			separator := strings.Repeat("=", 74) + "\n" + strings.Repeat("=", 74) + "\n"
			return one(pearaiLog, []byte("preamble\n"+strings.Repeat("\n"+separator+"Other:\nx\n", n)+
				strings.Repeat(pearaiLogRecord("m", 0), n/4)))
		}},
	}
}

// pearaiDamaged returns damaged record i: a string of a kilobyte where an object
// belongs, long enough that the work of reading it outweighs the buffers a
// parse reuses from one run to the next.
func pearaiDamaged(i int) string {
	return `"` + numbered(strings.Repeat("x", 1000), i) + `"`
}
