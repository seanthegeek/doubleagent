package analyze

import (
	"context"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/rowfilter"
	"github.com/seanthegeek/doubleagent/internal/secrets"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// groupedStore is a collection whose store repeats an id, read by the
// parser of agent.
type groupedStore struct {
	name  string
	agent string
	files func(t *testing.T) []collectedFile
}

// groupedStores returns a store of every parser that groups the rows of a
// repeated id, the one list the three properties below run over. A new
// store is one line in the function of its kind: databaseStores,
// moreGroupedStores, opencodeShapeStores, unitStores or familyStores; a
// store whose grouped rows and the rows taking from them are in different
// files goes in both read orders (bothOrders).
func groupedStores() []groupedStore {
	return slices.Concat(databaseStores(), moreGroupedStores(), opencodeShapeStores(), unitStores(),
		familyStores())
}

// databaseStores returns the stores of one database or file a parser
// groups the rows of.
func databaseStores() []groupedStore {
	file := func(agent, rel string, text func(t *testing.T) string) func(t *testing.T) []collectedFile {
		return func(t *testing.T) []collectedFile {
			return []collectedFile{{"alice", "/home/alice", agent, rel, text(t)}}
		}
	}
	testdata := func(parts ...string) func(t *testing.T) string {
		return func(t *testing.T) string {
			data, err := osfile.ReadFile(filepath.Join(append([]string{"..", "parser"}, parts...)...))
			if err != nil {
				t.Fatal(err)
			}
			return string(data)
		}
	}
	ollama := "Library/Application Support/Ollama/db.sqlite"
	opencode, kilo := ".local/share/opencode/opencode-dev.db", ".local/share/kilo/kilo-dev.db"
	zed := ".local/share/zed/db/0-stable/db.sqlite"
	return []groupedStore{
		{"hermes sessions", "hermes", file("hermes", ".hermes/state.db",
			testdata("hermes", "testdata", "repeated_session_rows.db"))},
		{"nanobot keys", "nanobot", file("nanobot",
			".nanobot/sessions/0123456789abcdef0123456789abcdef/sessions.sqlite3",
			testdata("nanobot", "testdata", "repeated_key_presets.db"))},
		{"ollama calls and attachments", "ollama", file("ollama", ollama, ollamaRepeatedDatabase)},
		{"ollama moved model", "ollama", file("ollama", ollama,
			func(t *testing.T) string { return ollamaMovedModelDatabase(t, "model-new") })},
		{"ollama moved model and a third", "ollama", file("ollama", ollama,
			func(t *testing.T) string { return ollamaMovedModelDatabase(t, "model-third") })},
		{"tabby pages", "tabby", file("tabby", ".tabby/ee/db.sqlite", tabbyRepeatedDatabase)},
		{"tabby projects", "tabby", file("tabby", ".tabby/ee/db.sqlite", tabbyProjectDatabase)},
		{"opencode messages", "opencode", file("opencode", opencode, opencodeRepeatV2)},
		{"opencode sessions", "opencode", file("opencode", opencode, opencodeSessionRepeatV2)},
		{"kilo code messages", "kilo-code", file("kilo-code", kilo, opencodeRepeatV2)},
		{"kilo code sessions", "kilo-code", file("kilo-code", kilo, opencodeSessionRepeatV2)},
		{"zed archived worktrees", "zed", file("zed", zed, testdata("zed", "testdata", "repeated_worktree_2.db"))},
		{"zed sidebar threads", "zed", file("zed", zed, zedRepeatedSidebar)},
		{"agent zero history", "agent-zero",
			func(*testing.T) []collectedFile { return []collectedFile{agentZeroChat(sharedIDChat)} }},
		{"agent zero empty later item", "agent-zero",
			func(*testing.T) []collectedFile { return []collectedFile{agentZeroChat(emptyLaterChat)} }},
		{"agent zero message file", "agent-zero", func(*testing.T) []collectedFile {
			return []collectedFile{agentZeroChat(messageFileChat),
				{"alice", "/home/alice", "agent-zero", "agent-zero/usr/chats/ctx/messages/1.txt", "FILE-RESULT"}}
		}},
	}
}

// opencodeShapeDatabase returns a 2.x OpenCode database of the sessions
// ses_c, ses_d and ses_e whose session_message and session_part hold the
// records and parts given.
func opencodeShapeDatabase(t *testing.T, records, parts [][]any) string {
	t.Helper()
	return buildDatabase(t,
		sqlitetest.Table{Name: "session_v2", SQL: "CREATE TABLE session_v2 (id text PRIMARY KEY, project_id text, " +
			"directory text, title text, time_created integer, time_updated integer)", Rows: [][]any{
			{"ses_c", nil, "/proj/c", "C", opencodeOld, opencodeOld},
			{"ses_d", nil, "/proj/d", "D", opencodeNew, opencodeNew},
			{"ses_e", nil, "/proj/e", "E", opencodeLater, opencodeLater}}},
		sqlitetest.Table{Name: "session_message", SQL: "CREATE TABLE session_message (id text, session_id text, " +
			"type text, seq integer, time_created integer, time_updated integer, data text)", Rows: records},
		sqlitetest.Table{Name: "session_part", SQL: "CREATE TABLE session_part (id text, message_id text, " +
			"session_id text, type text, ordinal integer, data text)", Rows: parts})
}

// opencodeLater is the time of a third stored record, two months after
// the new one.
const opencodeLater = opencodeNew + 5_000_000_000

// opencodeShapeRecord returns a session_message row of msg_v in the session
// at the time, with the content given as JSON and the model.
func opencodeShapeRecord(session string, created int, content, model string) []any {
	return []any{"msg_v", session, "assistant", 0, created, created,
		`{"content":` + content + `,"model":{"providerID":"p","id":"` + model + `"}}`}
}

// opencodeShapeStores returns OpenCode 2.x stores of a repeated message id
// in more shapes: three records; a later record whose content is empty, or
// is not an array; the same content in both records; and items with times
// of their own and a tool call.
func opencodeShapeStores() []groupedStore {
	item := [][]any{{"prt_v", "msg_v", "ses_c", "text", 0, `{"text":"ITEM-TEXT"}`}}
	store := func(name string, records, parts [][]any) groupedStore {
		return groupedStore{name, "opencode", func(t *testing.T) []collectedFile {
			return []collectedFile{{"alice", "/home/alice", "opencode", ".local/share/opencode/opencode-dev.db",
				opencodeShapeDatabase(t, records, parts)}}
		}}
	}
	text := func(value string) string { return `[{"type":"text","text":"` + value + `"}]` }
	first := opencodeShapeRecord("ses_c", opencodeOld, text("OWN-1"), "mA")
	second := opencodeShapeRecord("ses_d", opencodeNew, text("OWN-2"), "mB")
	return []groupedStore{
		store("opencode three records", [][]any{first, second,
			opencodeShapeRecord("ses_e", opencodeLater, text("OWN-3"), "mC")}, item),
		store("opencode later record with empty content", [][]any{first,
			opencodeShapeRecord("ses_d", opencodeNew, `[]`, "mB"),
			opencodeShapeRecord("ses_e", opencodeLater, text("OWN-3"), "mC")}, item),
		store("opencode later record with content not an array", [][]any{first,
			opencodeShapeRecord("ses_d", opencodeNew, `"bad"`, "mB")}, item),
		store("opencode same content in both records", [][]any{opencodeShapeRecord("ses_c", opencodeOld,
			text("SAME"), "mA"), opencodeShapeRecord("ses_c", opencodeNew, text("SAME"), "mA")}, item),
		store("opencode items with their own times and a tool call", [][]any{first, second}, [][]any{
			{"prt_v", "msg_v", "ses_c", "text", 0, `{"text":"ITEM-TEXT","time":{"start":1780358400000,` +
				`"created":1780358400000}}`},
			{"prt_w", "msg_v", "ses_c", "tool", 1, `{"type":"tool","tool":"bash","callID":"call_1","state":` +
				`{"status":"completed","input":{"command":"ls"},"output":"OUT","time":{"start":1780358400000,` +
				`"end":1780358401000}}}`}}),
	}
}

// codexState returns the Codex state database of a repeated thread id.
func codexState(t *testing.T) collectedFile {
	return collectedFile{"alice", "/home/alice", "codex-cli", ".codex/state_5.sqlite", codexRepeatState(t)}
}

// agentZeroFileChat returns the files of an Agent Zero chat whose history
// names the message file big.txt, which holds FILE-TEXT.
func agentZeroFileChat(chat string) func(*testing.T) []collectedFile {
	return func(*testing.T) []collectedFile {
		return []collectedFile{agentZeroChat(chat),
			{"alice", "/home/alice", "agent-zero", "agent-zero/usr/chats/ctx/messages/big.txt", "FILE-TEXT"}}
	}
}

// moreGroupedStores returns the stores of the parsers on the grouping
// mechanism that groupedStores does not build: Codex threads in both read
// orders, Copilot CLI and Goose sessions, and Agent Zero's message file
// and history message groups.
func moreGroupedStores() []groupedStore {
	file := func(agent, rel string, text func(t *testing.T) string) func(t *testing.T) []collectedFile {
		return func(t *testing.T) []collectedFile {
			return []collectedFile{{"alice", "/home/alice", agent, rel, text(t)}}
		}
	}
	return []groupedStore{
		{"codex threads, the state database first", "codex-cli", func(t *testing.T) []collectedFile {
			return append([]collectedFile{codexState(t)}, codexRepeatSiblings(t)...)
		}},
		{"codex threads, the state database last", "codex-cli", func(t *testing.T) []collectedFile {
			return append(codexRepeatSiblings(t), codexState(t))
		}},
		{"copilot cli sessions", "copilot-cli", file("copilot-cli", ".copilot/session-store.db", copilotRepeatStore)},
		{"copilot cli sessions without a summary", "copilot-cli",
			file("copilot-cli", ".copilot/session-store.db", copilotUnnamedStore)},
		{"goose sessions", "goose", file("goose", gooseSessionsRel, gooseRepeatedSession)},
		{"agent zero message file of two ids", "agent-zero", agentZeroFileChat(
			historyChat(toolMessage("t1", "")+","+toolMessage("t2", ""),
				toolItem("t1", january, "first own")+","+toolItem("t2", june, "second own")))},
		{"agent zero message file of two ids with two items each", "agent-zero", agentZeroFileChat(
			historyChat(toolMessage("t1", "")+","+toolMessage("t2", ""),
				toolItem("t1", january, "a1")+","+toolItem("t2", march, "b1")+","+toolItem("t1", june, "a2")+","+
					toolItem("t2", september, "b2")))},
		{"agent zero message file beside a tool result", "agent-zero", agentZeroFileChat(
			historyChat(toolMessage("t1", "")+","+toolMessage("t2", "SECOND-RESULT"),
				toolItem("t1", january, "a1")+","+toolItem("t2", march, "b1")+","+toolItem("t2", june, "b2")))},
		{"agent zero history message repeating an id", "agent-zero", func(*testing.T) []collectedFile {
			return []collectedFile{agentZeroChat(historyChat(userMessage("u", "FIRST-PROMPT")+","+
				userMessage("u", "LATER-PROMPT"), userItem("u", january, "")+","+userItem("u", june, "june ask")))}
		}},
	}
}

// unitStores returns the stores of Cursor, OpenClaw, Copilot Desktop,
// OpenCode projects and Tabby users, and the Agent Zero shapes beyond
// moreGroupedStores.
func unitStores() []groupedStore {
	file := func(agent, rel string, text func(t *testing.T) string) func(t *testing.T) []collectedFile {
		return func(t *testing.T) []collectedFile {
			return []collectedFile{{"alice", "/home/alice", agent, rel, text(t)}}
		}
	}
	cursor := func(state func(t *testing.T) string, stateFirst bool) func(t *testing.T) []collectedFile {
		return func(t *testing.T) []collectedFile { return cursorFiles(t, state(t), stateFirst) }
	}
	desktop, opencode := ".copilot/data.db", ".local/share/opencode/opencode.db"
	messages := userMessage("u", "FIRST-PROMPT") + "," + userMessage("u", "LATER-PROMPT")
	return []groupedStore{
		{"cursor composerData, the state database first", "cursor", cursor(cursorRepeatedData, true)},
		{"cursor composerData, the state database last", "cursor", cursor(cursorRepeatedData, false)},
		{"cursor composerHeaders", "cursor", cursor(cursorRepeatedHeaders, true)},
		{"cursor sandbox switch", "cursor", file("cursor", cursorStateRel, cursorRepeatedItems)},
		{"openclaw windows", "openclaw", file("openclaw", openclawAgentRel, openclawRepeatedWindow)},
		{"openclaw relayed windows", "openclaw", file("openclaw", openclawAgentRel, openclawRelayedWindow)},
		{"openclaw session keys", "openclaw", file("openclaw", openclawAgentRel, openclawRepeatedNode)},
		{"copilot desktop projects", "copilot-desktop", file("copilot-desktop", desktop, copilotdesktopRepeatedProject)},
		{"copilot desktop workspaces", "copilot-desktop",
			file("copilot-desktop", desktop, copilotdesktopRepeatedWorkspace)},
		{"copilot desktop aliases", "copilot-desktop", file("copilot-desktop", desktop, copilotdesktopRepeatedAlias)},
		{"copilot desktop chain", "copilot-desktop", file("copilot-desktop", desktop, copilotdesktopRepeatedChain)},
		{"opencode projects", "opencode", file("opencode", opencode, opencodeProjectRepeat)},
		{"opencode projects and sessions", "opencode", file("opencode", opencode, opencodeProjectAndSessionRepeat)},
		{"tabby users, which no group joins", "tabby", file("tabby", ".tabby/ee/db.sqlite", tabbyRepeatedUser)},
		{"agent zero message file of two ids with two items each and a tool result", "agent-zero",
			agentZeroFileChat(historyChat(toolMessage("t1", "")+","+toolMessage("t2", "SECOND-RESULT"),
				toolItem("t1", january, "a1")+","+toolItem("t2", march, "b1")+","+toolItem("t1", june, "a2")+","+
					toolItem("t2", september, "b2")))},
		{"agent zero history message repeating an id with no log item", "agent-zero",
			func(*testing.T) []collectedFile { return []collectedFile{agentZeroChat(historyChat(messages, ""))} }},
		{"agent zero history message repeating an id with one log item", "agent-zero",
			func(*testing.T) []collectedFile {
				return []collectedFile{agentZeroChat(historyChat(messages, userItem("u", june, "")))}
			}},
	}
}

// familyStores returns the stores of the Cline family's task indexes and
// Continue's session index, each in both read orders, since the index and
// the transcripts that take from its entries are separate files.
func familyStores() []groupedStore {
	roo := func(t *testing.T) []collectedFile {
		files := rooRepeatFiles(t)
		files[0].text = strings.ReplaceAll(files[0].text, `"modelId"`, `"mode":"code","modelId"`)
		return files
	}
	return slices.Concat(
		bothOrders(groupedStore{"cline task index", "cline", clineRepeatFiles}),
		bothOrders(groupedStore{"roo code task index", "roo-code", roo}),
		bothOrders(groupedStore{"kilo code pre-fork task index", "kilo-code", kiloRepeatFiles}),
		bothOrders(groupedStore{"pearai task history", "pearai", pearaiRepeatFiles}),
		bothOrders(groupedStore{"continue session index", "continue", continueRepeatFiles("continue", ".continue/")}),
		bothOrders(groupedStore{"pearai chat session index", "pearai", continueRepeatFiles("pearai", ".pearai/")}))
}

// bothOrders returns store with its files in their order and reversed, as
// a collection may list them either way.
func bothOrders(store groupedStore) []groupedStore {
	reversed := store
	reversed.name += ", files reversed"
	reversed.files = func(t *testing.T) []collectedFile {
		files := store.files(t)
		slices.Reverse(files)
		return files
	}
	return []groupedStore{store, reversed}
}

// zedRepeatedSidebar returns a Zed db.sqlite whose sidebar_threads table
// repeats the thread t1, with two drafts of it and an archived worktree
// linked to it.
func zedRepeatedSidebar(t *testing.T) string {
	t.Helper()
	return buildDatabase(t,
		sqlitetest.Table{Name: "sidebar_threads", SQL: "CREATE TABLE sidebar_threads (thread_id BLOB, " +
			"session_id TEXT, agent_id TEXT, title TEXT, updated_at TEXT, folder_paths TEXT)", Rows: [][]any{
			{"t1", "s-old", nil, "old", "2026-01-01T00:00:00Z", "/proj/a"},
			{"t1", "s-new", nil, "new", "2026-06-01T00:00:00Z", "/proj/b"}}},
		sqlitetest.Table{Name: "scoped_kv_store", SQL: "CREATE TABLE scoped_kv_store (namespace TEXT, key TEXT, " +
			"value TEXT)", Rows: [][]any{
			{"agent_draft_prompts", "t1", `[{"type":"text","text":"hello"}]`},
			{"agent_draft_prompts", "t1", `[{"type":"text","text":"again"}]`}}},
		sqlitetest.Table{Name: "archived_git_worktrees", SQL: "CREATE TABLE archived_git_worktrees (id INTEGER, " +
			"worktree_path TEXT, main_repo_path TEXT)", Rows: [][]any{{int64(7), "/w/tree", "/w/main"}}},
		sqlitetest.Table{Name: "thread_archived_worktrees", SQL: "CREATE TABLE thread_archived_worktrees " +
			"(thread_id TEXT, archived_worktree_id INTEGER)", Rows: [][]any{{"t1", int64(7)}}})
}

// timelineRows parses the collection at directory for agent as timeline
// does under filter, its patterns compiled from patterns, and returns the
// rows of the output; with grouped set it returns instead the grouped rows
// the parsers gave.
func timelineRows(t *testing.T, directory, agent string, filter rowfilter.RowFilter, patterns []string,
	grouped bool) []timeline.Row {
	t.Helper()
	ctx := context.Background()
	found, err := matcher()
	if err != nil {
		t.Fatal(err)
	}
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	collection, _, message := openInput(ctx, InputOptions{Input: directory}, found, env,
		secrets.NewRecorder(secrets.Setting{}))
	if collection == nil {
		t.Fatal(message)
	}
	defer func() { _ = collection.Cleanup() }()
	run := &timelineRun{options: TimelineOptions{Agents: []string{agent}}, env: env, collection: collection,
		registry: parsers.New(), filter: &filter}
	if grouped {
		result, err := run.registry.Collect(ctx, collection, parser.Options{}, run.options.Agents)
		if err != nil {
			t.Fatal(err)
		}
		var rows []timeline.Row
		for _, row := range result.Primary {
			if row.Group != nil {
				rows = append(rows, row)
			}
		}
		for _, secondary := range result.Secondary {
			if secondary.Row.Group != nil {
				rows = append(rows, secondary.Row)
			}
		}
		return rows
	}
	if run.filter.Patterns, err = rowfilter.CompilePatterns(patterns, false); err != nil {
		t.Fatal(err)
	}
	if err := run.parse(ctx); err != nil {
		t.Fatal(err)
	}
	return run.rows
}

// rowRecord returns a row's columns as one string.
func rowRecord(row timeline.Row) string {
	return strings.Join([]string{row.TimestampUTC, row.SessionID, row.ProjectPath, row.GitBranch, row.TurnType,
		row.Model, row.ToolName, row.ToolUseID, row.SourceFile, strconv.Itoa(row.SourceLine), row.Recovered,
		row.FullText()}, " ")
}

// keepFaults returns what is wrong with filtered, the output under a
// filter, against unfiltered, the output without one: a row it shows more
// often than unfiltered does, or a row of unfiltered the filter keeps
// (keeps) that it does not show, with its text or, for a row of the
// chosen parent with two texts, at its place.
func keepFaults(unfiltered, filtered []timeline.Row, keeps func(row *timeline.Row) bool) []string {
	var faults []string
	counts := map[string]int{}
	for _, row := range unfiltered {
		counts[rowRecord(row)]++
	}
	for record, shown := range countRecords(filtered) {
		if shown > max(counts[record], 1) {
			faults = append(faults, "shows "+strconv.Itoa(shown)+" times "+strconv.Quote(record))
		}
	}
	for _, hit := range unfiltered {
		if !keeps(&hit) {
			continue
		}
		if !slices.ContainsFunc(filtered, func(row timeline.Row) bool {
			chosen := row.Group != nil && row.SourceFile == hit.SourceFile && row.SourceLine == hit.SourceLine &&
				row.TurnType == hit.TurnType
			return row.FullText() == hit.FullText() || chosen
		}) {
			faults = append(faults, "hides "+strconv.Quote(hit.FullText()))
		}
	}
	return faults
}

// countRecords returns how many times each row's record is in rows.
func countRecords(rows []timeline.Row) map[string]int {
	counts := map[string]int{}
	for _, row := range rows {
		counts[rowRecord(row)]++
	}
	return counts
}

// For every parser that groups the rows of a repeated id, a window from or
// to the time of any row of the output without a filter shows the text of
// every dated row of that output inside it (or, for the row of the chosen
// parent with two texts, its chosen text at its place), and shows no row
// more often than that output does. The text may be shown at another time
// inside the window: a row the chosen stored row gives with the same text
// stands at its own time in place of one of another stored row.
func TestAWindowShowsEveryRowOfARepeatedIDInsideIt(t *testing.T) {
	for _, store := range groupedStores() {
		t.Run(store.name, func(t *testing.T) {
			t.Parallel()
			directory := writeCollection(t, filepath.Join(t.TempDir(), "col"),
				[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))}, store.files(t))
			unfiltered := timelineRows(t, directory, store.agent, rowfilter.RowFilter{}, nil, false)
			var times []string
			for _, row := range unfiltered {
				if row.TimestampUTC != "" && !slices.Contains(times, row.TimestampUTC) {
					times = append(times, row.TimestampUTC)
				}
			}
			for _, bound := range times {
				for _, window := range []rowfilter.RowFilter{{Since: bound}, {Until: bound}} {
					filtered := timelineRows(t, directory, store.agent, window, nil, false)
					if faults := keepFaults(unfiltered, filtered, func(row *timeline.Row) bool {
						return row.TimestampUTC != "" && (window.Since == "" || row.TimestampUTC >= window.Since) &&
							(window.Until == "" || row.TimestampUTC <= window.Until)
					}); len(faults) > 0 {
						t.Errorf("--since %q --until %q: %q", window.Since, window.Until, faults)
					}
				}
			}
		})
	}
}

