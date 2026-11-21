package analyze

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parsers"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// cursorKeysRun runs timeline over a home holding the state.vscdb given,
// with the options given, and returns each row of timeline.jsonl as
// "time session model text", an empty field as "".
func cursorKeysRun(t *testing.T, state string, options ...string) []string {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))},
		[]collectedFile{{"alice", "/home/alice", "cursor", cursorStateRel, state}})
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline", "--agent", "cursor"}, options...), collection, output)
	if code, _, stderr := runCommand(env, args...); code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	return recordsOf(t, filepath.Join(output, TimelineFile), "text", "timestamp_utc", "session_id", "model", "text")
}

// cursorEachOwn returns the parser: row of a stored row of a key the table
// holds twice whose stored rows each give their own rows.
func cursorEachOwn(table string) string {
	return "parser: " + table + " holds 2 rows of this row's key; each gives its own rows"
}

// cursorRepeatedBubble returns a state.vscdb whose composer c1 names the
// bubbles b1 and b2; cursorDiskKV holds the key of b1 twice, a prompt in
// January on m-old and a prompt in June on m-new, and b2 is a reply a
// day after the later.
func cursorRepeatedBubble(t *testing.T) string {
	t.Helper()
	return cursorStateOf(t, [][]any{
		{"composerData:c1", `{"composerId":"c1","status":"completed","fullConversationHeadersOnly":[` +
			`{"bubbleId":"b1","type":1},{"bubbleId":"b2","type":2}]}`},
		{"bubbleId:c1:b1", `{"type":1,"text":"FIRST-Q","createdAt":1767225600000,"modelInfo":{"modelName":"m-old"}}`},
		{"bubbleId:c1:b1", `{"type":1,"text":"SECOND-Q","createdAt":1780272000000,"modelInfo":{"modelName":"m-new"}}`},
		{"bubbleId:c1:b2", `{"type":2,"text":"THE-ANSWER","createdAt":1780358400000}`}}, nil)
}

