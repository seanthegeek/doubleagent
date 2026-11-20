package parsers

import (
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The VS Code growth cases: the chat sessions as JSON and as mutation
// logs, the cloud session markers, the state databases, the settings and
// the chat editing states.

// The directories of a case: one workspace's storage and the profile's
// global storage.
const (
	vscodeUser      = ".config/Code/User/"
	vscodeWorkspace = vscodeUser + "workspaceStorage/w1/"
	vscodeSessionID = "9ab0c1d2-0000-4000-8000-00000000c0de"
)

// vscodeRequest returns a request of the chat session at a time offset,
// its response parts given as JSON.
func vscodeRequest(i int, response string) string {
	return `{"requestId":"` + numbered("request_", i) + `","timestamp":` + strconv.Itoa(1791025201000+i) +
		`,"message":{"text":"run tests","parts":[]},"modelId":"copilot/gpt-4.1","modeInfo":{"kind":"agent",` +
		`"telemetryModeId":"agent","permissionLevel":"default"},"response":` + response +
		`,"responseTimestamp":` + strconv.Itoa(1791025202000+i) + `}`
}

// vscodeState returns a chat session state holding the requests, with
// extra members after them.
func vscodeState(requests, extra string) string {
	return `{"version":3,"sessionId":"` + vscodeSessionID + `","creationDate":1791025200000,` +
		`"initialLocation":"panel","responderUsername":"GitHub Copilot","requests":[` + requests + `]` + extra + `}`
}

// vscodeToolCall returns a response part calling a tool by id.
func vscodeToolCall(toolID string) string {
	return `{"kind":"toolInvocationSerialized","toolCallId":"` + toolID + `","toolId":"run_in_terminal",` +
		`"invocationMessage":"Running command","isComplete":true,"isConfirmed":{"type":4},` +
		`"resultDetails":{"input":"pytest","output":[{"type":"embed","value":"3 passed"}]}}`
}

// vscodeItems returns the ItemTable of a state database holding the key
// and value pairs.
func vscodeItems(pairs [][]any) sqlitetest.Table {
	return sqlitetest.Table{Name: "ItemTable", SQL: "CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, " +
		"value BLOB)", Rows: pairs}
}

// vscodeGrowth returns the VS Code growth set.
func vscodeGrowth() growthSet {
	var cases []growthCase
	for _, group := range [][]growthCase{vscodeSessionCases(), vscodeLogCases(), vscodeStateCases(),
		vscodeFileCases(), vscodeEditingCases()} {
		cases = append(cases, group...)
	}
	return growthSet{parser: "vscode", cases: cases}
}

// vscodeSessionCases returns the cases of a chat session saved as JSON.
func vscodeSessionCases() []growthCase {
	const kind = "chat session json"
	session := vscodeWorkspace + "chatSessions/" + vscodeSessionID + ".json"
	return []growthCase{
		{kind: kind, shape: shapeCarried + " (the working directory)", files: func(t *testing.T, n int) []growthFile {
			return one(session, []byte(vscodeState(joined(n, func(i int) string {
				return vscodeRequest(i, `["ok"]`)
			}), `,"workingDirectory":"file:///`+long("p")+`"`)))
		}},
		{kind: kind, shape: shapeNested + " (markdown nested around one large text)", n: 32,
			known: "allocated bytes grow 3.5 times from n=32 to 64 (1,336,032 to 4,681,568): each level of nested " +
				"arrays joins and copies the whole text of the levels inside it again (vscode/parts.go mdText)",
			files: func(t *testing.T, n int) []growthFile {
				content := `"` + strings.Repeat("w", 1000*n) + `"`
				for range n {
					content = `[` + content + `,""]`
				}
				return one(session, []byte(vscodeState(vscodeRequest(0, `[{"kind":"markdownContent","content":`+
					content+`}]`), "")))
			}},
		{kind: kind, shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(session, []byte(vscodeState(joined(n, func(i int) string { return `"bad"` })+","+
				vscodeRequest(0, `[{"kind":"toolInvocationSerialized","toolCallId":"c","toolId":"t",`+
					`"invocationMessage":`+nestedArrays(n, `"x"`)+`,"resultDetails":`+nestedObjects(n, "output", "1")+`}]`), "")))
		}},
		{kind: kind, shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(session, []byte(vscodeState(joined(n, func(i int) string {
				return vscodeRequest(0, `[`+vscodeToolCall("call_a")+`]`)
			}), "")))
		}},
		{kind: kind, shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			files := []growthFile{text(vscodeWorkspace+"workspace.json", `{"folder":"file:///srv/proj","pad":"`+
				strings.Repeat("x", 100*n)+`"}`)}
			return append(files, manyFiles(n, func(i int) string {
				return vscodeWorkspace + "chatSessions/" + numbered("s", i) + ".json"
			}, func(i int) string { return vscodeState(vscodeRequest(i, `["ok"]`), "") })...)
		}},
	}
}

