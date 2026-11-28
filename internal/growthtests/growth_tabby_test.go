package growthtests

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// The Tabby growth cases: the server database and its backups, the event
// logs, the config, the IntelliJ command history and the VS Code
// extension's state in an editor's state.vscdb.

const (
	tabbyDatabaseRel = ".tabby/ee/db.sqlite"
	tabbyTime        = "2026-10-01T09:00:00Z"
)

// tabbyTables are the columns of the server database's tables the cases
// fill, by name.
func tabbyTables() map[string]string {
	return map[string]string{
		"users":   "id INTEGER, email TEXT, name TEXT, auth_token TEXT, created_at TEXT",
		"threads": "id INTEGER, is_ephemeral INTEGER, user_id INTEGER, created_at TEXT, relevant_questions TEXT",
		"thread_messages": "id INTEGER, thread_id INTEGER, role TEXT, content TEXT, code_source_id TEXT, " +
			"attachment TEXT, created_at TEXT",
		"user_events":   "id INTEGER, user_id INTEGER, kind TEXT, created_at TEXT, payload TEXT",
		"pages":         "id INTEGER, author_id INTEGER, title TEXT, content TEXT, created_at TEXT",
		"page_sections": "id INTEGER, page_id INTEGER, title TEXT, content TEXT, position INTEGER, created_at TEXT",
		"integrations":  "id INTEGER, kind TEXT, display_name TEXT, api_base TEXT, created_at TEXT",
		"provided_repositories": "id INTEGER, integration_id INTEGER, name TEXT, git_url TEXT, active INTEGER, " +
			"updated_at TEXT",
	}
}

// tabbyDatabase returns a server database holding the rows given by table
// name.
func tabbyDatabase(t *testing.T, filled map[string][][]any) []byte {
	names := make([]string, 0, len(filled))
	for name := range filled {
		names = append(names, name)
	}
	slices.Sort(names)
	var tables []sqlitetest.Table
	for _, name := range names {
		tables = append(tables, sqlitetest.Table{Name: name, SQL: "CREATE TABLE " + name + " (" + tabbyTables()[name] + ")",
			Rows: filled[name]})
	}
	return database(t, tables...)
}

// tabbyMessages returns n messages of thread 1, the first holding the
// attachment.
func tabbyMessages(n int, attachment string) [][]any {
	return rows(n, func(i int) []any {
		held := any(nil)
		if i == 0 {
			held = attachment
		}
		return []any{i + 1, 1, "user", numbered("question ", i), nil, held, tabbyTime}
	})
}

// tabbyRepeatedKeyCases returns the cases of a server database whose
// users or integrations table repeats an id that many rows name.
func tabbyRepeatedKeyCases() []growthCase {
	return []growthCase{
		// Each stored row of a repeated user id gives its own rows, and the
		// threads and events naming the user name it by id alone.
		{kind: "db.sqlite", shape: shapeSharedID + " (users named by many threads and events)",
			files: func(t *testing.T, n int) []growthFile {
				return one(tabbyDatabaseRel, tabbyDatabase(t, map[string][][]any{
					"users": rows(n, func(i int) []any {
						return []any{1, numbered("a", i) + "@example.com", "A", numbered("token", i), tabbyTime}
					}),
					"threads": rows(n, func(i int) []any { return []any{i, 0, 1, tabbyTime, nil} }),
					"user_events": rows(n, func(i int) []any {
						return []any{i, 1, "chat", tabbyTime, `{"view":{"completion_id":"c","choice_index":0}}`}
					}),
				}))
			}},
		// Each section naming a page no row holds, in a database with no
		// pages table, is shown under a stand-in of its page.
		{kind: "db.sqlite", shape: shapeSharedID + " (sections naming no page)",
			files: func(t *testing.T, n int) []growthFile {
				return one(tabbyDatabaseRel, tabbyDatabase(t, map[string][][]any{
					"page_sections": rows(n, func(i int) []any { return []any{i, i, "s", "c", 0, tabbyTime} }),
				}))
			}},
		// Each stored row of a repeated integration id gives its own rows,
		// and the repositories cached from it name it by id alone.
		{kind: "db.sqlite", shape: shapeSharedID + " (integrations named by many repositories)",
			files: func(t *testing.T, n int) []growthFile {
				return one(tabbyDatabaseRel, tabbyDatabase(t, map[string][][]any{
					"integrations": rows(n, func(i int) []any {
						return []any{1, "github", numbered("hub", i), nil, tabbyTime}
					}),
					"provided_repositories": rows(n, func(i int) []any {
						return []any{i, 1, numbered("r", i), "https://g/" + numbered("r", i), 1, tabbyTime}
					}),
				}))
			}},
	}
}