// For every parser that groups the rows of a repeated id, a --match
// pattern that matches a row of the output without a filter shows that
// row, with its text or, in the row of the chosen parent with two texts,
// its chosen one, and shows no row more often than that output does. Each
// pattern is the text of one or two of the grouped rows the parser gave,
// or of one short row of that output, so a store whose rows of a repeated
// id the parser gives without a group is checked too.
func TestAMatchShowsEveryRowOfARepeatedIDItMatches(t *testing.T) {
	for _, store := range groupedStores() {
		t.Run(store.name, func(t *testing.T) {
			t.Parallel()
			directory := writeCollection(t, filepath.Join(t.TempDir(), "col"),
				[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))}, store.files(t))
			unfiltered := timelineRows(t, directory, store.agent, rowfilter.RowFilter{}, nil, false)
			grouped := windowTexts(timelineRows(t, directory, store.agent, rowfilter.RowFilter{}, nil, true), nil)
			var patterns []string
			for i, first := range grouped {
				for _, second := range grouped[i:] {
					patterns = append(patterns, first+"|"+second)
				}
			}
			for _, text := range windowTexts(nil, unfiltered) {
				if !slices.Contains(grouped, text) {
					patterns = append(patterns, text)
				}
			}
			for _, pattern := range patterns {
				filtered := timelineRows(t, directory, store.agent, rowfilter.RowFilter{}, []string{pattern}, false)
				matched := regexp.MustCompile(pattern)
				if faults := keepFaults(unfiltered, filtered, func(row *timeline.Row) bool {
					return row.Matches(matched)
				}); len(faults) > 0 {
					t.Errorf("--match %q: %q", pattern, faults)
				}
			}
		})
	}
}

