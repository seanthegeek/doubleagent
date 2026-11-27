package growthtests

import (
	"slices"
	"strconv"
	"testing"
)

// The Continue growth cases: the session files and the sessions.json index
// they and the dev_data files look up, the five dev_data logs, the cn CLI
// log, the input history and permissions.yaml.

// The directories and files of Continue, by path relative to the home.
const (
	continueSessions = ".continue/sessions/"
	continueIndex    = continueSessions + "sessions.json"
	continueDevData  = ".continue/dev_data/0.2.0/"
)

// continueEntry returns an index entry of the session at the workspace.
func continueEntry(sessionID, workspace string) string {
	return `{"sessionId":"` + sessionID + `","title":"t","dateCreated":"1767323045000","workspaceDirectory":"` +
		workspace + `"}`
}

// continueItem returns a history item of a message of the role holding
// content, with the members after it.
func continueItem(role, content, rest string) string {
	return `{"message":{"role":"` + role + `","content":"` + content + `"` + rest + `},"contextItems":[]}`
}

// continueSession returns a session file's text: its id, workspace and
// history items.
func continueSession(sessionID, workspace, items string) string {
	return `{"sessionId":"` + sessionID + `","title":"t","workspaceDirectory":"` + workspace +
		`","chatModelTitle":"gpt","history":[` + items + `]}`
}

// continueSessionCases returns the cases of the session files.
func continueSessionCases() []growthCase {
	return []growthCase{
		{kind: "session", shape: shapeManyFiles, files: func(t *testing.T, n int) []growthFile {
			index := text(continueIndex, "["+joined(n, func(i int) string { return continueEntry(numbered("s", i), "/w") })+"]")
			return append(manyFiles(n, func(i int) string { return continueSessions + numbered("s", i) + ".json" },
				func(i int) string {
					return continueSession(numbered("s", i), "", continueItem("user", "hi", ""))
				}), index)
		}},
		{kind: "session", shape: shapeSharedID + " in the index", files: func(t *testing.T, n int) []growthFile {
			index := text(continueIndex, "["+joined(n, func(i int) string { return continueEntry("s", numbered("/w", i)) })+"]")
			return []growthFile{index, text(continueSessions+"s.json", continueSession("s", "", joined(n, func(i int) string {
				return continueItem("user", numbered("hi", i), "")
			})))}
		}},
		{kind: "session", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(continueSessions+"s.json", []byte(continueSession("s", "/"+long("w"), joined(n, func(i int) string {
				return continueItem("user", numbered("hi", i), "")
			}))))
		}},
		{kind: "session", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			call := continueItem("assistant", "", `,"toolCalls":[{"id":"c","type":"function","function":{"name":"`+
				long("tool")+`","arguments":"{}"}}]`)
			return one(continueSessions+"s.json", []byte(continueSession("s", "/w", call+","+joined(n, func(i int) string {
				return continueItem("tool", numbered("out", i), `,"toolCallId":"c"`)
			}))))
		}},
		{kind: "session", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			deep := continueItem("assistant", "", `,"toolCalls":[{"id":"c","function":{"name":"x","arguments":`+
				nestedArrays(n, "1")+`}}]`)
			damaged := continueSession("s", "/w", joined(n, func(i int) string {
				return continueItem("user", numbered("hi", i), "")
			})+","+deep+`,{"message":`)
			return one(continueSessions+"s.json", []byte(damaged))
		}},
	}
}

