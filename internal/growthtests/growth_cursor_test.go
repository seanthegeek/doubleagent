package growthtests

import (
	"encoding/base64"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The Cursor growth cases: the state.vscdb store and the side databases,
// transcripts and settings files whose rows take their project from it.

const (
	cursorStorage  = "Library/Application Support/Cursor/User/globalStorage/"
	cursorStateRel = cursorStorage + "state.vscdb"
	cursorSettings = "src.vs.platform.reactivestorage.browser.reactiveStorageServiceImpl" +
		".persistentStorage.applicationUser"
)

// cursorState returns a state.vscdb whose cursorDiskKV and ItemTable hold
// the key-value rows given, and whose composerHeaders holds the headers.
func cursorState(t *testing.T, keyValues, items, headers [][]any) []byte {
	return database(t,
		sqlitetest.Table{Name: "ItemTable", SQL: "CREATE TABLE ItemTable (key TEXT, value BLOB)", Rows: items},
		sqlitetest.Table{Name: "cursorDiskKV", SQL: "CREATE TABLE cursorDiskKV (key TEXT, value BLOB)", Rows: keyValues},
		sqlitetest.Table{Name: "composerHeaders", SQL: "CREATE TABLE composerHeaders (composerId TEXT, createdAt INTEGER, " +
			"lastUpdatedAt INTEGER, isArchived INTEGER, value TEXT)", Rows: headers})
}

// cursorComposer returns the composerData row of a composer in the
// project, whose header list names the bubbles b0 to b<n-1>.
func cursorComposer(ident, project, branch string, n int) []any {
	return []any{"composerData:" + ident, `{"composerId":"` + ident + `","createdAt":1790000000000,` +
		`"workspaceIdentifier":{"uri":{"fsPath":"` + project + `"}},"trackedGitRepos":[{"repoPath":"` + project +
		`","branches":["` + branch + `"]}],"fullConversationHeadersOnly":[` + joined(n, func(i int) string {
		return `{"bubbleId":"b` + strconv.Itoa(i) + `","type":1}`
	}) + `]}`}
}

// cursorBubbles returns n prompt bubbles b0 to b<n-1> of a composer.
func cursorBubbles(ident string, n int) [][]any {
	return rows(n, func(i int) []any {
		return []any{"bubbleId:" + ident + ":b" + strconv.Itoa(i), `{"type":1,"text":"q` + strconv.Itoa(i) +
			`","createdAt":1790000000000,"modelInfo":{"modelName":"m"}}`}
	})
}

// cursorDeletedBlob returns a state.vscdb of n deleted composers, each
// with a header, and one agentKv blob of the role given whose content
// names every one of them.
func cursorDeletedBlob(t *testing.T, role string, n int) []byte {
	headers := rows(n, func(i int) []any {
		return []any{numbered("gone", i), 1790000000000, 1790000000000, 1, `{"name":"n"}`}
	})
	blob := `{"role":"` + role + `","content":"` + joined(n, func(i int) string { return numbered("gone", i) }) + `"}`
	return cursorState(t, [][]any{{"agentKv:blob:aa", blob}}, nil, headers)
}

// cursorLongProject returns the state.vscdb of one composer c1 with a
// long project path and branch and one bubble.
func cursorLongProject(t *testing.T) growthFile {
	return growthFile{rel: cursorStateRel, data: cursorState(t, append([][]any{cursorComposer("c1", "/"+long("project"),
		long("branch"), 1)}, cursorBubbles("c1", 1)...), nil, nil)}
}

// cursorGrowth returns the cursor growth set.
func cursorGrowth() growthSet {
	return growthSet{parser: "cursor", cases: slices.Concat(cursorStateCases(), cursorRepeatCases(),
		cursorValueRepeatCases(), cursorItemRepeatCases(), cursorSideCases(), cursorFileCases(), cursorSettingsCases())}
}

// cursorStateCases returns the cases of state.vscdb.
func cursorStateCases() []growthCase {
	return []growthCase{
		{kind: "state.vscdb", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(cursorStateRel, cursorState(t, append([][]any{cursorComposer("c1", "/"+long("project"),
				long("branch"), n)}, cursorBubbles("c1", n)...), nil, nil))
		}},
		{kind: "state.vscdb", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			composer := []any{"composerData:c1", `{"composerId":"c1","fullConversationHeadersOnly":[` +
				joined(n, func(i int) string { return `{"bubbleId":"b0","type":1}` }) + `]}`}
			bubbles := rows(n, func(i int) []any { return []any{"bubbleId:c1:b0", `{"type":1,"text":"q"}`} })
			return one(cursorStateRel, cursorState(t, append([][]any{composer}, bubbles...), nil, nil))
		}},
		{kind: "state.vscdb", shape: shapeJoins, files: func(t *testing.T, n int) []growthFile {
			keyValues := [][]any{{"composerData:c1", `{"composerId":"c1"}`},
				{"bubbleId:c1:b0", `{"type":2,"text":"` + strings.Repeat("a", 1024*n) + `"}`}}
			keyValues = append(keyValues, rows(n, func(i int) []any {
				return []any{"composerConvHeaders:c1:000001", `[{"bubbleId":"b0","type":2}]`}
			})...)
			return one(cursorStateRel, cursorState(t, keyValues, nil, nil))
		}},
		{kind: "state.vscdb", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			return one(cursorStateRel, cursorDeletedBlob(t, "user", n))
		}},
		// A role other than user or assistant puts a label before the
		// shared text; n is large enough that a copy of the whole text per
		// composer passes the allocation slack.
		{kind: "state.vscdb", shape: shapeLargeShared, n: 2048, files: func(t *testing.T, n int) []growthFile {
			return one(cursorStateRel, cursorDeletedBlob(t, "tool", n))
		}},
		// The role, read from the blob, is in the label of every
		// composer's row.
		{kind: "state.vscdb", shape: shapeCarried + " (a blob's role)", n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(cursorStateRel, cursorDeletedBlob(t, strings.Repeat("r", 36*n), n))
		}},
		// A permission decision names each applied allowlist option by
		// the option's command name, so n references to one option write
		// its long name n times.
		{kind: "state.vscdb", shape: shapeLargeShared + " (an applied allowlist option)", files: func(t *testing.T,
			n int) []growthFile {
			bubble := []any{"bubbleId:c1:b0", `{"type":2,"toolFormerData":{"name":"run_terminal_cmd",` +
				`"additionalData":{"status":"accepted","reviewData":{"status":"Done","allowlistOptions":[{"id":"a",` +
				`"commandName":"` + long("git") + `"}],"appliedAllowlistOptionIds":[` +
				joined(n, func(i int) string { return `"a"` }) + `]}}}}`}
			return one(cursorStateRel, cursorState(t, [][]any{cursorComposer("c1", "/p", "main", 1), bubble}, nil, nil))
		}},
		{kind: "state.vscdb", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			keyValues := append([][]any{cursorComposer("c1", "/p", "main", n)}, rows(n, func(i int) []any {
				return []any{"bubbleId:c1:b" + strconv.Itoa(i), `{"type":1,"text":`}
			})...)
			settings := `{"composerState":{"yoloCommandAllowlist":[` +
				joined(n, func(i int) string { return numbered(`"ls`, i) + `"` }) +
				`,` + nestedArrays(n, `"x"`) + `],"modes4":[` + joined(n, func(i int) string { return strconv.Itoa(i) }) + `]}}`
			return one(cursorStateRel, cursorState(t, keyValues, [][]any{{cursorSettings, settings}}, nil))
		}},
	}
}