// vscodeLogCases returns the cases of a chat session's mutation log.
func vscodeLogCases() []growthCase {
	const kind = "chat session mutation log"
	log := vscodeWorkspace + "chatSessions/" + vscodeSessionID + ".jsonl"
	start := func(extra string) string {
		return `{"kind":0,"v":` + vscodeState(vscodeRequest(0, `[]`), extra) + `}` + "\n"
	}
	return []growthCase{
		{kind: kind, shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(log, append([]byte(start("")), lines(n, func(i int) string {
				if i%2 == 0 {
					return `{"kind":1,"k":["requests",0,"responseTimestamp"],"v":` + strconv.Itoa(1791025211000+i) + `}`
				}
				return `{"kind":2,"k":["requests",0,"response"],"v":[` + vscodeToolCall("call_a") + `]}`
			})...))
		}},
		{kind: kind, shape: shapeCarried + " (the working directory)", files: func(t *testing.T, n int) []growthFile {
			return one(log, append([]byte(start(`,"workingDirectory":"file:///`+long("p")+`"`)),
				lines(n, func(i int) string {
					return `{"kind":2,"k":["requests"],"v":[` + vscodeRequest(i+1, `["ok"]`) + `]}`
				})...))
		}},
		{kind: kind, shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(log, append(append([]byte(start("")), lines(n, func(i int) string {
				return `{"kind":1,"k":["requests",` + strconv.Itoa(i+5) + `,"x"],"v":1}`
			})...), []byte(`{"kind":1,"k":["requests",0,"response"],"v":`+nestedArrays(n, `"x"`)+`}`+"\n")...))
		}},
	}
}

// vscodeUsage returns a GitHub account usage value of n extensions.
func vscodeUsage(n int) string {
	return "[" + joined(n, func(i int) string {
		return `{"extensionId":"` + numbered("ext.", i) + `","scopes":["repo"],"lastUsed":1791025200000}`
	}) + "]"
}

// vscodeStateCases returns the cases of the state databases.
func vscodeStateCases() []growthCase {
	const kind = "state.vscdb"
	profile, workspace := vscodeUser+"globalStorage/state.vscdb", vscodeWorkspace+"state.vscdb"
	grants := func(n int) string {
		return `{` + joined(n, func(i int) string { return `"` + numbered("tool", i) + `":true` }) + `}`
	}
	return []growthCase{
		{kind: kind, shape: shapeCarried + " (an account's label)", files: func(t *testing.T, n int) []growthFile {
			return one(profile, database(t, vscodeItems([][]any{{"github-" + long("login") + "-usages", vscodeUsage(n)}})))
		}},
		{kind: kind, shape: shapeCarried + " (the workspace's folder)", files: func(t *testing.T, n int) []growthFile {
			return []growthFile{text(vscodeWorkspace+"workspace.json", `{"folder":"file:///`+long("p")+`"}`),
				{rel: workspace, data: database(t, vscodeItems([][]any{{"chat/autoconfirm", grants(n)}}))}}
		}},
		{kind: kind, shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(profile, database(t, vscodeItems(rows(n, func(i int) []any {
				return []any{"chat/autoconfirm", `{"run_in_terminal":true}`}
			}))))
		}},
		{kind: kind, shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			copilot := `{"github.copilot.cloudAgent.ownedTasks":{` + joined(n, func(i int) string {
				return `"` + numbered("t", i) + `":` + strconv.Itoa(i)
			}) + `},"userSelectedRepositories":[` + joined(n, func(i int) string { return "1" }) + `,{"name":` +
				nestedArrays(n, `"r"`) + `}]}`
			return one(profile, database(t, vscodeItems([][]any{{"GitHub.copilot-chat", copilot},
				{"chat/autoconfirm", nestedObjects(n, "a", "1")}, {"github-u-usages", "[" + strings.Repeat("[", n)}})))
		}},
		{kind: kind, shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			var files []growthFile
			for i := range n {
				directory := vscodeUser + "workspaceStorage/" + numbered("w", i) + "/"
				files = append(files, text(directory+"workspace.json", `{"folder":"file:///srv/p`+strconv.Itoa(i)+`"}`),
					growthFile{rel: directory + "state.vscdb", data: database(t, vscodeItems([][]any{
						{"chat/autoconfirm", `{"run_in_terminal":true}`}}))})
			}
			return files
		}},
	}
}

