package growthtests

import (
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The Roo Code growth cases: the task files and the index entries they
// look up, the settings and state files, the editors' state databases and
// the CLI's history.

// The extension's storage root in VS Code, and the one task most cases
// write.
const (
	rooRoot = ".config/Code/User/globalStorage/rooveterinaryinc.roo-cline/"
	rooTask = rooRoot + "tasks/t1/"
)

// rooSay returns a ui_messages.json say message of the given kind and
// text at a millisecond time.
func rooSay(i int, say, text string) string {
	return `{"ts":` + strconv.Itoa(1767000000000+i) + `,"type":"say","say":"` + say + `","text":"` + text + `"}`
}

// rooToolAsk returns a ui_messages.json tool ask the user answered, at a
// millisecond time.
func rooToolAsk(i int) string {
	return `{"ts":` + strconv.Itoa(1767000000000+i) + `,"type":"ask","ask":"tool",` +
		`"text":"{\"tool\":\"readFile\",\"path\":\"a.py\"}","isAnswered":true}`
}

// rooUI returns a ui_messages.json array of the messages.
func rooUI(messages string) []byte {
	return []byte("[" + messages + "]")
}

// rooDenials returns an API history: one tool call of the given name and
// id, then n results of the call, each a denial with feedback.
func rooDenials(n int, name, identifier string) []byte {
	call := `{"role":"assistant","content":[{"type":"tool_use","id":"` + identifier + `","name":"` + name +
		`","input":{}}]}`
	return []byte("[" + call + "," + joined(n, func(i int) string {
		return `{"role":"user","ts":` + strconv.Itoa(1767000000000+i) + `,"content":[{"type":"tool_result",` +
			`"tool_use_id":"` + identifier + `","content":"{\"status\":\"denied\",\"feedback\":\"no ` +
			strconv.Itoa(i) + `\"}"}]}`
	}) + "]")
}

// rooHistoryItem returns a history_item.json of the task id in a
// workspace.
func rooHistoryItem(identifier, workspace string) string {
	return `{"id":"` + identifier + `","ts":1767000000000,"task":"fix the bug","workspace":"` + workspace +
		`","mode":"code","tokensIn":10,"tokensOut":20,"totalCost":0.01}`
}

// rooRetryRequest returns a queued Roo Code Cloud upload naming a task.
func rooRetryRequest(i int, task string) string {
	return `{"url":"https://app.roocode.com/api/events","timestamp":` + strconv.Itoa(1767000000000+i) +
		`,"retryCount":1,"type":"telemetry","options":{"headers":{"Authorization":"Bearer tok` + strconv.Itoa(i) +
		`"},"body":"{\"type\":\"Task Created\",\"properties\":{\"taskId\":\"` + task + `\"}}"}}`
}

// rooItemTable returns an editor's state.vscdb whose ItemTable holds the
// rows, each a key and its JSON value.
func rooItemTable(t *testing.T, items [][]any) []byte {
	return database(t, sqlitetest.Table{Name: "ItemTable",
		SQL: "CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB)", Rows: items})
}

// roocodeGrowth returns the roo-code growth set.
func roocodeGrowth() growthSet {
	var cases []growthCase
	for _, part := range [][]growthCase{rooUICases(), rooAPICases(), rooMetadataCases(), rooIndexCases(),
		rooSettingsCases(), rooStateCases()} {
		cases = append(cases, part...)
	}
	return growthSet{parser: "roo-code", cases: cases}
}

// rooUICases returns the cases of ui_messages.json: a project path and a
// model each task file inherits, many tasks looking up one index, and
// damaged messages.
func rooUICases() []growthCase {
	return []growthCase{
		{kind: "task ui_messages.json", shape: shapeCarried + " (the history item's workspace)",
			files: func(t *testing.T, n int) []growthFile {
				return []growthFile{text(rooTask+"history_item.json", rooHistoryItem("t1", "/"+long("w"))),
					{rel: rooTask + "ui_messages.json", data: rooUI(joined(n, func(i int) string {
						return rooSay(i, "text", "hi")
					}))}}
			}},
		{kind: "task ui_messages.json", shape: shapeCarried + " (the first message's model)",
			files: func(t *testing.T, n int) []growthFile {
				first := `{"ts":1767000000000,"type":"say","say":"api_req_started","text":"{}",` +
					`"modelInfo":{"modelId":"` + long("m") + `"}}`
				return one(rooTask+"ui_messages.json", rooUI(first+","+joined(n, func(i int) string {
					return rooSay(i+1, "text", "hi")
				})))
			}},
		{kind: "task ui_messages.json", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			index := `{"entries":[` + joined(n, func(i int) string {
				return `{"id":"` + numbered("t", i) + `","ts":1767000000000,"task":"x","workspace":"/w"}`
			}) + `]}`
			return append(manyFiles(n, func(i int) string { return rooRoot + "tasks/" + numbered("t", i) + "/ui_messages.json" },
				func(i int) string { return "[" + rooSay(i, "text", "hi") + "," + rooToolAsk(i+1) + "]" }),
				text(rooRoot+"tasks/_index.json", index))
		}},
		{kind: "task ui_messages.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(rooTask+"ui_messages.json", rooUI(joined(n, func(i int) string { return rooToolAsk(0) })))
		}},
		{kind: "task ui_messages.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(rooTask+"ui_messages.json", rooUI(joined(n, func(i int) string { return strconv.Itoa(i) })+
				`,{"ts":1767000000000,"type":"say","say":"text","text":"x","images":`+nestedArrays(n, `"x"`)+`}`))
		}},
	}
}

