package growthtests

import (
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The Cline growth cases: the legacy task files and their index under the
// extension's storage, the SDK sessions, databases and logs under
// ~/.cline/data, and the standing settings.

// The storage roots and the one task id of a case: a millisecond time, as
// the extension names its tasks.
const (
	clineExtension = ".config/Code/User/globalStorage/saoudrizwan.claude-dev/"
	clineData      = ".cline/data/"
	clineTaskID    = "1791021600000"
	// clineKeptTask marks a task directory's name for the case test, in the
	// cases of Cline and of the agents that fork it.
	clineKeptTask = `tasks/([^/]+)/`
	clineSession  = "sess-1"
)

// clineTaskFile returns the path of a file of the extension's task.
func clineTaskFile(taskID, name string) string {
	return clineExtension + "tasks/" + taskID + "/" + name
}

// clineHistory returns state/taskHistory.json holding one entry of the
// task, with a project path and model.
func clineHistory(taskID, project, model string) growthFile {
	return text(clineExtension+"state/taskHistory.json", `[{"id":"`+taskID+`","ts":1791021607001,"task":"fix tests",`+
		`"tokensIn":1200,"cwdOnTaskInitialization":"`+project+`","modelId":"`+model+`"}]`)
}

// clineSay returns a ui_messages.json "say" message of a kind at a time
// offset, its text given as JSON.
func clineSay(i int, say, textJSON string) string {
	return `{"ts":` + strconv.Itoa(1791021600000+i) + `,"type":"say","say":"` + say + `","text":` + textJSON + `}`
}

// clineAPIMessage returns an API history message of a role holding one
// text block.
func clineAPIMessage(role, words string) string {
	return `{"role":"` + role + `","content":[{"type":"text","text":"` + words + `"}]}`
}

// clineToolUse returns an assistant API message calling a tool by id.
func clineToolUse(toolID string) string {
	return `{"role":"assistant","content":[{"type":"tool_use","id":"` + toolID + `","name":"execute_command",` +
		`"input":{"command":"pytest -q"}}]}`
}

// clineUICases returns the cases of ui_messages.json.
func clineUICases() []growthCase {
	const kind = "task ui_messages.json"
	return []growthCase{
		{kind: kind, shape: shapeCarried + " (the index's project path)", files: func(t *testing.T, n int) []growthFile {
			return []growthFile{clineHistory(clineTaskID, "/"+long("p"), "m"),
				{rel: clineTaskFile(clineTaskID, "ui_messages.json"), data: []byte("[" + joined(n, func(i int) string {
					return clineSay(i, "text", `"hi"`)
				}) + "]")}}
		}},
		{kind: kind, shape: shapeCarried + " (the model of the first message)", files: func(t *testing.T, n int) []growthFile {
			first := `{"ts":1791021600000,"type":"say","say":"text","text":"a","modelInfo":{"modelId":"` + long("m") + `"}}`
			return one(clineTaskFile(clineTaskID, "ui_messages.json"), []byte("["+first+","+joined(n, func(i int) string {
				return clineSay(i+1, "text", `"hi"`)
			})+"]"))
		}},
		{kind: kind, shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(clineTaskFile(clineTaskID, "ui_messages.json"), []byte("["+joined(n, func(i int) string {
				return strconv.Itoa(i)
			})+","+clineSay(0, "text", nestedArrays(n, `"x"`))+","+clineSay(1, "api_req_started",
				`"`+strings.Repeat(`{\"a\":`, n)+`"`)+"]"))
		}},
		{kind: kind, shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			history := text(clineExtension+"state/taskHistory.json", "["+joined(n, func(i int) string {
				return `{"id":"` + strconv.Itoa(1791021600000+i) + `","ts":1791021607001,"task":"t",` +
					`"cwdOnTaskInitialization":"/p` + strconv.Itoa(i) + `","modelId":"m"}`
			})+"]")
			return append(manyFiles(n, func(i int) string {
				return clineTaskFile(strconv.Itoa(1791021600000+i), "ui_messages.json")
			}, func(i int) string { return "[" + clineSay(i, "text", `"hi"`) + "]" }), history)
		}},
	}
}

