package parsers

import (
	"strconv"
	"strings"
	"testing"
)

// The Agent Zero growth cases: the chats with their message files, the
// scheduler's tasks, the Telegram chat map, the plugin files and the
// connector's .env.

// agentzeroChats is the directory of the chats under a home install.
const agentzeroChats = "agent-zero/usr/chats/"

// agentzeroHistory returns an agent's history, as the JSON text Agent Zero
// stores it, holding the messages in its current topic.
func agentzeroHistory(messages string) string {
	history := `{"_cls":"History","current":{"_cls":"Topic","messages":[` + messages + `]}}`
	return `"` + strings.ReplaceAll(history, `"`, `\"`) + `"`
}

// agentzeroChat returns a chat.json of the id whose data is data, whose
// main agent's history holds the messages and whose log holds the items.
func agentzeroChat(identifier, data, messages, items string) string {
	return `{"id":"` + identifier + `","name":"chat","created_at":"2026-01-01T00:00:00Z","data":` + data +
		`,"agents":[{"number":0,"history":` + agentzeroHistory(messages) + `}],"log":{"logs":[` + items + `]}}`
}

// agentzeroTurns returns n log items: a user prompt, the agent's reply and
// a tool call, in turn, each with its own id.
func agentzeroTurns(n int) string {
	return joined(n, func(i int) string {
		number := strconv.Itoa(i)
		switch i % 3 {
		case 0:
			return `{"no":` + number + `,"id":"l` + number + `","type":"user","content":"ask ` + number + `"}`
		case 1:
			return `{"no":` + number + `,"id":"l` + number + `","type":"agent","kvps":{"headline":"h",` +
				`"reasoning":"r"},"content":"reply"}`
		}
		return `{"no":` + number + `,"id":"l` + number + `","type":"tool","kvps":{"_tool_name":"code",` +
			`"code":"ls"},"content":"out"}`
	})
}

// agentzeroGrowth returns the agent-zero growth set.
func agentzeroGrowth() growthSet {
	var cases []growthCase
	cases = append(cases, agentzeroChatCases()...)
	cases = append(cases, agentzeroJoinCases()...)
	cases = append(cases, agentzeroFileCases()...)
	cases = append(cases, agentzeroPluginCases()...)
	return growthSet{parser: "agent-zero", cases: cases}
}

// agentzeroChatCases returns the cases of a chat's own values carried onto
// its rows, of its nesting, and of many chats.
func agentzeroChatCases() []growthCase {
	const chat = agentzeroChats + "c1/chat.json"
	return []growthCase{
		{kind: "chat.json", shape: shapeCarried + " by the editor's project", files: func(t *testing.T, n int) []growthFile {
			return one(chat, []byte(agentzeroChat("c1", `{"acp_cwd":"/`+long("p")+`"}`, "", agentzeroTurns(n))))
		}},
		{kind: "chat.json", shape: shapeCarried + " by the chat id", files: func(t *testing.T, n int) []growthFile {
			return one(chat, []byte(agentzeroChat(long("c"), `{}`, "", agentzeroTurns(n))))
		}},
		{kind: "chat.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			message := `{"_cls":"Message","id":"m","ai":false,"content":{"user_message":"x"}}`
			deep := strings.Repeat(`{"_cls":"Topic","messages":[`, n) + message + strings.Repeat("]}", n)
			history := `"` + strings.ReplaceAll(`{"_cls":"History","current":`+deep+`}`, `"`, `\"`) + `"`
			return one(chat, []byte(`{"id":"c1","data":{},"agents":[{"number":0,"history":`+history+
				`},`+joined(n, agentzeroDamaged)+`],"log":{"logs":[`+joined(n, agentzeroDamaged)+`,{"no":1,"id":"m","type":"user","content":`+nestedArrays(n, `"x"`)+`}]}}`))
		}},
		{kind: "chat.json", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			files := manyFiles(n, func(i int) string { return agentzeroChats + numbered("c", i) + "/chat.json" },
				func(i int) string {
					return agentzeroChat(numbered("c", i), `{"project":"p"}`, "", agentzeroTurns(3))
				})
			return append(files, manyFiles(n, func(i int) string {
				return agentzeroChats + numbered("c", i) + "/messages/" + numbered("m", i) + ".txt"
			}, func(i int) string { return "output" })...)
		}},
		{kind: "legacy chat", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(agentzeroChats+"c1.json", []byte(agentzeroChat("c1", `{"acp_cwd":"/`+long("p")+`"}`, "",
				agentzeroTurns(n))))
		}},
		{kind: "legacy chat", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return "agent-zero/tmp/chats/" + numbered("c", i) + ".json" },
				func(i int) string { return agentzeroChat(numbered("c", i), `{}`, "", agentzeroTurns(3)) })
		}},
	}
}

