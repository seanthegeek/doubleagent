package growthtests

import (
	"strconv"
	"strings"
	"testing"
)

// The Qwen Code growth cases: the chat files and their sidecars, the
// prompt logs and past-use stores, the channel files and the standing
// settings.

// qwencodeSession is the session id the chat cases name their file by.
const qwencodeSession = "11111111-2222-3333-4444-555555555555"

// qwencodeChat is the chat file of qwencodeSession.
const qwencodeChat = ".qwen/projects/-p/chats/" + qwencodeSession + ".jsonl"

// qwencodeTime is the time every record of the cases is stamped with.
const qwencodeTime = `"2026-01-02T03:04:05.000Z"`

// qwencodeRecord returns a chat record of the given type whose message
// holds parts, after the other members in rest ("" or ending in a comma).
func qwencodeRecord(kind, rest, parts string) string {
	return `{"type":"` + kind + `",` + rest + `"timestamp":` + qwencodeTime + `,"message":{"role":"model",` +
		`"parts":` + parts + `}}`
}

// qwencodeText returns a parts array of one text part.
func qwencodeText(body string) string {
	return `[{"text":"` + body + `"}]`
}

// qwencodeGrowth returns the qwen-code growth set.
func qwencodeGrowth() growthSet {
	var cases []growthCase
	for _, group := range [][]growthCase{qwencodeChatCases(), qwencodeChatFileCases(), qwencodeLogCases(),
		qwencodeStoreCases(), qwencodeChannelCases(), qwencodeStandingCases()} {
		cases = append(cases, group...)
	}
	return growthSet{parser: "qwen-code", cases: cases}
}

// qwencodeChatCases returns the chat cases of one file's records: a
// carried session id and project, a model named by every record, one
// session id shared by records that switch model, and damaged lines.
func qwencodeChatCases() []growthCase {
	return []growthCase{
		{kind: "chat", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			first := qwencodeRecord("user", `"sessionId":"`+long("s")+`",`, qwencodeText("hi"))
			return one(qwencodeChat, append([]byte(first+"\n"), lines(n, func(i int) string {
				return qwencodeRecord("assistant", "", qwencodeText(numbered("a", i)))
			})...))
		}},
		{kind: "chat", shape: shapeCarried + " into problem lines", files: func(t *testing.T, n int) []growthFile {
			first := qwencodeRecord("user", `"sessionId":"s","cwd":"/`+long("p")+`",`, qwencodeText("hi"))
			return one(qwencodeChat, append([]byte(first+"\n"), lines(n, func(i int) string {
				return numbered(`{"bad`, i)
			})...))
		}},
		{kind: "chat", shape: shapeLargeShared, n: 32, files: func(t *testing.T, n int) []growthFile {
			context := `"sessionId":"s","executionContext":{"modelId":"` + long("model") + `","approvalMode":"yolo"},`
			return one(qwencodeChat, lines(n, func(i int) string {
				return qwencodeRecord("user", context, qwencodeText(numbered("u", i)))
			}))
		}},
		{kind: "chat", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(qwencodeChat, lines(n, func(i int) string {
				context := `"sessionId":"same","executionContext":{"modelId":"m` + strconv.Itoa(i%2) +
					`","approvalMode":"mode` + strconv.Itoa(i%3) + `"},`
				return qwencodeRecord("tool_result", context+`"toolCallResult":{"callId":"c","status":"cancelled"},`,
					`[{"functionResponse":{"id":"c","name":"shell","response":{"output":"o"}}}]`)
			}))
		}},
		{kind: "chat", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			deep := qwencodeRecord("assistant", `"sessionId":"s",`, `[{"functionCall":{"name":"f","args":`+
				nestedObjects(n, "a", `"x"`)+`}},`+joined(n, func(i int) string { return strconv.Itoa(i) })+`]`)
			return one(qwencodeChat, append(lines(n, func(i int) string { return `{"type":"user","message":` }),
				[]byte(deep+"\n")...))
		}},
	}
}

