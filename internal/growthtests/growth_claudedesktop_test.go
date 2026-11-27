package growthtests

import (
	"fmt"
	"slices"
	"strconv"
	"testing"
)

// The Claude Desktop growth cases: the transcripts and audit logs of the
// session dirs with the session records they look up, the staged imports,
// and the app's state files.

// The app-data directory, the Cowork and Code tab org dirs under it, and
// the full name of one session dir.
const (
	claudedesktopAppData = "Library/Application Support/Claude/"
	claudedesktopCowork  = claudedesktopAppData + "local-agent-mode-sessions/a/o/"
	claudedesktopCode    = claudedesktopAppData + "claude-code-sessions/a/o/"
	claudedesktopSession = "local_c0c0c0c0-1111-4222-8333-444444444444"
)

// claudedesktopShort returns the short name of the i-th session dir: eight
// hexadecimal digits.
func claudedesktopShort(i int) string {
	return fmt.Sprintf("%08x", i)
}

// claudedesktopRecordName returns the file name of the session record of
// the session dir called short.
func claudedesktopRecordName(short string) string {
	return "local_" + short + "-1111-4222-8333-444444444444.json"
}

// claudedesktopTranscript returns the path of a transcript in the session
// dir at session.
func claudedesktopTranscript(session string) string {
	return session + "/.claude/projects/-p/" + claudeSession + ".jsonl"
}

// claudedesktopAudit returns an audit line of the given type and fields.
func claudedesktopAudit(kind, fields string) string {
	return `{"type":"` + kind + `",` + fields + `,"session_id":"s","_audit_timestamp":"2026-10-01T09:00:00Z"}`
}

// claudedesktopUser returns an audit user line typed in the app holding
// text.
func claudedesktopUser(text string) string {
	return claudedesktopAudit("user", `"uuid":"u","client_platform":"desktop_app",`+
		`"message":{"role":"user","content":"`+text+`"}`)
}

// claudedesktopTranscriptCases returns the cases of the session and staged
// transcripts.
func claudedesktopTranscriptCases() []growthCase {
	session := claudedesktopCowork + claudedesktopSession
	transcript := claudedesktopTranscript(session)
	results := session + "/.claude/projects/-p/" + claudeSession + "/tool-results/"
	staged := claudedesktopCode + "imported-staging/"
	return []growthCase{
		{kind: "session transcript", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{text(claudedesktopCowork+claudedesktopSession+".json",
				`{"title":"t","userSelectedFolders":["/`+long("f")+`"]}`),
				{rel: transcript, data: lines(n, func(i int) string { return claudeLine(numbered("u", i), `"hi"`) })}}
		}},
		{kind: "session transcript", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{text(results+"abcdefghij.txt", long("output")),
				{rel: transcript, data: lines(n, func(i int) string {
					return claudeLine(numbered("u", i), claudeToolResult(i, "abcdefghij.txt"))
				})}}
		}},
		{kind: "session transcript", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(transcript, lines(n, func(i int) string { return claudeLine("same", `"hello"`) }))
		}},
		{kind: "session transcript", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			files := manyFiles(n, func(i int) string {
				return claudedesktopTranscript(claudedesktopCowork + claudedesktopShort(i))
			}, func(i int) string { return claudeLine(numbered("u", i), `"hi"`) + "\n" })
			return append(files, manyFiles(n, func(i int) string {
				return claudedesktopCowork + claudedesktopRecordName(claudedesktopShort(i))
			}, func(i int) string { return `{"title":"t","userSelectedFolders":["/p` + strconv.Itoa(i) + `"]}` })...)
		}},
		{kind: "imported-staging transcript", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(staged+claudeSession+".jsonl", lines(n, func(i int) string { return claudeLine("same", `"hello"`) }))
		}},
		{kind: "imported-staging transcript", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(staged+claudeSession+".jsonl", append(lines(n, func(i int) string { return `{"type":"user","message":` }),
				[]byte(claudeLine("deep", nestedArrays(n, `"x"`))+"\n")...))
		}},
		{kind: "imported-staging transcript", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return staged + numbered("s", i) + ".jsonl" },
				func(i int) string { return claudeLine(numbered("u", i), `"hi"`) + "\n" })
		}},
	}
}