// cursorRepeatCases returns the cases of a state.vscdb that stores a key
// more than once.
func cursorRepeatCases() []growthCase {
	return []growthCase{
		// Each stored server URL of a repeated MCP OAuth field gives its own
		// row, a member taking the time of the stored update time the
		// group goes with.
		{kind: "state.vscdb", shape: shapeSharedID + " (MCP OAuth fields)", files: func(t *testing.T, n int) []growthFile {
			key := func(field string) string {
				return "mcpOAuth.global." + base64.RawURLEncoding.EncodeToString([]byte("[srv] "+field))
			}
			items := slices.Concat(rows(n, func(i int) []any {
				return []any{key("mcp_server_url"), `"https://` + numbered("h", i) + `.invalid/"`}
			}), rows(n, func(i int) []any { return []any{key("mcp_oauth_updated_at_ms"), strconv.Itoa(1790000000000 + i)} }))
			return one(cursorStateRel, cursorState(t, nil, items, nil))
		}},
		// Each stored row of a repeated draft key gives its own row.
		{kind: "state.vscdb", shape: shapeSharedID + " (draft rows)", files: func(t *testing.T, n int) []growthFile {
			return one(cursorStateRel, cursorState(t, append([][]any{cursorComposer("c1", "/p", "main", 0)},
				rows(n, func(i int) []any { return []any{"composerDraft:c1", `{"text":"` + numbered("d", i) + `"}`} })...),
				nil, nil))
		}},
		// Each stored row of a repeated bubble key gives its own rows,
		// each on its own model.
		{kind: "state.vscdb", shape: shapeSharedID + " (bubble rows on their own models)",
			files: func(t *testing.T, n int) []growthFile {
				return one(cursorStateRel, cursorState(t, append([][]any{cursorComposer("c1", "/p", "main", 1)},
					rows(n, func(i int) []any {
						return []any{"bubbleId:c1:b0", `{"type":1,"text":"` + numbered("q", i) +
							`","modelInfo":{"modelName":"` + numbered("m", i) + `"}}`}
					})...), nil, nil))
			}},
		// Each stored composerData row of a repeated key is a parent of
		// the composer's group, with its own rows, and names its own
		// bubble and the first; each bubble is a member, shown once.
		{kind: "state.vscdb", shape: shapeSharedID + " (composerData rows in their own projects)",
			files: func(t *testing.T, n int) []growthFile {
				return one(cursorStateRel, cursorState(t, append(rows(n, func(i int) []any {
					return []any{"composerData:c1", `{"name":"` + numbered("n", i) + `","workspaceIdentifier":` +
						`{"uri":{"fsPath":"/` + numbered("p", i) + `"}},"fullConversationHeadersOnly":[` +
						`{"bubbleId":"b0","type":1},{"bubbleId":"b` + strconv.Itoa(i) + `","type":1}]}`}
				}), cursorBubbles("c1", n)...), nil, nil))
			}},
		// Each stored composerHeaders row of a repeated id is a parent too.
		{kind: "state.vscdb", shape: shapeSharedID + " (composerHeaders rows in their own projects)",
			files: func(t *testing.T, n int) []growthFile {
				headers := rows(n, func(i int) []any {
					return []any{"c1", 1790000000000, 1790000000000, 1, `{"name":"` + numbered("n", i) +
						`","workspaceIdentifier":{"uri":{"fsPath":"/` + numbered("p", i) + `"}}}`}
				})
				return one(cursorStateRel, cursorState(t, append([][]any{cursorComposer("c1", "/p", "main", n)},
					cursorBubbles("c1", n)...), nil, headers))
			}},
	}
}