func TestEachStoredRowOfARepeatedCursorBubbleKeyGivesItsOwnRows(t *testing.T) {
	notice := cursorEachOwn("cursorDiskKV") + ", and none gives its model to the rows after them"
	january, june := "2026-01-01T00:00:00.000Z c1 ", "2026-06-01T00:00:00.000Z c1 "
	for _, test := range []repeatCase{
		{"without a filter each prompt is shown and the reply takes neither model", nil, []string{
			january + " " + notice, january + "m-old model: m-old", january + "m-old FIRST-Q",
			june + " " + notice, june + "m-new model: m-new", june + "m-new SECOND-Q",
			"2026-06-02T00:00:00.000Z c1  THE-ANSWER"}},
		{"a window past the first prompt keeps the second", []string{"--since", "2026-03-01"}, []string{
			june + " " + notice, june + "m-new model: m-new", june + "m-new SECOND-Q",
			"2026-06-02T00:00:00.000Z c1  THE-ANSWER"}},
		{"a pattern finds the second prompt by its own text", []string{"--match", "SECOND-Q"},
			[]string{june + "m-new SECOND-Q"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := cursorKeysRun(t, cursorRepeatedBubble(t), test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}

// A composerDraft key cursorDiskKV holds twice gives each stored draft,
// one of them empty, at the composer's time; nothing takes from a draft.
func TestEachStoredRowOfARepeatedCursorDraftKeyIsShown(t *testing.T) {
	state := cursorStateOf(t, [][]any{
		{"composerData:c1", `{"composerId":"c1","createdAt":1767225600000,"status":"none"}`},
		{"composerDraft:c1", `{"text":"FIRST-DRAFT"}`},
		{"composerDraft:c1", `{"text":"SECOND-DRAFT"}`},
		{"composerDraft:c1", `{"text":""}`}}, nil)
	created := "2026-01-01T00:00:00.000Z c1  "
	notice := "parser: cursorDiskKV holds 3 rows of this row's key; each gives its own rows"
	for _, test := range []repeatCase{
		{"without a filter every stored draft is shown", nil, []string{created + notice, created + "unsent draft: FIRST-DRAFT",
			created + notice, created + "unsent draft: SECOND-DRAFT", created + notice}},
		{"a pattern finds the later draft", []string{"--match", "SECOND"}, []string{created + "unsent draft: SECOND-DRAFT"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			rows := cursorKeysRun(t, state, test.options...)
			if !slices.Equal(rows, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
}

// cursorRepeatedItems returns a state.vscdb whose ItemTable holds the
// settings blob, the sandbox switch, the repository list and the MCP
// authorisations each twice.
func cursorRepeatedItems(t *testing.T) string {
	t.Helper()
	repos, auth := "cloudEnvironmentTemplate.githubRepos", "cloudAgentRepository.mcpAuthResolution"
	return buildDatabase(t, sqlitetest.Table{Name: "ItemTable", SQL: "CREATE TABLE ItemTable (key TEXT, value BLOB)",
		Rows: [][]any{
			{cursorSettingsKey, `{"composerState":{"yoloCommandAllowlist":["ls"],"modes4":[{"id":"agent","autoRun":true}]}}`},
			{cursorSettingsKey, `{"composerState":{"yoloCommandAllowlist":["rm"]}}`},
			{"cursor/sandboxAutoRunSettingEnabled", "true"}, {"cursor/sandboxAutoRunSettingEnabled", "false"},
			{repos, `[{"url":"https://github.com/o/r1","type":"github"}]`}, {repos, `[{"url":"https://github.com/o/r2"}]`},
			{auth, `{"agentId":"bc-1","toolCallId":"t1","userDecision":"approve"}`},
			{auth, `{"agentId":"bc-2","toolCallId":"t2","userDecision":"deny"}`}}})
}

// cursorSettingsKey is the ItemTable key of the settings blob.
const cursorSettingsKey = "src.vs.platform.reactivestorage.browser.reactiveStorageServiceImpl" +
	".persistentStorage.applicationUser"

// cursorItemTexts runs timeline over the state above with the options
// given and returns the texts of its rows, sorted.
func cursorItemTexts(t *testing.T, options ...string) []string {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))},
		[]collectedFile{{"alice", "/home/alice", "cursor", cursorStateRel, cursorRepeatedItems(t)}})
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline", "--agent", "cursor"}, options...), collection, output)
	if code, _, stderr := runCommand(env, args...); code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	texts := recordsOf(t, filepath.Join(output, TimelineFile), "text", "text")
	slices.Sort(texts)
	return texts
}

func TestEachStoredRowOfARepeatedCursorItemTableKeyIsShown(t *testing.T) {
	texts := cursorItemTexts(t)
	for _, want := range []string{"yoloCommandAllowlist=ls", "yoloCommandAllowlist=rm", "github.com/o/r1",
		"github.com/o/r2", "cloud agent bc-1", "cloud agent bc-2"} {
		if count := countContaining(texts, want); count != 1 {
			t.Errorf("%d rows hold %q, want 1:\n%s", count, want, strings.Join(texts, "\n"))
		}
	}
	if count := countContaining(texts, "rows of this row's key; each gives its own rows"); count != 6 {
		t.Errorf("%d rows say a stored row gives its own rows, want 6", count)
	}
}

// countContaining returns how many of texts hold part.
func countContaining(texts []string, part string) int {
	count := 0
	for _, text := range texts {
		if strings.Contains(text, part) {
			count++
		}
	}
	return count
}

// The run-mode rows take the sandbox switch, so a switch key ItemTable
// holds twice shows them once, with the stored switch a filter keeps.
func TestTheRunModesOfARepeatedCursorSandboxSwitchGoWithTheStoredRowTheFilterKeeps(t *testing.T) {
	mode := func(label, sandbox string) string {
		return "permission setting: run mode agent=" + label + " autoRun=true smartModeAutoRun=- fullAutoRun=- " +
			"sandbox=" + sandbox + " (from state.vscdb composerState, file mtime, written late by the app)"
	}
	withSandbox, without := mode("Allowlist (with Sandbox)", "true"), mode("Allowlist", "false")
	for _, test := range []repeatCase{
		{"a pattern only the second switch's mode matches shows it", []string{"--match", "sandbox=false"},
			[]string{without}},
		{"a pattern only the first switch's mode matches shows it", []string{"--match", "sandbox=true"},
			[]string{withSandbox}},
		{"a pattern on the modes of both shows the first", []string{"--match", "run mode agent"}, []string{withSandbox}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if texts := cursorItemTexts(t, test.options...); !slices.Equal(texts, test.want) {
				t.Errorf("rows\n%s\nwant\n%s", strings.Join(texts, "\n"), strings.Join(test.want, "\n"))
			}
		})
	}
	texts := cursorItemTexts(t)
	if countContaining(texts, "run mode agent") != 1 || !slices.Contains(texts, withSandbox) {
		t.Errorf("without a filter, want the first switch's run mode alone:\n%s", strings.Join(texts, "\n"))
	}
	notice := "parser: ItemTable holds 2 rows of this row's key; its run mode rows are shown once, with "
	if !slices.Contains(texts, notice+"this row") || !slices.Contains(texts, notice+"another of them") {
		t.Errorf("want a parser: row per stored switch:\n%s", strings.Join(texts, "\n"))
	}
}
