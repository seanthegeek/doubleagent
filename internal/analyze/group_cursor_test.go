package analyze

import (
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/rowfilter"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The Cursor IDE's files in a home, relative to it.
const (
	cursorStateRel    = "Library/Application Support/Cursor/User/globalStorage/state.vscdb"
	cursorTrackingRel = ".cursor/ai-tracking/ai-code-tracking.db"
)

// cursorStateOf returns a state.vscdb whose cursorDiskKV holds the
// key-value rows given in rowid order and whose composerHeaders holds the
// headers, without the tables' key constraints, as only damage or an edit
// leaves a key stored twice.
func cursorStateOf(t *testing.T, keyValues, headers [][]any) string {
	t.Helper()
	return buildDatabase(t,
		sqlitetest.Table{Name: "cursorDiskKV", SQL: "CREATE TABLE cursorDiskKV (key TEXT, value BLOB)", Rows: keyValues},
		sqlitetest.Table{Name: "composerHeaders", SQL: "CREATE TABLE composerHeaders (composerId TEXT, " +
			"createdAt INTEGER, lastUpdatedAt INTEGER, isArchived INTEGER, value TEXT)", Rows: headers})
}

// cursorWorkspace returns the JSON members that put a composer in the
// project, on the branch.
func cursorWorkspace(project, branch string) string {
	return `"workspaceIdentifier":{"uri":{"fsPath":"` + project + `"}},"trackedGitRepos":[{"repoPath":"` +
		project + `","branches":["` + branch + `"]}]`
}

// cursorRepeatedData returns a state.vscdb whose cursorDiskKV holds the
// key composerData:c1 twice: an earlier conversation OLD in /proj/old on
// br-old created in January, then NEW in /proj/new on br-new created in
// June; both name the bubble b1, a prompt of the next day, and the
// composer has an unsent draft.
func cursorRepeatedData(t *testing.T) string {
	t.Helper()
	list := `"fullConversationHeadersOnly":[{"bubbleId":"b1","type":1}]`
	return cursorStateOf(t, [][]any{
		{"composerData:c1", `{"composerId":"c1","createdAt":1767225600000,"name":"OLD","status":"completed",` +
			cursorWorkspace("/proj/old", "br-old") + `,` + list + `}`},
		{"composerData:c1", `{"composerId":"c1","createdAt":1780272000000,"name":"NEW","status":"completed",` +
			cursorWorkspace("/proj/new", "br-new") + `,` + list + `}`},
		{"bubbleId:c1:b1", `{"type":1,"text":"the question","createdAt":1780358400000}`},
		{"composerDraft:c1", `{"text":"DRAFT-TEXT"}`}}, nil)
}

// cursorTracking returns an ai-code-tracking.db with one hash of the
// composer c1, a second after its prompt.
func cursorTracking(t *testing.T) string {
	t.Helper()
	return buildDatabase(t, sqlitetest.Table{Name: "ai_code_hashes", SQL: "CREATE TABLE ai_code_hashes " +
		"(fileName TEXT, source TEXT, model TEXT, requestId TEXT, conversationId TEXT, timestamp INTEGER)",
		Rows: [][]any{{"/src/f", "composer", "m", "r1", "c1", int64(1780358401000)}}})
}

// cursorFiles returns the collected files of a home holding the state
// given and the tracking database, the state first or last.
func cursorFiles(t *testing.T, state string, stateFirst bool) []collectedFile {
	t.Helper()
	files := []collectedFile{{"alice", "/home/alice", "cursor", cursorTrackingRel, cursorTracking(t)}}
	stored := collectedFile{"alice", "/home/alice", "cursor", cursorStateRel, state}
	if stateFirst {
		return append([]collectedFile{stored}, files...)
	}
	return append(files, stored)
}

// cursorNotice returns the parser: row a stored row of a repeated id gives
// in the table named, with this row or another of them.
func cursorNotice(table string, chosen bool) string {
	which := "another of them"
	if chosen {
		which = "this row"
	}
	return "parser: " + table + " holds 2 rows of this row's id; its conversation's messages, draft and " +
		"rows in other files are shown once, with " + which
}

// cursorDataRows returns the rows of the state above and the tracking
// database: each stored row's own (its notice and name, chosen or not)
// and the composer's, with the time of the draft and the project and
// branch of the stored row they go with.
func cursorDataRows(old, recent bool, chosenOld bool) []string {
	var rows []string
	if old {
		rows = append(rows, "2026-01-01T00:00:00.000Z c1 /proj/old br-old "+cursorNotice("cursorDiskKV", chosenOld),
			"2026-01-01T00:00:00.000Z c1 /proj/old br-old session name: OLD")
	}
	draft := "2026-01-01T00:00:00.000Z c1 /proj/old br-old unsent draft: DRAFT-TEXT"
	if chosenOld {
		rows = append(rows, draft)
	}
	if recent {
		rows = append(rows, "2026-06-01T00:00:00.000Z c1 /proj/new br-new "+cursorNotice("cursorDiskKV", !chosenOld),
			"2026-06-01T00:00:00.000Z c1 /proj/new br-new session name: NEW")
	}
	attributes := "/proj/old br-old "
	if !chosenOld {
		rows = append(rows, "2026-06-01T00:00:00.000Z c1 /proj/new br-new unsent draft: DRAFT-TEXT")
		attributes = "/proj/new br-new "
	}
	return append(rows, "2026-06-02T00:00:00.000Z c1 "+attributes+"the question",
		"2026-06-02T00:00:01.000Z c1 "+attributes+"ai code: /src/f source=composer model=m request=r1")
}

func TestTheRowsOfARepeatedCursorComposerDataKeyGoWithTheFirstStoredRowTheFilterKeeps(t *testing.T) {
	for _, test := range []repeatCase{
		{"without a filter they go with the first stored row", nil, cursorDataRows(true, true, true)},
		{"a window past the first stored row moves them to the second", []string{"--since", "2026-03-01"},
			cursorDataRows(false, true, false)},
		{"a pattern only the second row matches moves them to the second",
			[]string{"--match", "name: NEW|DRAFT|question|ai code"}, []string{
				"2026-06-01T00:00:00.000Z c1 /proj/new br-new session name: NEW",
				"2026-06-01T00:00:00.000Z c1 /proj/new br-new unsent draft: DRAFT-TEXT",
				"2026-06-02T00:00:00.000Z c1 /proj/new br-new the question",
				"2026-06-02T00:00:01.000Z c1 /proj/new br-new ai code: /src/f source=composer model=m request=r1"}},
		{"a pattern only the first row matches keeps them with the first",
			[]string{"--match", "name: OLD|DRAFT|question|ai code"}, []string{
				"2026-01-01T00:00:00.000Z c1 /proj/old br-old session name: OLD",
				"2026-01-01T00:00:00.000Z c1 /proj/old br-old unsent draft: DRAFT-TEXT",
				"2026-06-02T00:00:00.000Z c1 /proj/old br-old the question",
				"2026-06-02T00:00:01.000Z c1 /proj/old br-old ai code: /src/f source=composer model=m request=r1"}},
		{"a pattern neither row matches keeps them with the first", []string{"--match", "question|ai code"},
			[]string{"2026-06-02T00:00:00.000Z c1 /proj/old br-old the question",
				"2026-06-02T00:00:01.000Z c1 /proj/old br-old ai code: /src/f source=composer model=m request=r1"}},
		{"a window before the second stored row keeps them with the first", []string{"--until", "2026-03-01"},
			[]string{"2026-01-01T00:00:00.000Z c1 /proj/old br-old " + cursorNotice("cursorDiskKV", true),
				"2026-01-01T00:00:00.000Z c1 /proj/old br-old session name: OLD",
				"2026-01-01T00:00:00.000Z c1 /proj/old br-old unsent draft: DRAFT-TEXT"}},
	} {
		for _, stateFirst := range []bool{true, false} {
			t.Run(test.name+cursorOrderName(stateFirst), func(t *testing.T) {
				rows := repeatedIDRunOf(t, "cursor", cursorFiles(t, cursorRepeatedData(t), stateFirst), test.options...)
				if !slices.Equal(rows, test.want) {
					t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
				}
			})
		}
	}
}

// cursorOrderName names the order the state database is read in.
func cursorOrderName(stateFirst bool) string {
	if stateFirst {
		return ", the state database read first"
	}
	return ", the state database read last"
}

// cursorGroupedStores returns the Cursor stores of a repeated id, for the
// property every parser on the grouping mechanism keeps.
func cursorGroupedStores() []groupedStore {
	return []groupedStore{
		{"cursor composerData, the state database first", "cursor", func(t *testing.T) []collectedFile {
			return cursorFiles(t, cursorRepeatedData(t), true)
		}},
		{"cursor composerData, the state database last", "cursor", func(t *testing.T) []collectedFile {
			return cursorFiles(t, cursorRepeatedData(t), false)
		}},
	}
}

// The property of TestAWindowShowsEveryRowOfARepeatedIDInsideIt and
// TestAMatchShowsEveryRowOfARepeatedIDItMatches, for the Cursor stores.
func TestCursorRepeatedIDsKeepEveryFilteredRowShown(t *testing.T) {
	for _, store := range cursorGroupedStores() {
		t.Run(store.name, func(t *testing.T) {
			directory := writeCollection(t, filepath.Join(t.TempDir(), "col"),
				[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))}, store.files(t))
			checkWindowProperty(t, directory, store.agent)
			checkMatchProperty(t, directory, store.agent)
		})
	}
}

