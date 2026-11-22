package parsers

import (
	"slices"
	"strconv"
	"testing"
)

// The Gemini CLI growth cases: the session logs, replayed record by record
// with their $set, $patch and $rewindTo mutations; the past-use stores
// beside them under tmp/<slug>/, each of which looks up its project in the
// slug's .project_root and in projects.json; and the standing files.

// The runtime directory, a project's directory under it, and that
// project's side files.
const (
	geminiBase    = ".gemini/"
	geminiProject = geminiBase + "tmp/proj/"
	geminiRoot    = geminiProject + ".project_root"
	geminiIndex   = geminiBase + "projects.json"
	geminiSession = geminiProject + "chats/session-2026-01-02T03-04-abcdef.jsonl"
)

// geminiHeader returns a session file's first record, naming the
// projectHash.
func geminiHeader(projectHash string) string {
	return `{"sessionId":"s","projectHash":"` + projectHash + `","startTime":"2026-01-02T03:04:05Z","kind":"main"}`
}

// geminiMessage returns a message record of the id, type and text.
func geminiMessage(messageID, kind, text string) string {
	return `{"id":"` + messageID + `","timestamp":"2026-01-02T03:04:06Z","type":"` + kind + `","content":[{"text":"` +
		text + `"}]}`
}

// geminiItem returns a history item of a chat checkpoint, tool checkpoint
// or error report: a user turn of the text.
func geminiItem(text string) string {
	return `{"role":"user","parts":[{"text":"` + text + `"}]}`
}

// geminiSessionLines returns a session file: the header, then the lines.
func geminiSessionLines(projectHash string, n int, line func(i int) string) []byte {
	return append([]byte(geminiHeader(projectHash)+"\n"), lines(n, line)...)
}

// geminicliGrowth returns the gemini-cli growth set.
func geminicliGrowth() growthSet {
	return growthSet{parser: "gemini-cli", cases: slices.Concat(geminiSessionCases(), geminiReplayCases(),
		geminiStoreCases(), geminiCopyCases(), geminiStandingCases(), geminiGrantCases())}
}

// geminiSessionCases returns the cases of the session files' messages.
func geminiSessionCases() []growthCase {
	return []growthCase{
		{kind: "session", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(geminiSession, geminiSessionLines(long("hash"), n, func(i int) string {
				return geminiMessage(numbered("m", i), "user", numbered("hi", i))
			}))
		}},
		{kind: "session", shape: shapeManyFiles, files: func(t *testing.T, n int) []growthFile {
			index := text(geminiIndex, `{"projects":{`+joined(n, func(i int) string {
				return `"/p` + strconv.Itoa(i) + `":"` + numbered("slug", i) + `"`
			})+`,"/w":"proj"}}`)
			return append(manyFiles(n, func(i int) string { return geminiProject + "chats/" + numbered("s", i) + ".jsonl" },
				func(i int) string {
					return geminiHeader("h") + "\n" + geminiMessage("m", "user", "hi") + "\n"
				}), index)
		}},
		{kind: "session", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			deep := `{"id":"d","type":"gemini","content":"x","toolCalls":[{"id":"c","name":"n","args":` +
				nestedArrays(n, "1") + `}]}`
			return one(geminiSession, append(geminiSessionLines("h", n, func(i int) string {
				return `{"id":"` + numbered("m", i) + `","type":"gemini","toolCalls":7,"thoughts":[1]}`
			}), []byte(deep+"\n"+`{"id":`+"\n")...))
		}},
		{kind: "session", shape: shapeSharedID + " (legacy file)", files: func(t *testing.T, n int) []growthFile {
			return one(geminiProject+"chats/session-1.json", []byte(`{"sessionId":"s","projectHash":"h","messages":[`+
				joined(n, func(i int) string { return geminiMessage("m", "user", numbered("hi", i)) })+`]}`))
		}},
	}
}

// geminiReplayCases returns the cases of the session files' mutations.
func geminiReplayCases() []growthCase {
	call := `{"id":"big","type":"gemini","content":"` + long("content") + `","toolCalls":[{"id":"c","name":"read",` +
		`"args":{},"status":"success"}]}`
	return []growthCase{
		{kind: "session", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			return one(geminiSession, append([]byte(geminiHeader("h")+"\n"+call+"\n"), lines(n, func(i int) string {
				return `{"$patch":{"id":"big","toolCalls":[{"id":"c","result":"` + numbered("r", i) + `"}]}}`
			})...))
		}},
		{kind: "session", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(geminiSession, geminiSessionLines("h", 3*n, func(i int) string {
				switch {
				case i < n:
					return geminiMessage("m", "user", numbered("hi", i))
				case i < 2*n:
					return `{"$patch":{"orderIds":["m"],"updates":[{"id":"m","content":"` + numbered("u", i) + `"}]}}`
				}
				return geminiMessage(numbered("n", i), "gemini", "a") + "\n" + `{"$rewindTo":"` + numbered("n", i) + `"}`
			}))
		}},
		{kind: "session", shape: shapeSharedID + " ($set)", files: func(t *testing.T, n int) []growthFile {
			return one(geminiSession, geminiSessionLines("h", n, func(i int) string {
				return `{"$set":{"messages":[` + geminiMessage("m", "user", numbered("hi", i)) + `],"summary":"s"}}`
			}))
		}},
	}
}

