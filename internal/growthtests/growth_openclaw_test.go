package growthtests

import (
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The OpenClaw growth cases: the agent database, the Gateway state
// database, the JSONL transcripts with their sibling sessions.json, and
// the zstd-compressed cold transcripts and archives.

// The files the cases write.
const (
	openclawSessions = ".openclaw/sessions/"
	openclawAgent    = ".openclaw/agents/main/agent/openclaw-agent.sqlite"
	openclawState    = ".openclaw/state/openclaw.sqlite"
	openclawCold     = openclawSessions + "cold/"
)

// The tables of the agent database the cases fill, in the columns the
// parser reads.
const (
	openclawWindowsSQL = "CREATE TABLE session_windows (session_id TEXT, session_key TEXT, channel TEXT, " +
		"account_id TEXT, chat_type TEXT, model_provider TEXT, model TEXT, display_name TEXT, started_at INTEGER)"
	openclawEventsSQL = "CREATE TABLE transcript_events (session_id TEXT, seq INTEGER, event_json TEXT, " +
		"created_at INTEGER, event_zstd BLOB)"
	openclawArchivesSQL = "CREATE TABLE session_transcript_archives (session_id TEXT, session_key TEXT, " +
		"reason TEXT, encoding TEXT, archive_name TEXT, created_at INTEGER, archive_blob BLOB)"
	openclawColdSQL = "CREATE TABLE session_transcript_cold_archives (session_id TEXT, archive_name TEXT, " +
		"archived_at INTEGER, storage TEXT, archive_blob BLOB)"
	openclawChunksSQL = "CREATE TABLE session_transcript_cold_archive_chunks (session_id TEXT, " +
		"chunk_index INTEGER, archive_name TEXT, archive_blob BLOB)"
)

// openclawHeader returns a transcript's session header in the directory.
func openclawHeader(identifier, cwd string) string {
	return `{"type":"session","version":3,"id":"` + identifier + `","timestamp":"2026-01-01T00:00:00Z","cwd":"` + cwd + `"}`
}

// openclawEntry returns transcript entry i: a user prompt, an assistant
// reply calling a tool, or the tool's result, in turn.
func openclawEntry(i int) string {
	head := `{"type":"message","id":"` + numbered("e", i) + `","timestamp":"2026-01-01T00:00:00Z","message":`
	switch i % 3 {
	case 0:
		return head + `{"role":"user","content":"` + numbered("ask ", i) + `"}}`
	case 1:
		return head + `{"role":"assistant","content":[{"type":"text","text":"ok"},{"type":"toolCall",` +
			`"id":"` + numbered("c", i) + `","name":"bash","arguments":{"command":"ls"}}]}}`
	}
	return head + `{"role":"toolResult","toolCallId":"` + numbered("c", i-1) + `","toolName":"bash",` +
		`"content":[{"type":"text","text":"out"}]}}`
}

// openclawTranscript returns a transcript: the header line, then n entries.
func openclawTranscript(header string, n int) []byte {
	return append([]byte(header+"\n"), lines(n, openclawEntry)...)
}

// openclawGrowth returns the openclaw growth set.
func openclawGrowth() growthSet {
	var cases []growthCase
	cases = append(cases, openclawTranscriptCases()...)
	cases = append(cases, openclawColdCases()...)
	cases = append(cases, openclawAgentCases()...)
	cases = append(cases, openclawJoinCases()...)
	cases = append(cases, openclawRepeatCases()...)
	cases = append(cases, openclawStateCases()...)
	return growthSet{parser: "openclaw", cases: cases}
}

// openclawTranscriptCases returns the cases of the JSONL transcripts and
// their sibling sessions.json.
func openclawTranscriptCases() []growthCase {
	const transcript = openclawSessions + "s1.jsonl"
	return []growthCase{
		{kind: "transcript", shape: shapeCarried + " by the header's cwd", files: func(t *testing.T, n int) []growthFile {
			return one(transcript, openclawTranscript(openclawHeader("s1", "/"+long("p")), n))
		}},
		{kind: "transcript", shape: shapeCarried + " by a model change", files: func(t *testing.T, n int) []growthFile {
			change := `{"type":"model_change","provider":"p","modelId":"` + long("model") + `"}`
			return one(transcript, openclawTranscript(openclawHeader("s1", "/p")+"\n"+change, n))
		}},
		{kind: "transcript", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			files := []growthFile{text(openclawSessions+"sessions.json", "{"+joined(n, func(i int) string {
				return `"agent:main:` + strconv.Itoa(i) + `":{"sessionId":"` + numbered("s", i) + `"}`
			})+"}")}
			return append(files, manyFiles(n, func(i int) string { return openclawSessions + numbered("s", i) + ".jsonl" },
				func(i int) string { return string(openclawTranscript(openclawHeader(numbered("s", i), "/p"), 3)) })...)
		}},
		{kind: "transcript", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			files := []growthFile{text(openclawSessions+"sessions.json", "{"+joined(n, func(i int) string {
				return `"` + numbered("key", i) + `":{"sessionId":"s1"}`
			})+"}")}
			return append(files, growthFile{rel: transcript, data: lines(n, func(i int) string {
				return openclawHeader("s1", "/p") + "\n" + openclawEntry(i)
			})})
		}},
		{kind: "transcript", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			deep := `{"type":"message","message":{"role":"assistant","content":` + nestedArrays(n, `{"type":"text"}`) + `}}`
			return one(openclawSessions+"s1.jsonl.reset.2026-01-01T00-00-00.000Z",
				append(lines(n, func(i int) string { return `{"type":"message","message":` }), []byte(deep+"\n")...))
		}},
		{kind: "transcript", shape: shapeManyFiles + " compressed", n: 16, files: func(t *testing.T, n int) []growthFile {
			files := make([]growthFile, 0, n)
			for i := range n {
				files = append(files, growthFile{rel: openclawSessions + numbered("s", i) +
					".jsonl.deleted.2026-01-01T00-00-00.000Z.zst",
					data: zstdFrame(27, openclawTranscript(openclawHeader(numbered("s", i), "/p"), 3))})
			}
			return files
		}},
	}
}

