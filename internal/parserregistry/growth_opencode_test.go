package parserregistry

import (
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The OpenCode growth cases: the 1.x and 2.x databases, the legacy JSON
// trees of sessions, messages and parts in both layouts, and the terminal
// UI's state files. Kilo Code keeps the same stores under its own
// directories, so its set builds these cases again there.

// opencodeLayout is where one OpenCode build keeps its stores, relative to
// the home: the data directory, the state directory and a database's file
// name.
type opencodeLayout struct {
	data, state, database string
}

// opencodeSession is the one session id of a case.
const opencodeSession = "ses_1"

// opencodeGreek is the lower-case Greek letters a case spells ids from:
// a disk that matches names in any case, as macOS and Windows do, matches
// these in any case too, though they are not ASCII.
var opencodeGreek = []rune("αβγδεζηθικ")

// opencodeGreekSpelling returns the Greek id with its letters upper-cased
// where the bits of i say so, so each i below 2^10 spells it differently.
func opencodeGreekSpelling(i int) string {
	var out strings.Builder
	for bit, letter := range opencodeGreek {
		if i>>bit&1 == 1 {
			letter -= 'α' - 'Α'
		}
		out.WriteRune(letter)
	}
	return out.String()
}

// opencodeProjectTable returns the 1.x project table holding one project.
func opencodeProjectTable(worktree string) sqlitetest.Table {
	return sqlitetest.Table{Name: "project", SQL: "CREATE TABLE project (id text PRIMARY KEY, worktree text NOT NULL, " +
		"time_created integer, time_updated integer)", Rows: [][]any{{"p1", worktree, 1700000000000, 1700000000000}}}
}

// opencodeSessionTable returns a session table (session or session_v2)
// holding rows of (id, project_id, directory, title, time_created).
func opencodeSessionTable(name string, sessions [][]any) sqlitetest.Table {
	return sqlitetest.Table{Name: name, SQL: "CREATE TABLE " + name + " (id text PRIMARY KEY, project_id text, " +
		"directory text, title text, time_created integer)", Rows: sessions}
}

// opencodeV1Tables returns the 1.x message and part tables, rows of (id,
// session_id, time_created, data) and (id, message_id, session_id,
// time_created, data).
func opencodeV1Tables(messages, parts [][]any) []sqlitetest.Table {
	return []sqlitetest.Table{
		{Name: "message", SQL: "CREATE TABLE message (id text, session_id text, time_created integer, data text)",
			Rows: messages},
		{Name: "part", SQL: "CREATE TABLE part (id text, message_id text, session_id text, time_created integer, " +
			"data text)", Rows: parts}}
}

// opencodeV2Tables returns the 2.x session_message and session_part
// tables, rows of (id, session_id, type, seq, time_created, data) and (id,
// message_id, session_id, type, ordinal, data).
func opencodeV2Tables(messages, parts [][]any) []sqlitetest.Table {
	return []sqlitetest.Table{
		{Name: "session_message", SQL: "CREATE TABLE session_message (id text, session_id text, type text, " +
			"seq integer, time_created integer, data text)", Rows: messages},
		{Name: "session_part", SQL: "CREATE TABLE session_part (id text, message_id text, session_id text, " +
			"type text, ordinal integer, data text)", Rows: parts}}
}

// opencodeUserMessage returns a 1.x message row of the session: a user
// message by id at a time offset.
func opencodeUserMessage(messageID string, i int) []any {
	return []any{messageID, opencodeSession, 1700000000500 + i, `{"role":"user","time":{"created":` +
		strconv.Itoa(1700000000500+i) + `}}`}
}

// opencodeTextPart returns a 1.x part row of a message: a text part by id.
func opencodeTextPart(partID, messageID string, i int) []any {
	return []any{partID, messageID, opencodeSession, 1700000000500 + i, `{"type":"text","text":"hello there"}`}
}

// opencodeGrowth returns the OpenCode growth set.
func opencodeGrowth() growthSet {
	return growthSet{parser: "opencode", cases: opencodeCases(opencodeLayout{data: ".local/share/opencode",
		state: ".local/state/opencode", database: "opencode.db"})}
}

// opencodeCases returns the cases of the stores of one OpenCode build.
func opencodeCases(layout opencodeLayout) []growthCase {
	var cases []growthCase
	for _, group := range [][]growthCase{opencodeDatabaseCases(layout), opencodeOrphanCases(layout), opencodeRepeatCases(layout), opencodeJoinCases(layout), opencodeMoreDatabaseCases(layout),
		opencodeSessionFileCases(layout), opencodeMessageFileCases(layout), opencodeMoreMessageFileCases(layout),
		opencodeOldLayoutCases(layout),
		opencodeStateCases(layout)} {
		cases = append(cases, group...)
	}
	return cases
}

// opencodeOrphanCases returns the cases of a database whose parts and
// items name no message or record.
func opencodeOrphanCases(layout opencodeLayout) []growthCase {
	rel := layout.data + "/" + layout.database
	return []growthCase{
		// Each part naming no message, and each item naming no record, gives
		// its own rows, and one parser: row counts them per table.
		{kind: "database", shape: shapeSharedID + " (parts naming no message)", files: func(t *testing.T, n int) []growthFile {
			parts := rows(n, func(i int) []any { return opencodeTextPart(numbered("prt_", i), numbered("msg_", i), i) })
			items := sqlitetest.Table{Name: "session_part", SQL: "CREATE TABLE session_part (id text, message_id text, " +
				"session_id text, type text, ordinal integer, data text)", Rows: rows(n, func(i int) []any {
				return []any{numbered("itm_", i), numbered("rec_", i), opencodeSession, "text", 0, `{"text":"part"}`}
			})}
			records := sqlitetest.Table{Name: "session_message", SQL: "CREATE TABLE session_message (id text, " +
				"session_id text, type text, seq integer, time_created integer, data text)"}
			return one(rel, database(t, append([]sqlitetest.Table{opencodeSessionTable("session",
				[][]any{{opencodeSession, nil, "/work/proj", "t", 1700000000000}}), items, records},
				opencodeV1Tables(nil, parts)...)...))
		}},
	}
}

// opencodeDatabaseCases returns the cases of a database's repeated ids.
func opencodeDatabaseCases(layout opencodeLayout) []growthCase {
	rel := layout.data + "/" + layout.database
	session := []any{opencodeSession, nil, "/work/proj", "t", 1700000000000}
	return []growthCase{
		{kind: "database", shape: shapeSharedID + " (1.x)", n: 128, files: func(t *testing.T, n int) []growthFile {
			messages := rows(n, func(i int) []any { return opencodeUserMessage("msg_1", i) })
			parts := rows(n, func(i int) []any { return opencodeTextPart(numbered("prt_", i), "msg_1", i) })
			share := sqlitetest.Table{Name: "session_share", SQL: "CREATE TABLE session_share (session_id text, id text, " +
				"url text, time_created integer)", Rows: rows(n, func(i int) []any {
				return []any{opencodeSession, numbered("shr_", i), "https://opncd.example/s/x", 1700000002000}
			})}
			return one(rel, database(t, append([]sqlitetest.Table{opencodeSessionTable("session", [][]any{session}),
				share}, opencodeV1Tables(messages, parts)...)...))
		}},
		{kind: "database", shape: shapeSharedID + " (session rows)", n: 128, files: func(t *testing.T, n int) []growthFile {
			sessions := rows(n, func(i int) []any {
				return []any{opencodeSession, nil, "/work/" + numbered("proj_", i), "t", 1700000000000 + i}
			})
			messages := rows(n, func(i int) []any { return opencodeUserMessage(numbered("msg_", i), i) })
			parts := rows(n, func(i int) []any { return opencodeTextPart(numbered("prt_", i), numbered("msg_", i), i) })
			return one(rel, database(t, append([]sqlitetest.Table{{Name: "session", SQL: "CREATE TABLE session (id text, " +
				"project_id text, directory text, title text, time_created integer)", Rows: sessions}},
				opencodeV1Tables(messages, parts)...)...))
		}},
		// Each stored row of a repeated project id gives a parser: row; the
		// sessions naming no directory and the saved grants are members.
		{kind: "database", shape: shapeSharedID + " (project rows)", files: func(t *testing.T, n int) []growthFile {
			projects := sqlitetest.Table{Name: "project", SQL: "CREATE TABLE project (id text, worktree text, " +
				"time_created integer, time_updated integer)", Rows: rows(n, func(i int) []any {
				return []any{"p1", "/work/" + numbered("proj_", i), 1700000000000 + i, 1700000000000}
			})}
			sessions := rows(n, func(i int) []any { return []any{numbered("ses_", i), "p1", nil, "t", 1700000000000} })
			grants := sqlitetest.Table{Name: "permission", SQL: "CREATE TABLE permission (id text, project_id text, " +
				"action text, resource text, time_created integer)", Rows: rows(n, func(i int) []any {
				return []any{numbered("per_", i), "p1", "bash", numbered("cmd", i), 1700000000000 + i}
			})}
			return one(rel, database(t, projects, opencodeSessionTable("session", sessions), grants))
		}},
		{kind: "database", shape: shapeSharedID + " (2.x)", files: func(t *testing.T, n int) []growthFile {
			messages := rows(n, func(i int) []any {
				return []any{"msg_1", opencodeSession, "assistant", i, 1700000001000 + i, `{"content":[]}`}
			})
			parts := rows(n, func(i int) []any {
				return []any{numbered("prt_", i), "msg_1", opencodeSession, "text", i, `{"text":"part"}`}
			})
			return one(rel, database(t, append([]sqlitetest.Table{opencodeSessionTable("session_v2",
				[][]any{session})}, opencodeV2Tables(messages, parts)...)...))
		}},
	}
}

// opencodeRepeatedSessions returns a session table (session or
// session_v2) that holds the session id n times, each row in its own
// directory: the table declares no key, so it can.
func opencodeRepeatedSessions(name string, n int) sqlitetest.Table {
	return sqlitetest.Table{Name: name, SQL: "CREATE TABLE " + name + " (id text, project_id text, directory text, " +
		"title text, time_created integer)", Rows: rows(n, func(i int) []any {
		return []any{opencodeSession, nil, "/work/" + numbered("proj_", i), "t", 1700000000000 + i}
	})}
}

// opencodeRepeatCases returns the cases of a database whose session id is
// repeated: in session_v2 with a message each, and in either table with a
// message id repeated too and the parts of that id.
func opencodeRepeatCases(layout opencodeLayout) []growthCase {
	rel := layout.data + "/" + layout.database
	assistant := func(i int) []any {
		return []any{"msg_1", opencodeSession, "assistant", i, 1700000001000 + i, `{"content":[]}`}
	}
	return []growthCase{
		{kind: "database", shape: shapeSharedID + " (2.x session rows)", n: 128, files: func(t *testing.T, n int) []growthFile {
			messages := rows(n, func(i int) []any {
				return []any{numbered("msg_", i), opencodeSession, "user", i, 1700000001000 + i, `{"text":"hi"}`}
			})
			return one(rel, database(t, append([]sqlitetest.Table{opencodeRepeatedSessions("session_v2", n)},
				opencodeV2Tables(messages, nil)...)...))
		}},
		{kind: "database", shape: shapeSharedID + " (1.x session and message ids)", n: 128,
			files: func(t *testing.T, n int) []growthFile {
				messages := rows(n, func(i int) []any { return opencodeUserMessage("msg_1", i) })
				parts := rows(n, func(i int) []any { return opencodeTextPart(numbered("prt_", i), "msg_1", i) })
				return one(rel, database(t, append([]sqlitetest.Table{opencodeRepeatedSessions("session", n)},
					opencodeV1Tables(messages, parts)...)...))
			}},
		{kind: "database", shape: shapeSharedID + " (2.x session and message ids)", n: 128,
			files: func(t *testing.T, n int) []growthFile {
				parts := rows(n, func(i int) []any {
					return []any{numbered("prt_", i), "msg_1", opencodeSession, "text", i, `{"text":"part"}`}
				})
				return one(rel, database(t, append([]sqlitetest.Table{opencodeRepeatedSessions("session_v2", n)},
					opencodeV2Tables(rows(n, assistant), parts)...)...))
			}},
	}
}

// opencodeJoinCases returns the case of a database's sessions joined to
// one project.
func opencodeJoinCases(layout opencodeLayout) []growthCase {
	rel := layout.data + "/" + layout.database
	return []growthCase{
		{kind: "database", shape: shapeJoins, files: func(t *testing.T, n int) []growthFile {
			sessions := rows(n, func(i int) []any { return []any{numbered("ses_", i), "p1", nil, "t", 1700000000000 + i} })
			messages := rows(n, func(i int) []any {
				return []any{numbered("msg_", i), numbered("ses_", i), 1700000000500 + i, `{"role":"user"}`}
			})
			parts := rows(n, func(i int) []any {
				return []any{numbered("prt_", i), numbered("msg_", i), numbered("ses_", i), 1, `{"type":"text","text":"hi"}`}
			})
			return one(rel, database(t, append([]sqlitetest.Table{opencodeProjectTable("/" + strings.Repeat("w", 1000*n)),
				opencodeSessionTable("session", sessions)}, opencodeV1Tables(messages, parts)...)...))
		}},
	}
}

// opencodeMoreDatabaseCases returns the cases of a database's carried
// directory, damaged records and standing tables.
func opencodeMoreDatabaseCases(layout opencodeLayout) []growthCase {
	rel := layout.data + "/" + layout.database
	return []growthCase{
		{kind: "database", shape: shapeCarried + " (a repeated session id's directory)", files: func(t *testing.T,
			n int) []growthFile {
			sessions := [][]any{{opencodeSession, nil, "/" + long("p"), "t", 1700000000000},
				{opencodeSession, nil, "/" + long("q"), "t", 1700000000001}}
			messages := rows(n, func(i int) []any { return opencodeUserMessage(numbered("msg_", i), i) })
			parts := rows(n, func(i int) []any { return opencodeTextPart(numbered("prt_", i), numbered("msg_", i), i) })
			return one(rel, database(t, append([]sqlitetest.Table{{Name: "session", SQL: "CREATE TABLE session (id text, " +
				"project_id text, directory text, title text, time_created integer)", Rows: sessions}},
				opencodeV1Tables(messages, parts)...)...))
		}},
		{kind: "database", shape: shapeCarried + " (the session's directory)", files: func(t *testing.T, n int) []growthFile {
			session := []any{opencodeSession, nil, "/" + long("p"), "t", 1700000000000}
			messages := rows(n, func(i int) []any {
				return []any{numbered("msg_", i), opencodeSession, "user", i, 1700000001000 + i, `{"text":"hi"}`}
			})
			return one(rel, database(t, append([]sqlitetest.Table{opencodeSessionTable("session_v2",
				[][]any{session})}, opencodeV2Tables(messages, nil)...)...))
		}},
		{kind: "database", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			messages := append(rows(n, func(i int) []any {
				return []any{numbered("msg_", i), opencodeSession, 1700000000500, `{"role":`}
			}), opencodeUserMessage("deep", 0))
			parts := []any{"prt_deep", "deep", opencodeSession, 1, `{"type":"text","text":` + nestedArrays(n, `"x"`) + `}`}
			event := sqlitetest.Table{Name: "event", SQL: "CREATE TABLE event (id text, aggregate_id text, seq integer, " +
				"type text, data text)", Rows: rows(n, func(i int) []any {
				return []any{numbered("evt_", i), "ses_2", i, "session.updated.1", `{"info":` + nestedObjects(i%8, "share", "1") + `}`}
			})}
			return one(rel, database(t, append([]sqlitetest.Table{opencodeSessionTable("session", [][]any{
				{opencodeSession, nil, "/p", "t", 1}}), event}, opencodeV1Tables(messages, [][]any{parts})...)...))
		}},
		{kind: "database", shape: shapeLargeShared + " (the saved grants of one project)",
			files: func(t *testing.T, n int) []growthFile {
				grants := sqlitetest.Table{Name: "permission", SQL: "CREATE TABLE permission (id text, project_id text, " +
					"action text, resource text, time_created integer)", Rows: rows(n, func(i int) []any {
					return []any{numbered("per_", i), "p1", "bash", numbered("cmd", i), 1700000000000 + i}
				})}
				return one(rel, database(t, opencodeProjectTable("/work/proj"), grants))
			}},
	}
}

