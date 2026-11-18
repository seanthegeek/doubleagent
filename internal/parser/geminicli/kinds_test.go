package geminicli

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// problemRows writes text at rel under a new home, parses it and returns
// the parser's problem rows as "<source line> <text>".
func problemRows(t *testing.T, rel, text string) []string {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
	var rows []string
	err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) {
		if strings.HasPrefix(row.Text, parser.ProblemPrefix) {
			rows = append(rows, strconv.Itoa(row.SourceLine)+" "+row.Text)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

// kindCase is one member the format gives a kind: the file it is in, the
// file's text with the member's value in place of "@", the text with the
// member left out, the problem row a wrong kind gives, the values of the
// wrong kind and the empty value of the right kind.
type kindCase struct {
	name, rel, document, absent, want string
	wrong                             []string
	empty                             string
}

// sessionKindCases are the members of a session file's records.
func sessionKindCases() []kindCase {
	const rel = ".gemini/tmp/proj/chats/session-1.jsonl"
	const header = `{"sessionId": "s", "projectHash": "h", "startTime": "2026-01-01T00:00:00Z"}` + "\n"
	const message = `{"id": "m1", "type": "gemini", "content": "hi"`
	wrongArrays := []string{`"x"`, `7`, `{"a": 1}`}
	return []kindCase{
		{"toolCalls", rel, header + message + `, "toolCalls": @}`, header + message + `}`,
			"2 parser: toolCalls is not a JSON array", wrongArrays, `[]`},
		{"thoughts", rel, header + message + `, "thoughts": @}`, header + message + `}`,
			"2 parser: thoughts is not a JSON array", wrongArrays, `[]`},
		{"$patch.updates", rel, header + message + "}\n" + `{"$patch": {"updates": @}}`,
			header + message + "}\n" + `{"$patch": {}}`,
			"3 parser: $patch.updates is not a JSON array", wrongArrays, `[]`},
		{"$patch.removeIds", rel, header + message + "}\n" + `{"$patch": {"removeIds": @}}`,
			header + message + "}\n" + `{"$patch": {}}`,
			"3 parser: $patch.removeIds is not a JSON array", wrongArrays, `[]`},
		{"$patch.orderIds", rel, header + message + "}\n" + `{"$patch": {"orderIds": @}}`,
			header + message + "}\n" + `{"$patch": {}}`,
			"3 parser: $patch.orderIds is not a JSON array", wrongArrays, `[]`},
		{"$patch.toolCalls", rel, header + message + "}\n" + `{"$patch": {"id": "m1", "toolCalls": @}}`,
			header + message + "}\n" + `{"$patch": {"id": "m1"}}`,
			"3 parser: $patch.toolCalls is not a JSON array", wrongArrays, `[]`},
		{"an update's toolCalls", rel,
			header + message + "}\n" + `{"$patch": {"updates": [{"id": "m1", "toolCalls": @}]}}`,
			header + message + "}\n" + `{"$patch": {"updates": [{"id": "m1"}]}}`,
			"3 parser: $patch.updates[].toolCalls is not a JSON array", wrongArrays, `[]`},
		{"$set.messages", rel, header + `{"$set": {"messages": @}}`, header + `{"$set": {}}`,
			"2 parser: $set.messages is not a JSON array", wrongArrays, `[]`},
		{"legacy messages", ".gemini/tmp/proj/chats/session-1.json", `{"sessionId": "s", "messages": @}`,
			`{"sessionId": "s"}`, "1 parser: messages is not a JSON array", wrongArrays, `[]`},
	}
}

// copyKindCases are the history lists of a tool checkpoint and an error
// report.
func copyKindCases() []kindCase {
	const checkpoint = ".gemini/tmp/proj/checkpoints/2026-10-01T09-00-21_500Z-util.ts-replace.json"
	const report = ".gemini/gemini-client-error-Turn.run-2026-10-01T09-00-21-500Z.json"
	const reportStart = `{"error": {"message": "boom"}`
	wrongArrays := []string{`"x"`, `7`, `{"a": 1}`}
	return []kindCase{
		{"clientHistory", checkpoint, `{"messageId": "m", "clientHistory": @}`, `{"messageId": "m"}`,
			"0 parser: clientHistory is not a JSON array", wrongArrays, `[]`},
		// An object context is the form that holds requestContents.
		{"context", report, reportStart + `, "context": @}`, reportStart + `}`,
			"0 parser: context is not a JSON array", []string{`"x"`, `7`, `true`}, `[]`},
		{"context.requestContents", report, reportStart + `, "context": {"requestContents": @}}`,
			reportStart + `, "context": {}}`, "0 parser: context.requestContents is not a JSON array",
			wrongArrays, `[]`},
	}
}

func TestAMemberThatIsNotAnArrayIsReported(t *testing.T) {
	for _, test := range slices.Concat(sessionKindCases(), copyKindCases()) {
		t.Run(test.name, func(t *testing.T) {
			for _, value := range test.wrong {
				got := problemRows(t, test.rel, strings.ReplaceAll(test.document, "@", value))
				if want := []string{test.want}; !slices.Equal(got, want) {
					t.Errorf("as %s: problem rows %q, want %q", value, got, want)
				}
			}
			for _, document := range []string{strings.ReplaceAll(test.document, "@", "null"),
				strings.ReplaceAll(test.document, "@", test.empty), test.absent} {
				if got := problemRows(t, test.rel, document); len(got) != 0 {
					t.Errorf("%s: problem rows %q, want none", document, got)
				}
			}
		})
	}
}

func TestASandboxTableOfTheWrongKindIsReported(t *testing.T) {
	tests := []struct {
		name, text string
		want       []string
	}{
		{"tables", "[modes.plan]\nnetwork = false\n[commands.git]\nallow_network = true\n", nil},
		{"empty tables", "[modes]\n[commands]\n", nil},
		{"no tables", "", nil},
		{"modes as a string", "modes = \"x\"\n", []string{"1 parser: " + sandboxPolicy + " modes is not a table"}},
		{"commands as an array", "commands = [1]\n", []string{"1 parser: " + sandboxPolicy + " commands is not a table"}},
		{"a grant as a number", "[commands]\ngit = 1\n",
			[]string{"1 parser: " + sandboxPolicy + " commands.git is not a table"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := problemRows(t, sandboxPolicy, test.text); !slices.Equal(got, test.want) {
				t.Errorf("rows %q, want %q", got, test.want)
			}
		})
	}
}

func TestEntriesOfTheWrongKindAreReportedOncePerList(t *testing.T) {
	for _, test := range sessionKindCases() {
		t.Run(test.name, func(t *testing.T) {
			entries, kind := `[7, "x"]`, "objects"
			if strings.HasSuffix(test.name, "Ids") {
				entries, kind = `[{}, 7]`, "strings"
			}
			want := []string{strings.Replace(test.want, " is not a JSON array",
				" holds 2 entries that are not JSON "+kind, 1)}
			if got := problemRows(t, test.rel, strings.ReplaceAll(test.document, "@", entries)); !slices.Equal(got, want) {
				t.Errorf("problem rows %q, want %q", got, want)
			}
		})
	}
	t.Run("logs.json", func(t *testing.T) {
		got := problemRows(t, ".gemini/tmp/proj/logs.json", `[{"sessionId": "s", "message": "hi"}, "x"]`)
		if want := []string{"0 parser: logs.json holds 1 entry that is not a JSON object"}; !slices.Equal(got, want) {
			t.Errorf("problem rows %q, want %q", got, want)
		}
	})
}
