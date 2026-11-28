package growthtests

import (
	"encoding/base64"
	"strconv"
	"testing"
)

// The Letta growth cases: the transcripts and the sessions.jsonl and
// settings.json each of them looks up, the local backend's conversations,
// the settings, crons, runs, channel routes and the listener state of an
// interrupted turn.

// The paths of one local agent's transcript, one server agent's, one
// local-backend conversation and one listener state file.
const (
	lettaLocalTranscript  = ".letta/transcripts/agent-local-a1/c1/transcript.jsonl"
	lettaServerTranscript = ".letta/transcripts/agent-s1/"
	lettaListener         = ".letta/listener-state/0123456789abcdef01234567/agent-a1_c1.json"
)

// The parts of a path the case test keeps: a transcript's agent directory
// and a local-backend conversation's directory.
const (
	lettaKeptAgent = `transcripts/([^/]+)/`
	lettaKeptKey   = `conversations/([^/]+)/`
)

// lettaBackend returns the path of the local backend's messages of the
// conversation key, its directory named by the key in base64url.
func lettaBackend(key string) string {
	return ".letta/lc-local-backend/conversations/" + base64.RawURLEncoding.EncodeToString([]byte(key)) +
		"/messages.jsonl"
}

// lettaTranscriptLine returns a transcript line of a user turn captured at
// second i.
func lettaTranscriptLine(i int) string {
	return `{"kind":"user","text":"` + numbered("hello ", i) + `","captured_at":"2026-01-01T00:00:` +
		strconv.Itoa(10+i%50) + `Z"}`
}

// lettaTranscriptCases returns the cases of the transcripts: a project
// every line takes from sessions.jsonl, a server every server agent's
// transcript names, and many transcripts looking up one sessions.jsonl.
func lettaTranscriptCases() []growthCase {
	return []growthCase{
		// A transcript's agent directory is looked up by the agent ids
		// of sessions.jsonl and settings.json.
		{kind: "transcript", shape: shapeCarried + " (the project of sessions.jsonl)", keptCase: lettaKeptAgent,
			files: func(t *testing.T, n int) []growthFile {
				return []growthFile{text(".letta/sessions.jsonl", `{"agent_id":"agent-local-a1","session_id":"s1",`+
					`"project":"/`+long("p")+`","timestamp":"2026-01-01T00:00:00Z","model":"m"}`+"\n"),
					{rel: lettaLocalTranscript, data: lines(n, lettaTranscriptLine)}}
			}},
		// A transcript's agent directory is looked up by the agent ids of
		// sessions.jsonl and settings.json.
		{kind: "transcript", shape: shapeLargeShared + " (the server of settings.json)", n: 32, keptCase: lettaKeptAgent,
			files: func(t *testing.T, n int) []growthFile {
				return append(manyFiles(n, func(i int) string {
					return lettaServerTranscript + numbered("c", i) + "/transcript.jsonl"
				},
					func(i int) string { return lettaTranscriptLine(i) + "\n" }),
					text(".letta/settings.json", `{"agents":[{"agentId":"agent-s1","baseUrl":"https://`+long("h")+`"}]}`))
			}},
		// A transcript's agent directory is looked up by the agent ids of
		// sessions.jsonl and settings.json.
		{kind: "transcript", shape: shapeManyFiles, n: 32, keptCase: lettaKeptAgent,
			files: func(t *testing.T, n int) []growthFile {
				sessions := lines(n, func(i int) string {
					return `{"agent_id":"agent-local-a1","session_id":"` + numbered("s", i) + `","project":"` +
						numbered("/p", i) + `","timestamp":"2026-01-01T00:00:` + strconv.Itoa(10+i%50) + `Z"}`
				})
				return append(manyFiles(n, func(i int) string {
					return ".letta/transcripts/agent-local-a1/" + numbered("c", i) + "/transcript.jsonl"
				},
					func(i int) string {
						return lettaTranscriptLine(i) + "\n" + `{"kind":"tool_call","name":"Bash","argsText":"{\"command\":\"ls\"}",` +
							`"resultText":"ok","resultOk":false,"captured_at":"2026-01-01T00:00:30Z"}` + "\n"
					}), growthFile{rel: ".letta/sessions.jsonl", data: sessions})
			}},
		{kind: "transcript", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(lettaLocalTranscript, append(lines(n, func(i int) string { return `{"kind":` }),
				[]byte(`{"kind":"user","text":`+nestedArrays(n, `"x"`)+`}`+"\n")...))
		}},
	}
}

// lettaBackendCases returns the cases of the local backend's
// conversations: a cwd every entry's row inherits, many copies of one
// message id, and schema 1 bare messages with damaged lines.
func lettaBackendCases() []growthCase {
	path := lettaBackend("conversation:c1")
	return []growthCase{
		// The directory name is the conversation key in base64url.
		{kind: "local backend messages", keptCase: lettaKeptKey, shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(path, piSession(piHeader("c1", "/"+long("p"), ""), n))
		}},
		// The directory name is the conversation key in base64url.
		{kind: "local backend messages", keptCase: lettaKeptKey, shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(path, append([]byte(piHeader("c1", "/p", "")+"\n"), lines(n, func(i int) string {
				return `{"type":"message","id":"` + numbered("e", i) + `","timestamp":"2026-01-01T00:00:01Z",` +
					`"message":{"id":"same","role":"user","content":"` + numbered("draft ", i) + `"}}`
			})...))
		}},
		// The directory name is the conversation key in base64url.
		{kind: "local backend messages", keptCase: lettaKeptKey, shape: shapeNested + " (schema 1 bare messages)",
			files: func(t *testing.T, n int) []growthFile {
				return one(path, append(lines(n, func(i int) string {
					if i%2 == 1 {
						return `{"role":`
					}
					return `{"role":"user","content":"hi","metadata":{"created_at":"2026-01-01T00:00:00Z"}}`
				}), []byte(`{"role":"user","content":`+nestedArrays(n, `"x"`)+`}`+"\n")...))
			}},
	}
}