// opencodeLegacySession returns a legacy session file of a project, with
// extra members.
func opencodeLegacySession(sessionID, projectID, extra string) string {
	return `{"id":"` + sessionID + `","projectID":"` + projectID + `","title":"Old session","version":"0.9.0",` +
		`"time":{"created":1790931600000}` + extra + `}`
}

// opencodeSessionFileCases returns the cases of the legacy session files.
func opencodeSessionFileCases(layout opencodeLayout) []growthCase {
	storage := layout.data + "/storage/"
	return []growthCase{
		{kind: "session file", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			files := []growthFile{text(storage+"project/proj_a1.json", `{"id":"proj_a1","worktree":"/srv/proj","pad":"`+
				strings.Repeat("x", 100*n)+`"}`)}
			return append(files, manyFiles(n, func(i int) string {
				return storage + "session/proj_a1/" + numbered("ses_", i) + ".json"
			}, func(i int) string { return opencodeLegacySession(numbered("ses_", i), "proj_a1", "") })...)
		}},
		{kind: "session file", shape: shapeLargeShared + " by many spellings", n: 32, files: func(t *testing.T, n int) []growthFile {
			files := []growthFile{text(storage+"project/"+opencodeGreekSpelling(0)+".json",
				`{"worktree":"/srv/proj","pad":"`+strings.Repeat("x", 1000*n)+`"}`)}
			return append(files, manyFiles(n, func(i int) string {
				return storage + "session/proj_a1/" + numbered("ses_", i) + ".json"
			}, func(i int) string { return opencodeLegacySession(numbered("ses_", i), opencodeGreekSpelling(i), "") })...)
		}},
		{kind: "session file", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(storage+"session/proj_a1/"+opencodeSession+".json", []byte(opencodeLegacySession(opencodeSession,
				"proj_a1", `,"title":`+nestedArrays(n, `"t"`)+`,"share":`+nestedObjects(n, "url", "1"))))
		}},
	}
}

