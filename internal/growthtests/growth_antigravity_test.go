package growthtests

import (
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The Antigravity growth cases: the CLI history, the brain mirror and its
// chunks, the conversation and summaries databases, and the four standing
// permission files.

// The paths of the Antigravity files the cases write.
const (
	antigravityHistory  = ".gemini/antigravity-cli/history.jsonl"
	antigravityBrain    = ".gemini/antigravity/brain/c1/.system_generated/logs/"
	antigravityDatabase = ".gemini/antigravity/conversations/c1.db"
	antigravitySummary  = ".gemini/antigravity/conversation_summaries.db"
	antigravityConfig   = ".gemini/config/config.json"
	antigravitySettings = ".gemini/antigravity-cli/settings.json"
	antigravityProject  = ".gemini/config/projects/p1.json"
)

// antigravityVarint returns value as a protobuf varint.
func antigravityVarint(value uint64) []byte {
	var out []byte
	for value >= 0x80 {
		out = append(out, byte(value)|0x80)
		value >>= 7
	}
	return append(out, byte(value))
}

// antigravityNumber returns a protobuf varint field.
func antigravityNumber(number, value uint64) []byte {
	return append(antigravityVarint(number<<3), antigravityVarint(value)...)
}

// antigravityField returns a protobuf length-delimited field holding data.
func antigravityField(number uint64, data []byte) []byte {
	out := append(antigravityVarint(number<<3|2), antigravityVarint(uint64(len(data)))...)
	return append(out, data...)
}

// antigravityText returns a protobuf string field.
func antigravityText(number uint64, value string) []byte {
	return antigravityField(number, []byte(value))
}

// antigravityPrompt returns a Step blob of a user prompt created at
// i seconds after the start of the year: the user_input query and the metadata's created_at.
func antigravityPrompt(i int) []byte {
	created := antigravityField(1, antigravityNumber(1, uint64(1767225600+i)))
	return append(antigravityField(5, created), antigravityField(19, antigravityText(1, numbered("prompt ", i)))...)
}

// antigravityStepsTable returns the steps table holding the rows.
func antigravityStepsTable(stored [][]any) sqlitetest.Table {
	return sqlitetest.Table{Name: "steps", SQL: "CREATE TABLE steps (idx INTEGER, step_type INTEGER, " +
		"status INTEGER, metadata BLOB, step_payload BLOB)", Rows: stored}
}

// antigravityBlobTable returns a table of one data blob column named name.
func antigravityBlobTable(name string, stored [][]any) sqlitetest.Table {
	return sqlitetest.Table{Name: name, SQL: "CREATE TABLE " + name + " (data BLOB)", Rows: stored}
}

// antigravityConversation returns a conversation database of n prompts
// whose trajectory metadata names a workspace folder and branch.
func antigravityConversation(t *testing.T, n int, folder, branch string) []byte {
	workspace := append(antigravityText(1, "file://"+folder), antigravityText(4, branch)...)
	meta := append(antigravityField(1, workspace), antigravityField(2, antigravityNumber(1, 1767225600))...)
	return database(t, antigravityStepsTable(rows(n, func(i int) []any {
		return []any{i + 1, 14, 3, nil, antigravityPrompt(i)}
	})), antigravityBlobTable("trajectory_metadata_blob", [][]any{{meta}}))
}

// antigravityGenerators returns a database of n prompts at indexes 1 to n
// and one generator row naming every index as made by model.
func antigravityGenerators(t *testing.T, n int, model string) []byte {
	generator := antigravityField(1, antigravityText(19, model))
	for i := range n {
		generator = append(generator, antigravityNumber(2, uint64(i+1))...)
	}
	return database(t, antigravityStepsTable(rows(n, func(i int) []any {
		return []any{i + 1, 14, 3, nil, antigravityPrompt(i)}
	})), antigravityBlobTable("gen_metadata", [][]any{{generator}}))
}

// antigravityBrainLine returns a brain line of the given type and content
// with the rest of the members given, created at second i.
func antigravityBrainLine(i int, kind, content, rest string) string {
	return `{"type":"` + kind + `","content":"` + content + `","created_at":"2026-01-01T00:00:` +
		strconv.Itoa(10+i%50) + `Z","status":"DONE"` + rest + `}`
}

// antigravitySummaries returns a summaries database of n rows, row i
// having conversation id conversation(i) and workspace list uris(i).
func antigravitySummaries(t *testing.T, n int, conversation, uris func(i int) string) []byte {
	return database(t, sqlitetest.Table{Name: "conversation_summaries", SQL: "CREATE TABLE " +
		"conversation_summaries (conversation_id text, title text, preview text, step_count integer, " +
		"last_modified_time datetime, workspace_uris text, status text, agent_name text, " +
		"parent_conversation_id text)", Rows: rows(n, func(i int) []any {
		return []any{conversation(i), numbered("title ", i), "", i, "2026-01-02T03:04:" + strconv.Itoa(10+i%50) + "Z",
			uris(i), "idle", "agent", ""}
	})})
}

// antigravityGrowth returns the Antigravity growth set.
func antigravityGrowth() growthSet {
	cases := append(antigravityHistoryCases(), antigravityBrainCases()...)
	cases = append(cases, antigravityDatabaseCases()...)
	// A brain transcript looks for the chunks of its conversation under
	// chunks by that name.
	return growthSet{parser: "antigravity", fixedNames: []string{"chunks"},
		cases: append(cases, antigravityStandingCases()...)}
}

// antigravityHistoryCases returns the cases of the CLI's history.jsonl.
func antigravityHistoryCases() []growthCase {
	return []growthCase{
		{kind: "history.jsonl", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(antigravityHistory, lines(n, func(i int) string {
				return `{"display":"` + numbered("p", i) + `","timestamp":` + strconv.Itoa(1767225600000+i) +
					`,"workspace":"file:///w","conversationId":"same"}`
			}))
		}},
		{kind: "history.jsonl", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(antigravityHistory, append(lines(n, func(i int) string { return `{"display": [` }),
				[]byte(`{"display":`+nestedArrays(n, `"x"`)+`,"conversationId":"c"}`+"\n")...))
		}},
	}
}

