package parsers

import (
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The Codex CLI growth cases, written for a home prefix so that the forks
// that keep Codex's formats under a home of their own (Open Interpreter)
// run the same cases: the rollouts, plain and compressed, the prompt
// history, the SQLite projections and log databases, and the standing
// files.

// codexTime is the time every record of a case is written at.
const codexTime = "2026-01-02T00:00:01Z"

// codexTOMLDepth is how deep a TOML case nests: within the 32 levels the
// TOML reader takes, so that the document is read rather than refused.
const codexTOMLDepth = 30

// codexThread returns the thread id of number i, a UUID as a rollout's
// file name ends with.
func codexThread(i int) string {
	return fmt.Sprintf("0198f0a1-0000-7000-8000-%012d", i)
}

// codexRolloutRel returns the path of thread i's rollout under the home
// prefix.
func codexRolloutRel(prefix string, i int) string {
	return prefix + "/sessions/2026/01/02/rollout-2026-01-02T00-00-00-" + codexThread(i) + ".jsonl"
}

// codexLine returns a rollout line of the record type whose payload holds
// the given JSON members.
func codexLine(recordType, members string) string {
	return `{"timestamp":"` + codexTime + `","type":"` + recordType + `","payload":{` + members + `}}`
}

// codexMeta returns the session_meta line of thread i with the given
// working directory.
func codexMeta(i int, cwd string) string {
	return codexLine("session_meta", `"id":"`+codexThread(i)+`","cwd":"`+cwd+`","timestamp":"`+codexTime+`"`)
}

// codexPrompt returns a user message response item holding text.
func codexPrompt(text string) string {
	return codexLine("response_item", `"type":"message","role":"user","content":[{"type":"input_text",`+
		`"text":"`+text+`"}]`)
}

// codexGuardianRollout returns the rollout of guardian review thread i of
// thread "parent": its session_meta and a task_complete with a verdict.
func codexGuardianRollout(i int) string {
	return codexLine("session_meta", `"id":"`+codexThread(i)+`","thread_source":"guardian_review",`+
		`"parent_thread_id":"parent","timestamp":"`+codexTime+`"`) + "\n" +
		codexLine("event_msg", `"type":"task_complete","last_agent_message":"{\"outcome\":\"allow\"}"`) + "\n"
}

// codexLogsTable returns a logs table of a Codex log database holding the
// rows, each of ts, target, body and thread_id.
func codexLogsTable(rows [][]any) sqlitetest.Table {
	return sqlitetest.Table{Name: "logs", SQL: "CREATE TABLE logs (ts INTEGER, target TEXT, " +
		"feedback_log_body TEXT, thread_id TEXT)", Rows: rows}
}

// codexFeedbackRow returns a logs row of a /feedback report sent for the
// thread.
func codexFeedbackRow(thread string) []any {
	return []any{int64(1767312001), "codex_feedback", "feedback event uploaded to Sentry thread_id=" + thread +
		" uploaded_attachments=1", thread}
}

// codexThreadsTable returns a threads table of a state database holding
// the rows, each of id, rollout_path, created_at, cwd, title, git_branch and
// first_user_message.
func codexThreadsTable(rows [][]any) sqlitetest.Table {
	return sqlitetest.Table{Name: "threads", SQL: "CREATE TABLE threads (id TEXT, rollout_path TEXT, " +
		"created_at INTEGER, cwd TEXT, title TEXT, git_branch TEXT, first_user_message TEXT)", Rows: rows}
}

// codexThreadRow returns a threads row of the thread with the working
// directory and branch.
func codexThreadRow(thread, cwd, branch string) []any {
	return []any{thread, "/r.jsonl", int64(1767312001), cwd, "title", branch, "hello"}
}

// codexState returns a state database whose one thread "t" has the working
// directory and branch, and n guardian review feedback records of it.
func codexState(t *testing.T, n int, cwd, branch string) []byte {
	return codexRepeatedState(t, 1, n, cwd, branch)
}

// codexRepeatedState returns a state database whose threads table holds
// thread "t" in stored rows, each but the first in its own working
// directory below cwd, and n guardian review feedback records of it.
func codexRepeatedState(t *testing.T, stored, n int, cwd, branch string) []byte {
	feedback := sqlitetest.Table{Name: "guardian_review_feedback", SQL: "CREATE TABLE " +
		"guardian_review_feedback (id TEXT, thread_id TEXT, record TEXT)", Rows: rows(n, func(i int) []any {
		return []any{numbered("f", i), "t", `{"reviewed_thread_id":"t","status":"approved","target_item_id":"c` +
			strconv.Itoa(i) + `","decision":"ok","action":"run"}`}
	})}
	return database(t, codexThreadsTable(rows(stored, func(i int) []any {
		if i == 0 {
			return codexThreadRow("t", cwd, branch)
		}
		return codexThreadRow("t", cwd+numbered("/", i), branch)
	})), feedback)
}

// codexHistory returns a thread history database of n turns and n items,
// every one of thread "t" and turn "u", each item holding itemJSON(i).
func codexHistory(t *testing.T, n int, itemJSON func(i int) string) []byte {
	turns := sqlitetest.Table{Name: "thread_turns", SQL: "CREATE TABLE thread_turns (thread_id TEXT, " +
		"turn_id TEXT, rollout_ordinal INTEGER, status TEXT, started_at INTEGER, completed_at INTEGER)",
		Rows: rows(n, func(i int) []any { return []any{"t", "u", int64(1), "completed", int64(1767312001), nil} })}
	items := sqlitetest.Table{Name: "thread_items", SQL: "CREATE TABLE thread_items (thread_id TEXT, " +
		"turn_id TEXT, item_id TEXT, rollout_ordinal INTEGER, created_at_ms INTEGER, item_json TEXT)",
		Rows: rows(n, func(i int) []any {
			return []any{"t", "u", numbered("i", i), int64(1), int64(1767312001000), itemJSON(i)}
		})}
	return database(t, turns, items)
}

// codexUserItem returns a thread item of a user message holding text.
func codexUserItem(text string) string {
	return `{"type":"userMessage","content":[{"type":"text","text":"` + text + `"}]}`
}

// codexStandingCases returns the cases of the prompt history and
// config.toml.
func codexStandingCases(prefix string) []growthCase {
	return []growthCase{
		{kind: "history.jsonl", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(prefix+"/history.jsonl", lines(n, func(i int) string {
				return `{"session_id":"s","ts":` + strconv.Itoa(1767312001+i) + `,"text":"p` + strconv.Itoa(i) + `"}`
			}))
		}},
		{kind: "history.jsonl", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(prefix+"/history.jsonl", append(lines(n, func(i int) string { return `{"text": [` }),
				[]byte(`{"session_id":"s","ts":1767312001,"text":`+nestedArrays(n, `"x"`)+`}`+"\n")...))
		}},
		{kind: "config.toml", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(prefix+"/config.toml", append([]byte(`[mcp_servers."`+long("server")+`".tools]`+"\n"),
				lines(n, func(i int) string { return numbered("t", i) + ` = { approval_mode = "approve" }` })...))
		}},
		{kind: "config.toml", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(prefix+"/config.toml", []byte("approval_policy = "+nestedArrays(codexTOMLDepth, `"x"`)+"\n"+
				string(lines(n, func(i int) string { return "[projects." + numbered("p", i) + "]\ntrust_level = 1" }))))
		}},
	}
}