// openclawColdName returns the name of cold transcript i: 64 hexadecimal
// digits.
func openclawColdName(i int) string {
	digits := strconv.FormatInt(int64(i), 16)
	return openclawCold + strings.Repeat("0", 64-len(digits)) + digits + ".jsonl.zst"
}

// openclawEvent returns a cold archive's event record wrapping entry.
func openclawEvent(seq int, entry string) string {
	return `{"kind":"event","row":{"seq":` + strconv.Itoa(seq) + `,"created_at":1767000000000,"event_json":"` +
		strings.ReplaceAll(entry, `"`, `\"`) + `"}}`
}

// openclawColdCases returns the cases of the cold transcripts.
func openclawColdCases() []growthCase {
	return []growthCase{
		{kind: "cold transcript", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			files := make([]growthFile, 0, n)
			for i := range n {
				body := `{"kind":"header","sessionId":"` + numbered("s", i) + `"}` + "\n" + openclawEvent(1, openclawEntry(0))
				files = append(files, growthFile{rel: openclawColdName(i), data: zstdFrame(27, []byte(body))})
			}
			return files
		}},
		{kind: "cold transcript", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			body := `{"kind":"header","sessionId":"s1"}` + "\n" + openclawEvent(0, openclawHeader("s1", "/"+long("p"))) +
				"\n" + string(lines(n, func(i int) string { return openclawEvent(i+1, openclawEntry(i)) }))
			return []growthFile{{rel: openclawColdName(1), data: zstdFrame(27, []byte(body))}}
		}},
		{kind: "cold transcript", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			body := string(lines(n, func(i int) string { return `{"kind":"event","row":{"seq":1,"event_json":7}}` })) +
				openclawEvent(0, `{"type":"custom","customType":"x","data":`+nestedObjects(n, "a", "1")+`}`)
			return []growthFile{{rel: openclawColdName(2), data: zstdFrame(27, []byte(body))}}
		}},
	}
}

// openclawWindow returns a session_windows row of the session on the
// channel with the model and display name.
func openclawWindow(sessionID, model, name string) []any {
	return []any{sessionID, "agent:main:" + sessionID, "telegram", "acct", "direct", "openai", model, name,
		int64(1767000000000)}
}

// openclawEvents returns the transcript_events rows of the session: its
// header in the directory, then n entries, each compressed when zstd is
// set.
func openclawEvents(sessionID, cwd string, n int, zstd bool) [][]any {
	return rows(n+1, func(i int) []any {
		entry := openclawHeader(sessionID, cwd)
		if i > 0 {
			entry = openclawEntry(i - 1)
		}
		if zstd {
			return []any{sessionID, int64(i), nil, int64(1767000000000 + i), zstdFrame(27, []byte(entry))}
		}
		return []any{sessionID, int64(i), entry, int64(1767000000000 + i), nil}
	})
}

