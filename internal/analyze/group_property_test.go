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
// repeated id, with moreGroupedStores.
func groupedStores() []groupedStore {
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
	return append([]groupedStore{
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
	}, moreGroupedStores()...)
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
// to the time of any row of the output without a filter shows every dated
// row of that output inside it, with its text or, in the row of the
// chosen parent with two texts, its chosen one, and shows no row more
// often than that output does.
func TestAWindowShowsEveryRowOfARepeatedIDInsideIt(t *testing.T) {
	for _, store := range groupedStores() {
		t.Run(store.name, func(t *testing.T) {
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
// pattern is the text of one or two of the grouped rows the parser gave.
func TestAMatchShowsEveryRowOfARepeatedIDItMatches(t *testing.T) {
	for _, store := range groupedStores() {
		t.Run(store.name, func(t *testing.T) {
			directory := writeCollection(t, filepath.Join(t.TempDir(), "col"),
				[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))}, store.files(t))
			var texts []string
			for _, row := range timelineRows(t, directory, store.agent, rowfilter.RowFilter{}, nil, true) {
				if text := row.FullText(); text != "" && !slices.Contains(texts, regexp.QuoteMeta(text)) {
					texts = append(texts, regexp.QuoteMeta(text))
				}
			}
			if len(texts) == 0 {
				t.Fatal("the store gave no grouped row")
			}
			unfiltered := timelineRows(t, directory, store.agent, rowfilter.RowFilter{}, nil, false)
			for i, first := range texts {
				for _, second := range texts[i:] {
					pattern := first + "|" + second
					filtered := timelineRows(t, directory, store.agent, rowfilter.RowFilter{}, []string{pattern}, false)
					matched := regexp.MustCompile(pattern)
					if faults := keepFaults(unfiltered, filtered, func(row *timeline.Row) bool {
						return row.Matches(matched)
					}); len(faults) > 0 {
						t.Errorf("--match %q: %q", pattern, faults)
					}
				}
			}
		})
	}
}

// For every parser that groups the rows of a repeated id, a window from or
// to the time of any row of the output without a filter, with and without
// --keep-undated, together with a pattern of one text (a grouped row's, or
// a short row's of that output), shows every row of that output both keep,
// as a match or a window alone does. Each text is tried alone rather than
// in pairs, which the match test covers, and the stores run in parallel,
// to keep the test's time to a few seconds.
func TestAWindowAndAMatchShowEveryRowOfARepeatedIDBothKeep(t *testing.T) {
	for _, store := range groupedStores() {
		t.Run(store.name, func(t *testing.T) {
			t.Parallel()
			directory := writeCollection(t, filepath.Join(t.TempDir(), "col"),
				[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))}, store.files(t))
			unfiltered := timelineRows(t, directory, store.agent, rowfilter.RowFilter{}, nil, false)
			texts := windowTexts(timelineRows(t, directory, store.agent, rowfilter.RowFilter{}, nil, true), unfiltered)
			for _, window := range windowsOf(unfiltered) {
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

// windowsOf returns a window from and one to each time of rows, each with
// and without --keep-undated.
func windowsOf(rows []timeline.Row) []rowfilter.RowFilter {
	var times []string
	for _, row := range rows {
		if row.TimestampUTC != "" && !slices.Contains(times, row.TimestampUTC) {
			times = append(times, row.TimestampUTC)
		}
	}
	var windows []rowfilter.RowFilter
	for _, undated := range []bool{false, true} {
		for _, bound := range times {
			windows = append(windows, rowfilter.RowFilter{Since: bound, KeepUndated: undated},
				rowfilter.RowFilter{Until: bound, KeepUndated: undated})
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
