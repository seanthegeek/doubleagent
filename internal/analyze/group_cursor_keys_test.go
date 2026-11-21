package analyze

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parsers"
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