// openclawAgentDatabase returns an agent database of the windows and
// events and of the other tables.
func openclawAgentDatabase(t *testing.T, windows, events [][]any, others ...sqlitetest.Table) []byte {
	t.Helper()
	return database(t, append([]sqlitetest.Table{{Name: "session_windows", SQL: openclawWindowsSQL, Rows: windows},
		{Name: "transcript_events", SQL: openclawEventsSQL, Rows: events}}, others...)...)
}

// openclawAgentCases returns the cases of the agent database's windows,
// events and archives.
func openclawAgentCases() []growthCase {
	return []growthCase{
		{kind: "openclaw-agent.sqlite", shape: shapeCarried + " by a window's model", files: func(t *testing.T, n int) []growthFile {
			return one(openclawAgent, openclawAgentDatabase(t, [][]any{openclawWindow("s1", long("model"), "chat")},
				openclawEvents("s1", "/p", n, false)))
		}},
		{kind: "openclaw-agent.sqlite", shape: shapeCarried + " by the header's cwd", files: func(t *testing.T, n int) []growthFile {
			pending := sqlitetest.Table{Name: "session_pending_inputs", SQL: "CREATE TABLE session_pending_inputs " +
				"(seq INTEGER, input_id TEXT, session_id TEXT, run_id TEXT, message_json TEXT, state TEXT, " +
				"accepted_at INTEGER, consumed_event_id TEXT)", Rows: rows(n, func(i int) []any {
				return []any{int64(i + 1), numbered("in", i), "s1", "run", `{"role":"user","content":"queued"}`,
					"queued", int64(1767000000000), nil}
			})}
			return one(openclawAgent, openclawAgentDatabase(t, [][]any{openclawWindow("s1", "m", "chat")},
				openclawEvents("s1", "/"+long("p"), n, false), pending))
		}},
		{kind: "openclaw-agent.sqlite", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(openclawAgent, openclawAgentDatabase(t, rows(n, func(i int) []any {
				return openclawWindow("s1", numbered("m", i), "chat")
			}), openclawEvents("s1", "/p", n, true)))
		}},
		{kind: "openclaw-agent.sqlite", shape: shapeLargeShared + " by archives of one window", files: func(t *testing.T, n int) []growthFile {
			archive := zstdFrame(27, []byte(openclawHeader("s1", "/p")))
			return one(openclawAgent, openclawAgentDatabase(t, [][]any{openclawWindow("s1", "m", long("name"))}, nil,
				sqlitetest.Table{Name: "session_transcript_archives", SQL: openclawArchivesSQL,
					Rows: rows(n, func(i int) []any {
						return []any{"s1", "agent:main:s1", "reset", "zstd", numbered("a", i), int64(i), archive}
					})}))
		}},
		{kind: "openclaw-agent.sqlite", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			events := rows(n, func(i int) []any { return []any{"s1", int64(i), `{"type":`, int64(i), nil} })
			deep := `{"type":"message","message":{"role":"user","content":` + nestedArrays(n, `{"type":"text","text":"x"}`) + `}}`
			return one(openclawAgent, openclawAgentDatabase(t, nil, append(events, []any{"s1", int64(n), deep,
				int64(n), nil})))
		}},
	}
}