// antigravityBrainCases returns the cases of the brain mirror.
func antigravityBrainCases() []growthCase {
	full := antigravityBrain + "transcript_full.jsonl"
	return []growthCase{
		{kind: "brain transcript", shape: shapeLargeShared + " by many truncated fields",
			files: func(t *testing.T, n int) []growthFile {
				calls := joined(n, func(i int) string {
					return `{"id":"` + numbered("t", i) + `","name":"run","args":{"command":"ls"}}`
				})
				truncated := joined(n, func(i int) string { return `"` + numbered("f", i) + `"` })
				return one(full, []byte(antigravityBrainLine(0, "PLANNER_RESPONSE", "ok",
					`,"tool_calls":[`+calls+`],"truncated_fields":["tool_calls",`+truncated+`]`)+"\n"))
			}},
		{kind: "brain transcript", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			calls := joined(n, func(i int) string { return `{"id":"same","name":"run","args":{}}` })
			first := antigravityBrainLine(0, "PLANNER_RESPONSE", "ok", `,"tool_calls":[`+calls+`]`)
			return one(full, append([]byte(first+"\n"), lines(n, func(i int) string {
				return antigravityBrainLine(i, "RUN_COMMAND", "done", "")
			})...))
		}},
		{kind: "brain transcript", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			first := antigravityBrainLine(0, "PLANNER_RESPONSE", long("reply"),
				`,"tool_calls":[{"id":"t","name":"`+long("tool")+`"}]`)
			return one(full, append([]byte(first+"\n"), lines(n, func(i int) string {
				return antigravityBrainLine(i, "USER_INPUT", numbered("hi ", i), "")
			})...))
		}},
		{kind: "brain transcript", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(full, append(lines(n, func(i int) string { return `{"type":"USER_INPUT","content":[` }),
				[]byte(antigravityBrainLine(0, "PLANNER_RESPONSE", "deep",
					`,"tool_calls":[{"id":"t","args":`+nestedObjects(n, "a", `"x"`)+`}]`)+"\n")...))
		}},
		{kind: "brain transcript", shape: shapeManyFiles + " by chunks", files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string {
				return antigravityBrain + "chunks/transcript_full/" + strconv.Itoa(i) + ".jsonl"
			}, func(i int) string { return antigravityBrainLine(i, "USER_INPUT", numbered("hi ", i), "") + "\n" })
		}},
		{kind: "brain transcript", shape: shapeManyFiles + " by conversations", n: 32,
			files: func(t *testing.T, n int) []growthFile {
				return manyFiles(n, func(i int) string {
					return ".gemini/antigravity/brain/" + numbered("c", i) + "/.system_generated/logs/transcript.jsonl"
				}, func(i int) string { return antigravityBrainLine(i, "USER_INPUT", numbered("hi ", i), "") + "\n" })
			}},
	}
}

