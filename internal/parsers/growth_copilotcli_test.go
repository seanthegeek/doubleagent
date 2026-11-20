package parsers

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The Copilot CLI growth cases: the session events and workspace, the
// rewind snapshots, the session store, the command history, the standing
// grants, the sidebar joins and the process logs.

const (
	copilotcliSession = "5e55a0a1-0000-4000-8000-000000000001"
	copilotcliState   = ".copilot/session-state/"
	copilotcliEvents  = copilotcliState + copilotcliSession + "/events.jsonl"
	copilotcliSidebar = ".copilot/sidebar-sessions-state/"
	copilotcliTime    = "2026-10-01T09:00:00.000Z"
)

// copilotcliEvent returns an events.jsonl line of the type and data.
func copilotcliEvent(kind, data string) string {
	return `{"type":"` + kind + `","data":` + data + `,"id":"e","timestamp":"` + copilotcliTime + `"}`
}

// copilotcliSessionID returns the i-th session id.
func copilotcliSessionID(i int) string {
	digits := strconv.Itoa(i)
	return copilotcliSession[:36-len(digits)] + digits
}

// copilotcliSidebarName returns the i-th sidebar file's name: 64
// hexadecimal digits.
func copilotcliSidebarName(i int) string {
	digits := strconv.Itoa(i)
	return strings.Repeat("a", 64-len(digits)) + digits + ".json"
}

// copilotcliStoreTables are the columns of the session store's tables the
// cases fill, by name.
func copilotcliStoreTables() map[string]string {
	return map[string]string{
		"sessions":      "id TEXT, cwd TEXT, branch TEXT, summary TEXT, created_at TEXT",
		"turns":         "session_id TEXT, turn_index INTEGER, user_message TEXT, assistant_response TEXT, timestamp TEXT",
		"session_files": "session_id TEXT, file_path TEXT, tool_name TEXT, first_seen_at TEXT",
		"forge_trajectory_events": "session_id TEXT, tool_call_id TEXT, command TEXT, output TEXT, exit_code INTEGER, " +
			"created_at TEXT",
	}
}

// copilotcliStore returns a session-store.db holding the rows given by
// table name.
func copilotcliStore(t *testing.T, filled map[string][][]any) []byte {
	var tables []sqlitetest.Table
	for _, name := range []string{"sessions", "turns", "session_files", "forge_trajectory_events"} {
		if content, ok := filled[name]; ok {
			tables = append(tables, sqlitetest.Table{Name: name,
				SQL: "CREATE TABLE " + name + " (" + copilotcliStoreTables()[name] + ")", Rows: content})
		}
	}
	return database(t, tables...)
}

// copilotcliTurns returns n turns, n files and n tool events of a session.
func copilotcliTurns(n int, sessionID string) map[string][][]any {
	return map[string][][]any{
		"turns": rows(n, func(i int) []any {
			return []any{sessionID, i, numbered("prompt ", i), numbered("reply ", i), copilotcliTime}
		}),
		"session_files": rows(n, func(i int) []any {
			return []any{sessionID, numbered("/src/f", i), "edit", copilotcliTime}
		}),
		"forge_trajectory_events": rows(n, func(i int) []any {
			return []any{sessionID, numbered("call", i), "ls", "out", i % 2, copilotcliTime}
		}),
	}
}

// copilotcliGrowth returns the copilot-cli growth set.
func copilotcliGrowth() growthSet {
	return growthSet{parser: "copilot-cli", cases: slices.Concat(copilotcliEventCases(),
		copilotcliEventShapeCases(), copilotcliStoreCases(), copilotcliSessionFileCases(), copilotcliHomeFileCases(),
		copilotcliSettingsCases())}
}

// copilotcliStoreCases returns the cases of session-store.db.
func copilotcliStoreCases() []growthCase {
	const rel = ".copilot/session-store.db"
	return []growthCase{
		{kind: "session-store.db", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			filled := copilotcliTurns(n, "s1")
			filled["sessions"] = [][]any{{"s1", "/" + long("cwd"), long("branch"), "name", copilotcliTime}}
			return one(rel, copilotcliStore(t, filled))
		}},
		{kind: "session-store.db", shape: shapeJoins, files: func(t *testing.T, n int) []growthFile {
			filled := copilotcliTurns(n, "s1")
			filled["sessions"] = [][]any{{"s1", "/" + strings.Repeat("c", 1024*n), "main", "name", copilotcliTime}}
			return one(rel, copilotcliStore(t, filled))
		}},
		{kind: "session-store.db", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			filled := copilotcliTurns(n, "s1")
			filled["sessions"] = rows(n, func(i int) []any {
				return []any{"s1", numbered("/p", i), "main", numbered("name ", i), copilotcliTime}
			})
			return one(rel, copilotcliStore(t, filled))
		}},
		{kind: "session-store.db", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			filled := copilotcliTurns(n, "s1")
			filled["sessions"] = [][]any{{"s1", nil, "main", "name", copilotcliTime}}
			return append(manyFiles(n, func(i int) string { return copilotcliSidebar + copilotcliSidebarName(i) },
				func(i int) string {
					return `{"cwd":"/p` + strconv.Itoa(i) + `","sessionIds":["s1","s` + strconv.Itoa(i) + `"]}`
				}),
				growthFile{rel: rel, data: copilotcliStore(t, filled)})
		}},
	}
}