// openclawJoinCases returns the cases of the agent database's joined
// tables: the cold archives and their chunks, the conversations and the
// session metadata.
func openclawJoinCases() []growthCase {
	return []growthCase{
		{kind: "openclaw-agent.sqlite", shape: shapeJoins, files: func(t *testing.T, n int) []growthFile {
			large := openclawHeader("s1", "/p") + "\n" + `{"type":"message","message":{"role":"user","content":"` +
				strings.Repeat("x", 1024*n) + `"}}`
			return one(openclawAgent, openclawAgentDatabase(t, nil, nil,
				sqlitetest.Table{Name: "session_transcript_cold_archives", SQL: openclawColdSQL, Rows: rows(n, func(i int) []any {
					return []any{"s1", numbered("cold", i), int64(i), "sqlite", zstdFrame(27, []byte(openclawHeader("s1", "/p")))}
				})},
				sqlitetest.Table{Name: "session_transcript_cold_archive_chunks", SQL: openclawChunksSQL,
					Rows: rows(n, func(i int) []any {
						chunk := []byte(openclawEntry(i))
						if i == 0 {
							chunk = []byte(large)
						}
						return []any{"s1", int64(i), numbered("chunk", i), chunk}
					})}))
		}},
		{kind: "openclaw-agent.sqlite", shape: shapeLargeShared + " by a conversation", files: func(t *testing.T, n int) []growthFile {
			return one(openclawAgent, openclawAgentDatabase(t, nil, nil,
				sqlitetest.Table{Name: "conversations", SQL: "CREATE TABLE conversations (conversation_id TEXT, " +
					"channel TEXT, account_id TEXT, kind TEXT, peer_id TEXT, thread_id TEXT)",
					Rows: [][]any{{"c", "telegram", "acct", "direct", long("peer"), nil}}},
				sqlitetest.Table{Name: "session_conversations", SQL: "CREATE TABLE session_conversations " +
					"(session_id TEXT, conversation_id TEXT, role TEXT, first_seen_at INTEGER)",
					Rows: rows(n, func(i int) []any { return []any{numbered("s", i), "c", "primary", int64(i)} })},
				sqlitetest.Table{Name: "conversation_deliveries", SQL: "CREATE TABLE conversation_deliveries " +
					"(operation_id TEXT, operation_kind TEXT, conversation_id TEXT, source_session_key TEXT, " +
					"status TEXT, created_at INTEGER)", Rows: rows(n, func(i int) []any {
					return []any{numbered("op", i), "send", "c", "agent:main:s0", "sent", int64(i)}
				})}))
		}},
		{kind: "openclaw-agent.sqlite", shape: shapeSharedID + " by conversations rows", files: func(t *testing.T, n int) []growthFile {
			return one(openclawAgent, openclawAgentDatabase(t, nil, nil,
				sqlitetest.Table{Name: "conversations", SQL: "CREATE TABLE conversations (conversation_id TEXT, " +
					"channel TEXT, account_id TEXT, kind TEXT, peer_id TEXT, thread_id TEXT)",
					Rows: rows(n, func(i int) []any { return []any{"c", "telegram", "acct", "direct", numbered("peer", i), nil} })},
				sqlitetest.Table{Name: "session_conversations", SQL: "CREATE TABLE session_conversations " +
					"(session_id TEXT, conversation_id TEXT, role TEXT, first_seen_at INTEGER)",
					Rows: rows(n, func(i int) []any { return []any{numbered("s", i), "c", "primary", int64(i)} })}))
		}},
		{kind: "openclaw-agent.sqlite", shape: shapeSharedID + " in the session metadata", files: func(t *testing.T, n int) []growthFile {
			return one(openclawAgent, openclawAgentDatabase(t, [][]any{openclawWindow("s1", "m", "chat")},
				openclawEvents("s1", "/p", 3, false), sqlitetest.Table{Name: "session_nodes",
					SQL: "CREATE TABLE session_nodes (session_key TEXT, current_session_id TEXT, entry_json TEXT, " +
						"updated_at INTEGER)", Rows: rows(n, func(i int) []any {
						return []any{numbered("key", i), "s1", `{"modelOverride":"m2","execHost":"gateway",` +
							`"delivery":{"kind":"external","route":{"channel":"slack"}}}`, int64(i)}
					})},
				sqlitetest.Table{Name: "memory_session_tombstones", SQL: "CREATE TABLE memory_session_tombstones " +
					"(session_id TEXT, agent_id TEXT, reason TEXT, created_at INTEGER)",
					Rows: rows(n, func(i int) []any { return []any{"s1", "main", "forget", int64(i)} })}))
		}},
	}
}