// rooAPICases returns the cases of the API history under both its names:
// many decisions naming one long tool call, alone and beside
// ui_messages.json, and calls sharing one id.
func rooAPICases() []growthCase {
	var cases []growthCase
	for _, name := range []string{"api_conversation_history.json", "claude_messages.json"} {
		kind := "task " + name
		cases = append(cases,
			growthCase{kind: kind, shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
				return one(rooTask+name, rooDenials(n, long("tool"), "call"))
			}},
			growthCase{kind: kind, shape: shapeLargeShared + " beside ui_messages.json",
				files: func(t *testing.T, n int) []growthFile {
					return []growthFile{{rel: rooTask + name, data: rooDenials(n, long("tool"), "call")},
						{rel: rooTask + "ui_messages.json", data: rooUI(rooSay(0, "text", "hi"))}}
				}},
			growthCase{kind: kind, shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
				calls := joined(n, func(i int) string {
					return `{"role":"assistant","content":[{"type":"tool_use","id":"same","name":"bash","input":{}}]}`
				})
				return one(rooTask+name, append([]byte("["+calls+","), rooDenials(n, "bash", "same")[1:]...))
			}},
			growthCase{kind: kind, shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
				return one(rooTask+name, []byte("["+joined(n, func(i int) string { return `"bad"` })+
					`,{"role":"user","content":[{"type":"text","text":"x","extra":`+nestedArrays(n, "1")+`}]}]`))
			}})
	}
	return cases
}

// rooMetadataCases returns the cases of task_metadata.json: a model every
// ui message inherits, file events sharing one path and time, and
// damaged entries.
func rooMetadataCases() []growthCase {
	return []growthCase{
		{kind: "task task_metadata.json", shape: shapeCarried + " (a model the ui messages inherit)",
			files: func(t *testing.T, n int) []growthFile {
				return []growthFile{text(rooTask+"task_metadata.json", `{"model_usage":[{"ts":1767000000000,`+
					`"model_id":"`+long("m")+`","mode":"code"}]}`),
					{rel: rooTask + "ui_messages.json", data: rooUI(joined(n, func(i int) string {
						return rooSay(i+1, "text", "hi")
					}))}}
			}},
		{kind: "task task_metadata.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			files := joined(n, func(i int) string {
				return `{"path":"a.py","record_source":"read_tool","record_state":"active","cline_read_date":1767000000000}`
			})
			usage := joined(n, func(i int) string {
				return `{"ts":1767000000000,"model_id":"m` + strconv.Itoa(i%2) + `","mode":"code"}`
			})
			return one(rooTask+"task_metadata.json", []byte(`{"files_in_context":[`+files+`],"model_usage":[`+usage+`]}`))
		}},
		{kind: "task task_metadata.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(rooTask+"task_metadata.json", []byte(`{"files_in_context":[`+joined(n, strconv.Itoa)+
				`],"environment_history":[{"ts":1,"os_name":`+nestedArrays(n, `"x"`)+`}],"model_usage":7}`))
		}},
	}
}

