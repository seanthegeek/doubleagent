package qwencode

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parsedRows writes text at rel under a new home, parses it and returns its
// rows as "<source line> <text>".
func parsedRows(t *testing.T, rel, text string) []string {
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
	err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) {
		rows = append(rows, strconv.Itoa(row.SourceLine)+" "+row.Text)
	})
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

// problemsOf returns the problem rows among rows.
func problemsOf(rows []string) []string {
	var out []string
	for _, row := range rows {
		if _, text, _ := strings.Cut(row, " "); strings.HasPrefix(text, parserkit.ProblemPrefix) {
			out = append(out, row)
		}
	}
	return out
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

// kindCases are the members of Qwen Code's files that the format makes an
// array or an object.
func kindCases() []kindCase {
	const session = ".qwen/projects/p/chats/0123456789abcdef0123456789abcdef.jsonl"
	const record = `{"sessionId": "s", "type": "assistant", "timestamp": "2026-01-01T00:00:00Z"`
	wrongArrays := []string{`"x"`, `7`, `{"a": 1}`}
	wrongObjects := []string{`"x"`, `7`, `[1]`}
	return []kindCase{
		{"todos", ".qwen/todos/s.json", `{"sessionId": "s", "todos": @}`, `{"sessionId": "s"}`,
			"0 parser: todos is not a JSON array", wrongArrays, `[]`},
		{"sessions", ".qwen/projects/p/session-organization.v1.json", `{"schemaVersion": 1, "sessions": @}`,
			`{"schemaVersion": 1}`, "0 parser: sessions is not a JSON object", wrongObjects, `{}`},
		{"prs", ".qwen/projects/p/chats/0123456789abcdef0123456789abcdef.pr.json", `{"prs": @}`, `{}`,
			"0 parser: prs is not a JSON array", wrongArrays, `[]`},
		{"permissions", ".qwen/settings.json", `{"permissions": @}`, `{}`,
			"0 parser: permissions is not a JSON object", wrongObjects, `{}`},
		{"permissions.allow", ".qwen/settings.json", `{"permissions": {"allow": @}}`, `{"permissions": {}}`,
			"0 parser: permissions.allow is not a JSON array", wrongArrays, `[]`},
		{"permissions.ask", ".qwen/settings.json", `{"permissions": {"ask": @}}`, `{"permissions": {}}`,
			"0 parser: permissions.ask is not a JSON array", wrongArrays, `[]`},
		{"permissions.deny", ".qwen/settings.json", `{"permissions": {"deny": @}}`, `{"permissions": {}}`,
			"0 parser: permissions.deny is not a JSON array", wrongArrays, `[]`},
		{"message", session, record + `, "message": @}`, record + `}`,
			"1 parser: message is not a JSON object", wrongObjects, `{}`},
		{"message.parts", session, record + `, "message": {"role": "model", "parts": @}}`,
			record + `, "message": {"role": "model"}}`, "1 parser: message.parts is not a JSON array",
			wrongArrays, `[]`},
		{"groups", ".qwen/projects/p/session-organization.v1.json", `{"schemaVersion": 1, "groups": @}`,
			`{"schemaVersion": 1}`, "0 parser: groups is not a JSON array", wrongArrays, `[]`},
	}
}

func TestEntriesThatAreNotObjectsAreReportedOncePerList(t *testing.T) {
	for _, test := range kindCases() {
		if test.name != "message.parts" && test.name != "groups" {
			continue
		}
		t.Run(test.name, func(t *testing.T) {
			got := problemsOf(parsedRows(t, test.rel, strings.ReplaceAll(test.document, "@", `[{}, "x", 7]`)))
			want := strings.Replace(test.want, " is not a JSON array", " holds 2 entries that are not JSON objects", 1)
			if len(got) != 1 || got[0] != want {
				t.Errorf("problem rows %q, want %q", got, want)
			}
		})
	}
}

func TestAMemberOfTheWrongKindIsReported(t *testing.T) {
	for _, test := range kindCases() {
		t.Run(test.name, func(t *testing.T) {
			for _, value := range test.wrong {
				got := problemsOf(parsedRows(t, test.rel, strings.ReplaceAll(test.document, "@", value)))
				if len(got) != 1 || got[0] != test.want {
					t.Errorf("as %s: problem rows %q, want %q", value, got, test.want)
				}
			}
			for _, document := range []string{strings.ReplaceAll(test.document, "@", "null"),
				strings.ReplaceAll(test.document, "@", test.empty), test.absent} {
				if got := problemsOf(parsedRows(t, test.rel, document)); len(got) != 0 {
					t.Errorf("%s: problem rows %q, want none", document, got)
				}
			}
		})
	}
}

func TestTheRecordOfAMessageOfTheWrongKindIsKept(t *testing.T) {
	const rel = ".qwen/projects/p/chats/0123456789abcdef0123456789abcdef.jsonl"
	rows := parsedRows(t, rel, `{"sessionId": "s", "type": "user", "message": "hi"}`)
	want := []string{"1 ", "1 parser: message is not a JSON object"}
	if len(rows) != len(want) || rows[0] != want[0] || rows[1] != want[1] {
		t.Errorf("rows %q, want %q", rows, want)
	}
}

func TestAPullRequestsIssuesOfTheWrongKindAreReportedOncePerList(t *testing.T) {
	const rel = ".qwen/projects/p/chats/0123456789abcdef0123456789abcdef.pr.json"
	tests := []struct {
		name, issues string
		want         []string
	}{
		{"entries that are not objects", `[{"number": 3}, "x", 7]`,
			[]string{"1 parser: issues holds 2 entries that are not JSON objects"}},
		{"an issues that is not an array", `"x"`, []string{"1 parser: issues is not a JSON array"}},
		{"objects only", `[{"number": 3}]`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := problemsOf(parsedRows(t, rel, `{"prs": [{"url": "https://github.com/o/r/pull/7", "issues": `+
				test.issues+`}]}`))
			if !slices.Equal(got, test.want) {
				t.Errorf("problem rows %q, want %q", got, test.want)
			}
		})
	}
}

func TestAPathSpelledInAnotherCaseGivesTheSameRows(t *testing.T) {
	const name = "0123456789ABCDEF0123456789abcdef.pr.json"
	const prs = `{"prs": [{"url": "https://github.com/o/r/pull/7", "createdAt": "2026-01-01T00:00:00Z"}]}`
	rowsAt := func(rel string) []string {
		home := t.TempDir()
		path := filepath.Join(home, filepath.FromSlash(rel))
		if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := osfile.WriteFile(path, []byte(prs), 0o600); err != nil {
			t.Fatal(err)
		}
		var rows []string
		artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
		if err := New().Parse(artifact, parserkit.Options{}, func(row timeline.Row) {
			rows = append(rows, row.SessionID+" "+row.Text)
		}); err != nil {
			t.Fatal(err)
		}
		return rows
	}
	usual := rowsAt(".qwen/projects/p/chats/" + name)
	if len(usual) != 1 || !strings.HasPrefix(usual[0], "0123456789ABCDEF0123456789abcdef source host:") {
		t.Fatalf("usual rows %q", usual)
	}
	for _, rel := range []string{".QWEN/Projects/p/CHATS/" + name, ".Qwen/Managed-Local/HARNESS/projects/p/Chats/" + name} {
		if got := rowsAt(rel); !slices.Equal(got, usual) {
			t.Errorf("%s: rows %q, want %q", rel, got, usual)
		}
	}
}