// cursorValueRepeatCases returns the cases of a stored value that repeats
// an id inside itself.
func cursorValueRepeatCases() []growthCase {
	return []growthCase{
		// An allowlist option id n options give is named by each of their
		// names once.
		{kind: "state.vscdb", shape: shapeSharedID + " (allowlist options of one id)", files: func(t *testing.T,
			n int) []growthFile {
			bubble := []any{"bubbleId:c1:b0", `{"type":2,"toolFormerData":{"name":"run_terminal_cmd",` +
				`"additionalData":{"status":"accepted","reviewData":{"status":"Done","allowlistOptions":[` +
				joined(n, func(i int) string { return `{"id":"a","commandName":"` + numbered("git", i) + `"}` }) +
				`],"appliedAllowlistOptionIds":["a"]}}}}`}
			return one(cursorStateRel, cursorState(t, [][]any{cursorComposer("c1", "/p", "main", 1), bubble}, nil, nil))
		}},
	}
}

// cursorItemRepeatCases returns the cases of an ItemTable that stores a
// key more than once.
func cursorItemRepeatCases() []growthCase {
	return []growthCase{
		// Each stored settings blob of a repeated key gives its own rows,
		// and each stored sandbox switch of a repeated key is a parent of
		// the run-mode rows, which only its first stored row of each value
		// holds.
		{kind: "state.vscdb", shape: shapeSharedID + " (settings blobs and sandbox switches)",
			files: func(t *testing.T, n int) []growthFile {
				items := slices.Concat(rows(n, func(i int) []any {
					return []any{cursorSettings, `{"composerState":{"yoloCommandAllowlist":["` + numbered("ls", i) +
						`"],"modes4":[{"id":"agent","autoRun":true}]}}`}
				}), rows(n, func(i int) []any { return []any{"cursor/sandboxAutoRunSettingEnabled", strconv.FormatBool(i%2 == 0)} }),
					rows(n, func(i int) []any {
						return []any{"cloudEnvironmentTemplate.githubRepos", `[{"url":"` + numbered("r", i) + `"}]`}
					}))
				return one(cursorStateRel, cursorState(t, nil, items, nil))
			}},
		// Each stored usages list or display name of a repeated key is a
		// parent of the account's or host's row, given once per parent;
		// each stored account or enrolment row gives a parser: row.
		{kind: "state.vscdb", shape: shapeSharedID + " (integration keys)", files: func(t *testing.T, n int) []growthFile {
			items := slices.Concat(rows(n, func(i int) []any { return []any{"github-alice", `[{"id":"x"}]`} }),
				rows(n, func(i int) []any {
					return []any{"github-alice-usages", `[{"scopes":["` + numbered("s", i) + `"],"lastUsed":` +
						strconv.Itoa(1790000000000+i) + `}]`}
				}), rows(n, func(i int) []any { return []any{"remoteAgentHost.enrolled.u1", "true"} }),
				rows(n, func(i int) []any { return []any{"remoteAgentHost.displayName.u1", `"` + numbered("n", i) + `"`} }))
			return one(cursorStateRel, cursorState(t, nil, items, nil))
		}},
	}
}