// opencodeLegacyMessage returns a legacy message file of the session by
// id, with extra members.
func opencodeLegacyMessage(messageID, extra string) string {
	return `{"id":"` + messageID + `","sessionID":"` + opencodeSession + `","role":"user","time":{"created":` +
		`1790931601000},"agent":"build","model":{"providerID":"anthropic","modelID":"claude-sonnet-4"}` + extra + `}`
}

// opencodeMessageFileCases returns the cases of the legacy message files
// and their parts.
func opencodeMessageFileCases(layout opencodeLayout) []growthCase {
	storage := layout.data + "/storage/"
	messages := storage + "message/" + opencodeSession + "/"
	partFiles := func(n int, messageID string) []growthFile {
		return manyFiles(n, func(i int) string { return storage + "part/" + messageID + "/" + numbered("prt_", i) + ".json" },
			func(i int) string { return `{"type":"text","text":"` + strings.Repeat("p", 100) + `"}` })
	}
	return []growthCase{
		{kind: "message file", shape: shapeLargeShared + " by many spellings", n: 16, files: func(t *testing.T, n int) []growthFile {
			return append(partFiles(n, opencodeGreekSpelling(0)), manyFiles(n, func(i int) string {
				return messages + numbered("msg_", 1000+i) + ".json"
			}, func(i int) string { return opencodeLegacyMessage(opencodeGreekSpelling(i), "") })...)
		}},
		{kind: "message file", shape: shapeSharedID, n: 32, files: func(t *testing.T, n int) []growthFile {
			return append(partFiles(n, "msg_a"), manyFiles(n, func(i int) string {
				return messages + numbered("msg_", 1000+i) + ".json"
			}, func(i int) string { return opencodeLegacyMessage("msg_a", "") })...)
		}},
	}
}