// copilotcliEventCases returns the cases of events.jsonl where one long
// value stands in many rows.
func copilotcliEventCases() []growthCase {
	user := func(i int) string { return copilotcliEvent("user.message", `{"content":"p`+strconv.Itoa(i)+`"}`) }
	return []growthCase{
		{kind: "events.jsonl", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			start := copilotcliEvent("session.start", `{"copilotVersion":"1","context":{"cwd":"/`+long("cwd")+
				`","branch":"`+long("branch")+`"}}`) + "\n" +
				copilotcliEvent("session.model_change", `{"newModel":"`+long("model")+`"}`) + "\n"
			return one(copilotcliEvents, append([]byte(start), lines(n, user)...))
		}},
		{kind: "events.jsonl", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			start := copilotcliEvent("tool.execution_start", `{"toolCallId":"c1","toolName":"`+long("tool")+
				`","arguments":{"command":"ls"}}`) + "\n"
			return one(copilotcliEvents, append([]byte(start), lines(n, func(i int) string {
				return copilotcliEvent("tool.execution_complete", `{"toolCallId":"c1","success":true,`+
					`"result":{"content":"r`+strconv.Itoa(i)+`"}}`)
			})...))
		}},
		{kind: "events.jsonl", shape: shapeCarried + " in the permission mode",
			files: func(t *testing.T, n int) []growthFile {
				start := copilotcliEvent("permission.requested", `{"permissionMode":"`+long("mode")+
					`","permissionRequest":{"kind":"shell","toolCallId":"c0"}}`) + "\n"
				return one(copilotcliEvents, append([]byte(start), lines(n, func(i int) string {
					return copilotcliEvent("permission.requested", `{"agentMode":"a`+strconv.Itoa(i)+
						`","permissionRequest":{"kind":"shell","toolCallId":"c`+strconv.Itoa(i)+`"}}`)
				})...))
			}},
	}
}

// copilotcliEventShapeCases returns the cases of events.jsonl that repeat
// ids, nest and damage records, and spread sessions over many files.
func copilotcliEventShapeCases() []growthCase {
	user := func(i int) string { return copilotcliEvent("user.message", `{"content":"p`+strconv.Itoa(i)+`"}`) }
	return []growthCase{
		{kind: "events.jsonl", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(copilotcliEvents, lines(n, func(i int) string {
				return copilotcliEvent("assistant.message", `{"content":"a","toolRequests":[{"toolCallId":"c1",`+
					`"name":"bash","arguments":{"command":"ls"}}]}`) + "\n" +
					copilotcliEvent("tool.execution_start", `{"toolCallId":"c1","toolName":"bash"}`) + "\n" +
					copilotcliEvent("tool.execution_complete", `{"toolCallId":"c1","result":{"content":"r"}}`)
			}))
		}},
		{kind: "events.jsonl", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(copilotcliEvents, append(lines(n, func(i int) string {
				return copilotcliEvent("assistant.message", `{"content":"a","toolRequests":`+strconv.Itoa(i)+`}`) +
					"\n" + copilotcliEvent("user.message", strconv.Itoa(i)) + "\n" + `{"type":"user.message","data":`
			}), []byte(copilotcliEvent("hook.start", `{"hookType":"pre","input":`+nestedArrays(n, `"x"`)+`}`)+"\n")...))
		}},
		{kind: "events.jsonl", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			files := manyFiles(n, func(i int) string { return copilotcliSidebar + copilotcliSidebarName(i) },
				func(i int) string {
					return `{"cwd":"/p` + strconv.Itoa(i) + `","sessionIds":["` + copilotcliSessionID(i) + `"]}`
				})
			for i := range n {
				directory := copilotcliState + copilotcliSessionID(i) + "/"
				files = append(files, text(directory+"events.jsonl", user(i)+"\n"),
					text(directory+"workspace.yaml", "id: "+copilotcliSessionID(i)+"\nname: s\n"))
			}
			return files
		}},
	}
}

