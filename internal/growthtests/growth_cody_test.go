package growthtests

import (
	"fmt"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The Cody growth cases: the chat history in an editor's state.vscdb and
// in the JetBrains global state, its damaged copies, the model
// preferences, the editor settings and the legacy JetBrains history.

// The files the cases write; the state database is VS Code's.
const (
	codyState       = ".config/Code/User/globalStorage/state.vscdb"
	codyJetBrains   = ".config/Cody-nodejs/JetBrains-globalState/"
	codyEditorAgent = "vscode"
)

// codyStateDatabase returns a state.vscdb whose sourcegraph.cody-ai item
// holds value.
func codyStateDatabase(t *testing.T, value string) []byte {
	t.Helper()
	return database(t, sqlitetest.Table{Name: "ItemTable", SQL: "CREATE TABLE ItemTable (key TEXT, value BLOB)",
		Rows: [][]any{{"sourcegraph.cody-ai", value}}})
}

// codyChatDate returns chat key i: a date Cody keys a chat by, distinct for
// each i below 3,600.
func codyChatDate(i int) string {
	return fmt.Sprintf("Wed, 01 Oct 2026 08:%02d:%02d GMT", i/60%60, i%60)
}

// codyInteraction returns interaction i: a prompt and a reply calling a
// tool and holding its result.
func codyInteraction(i int) string {
	call := numbered("c", i)
	return `{"humanMessage":{"speaker":"human","text":"` + numbered("ask ", i) + `"},"assistantMessage":` +
		`{"speaker":"assistant","model":"m","text":"ok","content":[{"type":"tool_call","tool_call":{"id":"` + call +
		`","name":"bash","arguments":"ls"}},{"type":"tool_result","tool_result":{"id":"` + call + `","content":"out"}}]}}`
}

// codyChat returns a chat member of the key holding the interactions.
func codyChat(key, interactions string) string {
	return `"` + key + `":{"id":"` + key + `","chatTitle":"t","lastInteractionTimestamp":"2026-10-01T08:00:00Z",` +
		`"interactions":[` + interactions + `]}`
}

// codyHistory returns a chat history of one account holding the chats.
func codyHistory(chats string) string {
	return `{"acct":{"chat":{` + chats + `}}}`
}

// codyItem returns the sourcegraph.cody-ai item holding the history as
// JSON text.
func codyItem(history string) string {
	return `{"cody-local-chatHistory-v2":"` + strings.ReplaceAll(history, `"`, `\"`) + `"}`
}

// codyGrowth returns the cody growth set.
func codyGrowth() growthSet {
	var cases []growthCase
	cases = append(cases, codyStateCases()...)
	cases = append(cases, codyJetBrainsCases()...)
	cases = append(cases, codySettingCases()...)
	return growthSet{parser: "cody", cases: cases}
}

// codyStateCases returns the cases of the history in state.vscdb.
func codyStateCases() []growthCase {
	return []growthCase{
		{kind: "state.vscdb", shape: shapeCarried + " by the chat id", agent: codyEditorAgent, files: func(t *testing.T, n int) []growthFile {
			return one(codyState, codyStateDatabase(t, codyItem(codyHistory(codyChat(long("chat"),
				joined(n, codyInteraction))))))
		}},
		{kind: "state.vscdb", shape: shapeCarried + " by a tool call's name", agent: codyEditorAgent, files: func(t *testing.T, n int) []growthFile {
			call := `{"assistantMessage":{"speaker":"assistant","content":[{"type":"tool_call","tool_call":` +
				`{"id":"c","name":"` + long("tool") + `","arguments":"ls"}}]}}`
			results := joined(n, func(i int) string {
				return `{"assistantMessage":{"speaker":"assistant","content":[{"type":"tool_result",` +
					`"tool_result":{"id":"c","content":"out"}}]}}`
			})
			return one(codyState, codyStateDatabase(t, `{"cody-local-chatHistory-v2":`+
				codyHistory(codyChat(codyChatDate(0), call+","+results))+"}"))
		}},
		{kind: "state.vscdb", shape: shapeCarried + " by the account", agent: codyEditorAgent, files: func(t *testing.T, n int) []growthFile {
			chats := joined(n, func(i int) string { return codyChat(codyChatDate(i), "") })
			return one(codyState, codyStateDatabase(t, codyItem(`{"`+long("acct")+`":{"chat":{`+chats+`}}}`)))
		}},
		{kind: "state.vscdb", shape: shapeSharedID, agent: codyEditorAgent, files: func(t *testing.T, n int) []growthFile {
			accounts := joined(n, func(i int) string {
				return `"` + numbered("acct", i) + `":{"chat":{` + codyChat(codyChatDate(0), codyInteraction(i)) + `}}`
			})
			return one(codyState, codyStateDatabase(t, codyItem("{"+accounts+"}")))
		}},
		{kind: "state.vscdb", shape: shapeNested, agent: codyEditorAgent, files: func(t *testing.T, n int) []growthFile {
			chats := joined(n, func(i int) string { return codyChat(codyChatDate(i), codyInteraction(i)+",7") })
			return one(codyState, codyStateDatabase(t, codyItem(codyHistory(chats+","+
				codyChat(codyChatDate(n), `{"humanMessage":{"speaker":"human","text":`+nestedArrays(n, `"x"`)+`}}`))+
				`,"cody-model-preferences":{"e":{"selected":`+nestedObjects(n, "chat", `"m"`)+`}}}`)))
		}},
	}
}

// codyJetBrainsCases returns the cases of the JetBrains history, its
// damaged copy, the model preferences and the legacy history.
func codyJetBrainsCases() []growthCase {
	return []growthCase{
		{kind: "JetBrains chat history", shape: shapeNested + " recovered", n: 32, files: func(t *testing.T, n int) []growthFile {
			chats := joined(n, func(i int) string { return codyChat(codyChatDate(i), codyInteraction(i)) })
			return one(codyJetBrains+"cody-local-chatHistory-v2", []byte(codyHistory(chats+","+
				`"`+codyChatDate(n)+`":{"interactions":[{"x":`)))
		}},
		{kind: "JetBrains chat history", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(codyJetBrains+"cody-local-chatHistory-v2", []byte(codyHistory(codyChat(long("chat"),
				joined(n, codyInteraction)))))
		}},
		{kind: "JetBrains model preferences", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(codyJetBrains+"cody-model-preferences", []byte(`{"`+long("https://e")+`":{"selected":{`+
				joined(n, func(i int) string { return `"` + numbered("chat", i) + `":"m"` })+`}}}`))
		}},
		{kind: "JetBrains model preferences", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(codyJetBrains+"cody-model-preferences", []byte(`{"e":{"selected":{`+joined(n, func(i int) string {
				return `"` + numbered("chat", i) + `":` + codyDamaged(i)
			})+`},"defaults":`+nestedArrays(n, "1")+`}}`))
		}},
		{kind: "cody_history.xml", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			messages := strings.Repeat(`<message><speaker value="ASSISTANT"/><text value="ok"/></message>`, n)
			return one(".idea/cody_history.xml", []byte(`<application><component name="ChatHistory"><accountData>`+
				`<list><AccountData><accountId value="a"/><chats><list><chat><internalId value="i"/>`+
				`<updatedAt value="`+long("2026")+`"/><llm><model value="`+long("model")+`"/></llm><messages><list>`+
				messages+`</list></messages></chat></list></chats></AccountData></list></accountData></component></application>`))
		}},
		{kind: "cody_history.xml", shape: shapeCarried + " by the account", files: func(t *testing.T, n int) []growthFile {
			chats := strings.Repeat(`<chat><internalId value="i"/><updatedAt value="2026-01-01T00:00:00"/></chat>`, n)
			return one(".idea/cody_history.xml", []byte(`<application><component name="ChatHistory"><accountData>`+
				`<list><AccountData><accountId value="`+long("a")+`"/><chats><list>`+chats+
				`</list></chats></AccountData></list></accountData></component></application>`))
		}},
		{kind: "cody_history.xml", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			message := `<message><speaker value="HUMAN"/><text value="hi"/></message>`
			chats := strings.Repeat(`<chat><internalId value="i"/><messages>`+message+`</messages></chat>`, n)
			deep := `<chat><messages>` + strings.Repeat("<list>", n) + message + strings.Repeat("</list>", n) +
				`</messages></chat>`
			return one(".idea/cody_history.xml", []byte(`<application>`+chats+deep+`<chat><broken`))
		}},
	}
}