// geminiStoreCases returns the cases of the stores beside the sessions,
// each looking up its project.
func geminiStoreCases() []growthCase {
	longRoot := text(geminiRoot, "/"+long("root")+"\n")
	return []growthCase{
		{kind: "logs.json", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{longRoot, text(geminiProject+"logs.json", "["+joined(n, func(i int) string {
				return `{"sessionId":"s","messageId":` + strconv.Itoa(i) + `,"type":"user","message":"hi",` +
					`"timestamp":"2026-01-02T03:04:05Z"}`
			})+"]")}
		}},
		{kind: "logs.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(geminiProject+"logs.json", []byte("["+joined(n, strconv.Itoa)+`,{"sessionId":"s","message":`+
				nestedArrays(n, `"x"`)+"}]"))
		}},
		{kind: "shell_history", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{longRoot, {rel: geminiProject + "shell_history",
				data: lines(n, func(i int) string { return numbered("ls ", i) })}}
		}},
		{kind: "shell_history", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(geminiProject+"shell_history", append(lines(n, func(i int) string { return `echo a \` }),
				lines(n, func(i int) string { return "bad \xff" })...))
		}},
		{kind: "session activity log", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			request := `{"type":"network","sessionId":"s","timestamp":"2026-01-02T03:04:05Z","payload":{"id":"r",` +
				`"method":"POST","url":"https://example.com/` + long("path") + `"}}`
			return one(geminiProject+"logs/session-s.jsonl", append([]byte(request+"\n"), lines(n, func(i int) string {
				return `{"type":"network","sessionId":"s","payload":{"id":"r","response":{"status":200,"durationMs":` +
					strconv.Itoa(i) + `}}}`
			})...))
		}},
		{kind: "session activity log", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(geminiProject+"logs/session-s.jsonl", append(lines(n, func(i int) string {
				return `{"type":"other","payload":` + strconv.Itoa(i) + `}`
			}), []byte(`{"type":"console","payload":{"type":"log","content":`+nestedArrays(n, `"x"`)+`}}`+"\n")...))
		}},
		{kind: "memory", shape: shapeManyFiles, files: func(t *testing.T, n int) []growthFile {
			return append(manyFiles(n, func(i int) string { return geminiProject + "memory/" + numbered("note", i) + ".md" },
				func(i int) string { return numbered("remember ", i) }), text(geminiRoot, "/w\n"))
		}},
		{kind: "clipboard image", shape: shapeManyFiles, files: func(t *testing.T, n int) []growthFile {
			return append(manyFiles(n, func(i int) string {
				return geminiProject + "images/clipboard-" + strconv.Itoa(1767323045000+i) + ".png"
			}, func(i int) string { return "\x89PNG" }), text(geminiIndex, `{"projects":{"/w":"proj"}}`))
		}},
	}
}

// geminiCopyCases returns the cases of the history copies: the chat
// checkpoints, the bug report histories, the tool checkpoints and the
// error reports.
func geminiCopyCases() []growthCase {
	const toolCheckpoint = geminiProject + "checkpoints/2026-10-01T09-00-21_500Z-util.ts-replace.json"
	const errorName = "gemini-client-error-Turn.run-2026-10-01T09-00-21-500Z.json"
	items := func(n int) string { return joined(n, func(i int) string { return geminiItem(numbered("hi", i)) }) }
	return []growthCase{
		{kind: "checkpoint", shape: shapeManyFiles, files: func(t *testing.T, n int) []growthFile {
			index := text(geminiIndex, `{"projects":{`+joined(n, func(i int) string {
				return `"/p` + strconv.Itoa(i) + `":"` + numbered("slug", i) + `"`
			})+`}}`)
			return append(manyFiles(n, func(i int) string { return geminiProject + numbered("checkpoint-tag", i) + ".json" },
				func(i int) string { return `{"history":[` + geminiItem("hi") + `],"authType":"oauth"}` }), index)
		}},
		{kind: "checkpoint", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(geminiProject+"checkpoint-x.json", []byte(`{"history":[`+joined(n, strconv.Itoa)+","+
				geminiItem("hi")+`,{"role":"user","parts":[{"functionCall":{"name":"n","args":`+nestedArrays(n, "1")))
		}},
		{kind: "bug report history", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(geminiProject+"bug-report-history-1767323045000.json", []byte("["+items(n)+","+
				nestedArrays(n, "1")+`,{"role":`))
		}},
		{kind: "tool checkpoint", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(toolCheckpoint, []byte(`{"messageId":"`+long("session")+`########3","commitHash":"abc",`+
				`"toolCall":{"name":"replace","args":{"file_path":"/w/a.ts"}},"clientHistory":[`+items(n)+`]}`))
		}},
		{kind: "tool checkpoint", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(toolCheckpoint, []byte(`{"messageId":"s########1","clientHistory":[`+joined(n, strconv.Itoa)+
				`,{"role":"model","parts":[{"text":`+nestedArrays(n, `"x"`)+`}]}]}`))
		}},
		{kind: "error report", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(geminiBase+errorName, []byte(`{"error":{"message":"boom","stack":"at x"},"context":[`+
				joined(n, strconv.Itoa)+`,{"role":"user","parts":`+nestedArrays(n, `"x"`)+`}]}`))
		}},
		{kind: "project error report", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{text(geminiRoot, "/"+long("root")), text(geminiProject+errorName,
				`{"error":{"message":"boom"},"context":{"requestContents":[`+items(n)+`]}}`)}
		}},
	}
}

// geminiStandingCases returns the cases of trustedFolders.json,
// settings.json and the policies.
func geminiStandingCases() []growthCase {
	return []growthCase{
		{kind: "trustedFolders.json", shape: shapeSharedID, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(geminiBase+"trustedFolders.json", []byte("{\n"+joined(n, func(i int) string {
				return "  // trusted\n  \"/w\": \"TRUST_FOLDER\""
			})+",\n}"))
		}},
		{kind: "trustedFolders.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(geminiBase+"trustedFolders.json", []byte(`{"/deep":`+nestedArrays(n, "1")+","+
				joined(n, func(i int) string { return `"` + numbered("/p", i) + `":` + strconv.Itoa(i) })+"}"))
		}},
		{kind: "settings.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(geminiBase+"settings.json", []byte(`{"tools":{"allowed":[`+joined(n, func(i int) string {
				return `"run_shell_command(ls)"`
			})+`],"sandbox":"docker"},"general":{"sessionRetention":{"enabled":true,"maxAge":"30d"}}}`))
		}},
		{kind: "settings.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(geminiBase+"settings.json", []byte(`{"tools":{"exclude":[`+joined(n, strconv.Itoa)+`],"core":`+
				nestedArrays(n, `"x"`)+`},"security":`+nestedObjects(n, "folderTrust", "1")+`}`))
		}},
		{kind: "policy", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(geminiBase+"policies/p.toml", lines(n, func(i int) string {
				return "[[rule]]\ntoolName = \"run_shell_command\"\ndecision = \"allow\"\npriority = " + strconv.Itoa(i)
			}))
		}},
		{kind: "policy", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(geminiBase+"policies/sandbox.toml", []byte("rule = ["+joined(n, strconv.Itoa)+
				", {decision = \"deny\"}]\n"+string(lines(n, func(i int) string {
				return "[commands." + numbered("cmd", i) + "]\nallow_network = true"
			}))))
		}},
	}
}

// geminiGrantCases returns the cases of trusted_hooks.json and the agents
// acknowledgments, whose rows each name their project key.
func geminiGrantCases() []growthCase {
	return []growthCase{
		{kind: "trusted_hooks.json", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(geminiBase+"trusted_hooks.json", []byte(`{"/`+long("w")+`":[`+joined(n, func(i int) string {
				return `"` + numbered("hook", i) + `"`
			})+`]}`))
		}},
		{kind: "trusted_hooks.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(geminiBase+"trusted_hooks.json", []byte(`{"/w":[`+nestedArrays(n, `"h"`)+`],`+
				joined(n, func(i int) string { return `"` + numbered("/p", i) + `":` + strconv.Itoa(i) })+`}`))
		}},
		{kind: "agents acknowledgments", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(geminiBase+"acknowledgments/agents.json", []byte(`{"/`+long("w")+`":{`+
				joined(n, func(i int) string { return `"` + numbered("agent", i) + `":"hash"` })+`}}`))
		}},
		{kind: "agents acknowledgments", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(geminiBase+"acknowledgments/agents.json", []byte(`{"/w":{"a":`+nestedObjects(n, "k", "1")+`},`+
				joined(n, func(i int) string { return `"` + numbered("/p", i) + `":` + strconv.Itoa(i) })+`}`))
		}},
	}
}
