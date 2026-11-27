package goose

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The rows of every store are checked by the golden output of the analysis
// commands (internal/analyze), over inputs that hold every parsed agent;
// these tests cover the helpers that decide a row's fields.

// decode returns the JSON text decoded, failing the test when it is not.
func decode(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	value, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestHistoryEscapesTakeTheNextCharacter(t *testing.T) {
	tests := []struct{ line, want string }{
		{`a\nb\\c`, "a\nb\\c"},
		{`x\ty`, "xty"},
		{`trail\`, `trail\`},
		{`\\n`, `\n`},
	}
	for _, test := range tests {
		t.Run(test.line, func(t *testing.T) {
			if got := unescapeHistory(test.line); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestCreatedTimesAboveTenBillionAreMilliseconds(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{"1772355600000", "2026-03-01T09:00:00.000Z"},
		{"17723556000", "1970-07-25T03:12:36.000Z"},
		{"1772355600", "2026-03-01T09:00:00.000Z"},
		{"1772355600.5", "2026-03-01T09:00:00.500Z"},
		{`"2026-03-01T09:00:00Z"`, "2026-03-01T09:00:00.000Z"},
		{"true", ""},
		{"null", ""},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			if got := epochJSON(decode(t, test.value)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
	if got := epochColumn(sqlitefile.IntValue(1772355600000)); got != "2026-03-01T09:00:00.000Z" {
		t.Errorf("a millisecond column gave %q", got)
	}
}

func TestYAMLScalarsUnquoteAsTheSerialiserWritesThem(t *testing.T) {
	tests := []struct{ value, want string }{
		{"plain", "plain"},
		{"'it''s'", "it's"},
		{`"a\u00e9"`, "aé"},
		{`"bad\q"`, `bad\q`},
		{`"1"`, "1"},
		{"'", "'"},
		{`""`, ""},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			if got := unquote(test.value); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestPermissionYAMLReadsOnlyCategoriesLevelsAndItems(t *testing.T) {
	path := filepath.Join(t.TempDir(), "permission.yaml")
	content := "---\nuser:\n  always_allow:\n  - developer__shell\n  - 'quoted'\n  ask_before: []\n" +
		"  never_allow:\n    - \"x\\u00e9\"\n# c\n- orphan\nsmart_approve:\n  bad line\n  - \n{}\n"
	if err := osfile.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	var problems parserkit.Problems
	got := fmt.Sprint(permissionEntries(path, &problems), problems)
	want := fmt.Sprint([]permissionEntry{{4, "user", "always_allow", "developer__shell"},
		{5, "user", "always_allow", "quoted"}, {8, "user", "never_allow", "xé"},
		{10, "user", "never_allow", "orphan"}}, parserkit.Problems{
		{Line: 12, Message: "unparseable line (not a category, level or list item)", Raw: "  bad line"},
		{Line: 13, Message: "unparseable line (not a category, level or list item)", Raw: "  - "}})
	if got != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
}

func TestMergeDeltaExtendsTheLastBlockOfTheSameKind(t *testing.T) {
	blocks := &streamed{blocks: decode(t, `[{"type":"text","text":"a","annotations":{"audience":["user"]}}]`).Items()}
	blocks.merge(decode(t, `[{"type":"text","text":"b","annotations":{"audience":["user"]}}]`).Items())
	blocks.merge(decode(t, `[{"type":"text","text":"c"}]`).Items())
	want := `[{"type":"text","text":"ab","annotations":{"audience":["user"]}},{"type":"text","text":"c"}]`
	if got := jsonvalue.NewArray(blocks.values()...).JSON(); got != want {
		t.Errorf("text: got %s, want %s", got, want)
	}
	thinking := &streamed{blocks: decode(t, `[{"type":"thinking","thinking":"x"}]`).Items()}
	thinking.merge(decode(t, `[{"type":"thinking","thinking":"y","signature":"s"}]`).Items())
	thinking.merge(decode(t, `[{"type":"thinking","thinking":"z","signature":"other"}]`).Items())
	want = `[{"type":"thinking","thinking":"xy","signature":"s"},` +
		`{"type":"thinking","thinking":"z","signature":"other"}]`
	if got := jsonvalue.NewArray(thinking.values()...).JSON(); got != want {
		t.Errorf("thinking: got %s, want %s", got, want)
	}
}

func TestScheduleTextShowsADashForAnAbsentValue(t *testing.T) {
	tests := []struct{ job, rel, want string }{
		{`{"id":"j1","cron":"0 * * * *","paused":1,"source":"","last_run":"2026-03-01T09:00:00Z",` +
			`"parameters":{"b":1}}`, "x/schedule.json", "scheduled task: j1 0 * * * * paused=true " +
			"running=false source=- last_run=2026-03-01T09:00:00.000Z current_session=- " +
			`recipe_base_dir=- parameters={"b":1} (from x/schedule.json, file mtime)`},
		{`{"last_run":"not a time","recipe_base_dir":5}`, "s.json", "scheduled task: - - " +
			"paused=false running=false source=- last_run=not a time current_session=- " +
			"recipe_base_dir=5 (from s.json, file mtime)"},
	}
	for _, test := range tests {
		t.Run(test.job, func(t *testing.T) {
			if got := scheduleText(decode(t, test.job), test.rel); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestResultTextReadsEveryShapeOfAToolResult(t *testing.T) {
	tests := []struct {
		result  string
		want    string
		isError bool
	}{
		{"null", "null", false},
		{`"x"`, `"x"`, false},
		{`{"status":"error","error":"boom"}`, "boom", true},
		{`{"status":"success","value":[{"type":"text","text":"t"}]}`, "t", false},
		{`{"value":{"content":[],"structuredContent":{"b":1},"isError":true}}`, `{"b":1}`, true},
		{`{"value":{"content":"s","structuredContent":null}}`, "s", false},
		{`{"value":5}`, "", false},
	}
	for _, test := range tests {
		t.Run(test.result, func(t *testing.T) {
			got, isError := resultText(decode(t, test.result))
			if got != test.want || isError != test.isError {
				t.Errorf("got %q, %v; want %q, %v", got, isError, test.want, test.isError)
			}
		})
	}
}

// legacyRows parses the legacy session file at path as the file at rel and
// returns its rows with source_file cleared.
func legacyRows(t *testing.T, path, rel string) []timeline.Row {
	t.Helper()
	artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel,
		Home: &input.Home{DiskPath: filepath.Dir(path)}}
	var rows []timeline.Row
	if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) {
		row.SourceFile = ""
		rows = append(rows, row)
	}); err != nil {
		t.Fatal(err)
	}
	return rows
}

// The fixed parts of a legacy session's path are matched in any ASCII
// case, and the session id cut from its name keeps the disk's spelling.
func TestALegacySessionSpelledInAnotherCaseGivesTheSameRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "s.jsonl")
	if err := osfile.WriteFile(path, []byte(`{"working_dir": "/srv/proj"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	want := legacyRows(t, path, ".local/share/goose/sessions/Sess_A.jsonl")
	got := legacyRows(t, path, ".LOCAL/Share/Goose/SESSIONS/Sess_A.JSONL")
	if len(want) != 1 || want[0].SessionID != "Sess_A" || !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