// clineAPICases returns the cases of the API history, under both of its
// names.
func clineAPICases() []growthCase {
	var cases []growthCase
	for _, name := range []string{"api_conversation_history.json", "claude_messages.json"} {
		rel := clineTaskFile(clineTaskID, name)
		cases = append(cases,
			growthCase{kind: "task " + name, shape: shapeCarried + " (the metadata's model)",
				files: func(t *testing.T, n int) []growthFile {
					metadata := text(clineTaskFile(clineTaskID, "task_metadata.json"),
						`{"model_usage":[{"ts":1791021600000,"model_id":"`+long("m")+`","mode":"act"}]}`)
					return []growthFile{metadata, {rel: rel, data: []byte("[" + joined(n, func(i int) string {
						return clineAPIMessage("user", "hi")
					}) + "]")}}
				}},
			growthCase{kind: "task " + name, shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
				return one(rel, []byte("["+joined(n, func(i int) string { return clineToolUse("toolu_same") })+"]"))
			}},
			growthCase{kind: "task " + name, shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
				deep := `{"role":"user","content":[{"type":"tool_result","tool_use_id":"t","content":` +
					nestedArrays(n, `"x"`) + `}]}`
				return one(rel, []byte("["+joined(n, func(i int) string { return `"bad"` })+","+deep+"]"))
			}})
	}
	return cases
}

// clineMetadataCases returns the cases of task_metadata.json and of the
// task index.
func clineMetadataCases() []growthCase {
	metadata := clineTaskFile(clineTaskID, "task_metadata.json")
	return []growthCase{
		{kind: "task task_metadata.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(metadata, []byte(`{"files_in_context":[`+joined(n, func(i int) string {
				return `{"path":"src/app.py","record_state":"active","record_source":"read_tool",` +
					`"cline_read_date":1791021602000}`
			})+`],"model_usage":[`+joined(n, func(i int) string {
				return `{"ts":` + strconv.Itoa(1791021600000+i) + `,"model_id":"m` + strconv.Itoa(i%2) + `","mode":"act"}`
			})+`]}`))
		}},
		{kind: "task task_metadata.json", shape: shapeCarried + " (the index's project path)",
			files: func(t *testing.T, n int) []growthFile {
				return []growthFile{clineHistory(clineTaskID, "/"+long("p"), "m"), {rel: metadata,
					data: []byte(`{"environment_history":[` + joined(n, func(i int) string {
						return `{"ts":` + strconv.Itoa(1791021600000+i) + `,"os_name":"linux"}`
					}) + `]}`)}}
			}},
		{kind: "task task_metadata.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(metadata, []byte(`{"files_in_context":[`+joined(n, func(i int) string { return "1" })+
				`],"model_usage":[{"ts":1,"model_id":`+nestedArrays(n, `"m"`)+`}],"environment_history":`+
				nestedObjects(n, "a", "1")+`}`))
		}},
		{kind: "taskHistory.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(clineExtension+"state/taskHistory.json", []byte("["+joined(n, func(i int) string {
				return `{"id":"` + clineTaskID + `","ts":1791021607001,"task":"fix tests","modelId":"m"}`
			})+"]"))
		}},
		// A task directory's name is looked up among the ids of the
		// task history.
		{kind: "taskHistory.json", shape: shapeSharedID + " and the task's many records", keptCase: clineKeptTask, files: func(t *testing.T,
			n int) []growthFile {
			return []growthFile{text(clineExtension+"state/taskHistory.json", "["+joined(n, func(i int) string {
				return `{"id":"t1","ts":` + strconv.Itoa(1791021607001+i) + `,"task":"t","cwdOnTaskInitialization":"/p` +
					strconv.Itoa(i) + `","modelId":"m"}`
			})+"]"), text(clineTaskFile("t1", "api_conversation_history.json"), "["+joined(n, func(i int) string {
				return clineAPIMessage("user", "hi")
			})+"]")}
		}},
		{kind: "taskHistory.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(clineExtension+"state/taskHistory.json", []byte("["+joined(n, func(i int) string {
				return `"bad"`
			})+`,{"id":"1","task":`+nestedArrays(n, `"t"`)+`}]`))
		}},
	}
}

