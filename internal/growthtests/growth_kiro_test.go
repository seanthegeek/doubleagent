package growthtests

import (
	"slices"
	"strconv"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The Kiro (Amazon Q Developer CLI) growth cases: data.sqlite3, whose
// conversations table files a ConversationState under its working
// directory, the /save exports of one, the prompt history, the agent
// configurations and the delegate tool's records.

// The files of the CLI, by path relative to the home.
const (
	kiroDatabase = ".local/share/amazon-q/data.sqlite3"
	kiroExport   = "work/saved.json"
)

// kiroPrompt returns a history entry: a prompt and a response at second i.
func kiroPrompt(i int) string {
	return `{"user":{"content":{"Prompt":{"prompt":"` + numbered("ask", i) + `"}},"timestamp":"2026-01-02T03:04:05Z"},` +
		`"assistant":{"Response":{"message_id":"` + numbered("m", i) + `","content":"done"}},` +
		`"request_metadata":{"request_start_timestamp_ms":` + strconv.Itoa(1767323045000+i) + `}}`
}

// kiroState returns a ConversationState of the given id, model and
// history entries.
func kiroState(conversationID, model, history string) string {
	return `{"conversation_id":"` + conversationID + `","model":"` + model + `","history":[` + history + `]}`
}

// kiroConversations returns data.sqlite3 holding conversations rows of key
// and value.
func kiroConversations(t *testing.T, conversationRows [][]any) []byte {
	t.Helper()
	return database(t, sqlitetest.Table{Name: "conversations", SQL: "CREATE TABLE conversations (key TEXT, value TEXT)",
		Rows: conversationRows})
}

// kiroGrowth returns the kiro growth set.
func kiroGrowth() growthSet {
	return growthSet{parser: "kiro", cases: slices.Concat(kiroConversationCases(), kiroFileCases())}
}

// kiroConversationCases returns the cases of data.sqlite3 and the
// exports.
func kiroConversationCases() []growthCase {
	return []growthCase{
		{kind: "data.sqlite3", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(kiroDatabase, kiroConversations(t, [][]any{{"/" + long("w"),
				kiroState("c", "claude", joined(n, kiroPrompt))}}))
		}},
		{kind: "data.sqlite3", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			shell := sqlitetest.Table{Name: "history", SQL: "CREATE TABLE history (command TEXT, cwd TEXT, " +
				"session_id TEXT, time INTEGER, exit_code INTEGER)", Rows: rows(n, func(i int) []any {
				return []any{numbered("ls ", i), "/w", "s", int64(1767323045 + i), int64(0)}
			})}
			return one(kiroDatabase, database(t, sqlitetest.Table{Name: "conversations",
				SQL: "CREATE TABLE conversations (key TEXT, value TEXT)", Rows: rows(n, func(i int) []any {
					return []any{"/w", kiroState("c", "claude", kiroPrompt(i))}
				})}, shell))
		}},
		{kind: "conversation export", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(kiroExport, []byte(kiroState(long("c"), "claude", joined(n, kiroPrompt))))
		}},
		{kind: "conversation export", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			use := `{"user":{"content":{"Prompt":{"prompt":"go"}}},"assistant":{"ToolUse":{"content":"x",` +
				`"tool_uses":[{"id":"t","name":"` + long("tool") + `","args":{}}]}}}`
			results := `{"user":{"content":{"ToolUseResults":{"tool_use_results":[` + joined(n, func(i int) string {
				return `{"tool_use_id":"t","content":[{"Text":"` + numbered("out", i) + `"}],"status":"Success"}`
			}) + `]}}}}`
			return one(kiroExport, []byte(kiroState("c", "claude", use+","+results)))
		}},
		{kind: "conversation export", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			checkpoints := joined(n, func(i int) string {
				return `{"tag":"` + strconv.Itoa(i) + `","history_snapshot":[` + kiroPrompt(0) + `]}`
			})
			return one(kiroExport, []byte(`{"conversation_id":"c","history":[`+joined(n, func(i int) string {
				return kiroPrompt(0)
			})+`],"checkpoint_manager":{"checkpoints":[`+checkpoints+`]}}`))
		}},
		{kind: "conversation export", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			deep := `{"user":{"content":{"Prompt":{"prompt":"go"}}},"assistant":{"ToolUse":{"content":"x",` +
				`"tool_uses":[{"id":"t","name":"n","args":` + nestedArrays(n, "1") + `}]}}}`
			return one(kiroExport, []byte(kiroState("c", "claude", joined(n, strconv.Itoa)+","+deep)))
		}},
	}
}

// kiroFileCases returns the cases of the prompt history, the agent
// configurations and the delegate records.
func kiroFileCases() []growthCase {
	return []growthCase{
		{kind: ".cli_bash_history", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".aws/amazonq/.cli_bash_history", append([]byte("#V2\n"), lines(n, func(i int) string {
				return numbered(`line\nnext\\`, i) + "\xff"
			})...))
		}},
		{kind: ".cli_bash_history", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(".aws/amazonq/.cli_bash_history", lines(n, func(i int) string { return "same prompt" }))
		}},
		{kind: "agent configuration", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".aws/amazonq/cli-agents/a.json", []byte(`{"allowedTools":["fs_read"],"toolsSettings":{"`+
				long("tool")+`":{`+joined(n, func(i int) string { return `"` + numbered("key", i) + `":true` })+`}}}`))
		}},
		{kind: "agent configuration", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".amazonq/cli-agents/a.json", []byte(`{"allowedTools":[`+joined(n, strconv.Itoa)+
				`],"toolsSettings":{"shell":{"allowedCommands":`+nestedArrays(n, `"ls"`)+`}}}`))
		}},
		{kind: "subagent record", shape: shapeManyFiles, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return ".amazonq/.subagents/" + numbered("agent", i) + ".json" },
				func(i int) string {
					return `{"agent":"q_cli_default","task":"t","cwd":"/w","pid":` + strconv.Itoa(i) +
						`,"launched_at":1767323045,"completed_at":1767323046,"status":"completed","exit_code":0}`
				})
		}},
	}
}