// opencodeMoreMessageFileCases returns the cases of the legacy message
// files' carried directory, their listing and their damaged parts.
func opencodeMoreMessageFileCases(layout opencodeLayout) []growthCase {
	storage := layout.data + "/storage/"
	messages := storage + "message/" + opencodeSession + "/"
	return []growthCase{
		{kind: "message file", shape: shapeCarried + " (the session file's directory)", n: 32, files: func(t *testing.T, n int) []growthFile {
			files := []growthFile{text(storage+"session/proj_a1/"+opencodeSession+".json",
				opencodeLegacySession(opencodeSession, "proj_a1", `,"directory":"/`+long("p")+`"`))}
			for i := range n {
				messageID := numbered("msg_", 1000+i)
				files = append(files, text(messages+messageID+".json", opencodeLegacyMessage(messageID, "")),
					text(storage+"part/"+messageID+"/prt_1.json", `{"type":"text","text":"hi"}`))
			}
			return files
		}},
		{kind: "message file", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			files := []growthFile{text(storage+"session/proj_a1/"+opencodeSession+".json",
				opencodeLegacySession(opencodeSession, "proj_a1", `,"directory":"/srv/proj"`))}
			for i := range n {
				messageID := numbered("msg_", 1000+i)
				files = append(files, text(messages+messageID+".json", opencodeLegacyMessage(messageID, "")),
					text(storage+"part/"+messageID+"/prt_1.json", `{"type":"text","text":"hi"}`))
			}
			return files
		}},
		{kind: "message file", shape: shapeUnlisted + " (project directories of the sessions)", n: 32,
			files: func(t *testing.T, n int) []growthFile {
				var files []growthFile
				for i := range n {
					messageID, sessionID := numbered("msg_", 1000+i), numbered("ses_", i)
					files = append(files, text(storage+"session/"+numbered("proj_", i)+"/"+sessionID+".json",
						opencodeLegacySession(sessionID, numbered("proj_", i), "")),
						text(storage+"message/"+sessionID+"/"+messageID+".json", `{"id":"`+messageID+
							`","sessionID":"`+sessionID+`","role":"user","time":{"created":1790931601000}}`))
				}
				return files
			}, unlisted: func(n int) []string {
				rels := make([]string, 0, n)
				for i := range n {
					rels = append(rels, storage+"session/"+numbered("proj_", i))
				}
				return rels
			}},
		{kind: "message file", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			files := manyFiles(n, func(i int) string { return storage + "part/msg_a/" + numbered("prt_", i) + ".json" },
				func(i int) string { return `{"type":` })
			return append(files, text(messages+"msg_a.json", opencodeLegacyMessage("msg_a", `,"error":`+
				nestedObjects(n, "data", `{"message":"x"}`))), text(storage+"part/msg_a/prt_deep.json",
				`{"type":"text","text":`+nestedArrays(n, `"x"`)+`}`))
		}},
	}
}