// copilotcliSessionFileCases returns the cases of workspace.yaml and the
// rewind snapshot index.
func copilotcliSessionFileCases() []growthCase {
	const snapshots = copilotcliState + copilotcliSession + "/rewind-file-snapshots/index.json"
	snapshot := func(i int) string {
		return `{"timestamp":"` + copilotcliTime + `","userMessage":"m` + strconv.Itoa(i) +
			`","fileCount":1,"files":{"a":{"path":"/f"}}}`
	}
	return []growthCase{
		{kind: "workspace.yaml", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			files := manyFiles(n, func(i int) string { return copilotcliSidebar + copilotcliSidebarName(i) },
				func(i int) string {
					return `{"cwd":"/p` + strconv.Itoa(i) + `","sessionIds":["` + copilotcliSessionID(i) + `"]}`
				})
			return append(files, manyFiles(n, func(i int) string {
				return copilotcliState + copilotcliSessionID(i) + "/workspace.yaml"
			}, func(i int) string { return "id: " + copilotcliSessionID(i) + "\nname: s\nmc_session_id: m\n" })...)
		}},
		{kind: "workspace.yaml", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(copilotcliState+copilotcliSession+"/workspace.yaml", append(lines(n, func(i int) string {
				return strings.Repeat(" ", i%8) + "- key" + strconv.Itoa(i) + ": [" + strings.Repeat("v", 200)
			}), []byte("name: '"+strings.Repeat("''", n)+"'\ncwd: \"\\u00\"\n")...))
		}},
		{kind: "rewind snapshots index.json", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{text(copilotcliState+copilotcliSession+"/workspace.yaml", "cwd: /"+long("cwd")+"\n"),
				text(snapshots, `{"snapshots":[`+joined(n, snapshot)+`]}`)}
		}},
		{kind: "rewind snapshots index.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(snapshots, []byte(`{"snapshots":[`+joined(n, func(i int) string { return strconv.Itoa(i) })+
				`,{"userMessage":`+nestedArrays(n, `"x"`)+`}]}`))
		}},
	}
}

// copilotcliHomeFileCases returns the cases of the command history and
// the standing grants.
func copilotcliHomeFileCases() []growthCase {
	return []growthCase{
		{kind: "command-history-state.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".copilot/command-history-state.json", []byte(`{"commandHistory":[`+joined(n, func(i int) string {
				return strconv.Itoa(i) + `,"prompt ` + strconv.Itoa(i) + `"`
			})+`,`+nestedArrays(n, `"x"`)+`]}`))
		}},
		{kind: "permissions-config.json", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".copilot/permissions-config.json", []byte(`{"locations":{"/`+long("location")+
				`":{"tool_approvals":[`+joined(n, func(i int) string { return `"` + numbered("shell(git ", i) + `)"` })+
				`]}}}`))
		}},
		{kind: "permissions-config.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".copilot/permissions-config.json", []byte(`{"locations":{`+joined(n, func(i int) string {
				return `"/p` + strconv.Itoa(i) + `":{"tool_approvals":` + strconv.Itoa(i) + `}`
			})+`,"/deep":{"allowed_directories":[`+nestedObjects(n, "a", "1")+`]}}}`))
		}},
	}
}

// copilotcliSettingsCases returns the cases of settings.json, the sidebar
// files and the process logs.
func copilotcliSettingsCases() []growthCase {
	return []growthCase{
		{kind: "settings.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".copilot/settings.json", []byte("// settings\n{\"allowedUrls\":["+joined(n, func(i int) string {
				return strconv.Itoa(i) + `, "https://h` + strconv.Itoa(i) + `.example" /* c */`
			})+"],\"deniedUrls\":"+nestedArrays(n, "1")+"}"))
		}},
		{kind: "sidebar session state", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return copilotcliSidebar + copilotcliSidebarName(i) },
				func(i int) string {
					if i%2 == 0 {
						return `["not an object"]`
					}
					return `{"cwd":"/p","sessionIds":["` + copilotcliSessionID(i) + `"]}`
				})
		}},
		{kind: "process log", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".copilot/logs/process-1790845199000-4242.log", lines(n, func(i int) string {
				return copilotcliTime + " [INFO] Workspace initialized: " + copilotcliSession + "\n" + copilotcliTime +
					" [INFO] Signed in to github.com as user" + strconv.Itoa(i) + "."
			}))
		}},
	}
}