// clineManifest returns an SDK session manifest of the session.
func clineManifest(sessionID, cwd, extra string) string {
	return `{"session_id":"` + sessionID + `","started_at":"2026-01-02T03:04:05Z","cwd":"` + cwd + `",` +
		`"model":"claude-sonnet-4-5","provider":"anthropic","status":"done"` + extra + `}`
}

// clineMessages returns an SDK messages file of n user messages.
func clineMessages(n int) []byte {
	return []byte(`{"messages":[` + joined(n, func(i int) string {
		return `{"role":"user","ts":"2026-01-02T03:04:05Z","content":[{"type":"text","text":"hi"}]}`
	}) + `]}`)
}

// clineSessionCases returns the cases of the SDK session manifests and
// messages files.
func clineSessionCases() []growthCase {
	directory := clineData + "sessions/" + clineSession + "/"
	manifest := directory + clineSession + ".json"
	return []growthCase{
		{kind: "session manifest", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string {
				return clineData + "sessions/" + numbered("s", i) + "/" + numbered("s", i) + ".json"
			}, func(i int) string { return clineManifest(numbered("s", i), "/p", `,"ended_at":"2026-01-02T04:00:00Z"`) })
		}},
		{kind: "session manifest", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(manifest, []byte(clineManifest(clineSession, "/p", `,"metadata":{"title":`+
				nestedArrays(n, `"t"`)+`,"importedFrom":`+nestedObjects(n, "tool", "1")+`}`)))
		}},
		{kind: "session messages.json", shape: shapeLargeShared, n: 32, files: func(t *testing.T, n int) []growthFile {
			files := []growthFile{text(manifest, clineManifest(clineSession, "/p", `,"pad":"`+
				strings.Repeat("x", 1000*n)+`"`))}
			return append(files, manyFiles(n, func(i int) string { return directory + numbered("a", i) + ".messages.json" },
				func(i int) string { return string(clineMessages(1)) })...)
		}},
		{kind: "session messages.json", shape: shapeCarried + " (the manifest's working directory)",
			files: func(t *testing.T, n int) []growthFile {
				return []growthFile{text(manifest, clineManifest(clineSession, "/"+long("p"), "")),
					{rel: directory + clineSession + ".messages.json", data: clineMessages(n)}}
			}},
		{kind: "session messages.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(directory+clineSession+".messages.json", []byte(`{"messages":[`+joined(n, func(i int) string {
				return "1"
			})+`,{"role":"user","content":[{"type":"tool_result","tool_use_id":"t","content":`+
				nestedArrays(n, `"x"`)+`}]}]}`))
		}},
	}
}

// clineSessionsTable returns the sessions table of db/sessions.db holding
// rows.
func clineSessionsTable(rows [][]any) sqlitetest.Table {
	return sqlitetest.Table{Name: "sessions", SQL: "CREATE TABLE sessions (session_id TEXT, started_at TEXT, " +
		"ended_at TEXT, cwd TEXT, model TEXT, provider TEXT, status TEXT, title TEXT, metadata_json TEXT)", Rows: rows}
}