// For every parser that groups the rows of a repeated id, a window from or
// to the time of any grouped row the parser gave, and with --keep-undated
// when some row has no time, together with a pattern of one grouped row's
// text, shows every row of the output without a filter both keep, as a
// match or a window alone does. A store with no grouped row uses its output's times
// and short texts instead. The window and match tests try every output
// time and short text, and pairs of texts; this one crosses only the
// grouped rows' times and single texts, which decide the choice, to keep
// its time to a few seconds with the stores run in parallel.
func TestAWindowAndAMatchShowEveryRowOfARepeatedIDBothKeep(t *testing.T) {
	for _, store := range groupedStores() {
		t.Run(store.name, func(t *testing.T) {
			t.Parallel()
			directory := writeCollection(t, filepath.Join(t.TempDir(), "col"),
				[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))}, store.files(t))
			unfiltered := timelineRows(t, directory, store.agent, rowfilter.RowFilter{}, nil, false)
			grouped := timelineRows(t, directory, store.agent, rowfilter.RowFilter{}, nil, true)
			texts, windows := windowTexts(grouped, nil), windowsOf(grouped, unfiltered)
			if len(grouped) == 0 {
				texts, windows = windowTexts(nil, unfiltered), windowsOf(unfiltered)
			}
			for _, window := range windows {
				for _, text := range texts {
					filtered := timelineRows(t, directory, store.agent, window, []string{text}, false)
					matched := regexp.MustCompile(text)
					if faults := keepFaults(unfiltered, filtered, func(row *timeline.Row) bool {
						return insideWindow(window, row) && row.Matches(matched)
					}); len(faults) > 0 {
						t.Errorf("--since %q --until %q --keep-undated %v --match %q: %q", window.Since, window.Until,
							window.KeepUndated, text, faults)
					}
				}
			}
		})
	}
}