// opencodeOldLayoutCases returns the cases of the older per-project layout.
func opencodeOldLayoutCases(layout opencodeLayout) []growthCase {
	storage := layout.data + "/project/srv-proj/storage/session/"
	message := func(messageID string) string {
		return `{"id":"` + messageID + `","sessionID":"ses_old","role":"user","time":{"created":1790000001000},` +
			`"path":{"cwd":"/srv/proj","root":"/srv/proj"}}`
	}
	return []growthCase{
		{kind: "old session file", shape: shapeManyFiles, n: 32, files: func(t *testing.T, n int) []growthFile {
			return manyFiles(n, func(i int) string { return storage + "info/" + numbered("ses_", i) + ".json" },
				func(i int) string { return `{"id":"` + numbered("ses_", i) + `","directory":"/srv/proj","title":"t"}` })
		}},
		{kind: "old session file", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(storage+"info/ses_old.json", []byte(`{"id":"ses_old","title":`+nestedArrays(n, `"t"`)+`}`))
		}},
		{kind: "old message file", shape: shapeCarried + " (the session file's directory)", n: 32,
			files: func(t *testing.T, n int) []growthFile {
				files := []growthFile{text(storage+"info/ses_old.json", `{"id":"ses_old","directory":"/`+long("p")+`"}`)}
				for i := range n {
					messageID := numbered("msg_", 1000+i)
					files = append(files, text(storage+"message/ses_old/"+messageID+".json", message(messageID)),
						text(storage+"part/ses_old/"+messageID+"/prt_1.json", `{"type":"text","text":"hi"}`))
				}
				return files
			}},
		{kind: "old message file", shape: shapeSharedID, n: 32, files: func(t *testing.T, n int) []growthFile {
			files := manyFiles(n, func(i int) string { return storage + "part/ses_old/msg_o/" + numbered("prt_", i) + ".json" },
				func(i int) string { return `{"type":"text","text":"hi"}` })
			return append(files, manyFiles(n, func(i int) string {
				return storage + "message/ses_old/" + numbered("msg_", 1000+i) + ".json"
			}, func(i int) string { return message("msg_o") })...)
		}},
	}
}