// clineDatabaseCases returns the cases of db/sessions.db.
func clineDatabaseCases() []growthCase {
	const sessions = clineData + "db/sessions.db"
	return []growthCase{
		{kind: "sessions.db", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(sessions, database(t, clineSessionsTable(rows(n, func(i int) []any {
				return []any{clineSession, "2026-01-02T03:04:05Z", nil, "/p", "m", "anthropic", "done", "t",
					`{"enterprise.blobUpload":{"storage":{"adapterType":"s3","bucket":"b"}}}`}
			}))))
		}},
		{kind: "sessions.db", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			files := []growthFile{{rel: sessions, data: database(t, clineSessionsTable(rows(2*n, func(i int) []any {
				return []any{numbered("s", i), "2026-01-02T03:04:05Z", nil, "/p", "m", "anthropic", "done", "t", nil}
			})))}}
			return append(files, manyFiles(n, func(i int) string {
				return clineData + "sessions/" + numbered("s", 2*i) + "/" + numbered("s", 2*i) + ".json"
			}, func(i int) string { return clineManifest(numbered("s", 2*i), "/p", "") })...)
		}},
		{kind: "sessions.db", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			damaged := rows(n, func(i int) []any {
				return []any{numbered("s", i), i, 1.5, []byte{0xff}, nil, "p", "s", nil, `{"importedFrom":[`}
			})
			deep := []any{"deep", "2026-01-02T03:04:05Z", nil, "/p", "m", "p", "s", nil, `{"importedFrom":` +
				nestedObjects(n, "tool", `"x"`) + `,"title":` + nestedArrays(n, `"t"`) + `}`}
			return one(sessions, database(t, clineSessionsTable(append(damaged, deep))))
		}},
	}
}

// clineSearchTables returns the tables of db/session-search.db: the
// indexed sessions, and the search table over its content table of
// documents (session_id, document_id, ordinal, role, started_at,
// workspace_root, title, content).
func clineSearchTables(indexed, documents [][]any) []sqlitetest.Table {
	return []sqlitetest.Table{
		{Name: "indexed_sessions", SQL: "CREATE TABLE indexed_sessions (session_id TEXT PRIMARY KEY, title TEXT, " +
			"document_count INTEGER, indexed_at INTEGER)", Rows: indexed},
		{Name: "session_search", SQL: "CREATE VIRTUAL TABLE session_search USING fts5(session_id, document_id, " +
			"ordinal, role, started_at, workspace_root, title, content)"},
		{Name: "session_search_content", SQL: "CREATE TABLE session_search_content(id INTEGER PRIMARY KEY, c0, c1, " +
			"c2, c3, c4, c5, c6, c7)", Rows: documents}}
}

// clineDocument returns a row of the search content table: a document of
// the session at an ordinal with a role, workspace root and content.
func clineDocument(i int, sessionID, role, workspace, content string) []any {
	return []any{nil, sessionID, numbered("d", i), i, role, "2026-01-02T03:04:05Z", workspace, "t", content}
}

// clineSearchCases returns the cases of db/session-search.db.
func clineSearchCases() []growthCase {
	const search = clineData + "db/session-search.db"
	return []growthCase{
		{kind: "session-search.db", shape: shapeCarried + " (the first document's workspace root)",
			files: func(t *testing.T, n int) []growthFile {
				documents := append([][]any{clineDocument(0, clineSession, "session", "/"+long("p"), "fix tests")},
					rows(n, func(i int) []any { return clineDocument(i+1, clineSession, "user", "", "hi") })...)
				return one(search, database(t, clineSearchTables([][]any{{clineSession, "t", n, 1}}, documents)...))
			}},
		{kind: "session-search.db", shape: shapeJoins, files: func(t *testing.T, n int) []growthFile {
			indexed := [][]any{{clineSession, strings.Repeat("t", 1000*n), n, 1}}
			documents := append([][]any{clineDocument(0, clineSession, "session", "/p", strings.Repeat("c", 1000*n))},
				rows(n, func(i int) []any { return clineDocument(i+1, clineSession, "user", "/p", "hi") })...)
			return one(search, database(t, clineSearchTables(indexed, documents)...))
		}},
		{kind: "session-search.db", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			indexed := rows(n, func(i int) []any { return []any{clineSession, "t", 1, 1} })
			documents := rows(n, func(i int) []any { return clineDocument(i, clineSession, "user", "/p", "hi") })
			return one(search, database(t, clineSearchTables(indexed, documents)...))
		}},
		{kind: "session-search.db", shape: shapeLargeShared + " by many spellings", n: 32, files: func(t *testing.T, n int) []growthFile {
			const sessionID = "abcdefabcdefabcdefab"
			indexed := rows(n, func(i int) []any { return []any{spelling(sessionID, i), "t", 1, 1} })
			return []growthFile{{rel: search, data: database(t, clineSearchTables(indexed, nil)...)},
				text(clineData+"sessions/"+sessionID+"/"+sessionID+".json",
					clineManifest(sessionID, "/p", `,"pad":"`+strings.Repeat("x", 1000*n)+`"`))}
		}},
	}
}