// agentzeroJoinCases returns the cases of log items joined to the history
// and to the message files.
func agentzeroJoinCases() []growthCase {
	const chat = agentzeroChats + "c1/chat.json"
	return []growthCase{
		{kind: "chat.json", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			messages := `{"_cls":"Message","id":"t","ai":false,"content":{"tool_name":"code","file":"big.txt"}}`
			return []growthFile{text(agentzeroChats+"c1/messages/big.txt", long("output")),
				text(chat, agentzeroChat("c1", `{}`, messages, joined(n, func(i int) string {
					return `{"no":` + strconv.Itoa(i) + `,"id":"t","type":"tool","kvps":{"code":"ls"}}`
				})))}
		}},
		{kind: "chat.json", shape: shapeLargeShared + " by a history message", files: func(t *testing.T, n int) []growthFile {
			messages := `{"_cls":"Message","id":"a","ai":true,"content":"` + long("reply") + `",` +
				`"metadata":{"responses":{"provider_model_key":"m"}}}`
			return one(chat, []byte(agentzeroChat("c1", `{}`, messages, joined(n, func(i int) string {
				return `{"no":` + strconv.Itoa(i) + `,"id":"a","type":"agent","kvps":{}}`
			}))))
		}},
		{kind: "chat.json", shape: shapeCarried + " by a history message's tool name and model", files: func(t *testing.T, n int) []growthFile {
			messages := `{"_cls":"Message","id":"t","ai":false,"content":{"tool_name":"` + long("tool") + `"},` +
				`"metadata":{"responses":{"provider_model_key":"` + long("model") + `"}}}`
			return one(chat, []byte(agentzeroChat("c1", `{}`, messages, joined(n, func(i int) string {
				return `{"no":` + strconv.Itoa(i) + `,"id":"t","type":"` + []string{"tool", "agent"}[i%2] +
					`","kvps":{"code":"ls"},"content":"c"}`
			}))))
		}},
		{kind: "chat.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			messages := joined(n, func(i int) string {
				return `{"_cls":"Message","id":"u","ai":false,"content":{"user_message":"` + numbered("q", i) + `"}}`
			})
			return one(chat, []byte(agentzeroChat("c1", `{}`, messages, joined(n, func(i int) string {
				return `{"no":` + strconv.Itoa(i) + `,"id":"u","type":"user","content":"q"}`
			}))))
		}},
		{kind: "chat.json", shape: shapeLargeShared + " by the messages of many ids", files: func(t *testing.T, n int) []growthFile {
			messages := joined(n, func(i int) string {
				return `{"_cls":"Message","id":"` + numbered("t", i) + `","ai":false,"content":{"tool_name":"code",` +
					`"tool_result":"` + numbered("r", i) + `","file":"big.txt"}}`
			})
			return []growthFile{text(agentzeroChats+"c1/messages/big.txt", long("output")),
				text(chat, agentzeroChat("c1", `{}`, messages, joined(n, func(i int) string {
					return `{"no":` + strconv.Itoa(i) + `,"id":"` + numbered("t", i/2) + `","type":"tool",` +
						`"kvps":{"code":"ls"},"content":"c"}`
				})))}
		}},
	}
}