// opencodeStateCases returns the cases of the terminal UI's state files.
func opencodeStateCases(layout opencodeLayout) []growthCase {
	history, stash, pinned := layout.state+"/prompt-history.jsonl", layout.state+"/prompt-stash.jsonl",
		layout.state+"/session.json"
	return []growthCase{
		{kind: "prompt-history.jsonl", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(history, lines(n, func(i int) string {
				return `{"text":"git status","files":[],"agents":[],"mode":"shell"}`
			}))
		}},
		{kind: "prompt-history.jsonl", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(history, append(lines(n, func(i int) string { return `{"text":` }),
				[]byte(`{"input":`+nestedArrays(n, `"x"`)+`}`+"\n")...))
		}},
		{kind: "prompt-stash.jsonl", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(stash, lines(n, func(i int) string {
				return `{"prompt":{"text":"draft: rotate the keys"},"timestamp":1791028805000}`
			}))
		}},
		{kind: "prompt-stash.jsonl", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(stash, append(lines(n, func(i int) string { return `{"input":` }),
				[]byte(`{"prompt":`+nestedObjects(n, "text", `"x"`)+`}`+"\n")...))
		}},
		{kind: "session.json", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(pinned, []byte(`{"pinned":[`+joined(n, func(i int) string { return `"ses_01OC"` })+`]}`))
		}},
		{kind: "session.json", shape: shapeNested, n: 256, files: func(t *testing.T, n int) []growthFile {
			return one(pinned, []byte(`{"pinned":[`+joined(n, func(i int) string { return "1" })+`,`+
				nestedArrays(n, `"s"`)+`]}`))
		}},
	}
}