// windowTexts returns the patterns of the combined test: the text of each
// grouped row and of each row of unfiltered shorter than 200 bytes, quoted
// and each once.
func windowTexts(grouped, unfiltered []timeline.Row) []string {
	var texts []string
	for i, row := range slices.Concat(grouped, unfiltered) {
		text := row.FullText()
		if text != "" && (i < len(grouped) || len(text) < 200) && !slices.Contains(texts, regexp.QuoteMeta(text)) {
			texts = append(texts, regexp.QuoteMeta(text))
		}
	}
	return texts
}

// windowsOf returns a window from and one to each time of rows, and each
// again with --keep-undated when a row of others has no time, which is
// the only case where that option changes what is kept.
func windowsOf(rows []timeline.Row, others ...[]timeline.Row) []rowfilter.RowFilter {
	var times []string
	undated := []bool{false}
	for _, row := range slices.Concat(append(others, rows)...) {
		if row.TimestampUTC == "" && len(undated) == 1 {
			undated = append(undated, true)
		}
	}
	for _, row := range rows {
		if row.TimestampUTC != "" && !slices.Contains(times, row.TimestampUTC) {
			times = append(times, row.TimestampUTC)
		}
	}
	var windows []rowfilter.RowFilter
	for _, keep := range undated {
		for _, bound := range times {
			windows = append(windows, rowfilter.RowFilter{Since: bound, KeepUndated: keep},
				rowfilter.RowFilter{Until: bound, KeepUndated: keep})
		}
	}
	return windows
}

// insideWindow reports whether the window keeps row by its time: an
// undated row only with --keep-undated.
func insideWindow(window rowfilter.RowFilter, row *timeline.Row) bool {
	if row.TimestampUTC == "" {
		return window.KeepUndated
	}
	return (window.Since == "" || row.TimestampUTC >= window.Since) &&
		(window.Until == "" || row.TimestampUTC <= window.Until)
}