// claudedesktopAuditCases returns the cases of the audit logs.
func claudedesktopAuditCases() []growthCase {
	audit := claudedesktopCowork + claudedesktopSession + "/audit.jsonl"
	record := claudedesktopCowork + claudedesktopSession + ".json"
	return []growthCase{
		{kind: "audit.jsonl", shape: shapeCarried + " from the record", files: func(t *testing.T, n int) []growthFile {
			return []growthFile{text(record, `{"title":"t","cliSessionId":"`+long("c")+`",`+
				`"userSelectedFolders":["/`+long("f")+`"]}`),
				{rel: audit, data: lines(n, func(i int) string { return claudedesktopUser(numbered("hi", i)) })}}
		}},
		{kind: "audit.jsonl", shape: shapeCarried + " from the init line", files: func(t *testing.T, n int) []growthFile {
			first := claudedesktopAudit("system", `"subtype":"init","cwd":"/`+long("c")+`","model":"`+long("m")+`"`)
			return one(audit, append([]byte(first+"\n"), lines(n, func(i int) string {
				return claudedesktopUser(numbered("hi", i))
			})...))
		}},
		{kind: "audit.jsonl", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(audit, lines(n, func(i int) string {
				return claudedesktopAudit("assistant", `"uuid":"same","message":{"model":"m","content":[`+
					`{"type":"tool_use","id":"same","name":"Bash","input":{"command":"ls"}}]}`)
			}))
		}},
		{kind: "audit.jsonl", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			damaged := lines(n, func(i int) string {
				return claudedesktopAudit("user", `"message":{"content":[`+strconv.Itoa(i)+`]}`)
			})
			deep := claudedesktopAudit("system", `"subtype":"model_refusal_fallback","detail":`+nestedArrays(n, `"x"`))
			return one(audit, append(damaged, []byte(deep+"\n")...))
		}},
		{kind: "audit.jsonl", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			files := manyFiles(n, func(i int) string { return claudedesktopCowork + claudedesktopShort(i) + "/audit.jsonl" },
				func(i int) string { return claudedesktopUser("hi") + "\n" })
			return append(files, manyFiles(n, func(i int) string {
				return claudedesktopCowork + claudedesktopRecordName(claudedesktopShort(i))
			}, func(i int) string { return `{"title":"t","cliSessionId":"c` + strconv.Itoa(i) + `"}` })...)
		}},
	}
}

// claudedesktopRecordCases returns the cases of the session records.
func claudedesktopRecordCases() []growthCase {
	record := claudedesktopCowork + claudedesktopSession + ".json"
	return []growthCase{
		{kind: "session record", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(record, []byte(`{"title":"t","cliSessionId":"`+long("c")+`","model":"`+long("m")+`",`+
				`"userSelectedFolders":["/`+long("f")+`"],"createdAt":1790845200000,"approvedToolNames":[`+
				joined(n, func(i int) string { return `"` + numbered("tool", i) + `"` })+`]}`))
		}},
		{kind: "session record", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(record, []byte(`{"title":"t","alwaysAllowedReasons":`+nestedArrays(n, `"x"`)+
				`,"enabledMcpTools":{`+joined(n, func(i int) string {
				return `"t` + strconv.Itoa(i) + `":` + nestedObjects(4, "a", "[]")
			})+`},"sessionPermissionUpdates":{"bad":1}}`))
		}},
		// A CLI transcript is looked up by the name the record's
		// cliSessionId gives it.
		{kind: "session record", shape: shapeManyFiles, n: 16, keptCase: `projects/[^/]+/([^/]+\.jsonl)\z`, files: func(t *testing.T, n int) []growthFile {
			files := manyFiles(n, func(i int) string {
				return claudedesktopCode + claudedesktopRecordName(claudedesktopShort(i))
			}, func(i int) string {
				return `{"title":"t","cliSessionId":"c` + strconv.Itoa(i) + `","initialMessage":"hi",` +
					`"createdAt":1790845200000}`
			})
			return append(files, manyFiles(n, func(i int) string {
				return ".claude/projects/" + numbered("-p", i) + "/" + numbered("c", i) + ".jsonl"
			}, func(i int) string { return "\n" })...)
		}},
		{kind: "session record", shape: shapeSharedID + " (the session dir, holding many slugs)",
			files: func(t *testing.T, n int) []growthFile {
				files := manyFiles(n, func(i int) string {
					return claudedesktopCode + claudedesktopRecordName(claudedesktopShort(i))
				}, func(i int) string {
					return `{"sessionId":"` + claudedesktopSession + `","title":"t","initialMessage":"hi",` +
						`"createdAt":1790845200000}`
				})
				return append(files, manyFiles(n, func(i int) string {
					return claudedesktopCode + claudedesktopSession + "/.claude/projects/" + numbered("-p", i) +
						"/notes.txt"
				}, func(i int) string { return "x" })...)
			}},
		// A CLI transcript is looked up by the name the record's cliSessionId
		// gives it.
		{kind: "session record", shape: shapeUnlisted + " (slug directories)", n: 16,
			keptCase: `projects/[^/]+/([^/]+\.jsonl)\z`, files: func(t *testing.T, n int) []growthFile {
				files := manyFiles(n, func(i int) string {
					return claudedesktopCode + claudedesktopRecordName(claudedesktopShort(i))
				}, func(i int) string {
					return `{"title":"t","cliSessionId":"c` + strconv.Itoa(i) + `","initialMessage":"hi",` +
						`"createdAt":1790845200000}`
				})
				return append(files, manyFiles(n, func(i int) string {
					return ".claude/projects/" + numbered("-p", i) + "/" + numbered("c", i) + ".jsonl"
				}, func(i int) string { return "\n" })...)
			}, unlisted: func(n int) []string {
				rels := make([]string, 0, n)
				for i := range n {
					rels = append(rels, ".claude/projects/"+numbered("-p", i))
				}
				return rels
			}},
	}
}