// lettaSettingsCases returns the cases of the three settings files and of
// sessions.jsonl: lists given many times, keys sharing one name, and
// damaged entries.
func lettaSettingsCases() []growthCase {
	settings := func(n int) []byte {
		return []byte(`{"permissions":{"mode":"default","allow":[` + joined(n, func(i int) string {
			return `"` + numbered("Bash(cmd", i) + `)"`
		}) + `],"deny":` + nestedArrays(n, `"x"`) + `,"ask":{"bad":1}},"sessionsByServer":{` +
			joined(n, func(i int) string {
				return `"` + numbered("https://h", i) + `":{"agentId":"a","conversationId":"` + numbered("c", i) + `"}`
			}) + `},"env":{` + joined(n, func(i int) string { return `"` + numbered("KEY", i) + `":"secret"` }) +
			`},"refreshToken":"tok"}`)
	}
	return []growthCase{
		{kind: "settings.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".letta/settings.json", settings(n))
		}},
		// The transcript's agent directory is looked up by settings.json's
		// agent ids.
		{kind: "settings.json", shape: shapeSharedID + " (agents sharing one id)", keptCase: lettaKeptAgent,
			files: func(t *testing.T, n int) []growthFile {
				return []growthFile{text(".letta/settings.json", `{"agents":[`+joined(n, func(i int) string {
					return `{"agentId":"agent-s1","baseUrl":"` + numbered("https://h", i) + `"}`
				})+`]}`), text(lettaServerTranscript+"c1/transcript.jsonl", lettaTranscriptLine(0)+"\n")}
			}},
		{kind: "settings.local.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".letta/settings.local.json", settings(n))
		}},
		{kind: "legacy settings.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".config/letta/settings.json", settings(n))
		}},
		{kind: "sessions.jsonl", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".letta/sessions.jsonl", lines(n, func(i int) string {
				return `{"agent_id":"a1","session_id":"same","project":"/p","timestamp":"2026-01-01T00:00:00Z",` +
					`"model":"` + numbered("m", i%2) + `","provider":"letta"}`
			}))
		}},
		{kind: "sessions.jsonl", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".letta/sessions.jsonl", append(lines(n, func(i int) string { return `{"agent_id":` }),
				[]byte(`{"agent_id":"a1","exit_reason":"quit","duration":`+nestedObjects(n, "wall_ms", "1")+`}`+"\n")...))
		}},
	}
}

// lettaTaskCases returns the cases of the crons, the runs, the channel
// routes and the listener state: records sharing one id, a working
// directory every result of an interrupted turn inherits, and damaged
// entries.
func lettaTaskCases() []growthCase {
	return []growthCase{
		{kind: "crons.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".letta/crons.json", []byte(`{"tasks":[`+joined(n, func(i int) string {
				return `{"id":"same","name":"daily","status":"active","cron":"0 9 * * *","agent_id":"a1",` +
					`"conversation_id":"c1","created_at":"2026-01-01T00:00:00Z","prompt":"check"}`
			})+`]}`))
		}},
		{kind: "crons.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(".letta/crons.json", []byte(`{"tasks":[`+joined(n, strconv.Itoa)+`,{"id":"deep","prompt":`+
				nestedArrays(n, `"x"`)+`}]}`))
		}},
		{kind: "run", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".letta/runs/task1.jsonl", lines(n, func(i int) string {
				return `{"jobId":"same","action":"fire","status":"ok","conversationId":"c1","ts":"2026-01-01T00:00:00Z"}`
			}))
		}},
		{kind: "run", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".letta/runs/task1.jsonl", append(lines(n, func(i int) string { return `{"jobId":` }),
				[]byte(`{"jobId":"deep","summary":`+nestedArrays(n, `"x"`)+`}`+"\n")...))
		}},
		{kind: "channel routing", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".letta/channels/telegram/routing.json", []byte(`{"routes":[`+joined(n, func(i int) string {
				return `{"chatId":"same","chatType":"private","agentId":"a1","conversationId":"c1","enabled":true}`
			})+`]}`))
		}},
		{kind: "channel routing", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(".letta/channels/slack/routing.yaml", []byte(`{"routes":[`+joined(n, strconv.Itoa)+
				`,{"chatId":`+nestedArrays(n, `"x"`)+`}]}`))
		}},
		{kind: "listener state", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(lettaListener, []byte(`{"agentId":"a1","conversationId":"c1","runId":"r1",`+
				`"workingDirectory":"/`+long("w")+`","results":[`+joined(n, func(i int) string {
				return `{"tool_call_id":"` + numbered("t", i) + `","approve":false,"reason":"no"}`
			})+`]}`))
		}},
		{kind: "listener state", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(lettaListener, []byte(`{"agentId":"a1","conversationId":"c1","toolCallIds":[`+
				joined(n, func(i int) string { return `"same"` })+`],"results":[`+joined(n, func(i int) string {
				return `{"tool_call_id":"same","tool_return":"done","status":"error"}`
			})+`,7]}`))
		}},
	}
}

// lettaGrowth returns the letta growth set.
func lettaGrowth() growthSet {
	var cases []growthCase
	for _, part := range [][]growthCase{lettaTranscriptCases(), lettaBackendCases(), lettaSettingsCases(),
		lettaTaskCases()} {
		cases = append(cases, part...)
	}
	return growthSet{parser: "letta", cases: cases}
}