// codexPolicyFileCases returns the cases of the rules and the memory
// summaries.
func codexPolicyFileCases(prefix string) []growthCase {
	return []growthCase{
		{kind: "rules", shape: shapeNested, n: 512, files: func(t *testing.T, n int) []growthFile {
			return one(prefix+"/rules/default.rules", []byte("prefix_rule(pattern="+
				strings.Repeat("[\n", n)+`"x"`+strings.Repeat("]\n", n)+")\n"+strings.Repeat("prefix_rule([\n", n)))
		}},
		{kind: "rules", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return prefix + "/rules/" + numbered("r", i) + ".rules" },
				func(i int) string {
					return `prefix_rule(pattern=["git", "` + numbered("c", i) + `"], decision="allow")`
				})
		}},
		{kind: "rollout summary", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string {
				return prefix + "/memories/rollout_summaries/" + numbered("s", i) + ".md"
			}, func(i int) string {
				return "thread_id: " + codexThread(i) + "\nupdated_at: " + codexTime + "\ncwd: /p\n\nsummary\n"
			})
		}},
		{kind: "rollout summary", shape: shapeNested, n: 1024, files: func(t *testing.T, n int) []growthFile {
			return one(prefix+"/memories/rollout_summaries/s.md", []byte("thread_id: t\n"+
				strings.Repeat("cwd: /p\n", n)+"\n"+strings.Repeat("line\n", n)))
		}},
	}
}