// vscodeFileCases returns the cases of cloudSessions.json and the settings.
func vscodeFileCases() []growthCase {
	cloud := vscodeUser + "globalStorage/github.copilot-chat/cloudSessions.json"
	return []growthCase{
		{kind: "cloudSessions.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(cloud, []byte("{"+joined(n, func(i int) string {
				return `"` + vscodeSessionID + `":{"cloudSessionId":"cs-0001","cloudTaskId":"ct-0001"}`
			})+"}"))
		}},
		{kind: "cloudSessions.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(cloud, []byte("{"+joined(n, func(i int) string { return `"` + numbered("s", i) + `":1` })+
				`,"deep":{"cloudSessionId":`+nestedArrays(n, `"c"`)+`}}`))
		}},
		{kind: "settings.json", shape: shapeCarried + " (a setting's key)", files: func(t *testing.T, n int) []growthFile {
			return one(vscodeUser+"settings.json", []byte(`{"chat.tools.terminal.autoApprove`+long("k")+`":{`+
				joined(n, func(i int) string { return `"` + numbered("c", i) + `":true` })+`}}`))
		}},
		{kind: "settings.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(vscodeUser+"settings.json", []byte("// comment\n{"+joined(n, func(i int) string {
				return `"chat.tools.` + numbered("k", i) + `.autoApprove":[]`
			})+`,"chat.defaultModel":`+nestedArrays(n, `"m"`)+`,}`))
		}},
	}
}

// vscodeEditingCases returns the cases of the chat editing states.
func vscodeEditingCases() []growthCase {
	editing := vscodeWorkspace + "chatEditingSessions/" + vscodeSessionID + "/state.json"
	entry := func(i int) string {
		return `{"resource":"file:///srv/proj/` + numbered("f", i) + `.py","state":1,"telemetryInfo":{"requestId":"r"}}`
	}
	return []growthCase{
		{kind: "editing state.json", shape: shapeCarried + " (the workspace's folder)",
			files: func(t *testing.T, n int) []growthFile {
				return []growthFile{text(vscodeWorkspace+"workspace.json", `{"folder":"file:///`+long("p")+`"}`),
					text(editing, `{"version":2,"recentSnapshot":{"entries":[`+joined(n, entry)+`]}}`)}
			}},
		{kind: "editing state.json", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			files := []growthFile{text(vscodeWorkspace+"workspace.json", `{"folder":"file:///srv/proj","pad":"`+
				strings.Repeat("x", 100*n)+`"}`)}
			return append(files, manyFiles(n, func(i int) string {
				return vscodeWorkspace + "chatEditingSessions/" + numbered("s", i) + "/state.json"
			}, func(i int) string { return `{"version":2,"recentSnapshot":{"entries":[` + entry(i) + `]}}` })...)
		}},
		{kind: "editing state.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(editing, []byte(`{"recentSnapshot":{"entries":[`+joined(n, func(i int) string { return "1" })+
				`,{"resource":`+nestedArrays(n, `"r"`)+`}]}}`))
		}},
	}
}