// continueDevDataCases returns the cases of the dev_data logs.
func continueDevDataCases() []growthCase {
	interaction := func(i int) string {
		return `{"sessionId":"s","timestamp":"2026-01-02T03:04:05Z","modelTitle":"gpt","prompt":"` +
			numbered("p", i) + `","completion":"c","feedback":true}`
	}
	sharedIndex := text(continueIndex, "["+continueEntry("s", "/"+long("w"))+"]")
	return []growthCase{
		{kind: "dev_data chatInteraction.jsonl", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{sharedIndex, {rel: continueDevData + "chatInteraction.jsonl", data: lines(n, interaction)}}
		}},
		{kind: "dev_data chatInteraction.jsonl", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(continueDevData+"chatInteraction.jsonl", append(lines(n, func(i int) string { return `{"prompt":` }),
				[]byte(`{"sessionId":"s","prompt":`+nestedArrays(n, `"x"`)+`}`+"\n")...))
		}},
		{kind: "dev_data chatFeedback.jsonl", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{sharedIndex, {rel: continueDevData + "chatFeedback.jsonl", data: lines(n, interaction)}}
		}},
		{kind: "dev_data toolUsage.jsonl", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(continueDevData+"toolUsage.jsonl", lines(n, func(i int) string {
				return `{"timestamp":"2026-01-02T03:04:05Z","toolCallId":"c","functionName":"read",` +
					`"toolCallArgs":"{}","accepted":true,"succeeded":true,"output":[{"content":"` + numbered("o", i) + `"}]}`
			}))
		}},
		{kind: "dev_data toolUsage.jsonl", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(continueDevData+"toolUsage.jsonl", append(lines(n, strconv.Itoa),
				[]byte(`{"toolCallId":"c","functionParams":`+nestedArrays(n, "1")+`}`+"\n")...))
		}},
		{kind: "dev_data editInteraction.jsonl", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(continueDevData+"editInteraction.jsonl", lines(n, func(i int) string {
				return `{"timestamp":"2026-01-02T03:04:05Z","filepath":"/w/a.go","modelTitle":"gpt","prompt":"p",` +
					`"completion":"` + numbered("c", i) + `"}`
			}))
		}},
		{kind: "dev_data editOutcome.jsonl", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(continueDevData+"editOutcome.jsonl", lines(n, func(i int) string {
				return `{"timestamp":"2026-01-02T03:04:05Z","filepath":"/w/a.go","accepted":true,` +
					`"previousCodeLines":` + strconv.Itoa(i) + `,"newCodeLines":3}`
			}))
		}},
	}
}

// continueCLICases returns the cases of the cn log, the input history and
// permissions.yaml.
func continueCLICases() []growthCase {
	const logLine = "2026-01-02 03:04:05.000 [0000abcd] [info]: "
	const url = "https://api.continue.dev/agents/a/metadata"
	return []growthCase{
		{kind: "cn log", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".continue/logs/cn.log", lines(2*n, func(i int) string {
				if i < n {
					return logLine + "[metadata] POST " + url + ` {"agentId":"a","metadataKeys":["k"]}`
				}
				return logLine + "[metadata] Success " + url
			}))
		}},
		{kind: "cn log", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			deep := logLine + "[metadata] POST " + url + ` {"agentId":` + nestedArrays(n, `"a"`) + "}\n"
			return one(".continue/logs/cn.log", append(lines(n, func(i int) string {
				return logLine + "[metadata] POST " + url + ` {"agentId":` + numbered("damaged", i) + "}"
			}), deep...))
		}},
		{kind: "input_history.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".continue/input_history.json", []byte("["+joined(n, strconv.Itoa)+`,{"text":"hi"},`+
				nestedArrays(n, "1")+"]"))
		}},
		{kind: "input_history.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".continue/input_history.json", []byte("["+joined(n, func(i int) string {
				return `{"text":"same","timestamp":1767323045000}`
			})+"]"))
		}},
		{kind: "permissions.yaml", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".continue/permissions.yaml", append([]byte("allow:\n"), lines(n, func(i int) string {
				return `  - "Bash(ls)"`
			})...))
		}},
		{kind: "permissions.yaml", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(".continue/permissions.yaml", append(append([]byte("allow:\n"), lines(n, func(i int) string {
				return "  - " + numbered("rule", i)
			})...), []byte("  - - nested\n")...))
		}},
	}
}

// continuedevGrowth returns the continue growth set.
func continuedevGrowth() growthSet {
	return growthSet{parser: "continue", cases: slices.Concat(continueSessionCases(), continueDevDataCases(),
		continueCLICases())}
}