// clineHook returns a hooks.jsonl line of a hook with its own members.
func clineHook(hook, members string) string {
	return `{"ts":"2026-10-03T10:01:40.050Z","taskId":"t1","workspaceRoots":["/srv/proj"],"userId":"alice",` +
		`"hookName":"` + hook + `"` + members + `}`
}

// clineLogCases returns the cases of the hook log and the input history.
func clineLogCases() []growthCase {
	const hooks, history = clineData + "logs/hooks.jsonl", clineData + "cache/user_input_history.jsonl"
	return []growthCase{
		{kind: "hooks.jsonl", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(hooks, lines(n, func(i int) string {
				if i%2 == 0 {
					return clineHook("tool_call", `,"tool_call":{"id":"call_1","name":"bash","input":{"command":"ls"}}`)
				}
				return clineHook("tool_result", `,"tool_result":{"id":"call_1","name":"bash","output":"ok"}`)
			}))
		}},
		{kind: "hooks.jsonl", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(hooks, append(lines(n, func(i int) string { return `{"hookName":` }),
				[]byte(clineHook("tool_call", `,"tool_call":{"id":"c","name":"bash","input":`+
					nestedObjects(n, "command", `"ls"`)+`}`)+"\n")...))
		}},
		{kind: "user_input_history.jsonl", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(history, lines(n, func(i int) string { return `{"prompt":"fix tests","ts":1791021900000}` }))
		}},
		{kind: "user_input_history.jsonl", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(history, append(lines(n, func(i int) string { return `{"prompt":` }),
				[]byte(`{"prompt":`+nestedArrays(n, `"x"`)+`}`+"\n")...))
		}},
	}
}

// clineQueueCases returns the cases of the extension's sync queue.
func clineQueueCases() []growthCase {
	const queue = clineExtension + "cache/sync-queue.json"
	item := func(i int) string {
		return `"` + numbered("k", i) + `":{"taskId":"` + clineTaskID + `","key":"ui_messages.json",` +
			`"timestamp":1791021605000,"status":"synced","retryCount":0}`
	}
	return []growthCase{
		{kind: "sync-queue.json", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			history := text(clineExtension+"state/taskHistory.json", "["+joined(n, func(i int) string {
				return `{"id":"` + strconv.Itoa(1791021600000+n-i) + `","task":"` + strings.Repeat("t", 100) +
					`","cwdOnTaskInitialization":"/p"}`
			})+"]")
			return []growthFile{history, text(queue, `{"items":{`+joined(n, item)+`}}`)}
		}},
		{kind: "sync-queue.json", shape: shapeCarried + " (the index's project path)",
			files: func(t *testing.T, n int) []growthFile {
				return []growthFile{clineHistory(clineTaskID, "/"+long("p"), "m"), text(queue, `{"items":{`+
					joined(n, item)+`}}`)}
			}},
		{kind: "sync-queue.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(queue, []byte(`{"items":{`+joined(n, func(i int) string { return `"` + numbered("k", i) + `":1` })+
				`,"deep":{"taskId":"t","status":`+nestedArrays(n, `"s"`)+`}}}`))
		}},
	}
}