// qwencodeChatFileCases returns the cases of many sessions, each beside
// its worktree sidecar, which the reader looks for per file, and of the
// pull request sidecars.
func qwencodeChatFileCases() []growthCase {
	pullRequests := ".qwen/projects/-p/chats/" + qwencodeSession + ".pr.json"
	return []growthCase{
		// A chat's .worktree.json is opened by the chat's name and that suffix.
		{kind: "chat", shape: shapeManyFiles, n: 32, keptCase: `(\.worktree\.json)\z`, files: func(t *testing.T, n int) []growthFile {
			var files []growthFile
			for i := range n {
				name := ".qwen/projects/-p/chats/" + qwencodeSessionOf(i)
				files = append(files, text(name+".jsonl", qwencodeRecord("user", `"sessionId":"`+
					qwencodeSessionOf(i)+`",`, qwencodeText("hi"))+"\n"),
					text(name+".worktree.json", `{"worktreePath":"/w/`+numbered("t", i)+`"}`))
			}
			return files
		}},
		{kind: "pull requests", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(pullRequests, []byte(`{"prs":[`+joined(n, func(i int) string {
				return `{"url":"https://github.com/o/r/pull/` + strconv.Itoa(i) + `","state":"open",` +
					`"createdAt":` + qwencodeTime + `,"issues":[1,{"number":` + strconv.Itoa(i) + `}]}`
			})+`,{"issues":`+nestedArrays(n, "1")+`}]}`))
		}},
		{kind: "pull requests", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return ".qwen/projects/-p/chats/" + qwencodeSessionOf(i) + ".pr.json" },
				func(i int) string { return `{"prs":[{"url":"https://github.com/o/r/pull/1","source":"cli"}]}` })
		}},
	}
}

// qwencodeSessionOf returns the i-th session id of a case of many
// sessions, a UUID whose last group is i.
func qwencodeSessionOf(i int) string {
	return "11111111-2222-3333-4444-" + strings.Repeat("0", 12-len(strconv.Itoa(i))) + strconv.Itoa(i)
}

// qwencodeLogCases returns the cases of the prompt log and the token
// usage log: one session id shared by every entry, and entries of the
// wrong kind beside a deep one.
func qwencodeLogCases() []growthCase {
	const logs = ".qwen/tmp/abc/logs.json"
	const usage = ".qwen/usage/token-usage-2026-01.jsonl"
	return []growthCase{
		{kind: "logs.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(logs, []byte(`[`+joined(n, func(i int) string {
				kind := []string{`"user","message":"hi"`, `"model_switch","message":"{\"fromModel\":\"a\",` +
					`\"toModel\":\"b\",\"reason\":\"r\"}"`, `"error","message":{"code":1}`}[i%3]
				return `{"sessionId":"same","messageId":0,"timestamp":` + qwencodeTime + `,"type":` + kind + `}`
			})+`]`))
		}},
		{kind: "logs.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(logs, []byte(`[`+joined(n, func(i int) string { return strconv.Itoa(i) })+
				`,{"type":"error","message":`+nestedObjects(n, "a", `"x"`)+`},{"type":`))
		}},
		{kind: "token usage", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(usage, lines(n, func(i int) string {
				return `{"timestamp":` + qwencodeTime + `,"sessionId":"same","model":"qwen3-coder","authType":"o",` +
					`"source":"main","inputTokens":` + strconv.Itoa(i) + `,"outputTokens":2,"totalTokens":3,` +
					`"apiDurationMs":40}`
			}))
		}},
		{kind: "token usage", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(usage, append(lines(n, func(i int) string { return `{"model":` }),
				[]byte(`{"sessionId":"s","model":`+nestedArrays(n, `"x"`)+`}`+"\n")...))
		}},
	}
}

// qwencodeStoreCases returns the cases of the todo files and the Web
// Shell's session organization.
func qwencodeStoreCases() []growthCase {
	const organization = ".qwen/projects/-p/session-organization.v1.json"
	todo := func(i int) string {
		return `{"id":"` + strconv.Itoa(i) + `","content":"c","status":"pending","blockedBy":["1"]}`
	}
	return []growthCase{
		{kind: "todo", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".qwen/todos/a.json", []byte(`{"sessionId":"`+long("s")+`","todos":[`+joined(n, todo)+`]}`))
		}},
		{kind: "todo", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".qwen/todos/a.json", []byte(`{"todos":[`+joined(n, strconv.Itoa)+`,{"blockedBy":`+
				nestedArrays(n, "1")+`}]}`))
		}},
		{kind: "todo", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return ".qwen/todos/" + numbered("s", i) + ".json" },
				func(i int) string { return `{"todos":[` + todo(i) + `]}` })
		}},
		{kind: "session organization", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			return one(organization, []byte(`{"groups":[{"id":"g","name":"`+long("group")+`"}],"sessions":{`+
				joined(n, func(i int) string {
					return `"` + numbered("s", i) + `":{"groupId":"g","color":"red","updatedAt":` + qwencodeTime + `}`
				})+`}}`))
		}},
		{kind: "session organization", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(organization, []byte(`{"groups":[`+joined(n, strconv.Itoa)+`],"sessions":{`+
				joined(n, func(i int) string { return `"` + numbered("s", i) + `":` + strconv.Itoa(i) })+
				`,"deep":{"groupId":`+nestedArrays(n, `"x"`)+`}}}`))
		}},
	}
}