// tabbyDatabaseCases returns the cases of the server database that hold no
// long value.
func tabbyDatabaseCases() []growthCase {
	return []growthCase{
		{kind: "db.sqlite", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(tabbyDatabaseRel, tabbyDatabase(t, map[string][][]any{
				"threads":         rows(n, func(i int) []any { return []any{1, 0, 1, tabbyTime, nil} }),
				"thread_messages": tabbyMessages(n, `{"code":[{"git_url":"https://g/r","filepath":"a.go"}]}`),
				"pages":           rows(n, func(i int) []any { return []any{1, 1, "t", "c", tabbyTime} }),
				"page_sections":   rows(n, func(i int) []any { return []any{i, 1, "s", "c", i, tabbyTime} }),
			}))
		}},
		{kind: "db.sqlite", shape: shapeSharedID + " with undated sections", files: func(t *testing.T, n int) []growthFile {
			return one(tabbyDatabaseRel, tabbyDatabase(t, map[string][][]any{
				"pages":         rows(n, func(i int) []any { return []any{1, 1, "t", "c", tabbyTime} }),
				"page_sections": rows(n, func(i int) []any { return []any{i, 1, "s", "c", i, nil} }),
			}))
		}},
		{kind: "db.sqlite", shape: shapeJoins, files: func(t *testing.T, n int) []growthFile {
			return one(tabbyDatabaseRel, tabbyDatabase(t, map[string][][]any{
				"users":         {{1, "a@example.com", "A", nil, tabbyTime}},
				"pages":         {{1, 1, "title", strings.Repeat("c", 1024*n), tabbyTime}},
				"page_sections": rows(n, func(i int) []any { return []any{i, 1, "s", "c", 1, tabbyTime} }),
				"user_events": rows(n, func(i int) []any {
					return []any{i, 1, "chat", tabbyTime, `{"view":{"completion_id":"c","choice_index":0}}`}
				}),
			}))
		}},
		{kind: "db.sqlite", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(tabbyDatabaseRel, tabbyDatabase(t, map[string][][]any{
				"threads": {{1, 0, 1, tabbyTime, `[` + nestedArrays(n, `"q"`) + `]`}},
				"thread_messages": rows(n, func(i int) []any {
					return []any{i, 1, "user", "q", nil, `{"code":[` + strconv.Itoa(i) + `,{"git_url":`, tabbyTime}
				}),
				"user_events": rows(n, func(i int) []any { return []any{i, 1, "chat", tabbyTime, `{"completion":1}`} }),
			}))
		}},
		{kind: "database backup", shape: shapeManyFiles, n: 16, files: func(t *testing.T, n int) []growthFile {
			backup := tabbyDatabase(t, map[string][][]any{"threads": {{1, 0, 1, tabbyTime, nil}},
				"thread_messages": tabbyMessages(2, "")})
			return manyFiles(n, func(i int) string { return ".tabby/ee/db.backup-2026" + strconv.Itoa(1000+i) + ".sqlite" },
				func(i int) string { return string(backup) })
		}},
	}
}

// tabbyCarriedCases returns the cases where one long value of the server
// stores stands in many rows.
func tabbyCarriedCases() []growthCase {
	return []growthCase{
		{kind: "db.sqlite", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(tabbyDatabaseRel, tabbyDatabase(t, map[string][][]any{
				"threads":         {{1, 0, 1, tabbyTime, nil}},
				"thread_messages": tabbyMessages(n, `{"code":[{"git_url":"git@github.com:org/`+long("git")+`.git"}]}`),
			}))
		}},
		{kind: "db.sqlite", shape: shapeCarried + " by the rows of a repeated id", files: func(t *testing.T, n int) []growthFile {
			attachment := `{"code":[{"git_url":"git@github.com:org/` + long("git") + `.git"}]}`
			return one(tabbyDatabaseRel, tabbyDatabase(t, map[string][][]any{
				"threads":         rows(n, func(i int) []any { return []any{1, 0, 1, tabbyTime, nil} }),
				"thread_messages": tabbyMessages(1, attachment),
			}))
		}},
		{kind: "db.sqlite", shape: shapeLargeShared, files: func(t *testing.T, n int) []growthFile {
			return one(tabbyDatabaseRel, tabbyDatabase(t, map[string][][]any{
				"users": {{1, long("email"), "A", nil, tabbyTime}},
				"user_events": rows(n, func(i int) []any {
					return []any{i, 1, "chat", tabbyTime, `{"select":{"completion_id":"c","choice_index":0}}`}
				}),
			}))
		}},
		{kind: "db.sqlite", shape: shapeLargeShared + " in an integration", files: func(t *testing.T, n int) []growthFile {
			return one(tabbyDatabaseRel, tabbyDatabase(t, map[string][][]any{
				"integrations": {{1, long("kind"), long("name"), "https://git.example", tabbyTime}},
				"provided_repositories": rows(n, func(i int) []any {
					return []any{i, 1, numbered("r", i), numbered("https://git.example/r", i), 1, tabbyTime}
				}),
			}))
		}},
	}
}