// checkWindowProperty checks that a --since or --until at each time of
// the output without a filter shows every row of that output inside the
// window, and no row more often.
func checkWindowProperty(t *testing.T, directory, agent string) {
	t.Helper()
	unfiltered := timelineRows(t, directory, agent, rowfilter.RowFilter{}, nil, false)
	var times []string
	for _, row := range unfiltered {
		if row.TimestampUTC != "" && !slices.Contains(times, row.TimestampUTC) {
			times = append(times, row.TimestampUTC)
		}
	}
	for _, bound := range times {
		for _, window := range []rowfilter.RowFilter{{Since: bound}, {Until: bound}} {
			filtered := timelineRows(t, directory, agent, window, nil, false)
			if faults := keepFaults(unfiltered, filtered, func(row *timeline.Row) bool {
				return row.TimestampUTC != "" && (window.Since == "" || row.TimestampUTC >= window.Since) &&
					(window.Until == "" || row.TimestampUTC <= window.Until)
			}); len(faults) > 0 {
				t.Errorf("--since %q --until %q: %q", window.Since, window.Until, faults)
			}
		}
	}
}

// checkMatchProperty checks that a --match of any two texts of the
// grouped rows shows every row of the output without a filter it matches,
// and no row more often.
func checkMatchProperty(t *testing.T, directory, agent string) {
	t.Helper()
	var texts []string
	for _, row := range timelineRows(t, directory, agent, rowfilter.RowFilter{}, nil, true) {
		if text := row.FullText(); text != "" && !slices.Contains(texts, regexp.QuoteMeta(text)) {
			texts = append(texts, regexp.QuoteMeta(text))
		}
	}
	if len(texts) == 0 {
		t.Fatal("the store gave no grouped row")
	}
	unfiltered := timelineRows(t, directory, agent, rowfilter.RowFilter{}, nil, false)
	for i, first := range texts {
		for _, second := range texts[i:] {
			pattern := first + "|" + second
			filtered := timelineRows(t, directory, agent, rowfilter.RowFilter{}, []string{pattern}, false)
			matched := regexp.MustCompile(pattern)
			if faults := keepFaults(unfiltered, filtered, func(row *timeline.Row) bool {
				return row.Matches(matched)
			}); len(faults) > 0 {
				t.Errorf("--match %q: %q", pattern, faults)
			}
		}
	}
}