// rooIndexCases returns the cases of history_item.json and
// tasks/_index.json: many tasks beside one index, and index entries
// sharing one id.
func rooIndexCases() []growthCase {
	taskFiles := func(n int) []growthFile {
		index := `{"entries":[` + joined(2*n, func(i int) string {
			return `{"id":"` + numbered("t", i) + `","ts":1767000000000,"task":"x","workspace":"/w","mode":"code"}`
		}) + `]}`
		files := manyFiles(n, func(i int) string { return rooRoot + "tasks/" + numbered("t", i) + "/history_item.json" },
			func(i int) string { return rooHistoryItem(numbered("t", i), "/w") })
		return append(files, text(rooRoot+"tasks/_index.json", index))
	}
	return []growthCase{
		{kind: "history_item.json", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			return taskFiles(n)
		}},
		{kind: "history_item.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(rooTask+"history_item.json", []byte(`{"id":"t1","ts":1,"task":`+nestedArrays(n, `"x"`)+
				`,"tokensIn":[`+joined(n, func(i int) string { return `{"bad":1}` })+`]}`))
		}},
		{kind: "tasks _index.json", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			return taskFiles(n)
		}},
		{kind: "tasks _index.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(rooRoot+"tasks/_index.json", []byte(`{"entries":[`+joined(n, func(i int) string {
				return `{"id":"same","ts":1767000000000,"task":"x","workspace":"/w","mode":"code"}`
			})+`]}`))
		}},
		{kind: "tasks _index.json", shape: shapeSharedID + " and the task's many records", files: func(t *testing.T,
			n int) []growthFile {
			return []growthFile{text(rooRoot+"tasks/_index.json", `{"entries":[`+joined(n, func(i int) string {
				return `{"id":"same","ts":` + strconv.Itoa(1767000000000+i) + `,"task":"x","workspace":"/w` +
					strconv.Itoa(i) + `","mode":"code"}`
			})+`]}`), {rel: rooRoot + "tasks/same/ui_messages.json",
				data: rooUI(joined(n, func(i int) string { return rooSay(i, "text", "hi") }))}}
		}},
		{kind: "tasks _index.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(rooRoot+"tasks/_index.json", []byte(`{"entries":[`+joined(n, strconv.Itoa)+
				`,{"id":"deep","task":`+nestedArrays(n, `"x"`)+`}]}`))
		}},
	}
}

// rooMCP returns an MCP settings file of one server, its name given,
// allowing n tools without asking.
func rooMCP(n int, server string) []byte {
	return []byte(`{"mcpServers":{"` + server + `":{"command":"npx","alwaysAllow":[` +
		joined(n, func(i int) string { return `"` + numbered("tool", i) + `"` }) + `]}}}`)
}