// antigravityDatabaseCases returns the cases of the conversation and
// summaries databases.
func antigravityDatabaseCases() []growthCase {
	return []growthCase{
		{kind: "conversation database", shape: shapeCarried + " by the workspace", files: func(t *testing.T, n int) []growthFile {
			return one(antigravityDatabase, antigravityConversation(t, n, "/"+long("w"), long("b")))
		}},
		{kind: "conversation database", shape: shapeCarried + " by the generator model", files: func(t *testing.T, n int) []growthFile {
			return one(antigravityDatabase, antigravityGenerators(t, n, long("model")))
		}},
		{kind: "conversation database", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(antigravityDatabase, database(t, antigravityStepsTable(rows(n, func(i int) []any {
				payload := antigravityPrompt(i)
				if i%2 == 1 {
					payload = []byte{0xff, 0xff, 0xff}
				}
				return []any{i, 14, 3, []byte{0x0a, 0x7f}, payload}
			}))))
		}},
		{kind: "conversation_summaries.db", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(antigravitySummary, antigravitySummaries(t, n, func(i int) string { return "same" },
				func(i int) string { return `["file:///w"]` }))
		}},
		{kind: "conversation_summaries.db", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(antigravitySummary, antigravitySummaries(t, n, func(i int) string { return numbered("c", i) },
				func(i int) string {
					if i == 0 {
						return nestedArrays(n, `"file:///w"`)
					}
					return `[` + strings.Repeat("{", i%3)
				}))
		}},
	}
}

// antigravityStandingCases returns the cases of the standing permission
// files.
func antigravityStandingCases() []growthCase {
	grants := func(n int) string {
		return `[` + joined(n, func(i int) string { return `"` + numbered("read:/r", i) + `"` }) + `]`
	}
	return []growthCase{
		{kind: "config.json", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(antigravityConfig, []byte(`{"sidecars":{"`+long("sidecar")+`":{"enabled":true,`+
				`"resolvedPermissions":{"accessGrants":`+grants(n)+`}}}}`))
		}},
		{kind: "config.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(antigravityConfig, []byte(`{"userSettings":{"globalPermissionGrants":{"allow":`+
				nestedArrays(n, `"x"`)+`}},"sidecars":{`+joined(n, func(i int) string {
				return `"` + numbered("s", i) + `":` + strconv.Itoa(i)
			})+`}}`))
		}},
		{kind: "CLI settings.json", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(antigravitySettings, []byte(`{"permissions":{"allow":`+grants(n)+`,"deny":["`+long("deny")+
				`"]},"trustedWorkspaces":["file:///`+long("w")+`"]}`))
		}},
		{kind: "CLI settings.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(antigravitySettings, []byte(`{"permissions":{"allow":`+nestedArrays(n, `"x"`)+
				`},"trustedWorkspaces":[`+joined(n, func(i int) string { return strconv.Itoa(i) })+`]}`))
		}},
		{kind: "project permissions", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(antigravityProject, []byte(`{"projectResources":{"resources":[{"folderUri":"file:///`+
				long("p")+`"}]},"permissionGrants":{"permissionGrants":{"allow":`+grants(n)+`}}}`))
		}},
		{kind: "project permissions", shape: shapeManyFiles, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return ".gemini/config/projects/" + numbered("p", i) + ".json" },
				func(i int) string {
					return `{"projectResources":{"resources":[{"folderUri":"file:///w"}]},` +
						`"permissionGrants":{"permissionGrants":{"allow":["read:/r"]},"v2Migrated":true}}`
				})
		}},
		{kind: "sidecar.json", shape: shapeManyFiles, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string {
				return ".gemini/config/sidecars/" + numbered("s", i) + "/sidecar.json"
			}, func(i int) string { return `{"agent_permissions":{"access_grants":["read:/r"]}}` })
		}},
		{kind: "sidecar.json", shape: shapeNested, n: 512, files: func(t *testing.T, n int) []growthFile {
			return one(".gemini/config/sidecars/s/sidecar.json", []byte(`{"agent_permissions":{"access_grants":`+
				nestedArrays(n, `"x"`)+`},"other":[`+grants(n)+`]}`))
		}},
	}
}
