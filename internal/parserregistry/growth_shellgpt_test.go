package parserregistry

import (
	"strconv"
	"strings"
	"testing"
)

// The ShellGPT growth cases: the chat files, the settings, the functions
// and the response cache.

const (
	shellgptChats    = "AppData/Local/Temp/chat_cache/"
	shellgptCache    = "AppData/Local/Temp/shell_gpt/cache/"
	shellgptSettings = ".config/shell_gpt/.sgptrc"
)

// shellgptCacheName returns the 32 hexadecimal digits of the i-th cache
// file's name.
func shellgptCacheName(i int) string {
	digits := strconv.FormatInt(int64(i), 16)
	return strings.Repeat("0", 32-len(digits)) + digits
}

// shellgptToolCall returns an assistant message calling one tool.
func shellgptToolCall(callID, name string) string {
	return `{"role":"assistant","content":null,"tool_calls":[{"id":"` + callID + `","type":"function",` +
		`"function":{"name":"` + name + `","arguments":"{\"shell_command\":\"ls\"}"}}]}`
}

// shellgptGrowth returns the shellgpt growth set.
func shellgptGrowth() growthSet {
	return growthSet{parser: "shellgpt", cases: append(shellgptChatCases(), shellgptFileCases()...)}
}

// shellgptChatCases returns the cases of the chat files.
func shellgptChatCases() []growthCase {
	return []growthCase{
		{kind: "chat", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			return one(shellgptChats+"deploy-check", []byte(`[{"role":"user","content":"go"},`+
				shellgptToolCall("call_1", long("tool"))+","+joined(n, func(i int) string {
				return `{"role":"tool","tool_call_id":"call_1","content":"r` + strconv.Itoa(i) + `"}`
			})+`]`))
		}},
		{kind: "chat", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(shellgptChats+"deploy-check", []byte(`[`+joined(n, func(i int) string {
				return shellgptToolCall("call_1", "execute_shell_command") +
					`,{"role":"tool","tool_call_id":"call_1","content":"ok"}`
			})+`]`))
		}},
		{kind: "chat", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			first := `{"role":"assistant","content":"","function_call":{"name":"` + long("fn") +
				`","arguments":"{}"}},{"role":"function","content":"done"}`
			return one(shellgptChats+"old-functions", []byte(`[`+first+`,`+joined(n, func(i int) string {
				return `{"role":"function","content":"r` + strconv.Itoa(i) + `"}`
			})+`]`))
		}},
		{kind: "chat", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(shellgptChats+"deploy-check", []byte(`[{"role":"user","content":`+nestedArrays(n, `"x"`)+
				`},`+joined(n, func(i int) string { return strconv.Itoa(i) })+`,`+joined(n, func(i int) string {
				return `{"role":"assistant","tool_calls":` + strconv.Itoa(i) + `}`
			})+`,{"role":"user","content":"cut`))
		}},
		{kind: "chat", shape: shapeManyFiles, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return shellgptChats + numbered("chat", i) },
				func(i int) string { return `[{"role":"user","content":"q"},{"role":"assistant","content":"a"}]` })
		}},
	}
}

// shellgptFileCases returns the cases of the settings, functions and
// response cache.
func shellgptFileCases() []growthCase {
	return []growthCase{
		{kind: ".sgptrc", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(shellgptSettings, append([]byte("DEFAULT_MODEL="+long("model")+"\n"),
				lines(n, func(i int) string { return "OPENAI_USE_FUNCTIONS=true" })...))
		}},
		{kind: ".sgptrc", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(shellgptSettings, lines(n, func(i int) string {
				return "sk-damaged-line-" + strconv.Itoa(i) + "\rSHELL_INTERACTION=false"
			}))
		}},
		{kind: "function", shape: shapeManyFiles, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return ".config/shell_gpt/functions/" + numbered("tool", i) + ".py" },
				func(i int) string { return "def function():\n    pass\n" })
		}},
		{kind: "cache", shape: shapeManyFiles, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return shellgptCache + shellgptCacheName(i) },
				func(i int) string { return "cached answer " + strconv.Itoa(i) })
		}},
		{kind: "cache", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(shellgptCache+shellgptCacheName(1), []byte(strings.Repeat("answer\n", n)+long("tail")))
		}},
	}
}