// codexRolloutCases returns the cases of the plain rollouts.
func codexRolloutCases(prefix string) []growthCase {
	return []growthCase{
		{kind: "rollout", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(codexRolloutRel(prefix, 0), append([]byte(codexMeta(0, "/"+long("p"))+"\n"),
				lines(n, func(i int) string { return codexPrompt(numbered("p", i)) })...))
		}},
		{kind: "rollout", shape: shapeCarried + " of a policy value as long as the input", files: codexPolicy(prefix)},
		{kind: "rollout", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(codexRolloutRel(prefix, 0), append([]byte(codexMeta(0, "/p")+"\n"), lines(2*n, func(i int) string {
				if i%2 == 0 {
					return codexLine("response_item", `"type":"function_call","name":"shell","call_id":"same",`+
						`"arguments":"{}"`)
				}
				return codexLine("response_item", `"type":"function_call_output","call_id":"same","output":"ok"`)
			})...))
		}},
		{kind: "rollout", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			completed := codexLine("event_msg", `"type":"item_completed","item":{"type":"CommandExecution",`+
				`"id":"same","command":["ls"],"aggregated_output":"","exit_code":"`+long("code")+`"}`)
			return one(codexRolloutRel(prefix, 0), append([]byte(codexMeta(0, "/p")+"\n"+completed+"\n"),
				lines(n, func(i int) string {
					return codexLine("response_item", `"type":"function_call_output","call_id":"same","output":"ok"`)
				})...))
		}},
		{kind: "rollout", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(codexRolloutRel(prefix, 0), append(lines(n, func(i int) string {
				return `{"type":"response_item","payload":`
			}), []byte(codexMeta(0, "/p")+"\n"+codexLine("response_item", `"type":"message","role":"user",`+
				`"content":`+nestedArrays(n, `{"type":"input_text","text":"x"}`))+"\n")...))
		}},
		{kind: "rollout", shape: shapeManyFiles, n: 16, files: codexGuardianHome(prefix)},
	}
}

// codexGuardianHome returns the files of a home of n guardian review
// rollouts beside n log databases, each rollout looking its reviewed call
// up in every log database.
func codexGuardianHome(prefix string) func(t *testing.T, n int) []growthFile {
	return func(t *testing.T, n int) []growthFile {
		files := manyFiles(n, func(i int) string { return codexRolloutRel(prefix, i) }, codexGuardianRollout)
		for i := range n {
			files = append(files, growthFile{rel: prefix + "/logs_" + strconv.Itoa(i) + ".sqlite",
				data: database(t, codexLogsTable([][]any{{int64(1767312001), "codex_core::guardian",
					"guardian_review{thread_id=parent tool_call_id=" + numbered("c", i) + "}", "parent"}}))})
		}
		return files
	}
}

// codexPolicy returns the files of a rollout whose turn_context sets a
// network policy of 100 bytes a unit, after which n thread settings events
// alternate the approval policy, so that each "permission mode:" row
// carries the long network value its event did not set.
func codexPolicy(prefix string) func(t *testing.T, n int) []growthFile {
	return func(t *testing.T, n int) []growthFile {
		context := codexLine("turn_context", `"cwd":"/p","approval_policy":"never","network":"`+
			strings.Repeat("n", 100*n)+`"`)
		return one(codexRolloutRel(prefix, 0), append([]byte(codexMeta(0, "/p")+"\n"+context+"\n"),
			lines(n, func(i int) string {
				return codexLine("event_msg", `"type":"thread_settings_applied","thread_settings":{`+
					`"approval_policy":"`+"ab"[i%2:i%2+1]+`"}`)
			})...))
	}
}