// codySettingCases returns the cases of the editor settings.
func codySettingCases() []growthCase {
	return []growthCase{
		{kind: "settings.json", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".config/Code/User/settings.json", []byte(`{"cody.chat.agenticContext":{"`+long("key")+`":{`+
				joined(n, func(i int) string { return `"` + numbered("m", i) + `":true` })+`}}}`))
		}},
		{kind: "settings.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			key := strings.Repeat("k", 200)
			return one(".config/Code/User/settings.json", []byte(`{// a comment
"cody.agentic.context.experimentalShell":`+nestedObjects(n, key, "true")+`,"cody.mcpServers":{`+
				joined(n, func(i int) string { return `"` + numbered("s", i) + `":{"disabledTools":["x"]}` })+`}}`))
		}},
		{kind: "settings.json", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return numbered(".config/Code", i) + "/User/settings.json" },
				func(i int) string { return `{"cody.mcpServers":{"s":{"disabled":true}}}` })
		}},
		{kind: "state.vscdb", shape: shapeManyFiles, agent: codyEditorAgent, n: 16, files: func(t *testing.T, n int) []growthFile {
			files := make([]growthFile, 0, n)
			for i := range n {
				files = append(files, growthFile{rel: numbered(".config/Code", i) + "/User/globalStorage/state.vscdb",
					data: codyStateDatabase(t, codyItem(codyHistory(codyChat(codyChatDate(i), codyInteraction(i)))))})
			}
			return files
		}},
	}
}

// codyDamaged returns damaged record i: a string of a kilobyte where an object
// belongs, long enough that the work of reading it outweighs the buffers a
// parse reuses from one run to the next.
func codyDamaged(i int) string {
	return `"` + numbered(strings.Repeat("x", 1000), i) + `"`
}