// clineSettingsCases returns the cases of the standing settings files and
// the sync queue.
func clineSettingsCases() []growthCase {
	return append(clineQueueCases(), []growthCase{
		{kind: "globalState.json", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(clineData+"globalState.json", []byte(`{"autoApprovalSettings":{"enabled":true,"favorites":[`+
				joined(n, func(i int) string { return `"` + numbered("f", i) + `"` })+`],"actions":{`+
				joined(n, func(i int) string { return `"` + numbered("a", i) + `":true` })+`}},"mode":"`+long("mode")+
				`","cloudSessionStatuses":{`+joined(n, func(i int) string {
				return `"` + numbered("ses-", i) + `":{"status":"completed","observedAt":1791021800000}`
			})+`}}`))
		}},
		{kind: "globalState.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(clineData+"globalState.json", []byte(`{"autoApprovalSettings":{"favorites":`+
				nestedArrays(n, "1")+`},"cloudSessionStatuses":{`+joined(n, func(i int) string {
				return `"` + numbered("s", i) + `":` + strconv.Itoa(i)
			})+`}}`))
		}},
		{kind: "global-settings.json", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(clineData+"settings/global-settings.json", []byte(`{"toolAutoApprove":"`+long("a")+
				`","disabledTools":[`+joined(n, func(i int) string { return `"` + numbered("t", i) + `"` })+
				`],"tools":{`+joined(n, func(i int) string { return `"` + numbered("t", i) + `":{"enabled":false}` })+`}}`))
		}},
		{kind: "global-settings.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(clineData+"settings/global-settings.json", []byte(`{"disabledTools":[`+
				joined(n, func(i int) string { return "{}" })+`,`+nestedArrays(n, "1")+`],"tools":`+
				nestedObjects(n, "enabled", "true")+`}`))
		}},
	}...)
}

// clineServerCases returns the cases of the MCP settings and the pending
// cloud starts.
func clineServerCases() []growthCase {
	return []growthCase{
		{kind: "cline_mcp_settings.json", shape: shapeCarried + " (a server's name)", files: func(t *testing.T, n int) []growthFile {
			return one(clineData+"settings/cline_mcp_settings.json", []byte(`{"mcpServers":{"`+long("server")+
				`":{"autoApprove":[`+joined(n, func(i int) string { return `"` + numbered("t", i) + `"` })+`]}}}`))
		}},
		{kind: "cline_mcp_settings.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(clineData+"settings/cline_mcp_settings.json", []byte(`{"mcpServers":{`+
				joined(n, func(i int) string { return `"` + numbered("s", i) + `":1` })+`,"deep":{"autoApprove":`+
				nestedArrays(n, `"t"`)+`}}}`))
		}},
		{kind: "cloud pending start", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string {
				return clineData + "cloud-pending-starts/" + strconv.Itoa(4000+i) + "-0b6f2c1e-6a1d.json"
			}, func(i int) string {
				return `[{"sessionId":"` + numbered("ses-", i) + `","account":"user:u","endpoint":"https://api.cline.bot"}]`
			})
		}},
		{kind: "cloud pending start", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(clineData+"cloud-pending-starts/4242-0b6f2c1e.json", []byte("["+joined(n, func(i int) string {
				return `{"account":1}`
			})+`,{"sessionId":"s","account":`+nestedArrays(n, `"a"`)+`}]`))
		}},
	}
}

// clineGrowth returns the Cline growth set.
func clineGrowth() growthSet {
	var cases []growthCase
	for _, group := range [][]growthCase{clineUICases(), clineAPICases(), clineMetadataCases(),
		clineSessionCases(), clineDatabaseCases(), clineSearchCases(), clineServerCases(), clineLogCases(), clineSettingsCases()} {
		cases = append(cases, group...)
	}
	return growthSet{parser: "cline", cases: cases}
}