// rooSettingsCases returns the cases of the MCP settings, the CLI's
// settings and history and the CLI's global state: a long server name
// each tool repeats, keys given many times, and damaged entries.
func rooSettingsCases() []growthCase {
	return []growthCase{
		{kind: "mcp_settings.json", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(rooRoot+"settings/mcp_settings.json", rooMCP(n, long("server")))
		}},
		{kind: "project mcp.json", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".roo/mcp.json", rooMCP(n, long("server")))
		}},
		{kind: "project mcp.json", shape: shapeSharedID, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(".roo/mcp.json", []byte(`{"mcpServers":{`+joined(n, func(i int) string {
				return `"same":{"alwaysAllow":["` + numbered("tool", i) + `"]}`
			})+`}}`))
		}},
		{kind: "cli-settings.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".roo/cli-settings.json", []byte(`{"requireApproval":[`+joined(n, func(i int) string {
				return `{"bad":` + strconv.Itoa(i) + `}`
			})+`],"mode":`+nestedArrays(n, `"code"`)+`,"dangerouslySkipPermissions":true}`))
		}},
		{kind: "cli-history.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".roo/cli-history.json", []byte(`{"entries":[`+joined(n, func(i int) string {
				return `"the same prompt"`
			})+`]}`))
		}},
		{kind: "cli-history.json", shape: shapeNested + " (a file cut mid-write)",
			files: func(t *testing.T, n int) []growthFile {
				return one(".roo/cli-history.json", []byte(`{"entries":[`+joined(n, func(i int) string {
					return `"` + numbered("prompt ", i) + `",7`
				})+`,`+nestedArrays(n, `"x"`)+`,{"bad": `))
			}},
		{kind: "global-state.json", shape: shapeSharedID, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(".vscode-mock/global-storage/global-state.json", []byte(`{`+joined(n, func(i int) string {
				return `"allowedCommands":["` + numbered("cmd", i) + `"]`
			})+`}`))
		}},
		{kind: "global-state.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".vscode-mock/global-storage/global-state.json", []byte(`{"deniedCommands":[`+
				joined(n, func(i int) string { return `{"bad":` + strconv.Itoa(i) + `}` })+`],"allowedCommands":`+
				nestedArrays(n, `"x"`)+`,"organization-settings":{"cloudSettings":{"recordTaskMessages":true}}}`))
		}},
	}
}

// rooStateCases returns the cases of the workspace state's retry queue,
// in workspace-state.json and in an editor's workspace state.vscdb, and of
// the extension's global state in an editor's state.vscdb.
func rooStateCases() []growthCase {
	queue := func(n int, task string) string {
		return `{"roo.retryQueue":[` + joined(n, func(i int) string { return rooRetryRequest(i, task) }) + `]}`
	}
	const workspaceDatabase = ".config/Code/User/workspaceStorage/abc/state.vscdb"
	const globalDatabase = ".config/Code/User/globalStorage/state.vscdb"
	return []growthCase{
		{kind: "workspace-state.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".vscode-mock/workspace-storage/w1/workspace-state.json", []byte(queue(n, "same")))
		}},
		{kind: "workspace-state.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(".vscode-mock/workspace-storage/w1/workspace-state.json", []byte(`{"roo.retryQueue":[`+
				joined(n, strconv.Itoa)+`,{"url":"u","lastError":`+nestedArrays(n, `"x"`)+`}]}`))
		}},
		{kind: "workspace state.vscdb", shape: shapeSharedID, agent: "vscode",
			files: func(t *testing.T, n int) []growthFile {
				return []growthFile{{rel: workspaceDatabase, data: rooItemTable(t, [][]any{
					{"RooVeterinaryInc.roo-cline", []byte(queue(n, "same"))}})}}
			}},
		{kind: "workspace state.vscdb", shape: shapeManyFiles, agent: "vscode", n: 16,
			files: func(t *testing.T, n int) []growthFile {
				files := make([]growthFile, 0, n)
				for i := range n {
					files = append(files, growthFile{rel: ".config/Code/User/workspaceStorage/" + numbered("w", i) +
						"/state.vscdb", data: rooItemTable(t, [][]any{{"RooVeterinaryInc.roo-cline", []byte(queue(1, "t"))}})})
				}
				return files
			}},
		{kind: "state.vscdb", shape: shapeJoins + " (one key on every row)", agent: "vscode", n: 16,
			files: func(t *testing.T, n int) []growthFile {
				return []growthFile{{rel: globalDatabase, data: rooItemTable(t, rows(n, func(i int) []any {
					return []any{"RooVeterinaryInc.roo-cline", `{"allowedCommands":["` + numbered("cmd", i) + `"],` +
						`"mode":"` + strings.Repeat("x", 1024*i/n) + `"}`}
				}))}}
			}},
		{kind: "state.vscdb", shape: shapeNested, agent: "vscode", n: 32, files: func(t *testing.T, n int) []growthFile {
			value := `{"allowedCommands":[` + joined(n, func(i int) string { return `{"bad":1}` }) +
				`],"deniedCommands":` + nestedArrays(n, `"x"`) + `}`
			return []growthFile{{rel: globalDatabase, data: rooItemTable(t, [][]any{
				{"RooVeterinaryInc.roo-cline", value}, {"rooveterinaryinc.roo-cline", "not json"}})}}
		}},
	}
}
