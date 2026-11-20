package parsers

import (
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

// cursorLongProject returns the state.vscdb of one composer c1 with a
// long project path and branch and one bubble.
func cursorLongProject(t *testing.T) growthFile {
	return growthFile{rel: cursorStateRel, data: cursorState(t, append([][]any{cursorComposer("c1", "/"+long("project"),
		long("branch"), 1)}, cursorBubbles("c1", 1)...), nil, nil)}
}

// cursorGrowth returns the cursor growth set.
func cursorGrowth() growthSet {
	return growthSet{parser: "cursor", cases: slices.Concat(cursorStateCases(), cursorSideCases(), cursorFileCases(),
		cursorSettingsCases())}
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
			headers := rows(n, func(i int) []any {
				return []any{numbered("gone", i), 1790000000000, 1790000000000, 1, `{"name":"n"}`}
			})
			blob := `{"role":"user","content":"` + joined(n, func(i int) string { return numbered("gone", i) }) + `"}`
			return one(cursorStateRel, cursorState(t, [][]any{{"agentKv:blob:aa", blob}}, nil, headers))
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
		{kind: "conversation-search.db", shape: shapeJoins, known: "allocated bytes grow 3.76 times from n=64 to " +
			"128 (4,725,000 to 17,751,256): every conversation naming one fts_rowid converts and compacts the shared " +
			"n-kilobyte body again before it is found already written (cursor/side.go Parser.search)",
			files: func(t *testing.T, n int) []growthFile {
				return one(search, cursorConversations(t, rows(n, func(i int) []any {
					return []any{1, numbered("c", i), numbered("title ", i), "local", 0, 1790000000000}
				}), [][]any{{1, "title", strings.Repeat("b", 1024*n)}}))
			}},
		{kind: "conversation-search.db", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{cursorLongProject(t), {rel: search, data: cursorConversations(t,
				rows(n, func(i int) []any { return []any{i, "c1", numbered("title ", i), "local", 0, 1790000000000} }), nil)}}
		}},
		{kind: "ai-code-tracking.db", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{cursorLongProject(t), {rel: tracking, data: cursorHashes(t, n)}}
		}},
		{kind: "ai-code-tracking.db", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(tracking, cursorHashes(t, n))
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
		{kind: "agent transcript", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{cursorLongProject(t), {rel: transcript("c1"), data: lines(n, prompt)}}
		}},
		{kind: "agent transcript", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
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
