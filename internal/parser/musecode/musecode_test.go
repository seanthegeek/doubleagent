package musecode

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// The rows of the logs, the history and trust.json are proved against the
// Python analyzer by the parser replay (internal/parity/replay); these
// tests port the helpers the Python tests call directly, with the values
// the analyzer gives them.

// decoded returns the value of JSON text.
func decoded(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	value, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestMicrosToUTCReadsNumbersAndASCIIDigitTextOnly(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{"digit text", `"1000000"`, "1970-01-01T00:00:01.000Z"},
		{"an Arabic-Indic digit", "\"١000000\"", ""},
		{"null", `null`, ""},
		{"a boolean", `true`, ""},
		{"an integer", `1790000000000000`, "2026-09-21T14:13:20.000Z"},
		{"a float", `1500000.0`, "1970-01-01T00:00:01.500Z"},
		{"empty text", `""`, ""},
		{"words", `"abc"`, ""},
		{"zero", `0`, "1970-01-01T00:00:00.000Z"},
		{"text with a dot", `"12.5"`, ""},
		{"a negative integer", `-5`, "1969-12-31T23:59:59.999Z"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := microsToUTC(decoded(t, test.json)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestArgumentsSummaryPicksTheFirstKnownArgument(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{"a JSON text with a url", `"{\"url\": \"https://example.org\"}"`, "https://example.org"},
		{"an object with a cmd", `{"cmd":"make"}`, "make"},
		{"text that is not JSON", `"not json"`, "not json"},
		{"no known argument is compact JSON in key order", `"{\"x\":1,\"b\":[2]}"`, `{"x":1,"b":[2]}`},
		{"an empty command is skipped", `"{\"command\":\"\"}"`, `{"command":""}`},
		{"a zero command is kept", `"{\"command\":0}"`, "0"},
		{"JSON text of a number", `"5"`, "5"},
		{"JSON text of null", `"null"`, ""},
		{"JSON text of a string", `"\"q\""`, "q"},
		{"null", `null`, ""},
		{"a float", `3.5`, "3.5"},
		{"an array", `[1,"a"]`, `[1,"a"]`},
		{"an object argument is JSON", `{"path":{"k":1}}`, `{"k":1}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := argumentsSummary(decoded(t, test.json)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestResultTextUnwrapsABashResult(t *testing.T) {
	tests := []struct {
		name string
		json string
		want string
	}{
		{"exit zero", `"{\"output\": \"ok\", \"exit_code\": 0}"`, "ok"},
		{"plain text", `"tool failed: 403"`, "tool failed: 403"},
		{"a failing exit", `"{\"output\":\"o\",\"exit_code\":2}"`, "[exit 2] o"},
		{"exit false", `"{\"output\":\"o\",\"exit_code\":false}"`, "o"},
		{"exit true", `"{\"output\":\"o\",\"exit_code\":true}"`, "[exit true] o"},
		{"exit text", `"{\"output\":\"o\",\"exit_code\":\"x\"}"`, "[exit x] o"},
		{"exit zero as a float", `"{\"output\":\"o\",\"exit_code\":0.0}"`, "o"},
		{"output that is an object", `"{\"output\":{\"a\":1}}"`, `{"a":1}`},
		{"an object without output", `"{\"nope\":1}"`, `{"nope":1}`},
		{"broken JSON", `"{bad"`, "{bad"},
		{"a number", `5`, "5"},
		{"null", `null`, ""},
		{"an object", `{"a":1}`, `{"a":1}`},
		{"an array", `[1]`, "[1]"},
		{"null output and exit", `"{\"output\":null,\"exit_code\":null}"`, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := resultText(decoded(t, test.json)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestWantsAndSecondaryNameTheLogsHistoryAndTrust(t *testing.T) {
	const logs = ".local/share/muse/sessions/2026/10/01/abc/"
	tests := []struct {
		rel       string
		wants     bool
		secondary bool
	}{
		{logs + "session.jsonl", true, false},
		{logs + "subagent/child/session.jsonl", true, false},
		{logs + "subagent/child/subagent/grand/session.jsonl", true, false},
		{logs + "other.jsonl", false, false},
		{".local/share/muse/sessions/26/10/01/abc/session.jsonl", false, false},
		{".local/share/muse/tui-history.jsonl", true, true},
		{".config/muse/trust.json", true, false},
		{"x/.config/muse/trust.json", false, false},
	}
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			artifact := &input.Artifact{Rel: test.rel}
			subject := New()
			if subject.Wants(artifact) != test.wants || subject.Secondary(artifact) != test.secondary {
				t.Errorf("wants %v secondary %v", subject.Wants(artifact), subject.Secondary(artifact))
			}
		})
	}
}

func TestFrameChildrenAreReadInChildOrder(t *testing.T) {
	log := `{"retained_frame":1,"children":[` +
		`{"child_index":2,"record_json":"{\"payload_type\":\"b\"}"},` +
		`{"child_index":1,"record_json":"{\"payload_type\":\"a\"}"},` +
		`{"record_json":"{\"payload_type\":\"zero\"}"},` +
		`{"child_index":3,"record_json":"{bad"},` +
		`{"child_index":4,"record_json":5},"skipped"]}` + "\n" +
		`{"retained_marker":1}` + "\n" +
		`{"stream":{"id":"s"},"payload_type":"c"}` + "\n"
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := osfile.WriteFile(path, []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}
	var problems parser.Problems
	var kinds []string
	for number, record := range records(path, &problems) {
		kinds = append(kinds, record.Get("payload_type").StringValue())
		if number != 1 && number != 3 {
			t.Errorf("record on line %d", number)
		}
	}
	if want := []string{"zero", "a", "b", "c"}; !slices.Equal(kinds, want) {
		t.Errorf("order %q, want %q", kinds, want)
	}
	if len(problems) != 2 || problems[0].Line != 1 || problems[0].Raw != "{bad" {
		t.Errorf("problems %+v", problems)
	}
}

func TestALogThatStartsWithAnotherRecordIsNotMuse(t *testing.T) {
	path := filepath.Join(t.TempDir(), "session.jsonl")
	if err := osfile.WriteFile(path, []byte("{\"a\":1}\n{\"stream\":{},\"payload_type\":\"x\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var problems parser.Problems
	for range records(path, &problems) {
		t.Fatal("a record was yielded")
	}
}