// openclawRepeatCases returns the cases of the agent database's keys
// stored more than once: a window id with the archives of its session,
// and a session key with the deliveries from it.
func openclawRepeatCases() []growthCase {
	return []growthCase{
		{kind: "openclaw-agent.sqlite", shape: shapeSharedID + " by window rows and archives", files: func(t *testing.T, n int) []growthFile {
			archive := zstdFrame(27, []byte(openclawHeader("s1", "/p")+"\n"+openclawEntry(1)))
			return one(openclawAgent, openclawAgentDatabase(t, rows(n, func(i int) []any {
				return openclawWindow("s1", numbered("m", i), long("name"))
			}), openclawEvents("s1", "/p", n, false), sqlitetest.Table{Name: "session_transcript_archives",
				SQL: openclawArchivesSQL, Rows: rows(n, func(i int) []any {
					return []any{"s1", "agent:main:s1", "reset", "zstd", numbered("a", i), int64(i), archive}
				})}))
		}},
		{kind: "openclaw-agent.sqlite", shape: shapeSharedID + " by session_nodes rows of one key", files: func(t *testing.T, n int) []growthFile {
			return one(openclawAgent, openclawAgentDatabase(t, nil, openclawEvents("s1", "/"+long("p"), 3, false),
				sqlitetest.Table{Name: "session_nodes", SQL: "CREATE TABLE session_nodes (session_key TEXT, " +
					"current_session_id TEXT, entry_json TEXT, updated_at INTEGER)", Rows: rows(n, func(i int) []any {
					return []any{"agent:main:s0", numbered("s", i%2+1), `{"execHost":"gateway"}`, int64(i)}
				})},
				sqlitetest.Table{Name: "conversations", SQL: "CREATE TABLE conversations (conversation_id TEXT, " +
					"channel TEXT)", Rows: [][]any{{"c", "telegram"}}},
				sqlitetest.Table{Name: "conversation_deliveries", SQL: "CREATE TABLE conversation_deliveries " +
					"(operation_id TEXT, operation_kind TEXT, conversation_id TEXT, source_session_key TEXT, " +
					"status TEXT, created_at INTEGER)", Rows: rows(n, func(i int) []any {
					return []any{numbered("op", i), "send", "c", "agent:main:s0", "sent", int64(i)}
				})}))
		}},
	}
}

// openclawStateCases returns the cases of the Gateway state database.
func openclawStateCases() []growthCase {
	const config = "CREATE TABLE exec_approvals_config (raw_json TEXT, updated_at_ms INTEGER)"
	return []growthCase{
		{kind: "openclaw.sqlite", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			raw := `{"socket":{"token":"t"},"agents":{"` + long("agent") + `":{"security":"allowlist","allowlist":[` +
				joined(n, func(i int) string { return `{"pattern":"` + numbered("cmd", i) + `"}` }) + `]}}}`
			return one(openclawState, database(t, sqlitetest.Table{Name: "exec_approvals_config", SQL: config,
				Rows: [][]any{{raw, int64(1767000000000)}}}))
		}},
		{kind: "openclaw.sqlite", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(openclawState, database(t,
				sqlitetest.Table{Name: "operator_approvals", SQL: "CREATE TABLE operator_approvals (approval_id TEXT, " +
					"kind TEXT, status TEXT, decision TEXT, terminal_reason TEXT, resolver_kind TEXT, " +
					"source_session_id TEXT, source_tool_call_id TEXT, source_tool_name TEXT, created_at_ms INTEGER, " +
					"resolved_at_ms INTEGER)", Rows: rows(n, func(i int) []any {
					return []any{"a", "exec", "resolved", "allow-once", "user", "operator", "s1", "c", "bash", int64(i),
						int64(i + 1)}
				})},
				sqlitetest.Table{Name: "channel_pairing_allow_entries", SQL: "CREATE TABLE channel_pairing_allow_entries " +
					"(channel_key TEXT, account_id TEXT, entry TEXT, sort_order INTEGER, updated_at INTEGER)",
					Rows: rows(n, func(i int) []any { return []any{"telegram", "acct", "42", int64(0), int64(i)} })},
				sqlitetest.Table{Name: "projects", SQL: "CREATE TABLE projects (repo_root TEXT, origin_url TEXT, " +
					"created_at_ms INTEGER)", Rows: rows(n, func(i int) []any {
					return []any{"/p", "https://github.com/o/r", int64(i)}
				})}))
		}},
		{kind: "openclaw.sqlite", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			raw := `{"defaults":{"ask":` + nestedArrays(n, "1") + `,"allowlist":[` + joined(n, func(i int) string {
				return "7"
			}) + `]}}`
			return one(openclawState, database(t, sqlitetest.Table{Name: "exec_approvals_config", SQL: config,
				Rows: [][]any{{raw, int64(0)}}}, sqlitetest.Table{Name: "backup_runs", SQL: "CREATE TABLE backup_runs " +
				"(id TEXT, created_at INTEGER, archive_path TEXT, status TEXT, manifest_json TEXT)",
				Rows: rows(n, func(i int) []any { return []any{numbered("b", i), int64(i), "/a", "ok", "{"} })}))
		}},
	}
}