// tabbyEventCases returns the cases of the event logs.
func tabbyEventCases() []growthCase {
	return []growthCase{
		{kind: "event log", shape: shapeCarried, files: func(t *testing.T, n int) []growthFile {
			return one(".tabby/events/2025-10-01.json", []byte(`{"user":"u","ts":1790000000000,"event":{"completion":`+
				`{"completion_id":"c1","language":"go","prompt":"p","segments":{"git_url":"git@github.com:org/`+long("git")+
				`.git"},"choices":[`+joined(n, func(i int) string { return `{"index":` + strconv.Itoa(i) + `,"text":"t"}` })+
				`]}}}`+"\n"))
		}},
		{kind: "event log", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".tabby/events/2025-10-01.json", append(lines(n, func(i int) string {
				return `{"user":"u","ts":1,"event":{"view":` + strconv.Itoa(i) + `}}` + "\n" + `{"ts":1}` + "\n" + `{"event":`
			}), []byte(`{"event":{"other":`+nestedObjects(n, "a", "1")+`}}`+"\n")...))
		}},
	}
}

// tabbyFileCases returns the cases of the config and the command
// histories.
func tabbyFileCases() []growthCase {
	return []growthCase{
		{kind: "config.toml", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".tabby/config.toml", []byte("[model.chat.http]\nkind = \"openai/chat\"\nmodel_name = \"m\"\n"+
				"supported_models = ["+joined(n, func(i int) string { return `"m` + strconv.Itoa(i) + `"` })+", "+
				nestedArrays(n, `"x"`)+"]\n"))
		}},
		{kind: "IntelliJ command history", shape: shapeNested, files: func(t *testing.T, n int) []growthFile {
			return one(".config/JetBrains/IntelliJIdea2025.2/options/intellij-tabby-command-history.xml",
				[]byte(`<application><component name="com.tabbyml.intellijtabby.inlineChat.CommandHistory">`+
					`<option name="history"><list>`+strings.Repeat(`<option value="cmd"/><bad>`+"</bad>", n)+
					strings.Repeat(`<option name="history">`, 16)+`<option value="deep"/>`+
					strings.Repeat(`</option>`, 16)+`</list></option></component></application>`))
		}},
		{kind: "editor state.vscdb", shape: shapeNested, agent: "vscode", files: func(t *testing.T, n int) []growthFile {
			state := `{"server.serverRecords":{` + joined(n, func(i int) string {
				return `"http://h` + strconv.Itoa(i) + `":{"token":"auth_` + strconv.Itoa(i) + `"},"http://b` +
					strconv.Itoa(i) + `":` + strconv.Itoa(i)
			}) + `},"edit.recentlyCommand":[` + joined(n, func(i int) string {
				return `"{\"command\":\"fix ` + strconv.Itoa(i) + `\",\"context\":[{\"uri\":\"file:///a\"}]}"`
			}) + `,` + nestedArrays(n, "1") + `]}`
			return one(".config/Code/User/globalStorage/state.vscdb", database(t, sqlitetest.Table{Name: "ItemTable",
				SQL:  "CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB)",
				Rows: [][]any{{"TabbyML.vscode-tabby", state}}}))
		}},
	}
}

// tabbyGrowth returns the tabby growth set.
func tabbyGrowth() growthSet {
	return growthSet{parser: "tabby",
		cases: slices.Concat(tabbyDatabaseCases(), tabbyRepeatedKeyCases(), tabbyCarriedCases(),
			tabbyEventCases(), tabbyFileCases())}
}