// qwencodeRoute returns a channel route entry of the session.
func qwencodeRoute(session string, i int) string {
	return `{"sessionId":"` + session + `","cwd":"/p","startedAt":` + qwencodeTime + `,"target":` +
		`{"channelName":"tg","chatId":"` + strconv.Itoa(i) + `","senderId":"u","threadId":"t"}}`
}

// qwencodeChannelCases returns the cases of the channel files: routes of
// one session, many daemons' routes, a group history of one chat key with
// one clear after all its messages, and the GitHub channel's audit log.
func qwencodeChannelCases() []growthCase {
	const history = ".qwen/channels/tg-group-history.jsonl"
	const audit = ".qwen/channels/github/bot-0123456789abcdef-github-audit.jsonl"
	return []growthCase{
		{kind: "channels", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".qwen/channels/sessions.json", []byte(`{`+joined(n, func(i int) string {
				return `"` + numbered("k", i) + `":` + qwencodeRoute("same", i)
			})+`}`))
		}},
		{kind: "channels", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".qwen/channels/sessions.json", []byte(`{`+joined(n, func(i int) string {
				return `"` + numbered("k", i) + `":` + strconv.Itoa(i)
			})+`,"deep":{"target":`+nestedObjects(n, "chatId", `"x"`)+`}}`))
		}},
		{kind: "daemon routes", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return ".qwen/channels/daemon/" + numbered("d", i) + "/routes.json" },
				func(i int) string { return `{"k":` + qwencodeRoute("same", i) + `}` })
		}},
		{kind: "group history", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			body := lines(n, func(i int) string {
				return `{"type":"message","key":"chat","entry":{"senderName":"u","text":"` + numbered("m", i) +
					`","timestamp":` + qwencodeTime + `}}`
			})
			return one(history, append(body, []byte(`{"type":"clear","key":"chat","recordedAt":`+qwencodeTime+"}\n")...))
		}},
		{kind: "group history", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(history, append(lines(n, func(i int) string { return `{"type":"other","n":` + strconv.Itoa(i) + `}` }),
				[]byte(`{"type":"message","key":`+nestedArrays(n, "1")+`,"entry":{"text":"x"}}`+"\n")...))
		}},
		{kind: "audit", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(audit, lines(n, func(i int) string {
				return `{"at":` + qwencodeTime + `,"sessionId":"same","repository":"o/r","actor":"a","outcome":"ok",` +
					`"triggerKind":"issue_comment","number":` + strconv.Itoa(i) + `,"commentUrl":"https://github.com/c"}`
			}))
		}},
		{kind: "audit", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(audit, append(lines(n, func(i int) string { return `{"at":` }),
				[]byte(`{"repository":`+nestedArrays(n, `"x"`)+`}`+"\n")...))
		}},
	}
}

// qwencodeStandingCases returns the cases of the trusted folders, the
// settings, the usage records (of the home and of a managed worker) and
// the project summary.
func qwencodeStandingCases() []growthCase {
	usage := func(n int) []byte {
		return lines(n, func(i int) string {
			return `{"sessionId":"same","project":"/p","startTime":` + qwencodeTime + `,"durationMs":5,` +
				`"models":{"qwen":{}},"tools":{"byName":{"shell":{"count":` + strconv.Itoa(i) + `},"read":2}}}`
		})
	}
	return []growthCase{
		{kind: "trustedFolders.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".qwen/trustedFolders.json", []byte(`{`+joined(n, func(i int) string {
				return `"/` + numbered("p", i) + `":"TRUST_FOLDER"`
			})+`,"/deep":`+nestedArrays(n, "1")+`}`))
		}},
		{kind: "settings.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".qwen/settings.json", []byte(`{"permissions":{"allow":[`+joined(n, func(i int) string {
				return `"Bash(` + numbered("c", i) + `)"`
			})+`],"deny":`+nestedArrays(n, `"x"`)+`},"tools":{"approvalMode":"yolo"}}`))
		}},
		{kind: "usage_record.jsonl", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".qwen/usage_record.jsonl", usage(n))
		}},
		{kind: "worker usage record", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".qwen/managed-local/worker-home/usage_record.jsonl", usage(n))
		}},
		{kind: "PROJECT_SUMMARY.md", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".qwen/PROJECT_SUMMARY.md", lines(n, func(i int) string { return "# " + long(numbered("s", i)) }))
		}},
	}
}