// agentzeroFileCases returns the cases of the scheduler's tasks, the
// Telegram chat map and the connector's .env.
func agentzeroFileCases() []growthCase {
	const telegram = "agent-zero/usr/plugins/_telegram_integration/state.json"
	return []growthCase{
		{kind: "scheduler tasks.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one("agent-zero/usr/scheduler/tasks.json", []byte(`{"tasks":[`+joined(n, func(i int) string {
				return `{"uuid":"u","name":"task","type":"scheduled","state":"idle","project_name":"p",` +
					`"created_at":"2026-01-01T00:00:00Z","prompt":"do it","last_result":"done",` +
					`"last_run":"2026-01-01T00:00:00Z"}`
			})+"]}"))
		}},
		{kind: "scheduler tasks.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one("agent-zero/tmp/scheduler/tasks.json", []byte(`{"tasks":[`+joined(n, agentzeroDamaged)+`,{"uuid":"u","name":`+nestedArrays(n, `"x"`)+`,"prompt":`+nestedObjects(n, "a", "1")+`}]}`))
		}},
		{kind: "Telegram state.json", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			files := []growthFile{text(telegram, `{"chats":{`+joined(n, func(i int) string {
				return `"bot:` + strconv.Itoa(i) + `:42":"` + numbered("c", i) + `"`
			})+"}}")}
			return append(files, manyFiles(n/2, func(i int) string { return agentzeroChats + numbered("c", i) + "/chat.json" },
				func(i int) string { return agentzeroChat(numbered("c", i), `{}`, "", "") })...)
		}},
		{kind: "Telegram state.json", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			return one(telegram, []byte(`{"chats":{`+joined(n, func(i int) string {
				return `"bot:` + strconv.Itoa(i) + `:42":"gone"`
			})+"}}"))
		}},
		{kind: "connector .env", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".agent-zero/.env", lines(n, func(i int) string {
				return "AGENT_ZERO_LAST_CONTEXT_ID=" + numbered(strings.Repeat("c", 1000), i) + "\nAGENT_ZERO_LAST_CONTEXT_HOST=h"
			}))
		}},
		{kind: "connector .env", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".agent-zero/.env", append([]byte("AGENT_ZERO_LAST_CONTEXT_ID="+long("c")+"\n"),
				lines(n, func(i int) string { return numbered("export KEY", i) + "='value'" })...))
		}},
	}
}

// agentzeroPluginCases returns the cases of the plugin settings and
// toggles.
func agentzeroPluginCases() []growthCase {
	const plugins = "agent-zero/usr/plugins/"
	return []growthCase{
		{kind: "plugin config.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(plugins+"_tool_access/config.json", []byte(`{"mode":"allow","allowed":[`+joined(n,
				func(i int) string { return `"code"` })+`],"blocked":[`+joined(n, func(i int) string {
				return `{"tool":"x"}`
			})+"]}"))
		}},
		{kind: "plugin config.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one("agent-zero/usr/agents/a/plugins/_orchestrator/config.json", []byte(`{"codex":{"bypass_sandbox":`+
				nestedArrays(n, "true")+`},"claude":{"allowed_tools":`+nestedObjects(n, "x", "1")+`},"unread":[`+joined(n, agentzeroDamaged)+`]}`))
		}},
		{kind: "plugin config.json", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string {
				return "agent-zero/usr/projects/" + numbered("p", i) + "/.a0proj/plugins/_tool_access/config.json"
			}, func(i int) string { return `{"default":"deny","allowed":["code"]}` })
		}},
		{kind: "plugin toggle", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string {
				return "agent-zero/usr/projects/" + numbered("p", i) + "/.a0proj/agents/a/plugins/_infection_check/.toggle-" +
					strconv.Itoa(i%2)
			}, func(i int) string { return "" })
		}},
	}
}

// agentzeroDamaged returns damaged record i: a string of a kilobyte where an object
// belongs, long enough that the work of reading it outweighs the buffers a
// parse reuses from one run to the next.
func agentzeroDamaged(i int) string {
	return `"` + numbered(strings.Repeat("x", 1000), i) + `"`
}
