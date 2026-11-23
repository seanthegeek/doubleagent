package analyze

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserregistry"
)

// The texts the shared-text fixture's rows name: a paste three history
// lines name, a persisted tool output two tool results name, and a Cursor
// full-text body three conversations join. The paste holds a URL
// credential, which a run that withholds secrets replaces.
const (
	sharedPaste  = "PASTE-TEXT https://user:pw@example.com/x"
	sharedOutput = "PERSISTED-OUTPUT-TEXT"
	sharedBody   = "the shared body"
	transcript   = "/home/alice/.claude/projects/-p/s.jsonl"
	historyFile  = "/home/alice/.claude/history.jsonl"
)

// sharedTextFiles returns the files of a collection whose rows share long
// texts: three history lines at :01, :02 and :03 naming one paste-cache
// file, the first also recorded in the transcript; two tool results at :10
// and :20, the second an error, naming one persisted output; and three
// Cursor conversations at :05, :06 and :07 joining one full-text body.
func sharedTextFiles(t *testing.T) []collectedFile {
	t.Helper()
	body, err := osfile.ReadFile(filepath.Join("..", "parsers", "cursor", "testdata", "shared_body_times.db"))
	if err != nil {
		t.Fatal(err)
	}
	history := func(display, millis string) string {
		return `{"display": "` + display + `", "pastedContents": {"1": {"contentHash": "h1"}}, ` +
			`"timestamp": ` + millis + `, "sessionId": "s", "project": "/p"}` + "\n"
	}
	stub := "<persisted-output>\nFull output saved to: /home/alice/.claude/projects/-p/s/tool-results/a.txt" +
		"\n</persisted-output>"
	result := func(second, toolUseID, isError string) string {
		return `{"type": "user", "sessionId": "s", "cwd": "/p", "timestamp": "2026-01-02T03:04:` + second +
			`.000Z", "message": {"role": "user", "content": [{"type": "tool_result", "tool_use_id": "` + toolUseID +
			`", "is_error": ` + isError + `, "content": ` + jsonvalue.NewString(stub).JSON() + `}]}}` + "\n"
	}
	prompt := `{"type": "user", "sessionId": "s", "cwd": "/p", "timestamp": "2026-01-02T03:04:01.000Z", ` +
		`"message": {"role": "user", "content": ` + jsonvalue.NewString("one "+sharedPaste+" and "+sharedPaste).JSON() +
		`}}` + "\n"
	return []collectedFile{
		{"alice", "/home/alice", "claude-code", ".claude/history.jsonl",
			history("one [Pasted text #1] and [Pasted text #1]", "1767323041000") +
				history("two [Pasted text #1]", "1767323042000") + history("three [Pasted text #1]", "1767323043000")},
		{"alice", "/home/alice", "claude-code", ".claude/paste-cache/h1.txt", sharedPaste},
		{"alice", "/home/alice", "claude-code", ".claude/projects/-p/s.jsonl",
			prompt + result("10", "t1", "false") + result("20", "t2", "true")},
		{"alice", "/home/alice", "claude-code", ".claude/projects/-p/s/tool-results/a.txt", sharedOutput},
		{"alice", "/home/alice", "cursor", ".config/Cursor/User/globalStorage/conversation-search.db",
			string(body)},
	}
}

// sharedRow is what a test reads of a row of timeline.jsonl.
type sharedRow struct {
	file, text string
	line       int64
}

// sharedTimeline runs timeline over the shared-text collection with the
// options given and returns the rows of timeline.jsonl, in order.
func sharedTimeline(t *testing.T, options ...string) []sharedRow {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))}, sharedTextFiles(t))
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parserregistry.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline"}, options...), collection, output)
	if code, _, stderr := runCommand(env, args...); code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	data, err := osfile.ReadFile(filepath.Join(output, TimelineFile))
	if err != nil {
		t.Fatal(err)
	}
	var rows []sharedRow
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		record, err := jsonvalue.DecodeString(line)
		if err != nil {
			t.Fatal(err)
		}
		number, _ := record.Get("source_line").Int()
		rows = append(rows, sharedRow{record.Get("source_file").StringValue(), record.Get("text").StringValue(), number})
	}
	return rows
}