// cursorConversations returns a conversation-search.db holding the
// conversations and the full-text bodies.
func cursorConversations(t *testing.T, records [][]any, bodies [][]any) []byte {
	return database(t, sqlitetest.Table{Name: "conversations", SQL: "CREATE TABLE conversations (fts_rowid INTEGER, " +
		"id TEXT, title TEXT, source TEXT, is_archived INTEGER, updated_at INTEGER)", Rows: records},
		sqlitetest.Table{Name: "conversation_fts_content", SQL: "CREATE TABLE conversation_fts_content (id INTEGER, " +
			"c0 TEXT, c1 TEXT)", Rows: bodies})
}

// cursorHashes returns an ai-code-tracking.db of n hashes of composer c1.
func cursorHashes(t *testing.T, n int) []byte {
	return database(t, sqlitetest.Table{Name: "ai_code_hashes", SQL: "CREATE TABLE ai_code_hashes (fileName TEXT, " +
		"source TEXT, model TEXT, requestId TEXT, conversationId TEXT, timestamp INTEGER)", Rows: rows(n, func(i int) []any {
		return []any{numbered("/src/f", i), "composer", "m", "r1", "c1", 1790000000000}
	})})
}

// cursorSideCases returns the cases of the side databases, each joined to
// the home's state.vscdb.
func cursorSideCases() []growthCase {
	const search, tracking = cursorStorage + "conversation-search.db", ".cursor/ai-tracking/ai-code-tracking.db"
	return []growthCase{
		{kind: "conversation-search.db", shape: shapeJoins, files: func(t *testing.T, n int) []growthFile {
			return one(search, cursorConversations(t, rows(n, func(i int) []any {
				return []any{1, numbered("c", i), numbered("title ", i), "local", 0, 1790000000000}
			}), [][]any{{1, "title", strings.Repeat("b", 1024*n)}}))
		}},
		{kind: "conversation-search.db", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{cursorLongProject(t), {rel: search, data: cursorConversations(t,
				rows(n, func(i int) []any { return []any{i, "c1", numbered("title ", i), "local", 0, 1790000000000} }), nil)}}
		}},
		// Each entry of a bcId the cloud agent list repeats is a parent of
		// the agent's group, and each conversation of it a member.
		{kind: "conversation-search.db", shape: shapeSharedID + " (a cloud agent the state repeats)",
			files: func(t *testing.T, n int) []growthFile {
				agents := "[" + joined(n, func(i int) string {
					return `{"bcId":"bc-1","repoUrl":"` + numbered("github.com/o/r", i) + `","createdAt":1790000000000}`
				}) + "]"
				state := cursorState(t, nil, [][]any{{"cloudAgentRepository.agents.acct", agents}}, nil)
				return []growthFile{{rel: cursorStateRel, data: state}, {rel: search, data: cursorConversations(t,
					rows(n, func(i int) []any { return []any{nil, "bc-1", numbered("t", i), "cloud-cache", 0, 1790000000000} }),
					nil)}}
			}},
		// Each stored row of a repeated full-text id gives its own row.
		{kind: "conversation-search.db", shape: shapeSharedID + " (full-text rows of one id)",
			files: func(t *testing.T, n int) []growthFile {
				return one(search, cursorConversations(t, [][]any{{1, "c1", "title", "local", 0, 1790000000000}},
					rows(n, func(i int) []any { return []any{1, "title", numbered("body ", i)} })))
			}},
		{kind: "ai-code-tracking.db", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{cursorLongProject(t), {rel: tracking, data: cursorHashes(t, n)}}
		}},
		{kind: "ai-code-tracking.db", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(tracking, cursorHashes(t, n))
		}},
		// The home's state repeats the composer's key in n rows, so each
		// hash is a member of a group of n parents.
		{kind: "ai-code-tracking.db", shape: shapeSharedID + " (a composer the state repeats)",
			files: func(t *testing.T, n int) []growthFile {
				state := cursorState(t, rows(n, func(i int) []any {
					return cursorComposer("c1", "/"+numbered("p", i), "main", 1)
				}), nil, nil)
				return []growthFile{{rel: cursorStateRel, data: state}, {rel: tracking, data: cursorHashes(t, n)}}
			}},
	}
}