// codexCompressedCases returns the cases of the zstd-compressed rollouts.
func codexCompressedCases(prefix string) []growthCase {
	return []growthCase{
		{kind: "compressed rollout", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			files := make([]growthFile, 0, n)
			for i := range n {
				files = append(files, growthFile{rel: codexRolloutRel(prefix, i) + ".zst",
					data: zstdFrame(17, []byte(codexMeta(i, "/p")+"\n"+codexPrompt("hi")+"\n"))})
			}
			return files
		}},
		{kind: "compressed rollout", shape: shapeNested + " in many frames", files: func(t *testing.T, n int) []growthFile {
			data := zstdFrame(17, []byte(codexMeta(0, "/p")+"\n"))
			for i := range n {
				data = append(data, zstdFrame(17, []byte(`{"type":"response_item","payload":`+"\n"))...)
				data = append(data, zstdFrame(17, []byte(codexPrompt(numbered("p", i))+"\n"))...)
			}
			return one(codexRolloutRel(prefix, 0)+".zst", data)
		}},
		{kind: "compressed rollout", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			data := codexMeta(0, "/p") + "\n" + codexLine("turn_context", `"cwd":"/p","model":"`+long("m")+`"`) + "\n"
			return one(codexRolloutRel(prefix, 0)+".zst", zstdFrame(17, append([]byte(data),
				lines(n, func(i int) string { return codexPrompt(numbered("p", i)) })...)))
		}},
	}
}

// codexStateCases returns the cases of the state database and its
// backups.
func codexStateCases(prefix string) []growthCase {
	state := prefix + "/state_5.sqlite"
	return []growthCase{
		{kind: "state_5.sqlite", shape: shapeJoins, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{{rel: state, data: codexState(t, n, "/"+long("p"), long("b"))}}
		}},
		{kind: "state_5.sqlite", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{{rel: state, data: database(t, codexThreadsTable(rows(n, func(i int) []any {
				return codexThreadRow("same", "/p", "main")
			})))}}
		}},
		{kind: "state_5.sqlite", shape: shapeSharedID + " (stored threads rows with the thread's feedback)",
			files: func(t *testing.T, n int) []growthFile {
				return []growthFile{{rel: state, data: codexRepeatedState(t, n, n, "/p", "main")}}
			}},
		{kind: "state backup", shape: shapeJoins, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{{rel: prefix + "/db-backups/b/state_5.sqlite", data: codexState(t, n, "/p", "main")}}
		}},
		{kind: "state backup", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			files := make([]growthFile, 0, n)
			for i := range n {
				files = append(files, growthFile{rel: prefix + "/db-backups/" + numbered("b", i) + "/state_5.sqlite",
					data: codexState(t, 1, "/p", "main")})
			}
			return files
		}},
	}
}

// codexHistoryCases returns the cases of the thread history, which looks
// its threads up in the state database beside it.
func codexHistoryCases(prefix string) []growthCase {
	state := prefix + "/state_5.sqlite"
	return []growthCase{
		{kind: "thread_history_1.sqlite", shape: shapeJoins, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{{rel: state, data: codexState(t, 0, "/"+long("p"), "main")},
				{rel: prefix + "/thread_history_1.sqlite", data: codexHistory(t, n, func(i int) string {
					return codexUserItem(numbered("p", i))
				})}}
		}},
		{kind: "thread_history_1.sqlite", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			changes := joined(n, func(i int) string { return `{"path":"/same","kind":"add"}` })
			return one(prefix+"/thread_history_1.sqlite", codexHistory(t, n, func(i int) string {
				if i > 0 {
					changes = `{"path":"/same","kind":"add"}`
				}
				return `{"type":"fileChange","id":"same","changes":[` + changes + `],"status":"completed"}`
			}))
		}},
		{kind: "thread_history_1.sqlite", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(prefix+"/thread_history_1.sqlite", codexHistory(t, n, func(i int) string {
				if i == 0 {
					return `{"type":"mcpToolCall","arguments":` + nestedArrays(n, "1") + `}`
				}
				return `{"type":"userMessage","content":`
			}))
		}},
	}
}