// sharedTexts returns the texts of the rows from file whose text starts
// with prefix, in order.
func sharedTexts(rows []sharedRow, file, prefix string) []string {
	var texts []string
	for _, row := range rows {
		if strings.HasSuffix(row.file, file) && strings.HasPrefix(row.text, prefix) {
			texts = append(texts, row.text)
		}
	}
	return texts
}

// The texts each kind of row gives in full and as a pointer.
const (
	pasteHeld     = "PASTE-TEXT https://user:pw@example.com/x"
	outputPointer = "<persisted-output>\nFull output saved to: /home/alice/.claude/projects/-p/s/tool-results/a.txt" +
		"\n</persisted-output>\n[persisted output shown in full above (" + transcript + " line 2): tool-results/a.txt]"
	bodyRow = "cached conversation text (conversation-search.db, source="
)

// The rows of each kind when every row is kept but the history line its
// transcript record holds.
var (
	allHistory = []string{"two " + pasteHeld, "three [Pasted text #1, shown above in " + historyFile + " line 2]"}
	allOutputs = []string{sharedOutput, "[error] " + outputPointer}
	allBodies  = []string{bodyRow + "local): " + sharedBody, bodyRow + "remote): " + bodyPointerTo(1),
		bodyRow + "remote): " + bodyPointerTo(1)}
	laterBodies = []string{bodyRow + "remote): " + sharedBody, bodyRow + "remote): " + bodyPointerTo(2)}
)

// bodyPointerTo is the text after the prefix of a conversation's row whose
// body the conversation of source line holds.
func bodyPointerTo(line int) string {
	return "[shown in full above, in the row of source line " + strconv.Itoa(line) + "]"
}

func TestASharedTextIsWrittenInFullAtItsFirstReferenceTheOutputKeeps(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name                   string
		options                []string
		history, outputs, body []string
	}{
		{"the first history line, a duplicate of its record, leaves the paste to the second", nil,
			allHistory, allOutputs, allBodies},
		{"a window past the first history line", []string{"--since", "2026-01-02T03:04:02.500Z"},
			[]string{"three " + pasteHeld}, allOutputs, allBodies},
		{"a window past the first conversation", []string{"--since", "2026-01-02T03:04:05.500Z"},
			nil, allOutputs, laterBodies},
		{"a window past the first tool result", []string{"--since", "2026-01-02T03:04:15Z"},
			nil, []string{"[error] " + sharedOutput}, nil},
		{"a pattern only later rows match", []string{"--match", `^(two|three|\[error\])|source=remote`},
			allHistory, []string{"[error] " + sharedOutput}, laterBodies},
		{"a pattern only the shared text matches keeps every row naming it", []string{"--match", "PASTE-TEXT"},
			allHistory, nil, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rows := sharedTimeline(t, test.options...)
			checks := []struct {
				label     string
				got, want []string
			}{{"history", sharedTexts(rows, historyFile, ""), test.history},
				{"tool results", toolResultTexts(sharedTexts(rows, transcript, "")), test.outputs},
				{"bodies", sharedTexts(rows, "conversation-search.db", bodyRow), test.body}}
			for _, check := range checks {
				if !slices.Equal(check.got, check.want) {
					t.Errorf("%s: got %q\nwant %q", check.label, check.got, check.want)
				}
			}
		})
	}
}

// toolResultTexts returns the texts of the transcript's rows that are its
// tool results, which name the persisted output.
func toolResultTexts(texts []string) []string {
	var results []string
	for _, text := range texts {
		if strings.Contains(text, sharedOutput) || strings.Contains(text, "<persisted-output>") {
			results = append(results, text)
		}
	}
	return results
}

func TestWithholdingSecretsKeepsTheRowsThatHoldAndPointToASharedText(t *testing.T) {
	t.Parallel()
	for _, options := range [][]string{nil, {"--since", "2026-01-02T03:04:02.500Z"}} {
		shown := sharedTimeline(t, options...)
		withheld := sharedTimeline(t, append([]string{"--no-secrets"}, options...)...)
		if len(shown) != len(withheld) {
			t.Fatalf("%q: %d rows shown, %d withheld", options, len(shown), len(withheld))
		}
		for i := range shown {
			replaced := strings.ReplaceAll(shown[i].text, "user:pw@", "REDACTED@")
			if shown[i].file != withheld[i].file || shown[i].line != withheld[i].line || replaced != withheld[i].text {
				t.Errorf("%q row %d: shown %+v, withheld %+v", options, i, shown[i], withheld[i])
			}
		}
	}
}
