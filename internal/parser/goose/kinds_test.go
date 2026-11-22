package goose

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parseFile writes text at rel under a new home and returns the texts of
// the rows the parser gives it.
func parseFile(t *testing.T, rel, text string) []string {
	t.Helper()
	home := t.TempDir()
	path := filepath.Join(home, filepath.FromSlash(rel))
	if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{DiskPath: path, Original: "/h/" + rel, Rel: rel, Agent: "goose",
		Home: &input.Home{DiskPath: home}, Manifest: jsonvalue.NewNull()}
	var texts []string
	if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
		t.Fatal(err)
	}
	return texts
}

// count returns how many of the rows have exactly the given text.
func count(rows []string, text string) int {
	total := 0
	for _, row := range rows {
		if row == text {
			total++
		}
	}
	return total
}

// kindTest is one value of a member: the member's JSON, and whether it
// must give the problem row.
type kindTest struct {
	name, json string
	reported   bool
}

// kindTests returns the values a member of the wrong kind can hold, and the
// ones that stay silent: null and an empty value of the right kind. other
// is a value of the other container kind; when text is true a string is a
// valid value too and so is not in the list.
func kindTests(other string, text bool) []kindTest {
	tests := []kindTest{
		{"a number", `7`, true},
		{"the other container", other, true},
		{"null", `null`, false},
	}
	if !text {
		tests = append(tests, kindTest{"a string", `"oops"`, true})
	}
	return tests
}

// checkMember runs build on each value of a member and checks the problem
// row appears exactly when the value is of the wrong kind; absent is the
// text with the member left out.
func checkMember(t *testing.T, tests []kindTest, problem string, build func(value string) []string, absent []string) {
	t.Helper()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want := 0
			if test.reported {
				want = 1
			}
			if got := count(build(test.json), problem); got != want {
				t.Errorf("%d rows %q, want %d", got, problem, want)
			}
		})
	}
	t.Run("absent", func(t *testing.T) {
		if got := count(absent, problem); got != 0 {
			t.Errorf("%d rows %q, want none", got, problem)
		}
	})
}

func TestARoamingTrustListThatIsNotAnArrayIsReported(t *testing.T) {
	const rel = ".config/goose/roaming_trust.json"
	for _, key := range []string{"allowed", "revoked_keys"} {
		t.Run(key, func(t *testing.T) {
			checkMember(t, kindTests(`{"k": 1}`, false), "parser: "+parser.WrongKind(key, jsonvalue.Array),
				func(value string) []string { return parseFile(t, rel, `{"`+key+`": `+value+`}`) },
				parseFile(t, rel, `{}`))
		})
	}
}

func TestALegacyContentThatIsNotAnArrayIsReported(t *testing.T) {
	const rel = ".local/share/goose/sessions/s.jsonl"
	problem := "parser: " + parser.WrongKind("content", jsonvalue.Array)
	checkMember(t, kindTests(`{"type": "text"}`, true), problem,
		func(value string) []string {
			return parseFile(t, rel, `{"id": "s"}`+"\n"+`{"role": "user", "content": `+value+`}`+"\n")
		}, parseFile(t, rel, `{"id": "s"}`+"\n"+`{"role": "user"}`+"\n"))
	t.Run("a string and an empty array", func(t *testing.T) {
		for _, value := range []string{`"hello"`, `[]`} {
			rows := parseFile(t, rel, `{"id": "s"}`+"\n"+`{"role": "user", "content": `+value+`}`+"\n")
			if got := count(rows, problem); got != 0 {
				t.Errorf("content %s gave %d problem rows", value, got)
			}
		}
	})
}

func TestARequestLogMemberOfTheWrongKindIsReported(t *testing.T) {
	const rel = ".local/state/goose/logs/llm_request.0.jsonl"
	chunk := func(content string) string {
		return `{"input": {"messages": []}}` + "\n" + `{"data": {"id": "m", "role": "assistant", "content": ` + content + `}}` + "\n"
	}
	t.Run("data.content", func(t *testing.T) {
		checkMember(t, kindTests(`{"type": "text"}`, true), "parser: "+parser.WrongKind("data.content", jsonvalue.Array),
			func(value string) []string { return parseFile(t, rel, chunk(value)) },
			parseFile(t, rel, `{"input": {"messages": []}}`+"\n"+`{"data": {"id": "m"}}`+"\n"))
	})
	t.Run("input.messages", func(t *testing.T) {
		checkMember(t, kindTests(`{"role": "user"}`, false), "parser: "+parser.WrongKind("input.messages", jsonvalue.Array),
			func(value string) []string { return parseFile(t, rel, `{"input": {"messages": `+value+`}}`+"\n") },
			parseFile(t, rel, `{"input": {}}`+"\n"))
	})
	t.Run("input", func(t *testing.T) {
		checkMember(t, kindTests(`[1]`, false), "parser: "+parser.WrongKind("input", jsonvalue.Object),
			func(value string) []string {
				return parseFile(t, rel, `{"model_config": {"model_name": "m"}, "input": `+value+`}`+"\n")
			}, parseFile(t, rel, `{"model_config": {"model_name": "m"}}`+"\n"))
	})
	t.Run("data", func(t *testing.T) {
		checkMember(t, kindTests(`[1]`, false), "parser: "+parser.WrongKind("data", jsonvalue.Object),
			func(value string) []string {
				return parseFile(t, rel, `{"input": {}}`+"\n"+`{"data": `+value+`}`+"\n")
			}, parseFile(t, rel, `{"input": {}}`+"\n"+`{"usage": {}}`+"\n"))
	})
}

func TestContentBlocksThatAreNotObjectsAreReportedOncePerMessage(t *testing.T) {
	tests := []struct {
		name, rel, text, want string
	}{
		{"a legacy message", ".local/share/goose/sessions/s.jsonl",
			`{"id": "s"}` + "\n" + `{"role": "user", "content": [{"type": "text", "text": "hi"}, 7, "x"]}` + "\n",
			"parser: content holds 2 entries that are not JSON objects"},
		{"a request log chunk", ".local/state/goose/logs/llm_request.0.jsonl",
			`{"input": {"messages": []}}` + "\n" + `{"data": {"id": "m", "role": "assistant", "content": [7]}}` + "\n",
			"parser: data.content holds 1 entry that is not a JSON object"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := count(parseFile(t, test.rel, test.text), test.want); got != 1 {
				t.Errorf("%q given %d times, want once", test.want, got)
			}
		})
	}
	t.Run("a sessions.db message", func(t *testing.T) {
		rows := parseDatabase(t, "block_kinds.db")
		if want := "parser: content holds 1 entry that is not a JSON object"; count(rows, want) != 1 {
			t.Errorf("rows %q, want %q once", rows, want)
		}
	})
}
