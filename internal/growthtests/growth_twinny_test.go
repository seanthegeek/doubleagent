package growthtests

import (
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The twinny growth cases: the extension's item in an editor's global and
// workspace state.vscdb, its agent-changes records and its log.

// The files the cases write, under VS Code's user directory.
const (
	twinnyUser    = ".config/Code/User/"
	twinnyState   = twinnyUser + "globalStorage/state.vscdb"
	twinnyChanges = twinnyUser + "globalStorage/rjmacarthy.twinny/agent-changes/"
	twinnyLog     = ".config/Code/logs/20260101T000000/window1/exthost/rjmacarthy.twinny/Twinny.log"
	twinnyEditor  = "vscode"
)

// twinnyDatabase returns a state.vscdb whose rjmacarthy.twinny item holds
// value.
func twinnyDatabase(t *testing.T, value string) []byte {
	t.Helper()
	return database(t, sqlitetest.Table{Name: "ItemTable", SQL: "CREATE TABLE ItemTable (key TEXT, value BLOB)",
		Rows: [][]any{{"rjmacarthy.twinny", value}}})
}

// twinnyMessage returns conversation message i: a prompt, or a reply that
// thinks and runs a tool, in turn.
func twinnyMessage(i int) string {
	if i%2 == 0 {
		return `{"role":"user","content":"` + numbered("ask ", i) + `"}`
	}
	return `{"role":"assistant","content":"<think>hmm</think>ok","meta":{"model":"m"},"toolSteps":[{"id":"` +
		numbered("t", i) + `","name":"run","command":"ls","output":"out","status":"done"}]}`
}

// twinnyConversation returns a conversation of the id holding the
// messages.
func twinnyConversation(identifier, messages string) string {
	return `{"id":"` + identifier + `","title":"t","updatedAt":1767000000000,"messages":[` + messages + `]}`
}

// twinnyDamaged returns damaged record i: a string of a kilobyte where an object
// belongs, long enough that the work of reading it outweighs the buffers a
// parse reuses from one run to the next.
func twinnyDamaged(i int) string {
	return `"` + numbered(strings.Repeat("x", 1000), i) + `"`
}

// twinnyStateCases returns the cases of the extension's item in the
// global and workspace state databases.
func twinnyStateCases() []growthCase {
	return []growthCase{
		{kind: "state.vscdb", shape: shapeCarried + " by the conversation id", agent: twinnyEditor, files: func(t *testing.T, n int) []growthFile {
			return one(twinnyState, twinnyDatabase(t, `{"twinny.conversations":[`+
				twinnyConversation(long("c"), joined(n, twinnyMessage))+`]}`))
		}},
		{kind: "state.vscdb", shape: shapeCarried + " by a team policy key", agent: twinnyEditor, files: func(t *testing.T, n int) []growthFile {
			return one(twinnyState, twinnyDatabase(t, `{"twinny.teamPolicy":{"`+long("key")+`":[`+
				joined(n, func(i int) string { return `"` + numbered("tool", i) + `"` })+`]}}`))
		}},
		{kind: "state.vscdb", shape: shapeSharedID, agent: twinnyEditor, files: func(t *testing.T, n int) []growthFile {
			conversations := joined(n, func(i int) string { return twinnyConversation("c", twinnyMessage(i)) })
			return one(twinnyState, twinnyDatabase(t, `{"twinny.conversations":[`+conversations+
				`],"twinny.active-conversation":`+twinnyConversation("c", twinnyMessage(0))+
				`,"twinny-global-context-alwaysRunCommands":[`+joined(n, func(i int) string { return `"ls"` })+`]}`))
		}},
		{kind: "state.vscdb", shape: shapeNested, agent: twinnyEditor, files: func(t *testing.T, n int) []growthFile {
			messages := joined(n, twinnyDamaged) + `,{"role":"assistant","content":` +
				nestedArrays(n, `{"type":"text","text":"x"}`) + `,"toolSteps":[` + joined(n, twinnyDamaged) + `]},{"role":"assistant","content":"` + strings.Repeat("<think>a</think><", n) + `"}`
			return one(twinnyState, twinnyDatabase(t, `{"twinny.conversations":{"c":`+twinnyConversation("c", messages)+
				`},"twinny.teamPolicy":`+nestedObjects(n, strings.Repeat("k", 100), "true")+`}`))
		}},
		{kind: "workspace state.vscdb", shape: shapeManyFiles, agent: twinnyEditor, n: 16,
			files: func(t *testing.T, n int) []growthFile {
				files := make([]growthFile, 0, n)
				for i := range n {
					files = append(files, growthFile{rel: twinnyUser + "workspaceStorage/" + numbered("w", i) + "/state.vscdb",
						data: twinnyDatabase(t, `{"twinny-workspace-context-reviewOwner":"o",`+
							`"twinny-workspace-context-reviewRepo":"`+numbered("r", i)+`"}`)})
				}
				return files
			}},
	}
}

// twinnyOpenState returns a workspace state value whose open conversation
// is the conversation given.
func twinnyOpenState(conversation string) string {
	return `{"twinny.active-conversation":` + conversation + `}`
}

// twinnyOpenCases returns the cases of a workspace row's open conversation,
// which the global row of the same User directory decides is shown.
func twinnyOpenCases() []growthCase {
	return []growthCase{
		{kind: "workspace state.vscdb", shape: shapeManyFiles + " beside one large global row", agent: twinnyEditor,
			n: 16, files: func(t *testing.T, n int) []growthFile {
				saved := joined(n, func(i int) string {
					return `"` + numbered("s", i) + `":` + twinnyConversation(numbered("s", i), twinnyMessage(i))
				})
				files := []growthFile{{rel: twinnyState, data: twinnyDatabase(t, `{"twinny.conversations":{`+saved+`}}`)}}
				for i := range n {
					workspace := twinnyUser + "workspaceStorage/" + numbered("w", i) + "/"
					files = append(files, growthFile{rel: workspace + "state.vscdb",
						data: twinnyDatabase(t, twinnyOpenState(twinnyConversation(numbered("o", i), twinnyMessage(0))))},
						text(workspace+"workspace.json", `{"folder":"file:///srv/`+numbered("p", i)+`"}`))
				}
				return files
			}},
		{kind: "workspace state.vscdb", shape: shapeCarried + " by the conversation id", agent: twinnyEditor,
			files: func(t *testing.T, n int) []growthFile {
				return one(twinnyUser+"workspaceStorage/w/state.vscdb",
					twinnyDatabase(t, twinnyOpenState(twinnyConversation(long("o"), joined(n, twinnyMessage)))))
			}},
		{kind: "workspace state.vscdb", shape: shapeNested, agent: twinnyEditor, files: func(t *testing.T, n int) []growthFile {
			messages := joined(n, twinnyDamaged) + `,{"role":"assistant","content":` +
				nestedArrays(n, `{"type":"text","text":"x"}`) + `,"toolSteps":[` + joined(n, twinnyDamaged) + `]}`
			return one(twinnyUser+"workspaceStorage/w/state.vscdb",
				twinnyDatabase(t, twinnyOpenState(twinnyConversation("o", messages))))
		}},
	}
}

// twinnyFileCases returns the cases of the agent-changes records and the
// log.
func twinnyFileCases() []growthCase {
	return []growthCase{
		{kind: "agent changes", shape: shapeCarried + " by the root", files: func(t *testing.T, n int) []growthFile {
			return one(twinnyChanges+"c.json", []byte(`{"id":"r","root":"/`+long("p")+`","createdAt":1767000000000,`+
				`"files":[`+joined(n, func(i int) string {
				return `{"status":"modified","path":"` + numbered("f", i) + `","added":1,"removed":0}`
			})+"]}"))
		}},
		{kind: "agent changes", shape: shapeCarried + " by the reply id", files: func(t *testing.T, n int) []growthFile {
			return one(twinnyChanges+"c.json", []byte(`{"id":"`+long("r")+`","root":"/p","files":[`+
				joined(n, func(i int) string { return `{"status":"added","path":"` + numbered("f", i) + `"}` })+"]}"))
		}},
		{kind: "agent changes", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string {
				return twinnyUser + "workspaceStorage/" + numbered("w", i) + "/rjmacarthy.twinny/agent-changes/c.json"
			}, func(i int) string { return `{"id":"r","root":"/p","files":[{"status":"added","path":"f"}]}` })
		}},
		{kind: "agent changes", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(twinnyChanges+"c.json", []byte(`{"id":`+nestedArrays(n, "1")+`,"files":[`+joined(n, twinnyDamaged)+`,{"path":`+nestedObjects(n, "a", `"f"`)+`}]}`))
		}},
		{kind: "Twinny.log", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(twinnyLog, lines(n, func(i int) string {
				return "2026-01-01 00:00:00.000 [info] " + numbered("step ", i) + "\n    more\n\tdetail\ndamaged"
			}))
		}},
		{kind: "Twinny.log", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(twinnyLog+"_"+strconv.Itoa(2), append([]byte("2026-01-01 00:00:00.000 [error] "+long("e")+"\n"),
				lines(n, func(i int) string { return "    " + numbered("frame ", i) })...))
		}},
	}
}

// twinnyGrowth returns the twinny growth set.
func twinnyGrowth() growthSet {
	var cases []growthCase
	cases = append(cases, twinnyStateCases()...)
	cases = append(cases, twinnyOpenCases()...)
	cases = append(cases, twinnyFileCases()...)
	return growthSet{parser: "twinny", cases: cases}
}