// claudedesktopStateCases returns the cases of the scheduled tasks, the
// worktree list and the remote session spaces.
func claudedesktopStateCases() []growthCase {
	tasks := claudedesktopCowork + "scheduled-tasks.json"
	worktrees := claudedesktopAppData + "git-worktrees.json"
	spaces := claudedesktopCowork + "remote-session-spaces.json"
	return []growthCase{
		{kind: "scheduled-tasks.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(tasks, []byte(`{"scheduledTasks":[`+joined(n, func(i int) string {
				return `{"id":"same","cronExpression":"0 9 * * *","enabled":true,"cwd":"/p","createdAt":1790845200000}`
			})+`]}`))
		}},
		{kind: "scheduled-tasks.json", shape: shapeNested, n: 1024, files: func(t *testing.T, n int) []growthFile {
			return one(tasks, []byte(`{"scheduledTasks":[`+joined(n, func(i int) string { return strconv.Itoa(i) })+
				`,{"id":"deep","cronExpression":`+nestedArrays(n, `"x"`)+`}]}`))
		}},
		{kind: "scheduled-tasks.json", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string {
				return claudedesktopAppData + "local-agent-mode-sessions/a/" + numbered("o", i) + "/scheduled-tasks.json"
			}, func(i int) string { return `{"scheduledTasks":[{"id":"t","fireAt":1790845200000}]}` })
		}},
		{kind: "git-worktrees.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(worktrees, []byte(`{"worktrees":{`+joined(n, func(i int) string {
				return `"same":{"path":"/w` + strconv.Itoa(i) + `","branch":"b","baseRepo":"/r","createdAt":1790845200000}`
			})+`}}`))
		}},
		{kind: "git-worktrees.json", shape: shapeNested, n: 512, files: func(t *testing.T, n int) []growthFile {
			return one(worktrees, []byte(`{"worktrees":{`+joined(n, func(i int) string {
				return `"w` + strconv.Itoa(i) + `":` + strconv.Itoa(i)
			})+`,"deep":{"path":"/w","leasedBy":`+nestedArrays(n, `"x"`)+`}}}`))
		}},
		{kind: "remote-session-spaces.json", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(spaces, []byte(`{"entries":[{"sessionId":"`+long("s")+`","folders":[`+
				joined(n, func(i int) string { return `"/` + numbered("f", i) + `"` })+`]}]}`))
		}},
		{kind: "remote-session-spaces.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(spaces, []byte(`{"entries":[`+joined(n, func(i int) string {
				return `{"sessionId":"same","folders":["/f` + strconv.Itoa(i) + `"]}`
			})+`]}`))
		}},
		{kind: "remote-session-spaces.json", shape: shapeNested, n: 512, files: func(t *testing.T, n int) []growthFile {
			return one(spaces, []byte(`{"entries":[`+joined(n, func(i int) string { return `{"folders":7}` })+
				`,{"sessionId":"r","folders":[`+nestedArrays(n, `"x"`)+`]}]}`))
		}},
	}
}

// claudedesktopGrowth returns the Claude Desktop growth set.
func claudedesktopGrowth() growthSet {
	return growthSet{parser: "claude-desktop", cases: slices.Concat(claudedesktopTranscriptCases(),
		claudedesktopAuditCases(), claudedesktopRecordCases(), claudedesktopStateCases())}
}