// codexLogCases returns the cases of the log databases, which look their
// threads up in the state database beside them.
func codexLogCases(prefix string) []growthCase {
	state := prefix + "/state_5.sqlite"
	return []growthCase{
		{kind: "logs database", shape: shapeJoins, files: func(t *testing.T, n int) []growthFile {
			return []growthFile{{rel: state, data: codexState(t, 0, "/"+long("p"), long("b"))},
				{rel: prefix + "/logs_1.sqlite", data: database(t, codexLogsTable(rows(n, func(i int) []any {
					return codexFeedbackRow("t")
				})))}}
		}},
		{kind: "logs database", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(prefix+"/logs_1.sqlite", database(t, codexLogsTable(rows(2*n, func(i int) []any {
				if i%2 == 0 {
					return []any{int64(1767312001), "codex_core::session::handlers",
						`op=ExecApproval { id: "same", turn_id: Some("u"), decision: Approved }`, "t"}
				}
				return []any{int64(1767312001), "codex_app_server::outgoing_message",
					`CommandExecutionRequestApproval { request_id: "same", decision: Approved }`, "t"}
			}))))
		}},
		{kind: "logs database", shape: shapeSharedID + " (stored threads rows of the state database beside)",
			files: func(t *testing.T, n int) []growthFile {
				return []growthFile{{rel: state, data: codexRepeatedState(t, n, 0, "/p", "main")},
					{rel: prefix + "/logs_1.sqlite", data: database(t, codexLogsTable(rows(n, func(i int) []any {
						return codexFeedbackRow("t")
					})))}}
			}},
		{kind: "logs database", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			files := []growthFile{{rel: state, data: codexState(t, 0, "/p", "main")}}
			for i := range n {
				files = append(files, growthFile{rel: prefix + "/logs_" + strconv.Itoa(i) + ".sqlite",
					data: database(t, codexLogsTable([][]any{codexFeedbackRow("t")}))})
			}
			return files
		}},
	}
}

// codexCases returns the cases of every kind an agent of the Codex format
// reads under the home prefix.
func codexCases(prefix string) []growthCase {
	cases := append(codexStandingCases(prefix), codexPolicyFileCases(prefix)...)
	cases = append(cases, codexRolloutCases(prefix)...)
	cases = append(cases, codexCompressedCases(prefix)...)
	cases = append(cases, codexStateCases(prefix)...)
	cases = append(cases, codexHistoryCases(prefix)...)
	return append(cases, codexLogCases(prefix)...)
}

// codexDesktopCases returns the cases of the files only Codex itself
// reads: the desktop app's global state and its automations.
func codexDesktopCases() []growthCase {
	const globalState = ".codex/.codex-global-state.json"
	return []growthCase{
		{kind: ".codex-global-state.json", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(globalState, []byte(`{"thread-workspace-root-hints":{"t":"/`+long("p")+`"},`+
				`"electron-persisted-atom-state":{"prompt-history":{"t":[`+joined(n, func(i int) string {
				return `{"markdown":"` + numbered("p", i) + `"}`
			})+`]}}}`))
		}},
		{kind: ".codex-global-state.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			entries := joined(n, func(i int) string { return `{"markdown":"same"}` })
			return one(globalState, []byte(`{"electron-persisted-atom-state":{"prompt-history":{"global":[`+
				entries+`],"t":[`+entries+`]},"thread-descriptions-v1":{`+joined(n, func(i int) string {
				return `"` + numbered("t", i) + `":"d"`
			})+`}}}`))
		}},
		{kind: ".codex-global-state.json", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(globalState, []byte(`{"electron-persisted-atom-state":{"heartbeat-thread-permissions-by-id":{`+
				joined(n, func(i int) string { return `"` + numbered("t", i) + `":` + strconv.Itoa(i) })+
				`,"deep":{"sandboxPolicy":`+nestedObjects(n, "a", "1")+`}}}}`))
		}},
		{kind: "automation.toml", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return ".codex/automations/" + numbered("a", i) + "/automation.toml" },
				func(i int) string {
					return `id = "` + numbered("a", i) + `"` + "\nname = \"n\"\nstatus = \"ACTIVE\"\nprompt = \"p\"\n"
				})
		}},
		{kind: "automation.toml", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".codex/automations/a/automation.toml", []byte(`id = "a"`+"\nprompt = "+
				nestedArrays(codexTOMLDepth, `"x"`)+"\nname = ["+joined(n, func(i int) string { return `{a = ` + strconv.Itoa(i) + `}` })+"]\n"))
		}},
	}
}

// codexGrowth returns the codex-cli growth set.
func codexGrowth() growthSet {
	return growthSet{parser: "codex-cli", cases: append(codexCases(".codex"), codexDesktopCases()...)}
}