// cursorFileCases returns the cases of the agent transcripts.
func cursorFileCases() []growthCase {
	transcript := func(ident string) string {
		return ".cursor/projects/srv-proj/agent-transcripts/" + ident + "/" + ident + ".jsonl"
	}
	prompt := func(i int) string {
		return `{"role":"user","message":{"content":[{"type":"text","text":"<user_query>q` + strconv.Itoa(i) +
			`</user_query>"}]}}`
	}
	return []growthCase{
		// A transcript's name is its composer id, looked up
		// in the state database.
		{kind: "agent transcript", shape: shapeCarried, keptCase: `agent-transcripts/[^/]+/([^/]+)\.jsonl\z`, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{cursorLongProject(t), {rel: transcript("c1"), data: lines(n, prompt)}}
		}},
		// A transcript's name is its composer id, looked up in the state
		// database.
		{kind: "agent transcript", shape: shapeManyFiles, n: 16, keptCase: `agent-transcripts/[^/]+/([^/]+)\.jsonl\z`, files: func(t *testing.T, n int) []growthFile {
			state := cursorState(t, append([][]any{cursorComposer("c1", "/p", "main", 1)}, cursorBubbles("c1", 1)...), nil, nil)
			return append(manyFiles(n, func(i int) string { return transcript(numbered("c", i)) },
				func(i int) string { return prompt(i) + "\n" }), growthFile{rel: cursorStateRel, data: state})
		}},
		{kind: "agent transcript", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(transcript("c1"), append(lines(n, func(i int) string {
				return `{"role":"assistant","message":{"content":` + strconv.Itoa(i) + `}}` + "\n" + `{"role":"user",`
			}), []byte(`{"role":"assistant","message":{"content":[{"type":"tool_use","name":"t","input":`+
				nestedArrays(n, "1")+`}]}}`+"\n")...))
		}},
	}
}

// cursorSettingsCases returns the cases of the settings files and the MCP
// sign-in attempts.
func cursorSettingsCases() []growthCase {
	return []growthCase{
		{kind: "cli-config.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".cursor/cli-config.json", []byte(`{"approvalMode":"allowlist","permissions":{"allow":[`+
				joined(n, func(i int) string { return `"Shell(` + numbered("git", i) + `)"` })+`],"deny":[`+
				nestedObjects(n, "a", "1")+`]},"sandbox":{"mode":"enabled"}}`))
		}},
		{kind: "permissions.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".cursor/permissions.json", []byte(`{"allowlists":{"terminal":[`+joined(n, func(i int) string {
				return `"` + numbered("ls", i) + `"`
			})+`],"mcp":[`+nestedArrays(n, `"x"`)+`]},"allowInstructions":[`+joined(n, func(i int) string {
				return strconv.Itoa(i)
			})+`]}`))
		}},
		{kind: "mcp-oauth attempt", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return cursorStorage + "mcp-oauth-attempts/" + numbered("a", i) + ".json" },
				func(i int) string {
					return `{"serverUrl":"https://mcp.example/` + strconv.Itoa(i) + `","createdAtMs":1790000000000,` +
						`"clientInformation":{"client_id":"id"},"owner":{"workspaceId":"w"}}`
				})
		}},
	}
}
