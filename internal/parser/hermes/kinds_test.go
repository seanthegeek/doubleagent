package hermes

import (
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// kindCases are the values a member that must be an array or an object
// can hold: the wrong kinds (reported) and the silent ones. The wrong
// column is the problem text, or empty when the case must stay silent.
func kindCases(member string, otherKind string) []struct{ name, json, wrong string } {
	wrong := parser.WrongKind(member, jsonvalue.Array)
	return []struct{ name, json, wrong string }{
		{"a string", `"oops"`, wrong},
		{"a number", `7`, wrong},
		{"an object", otherKind, wrong},
		{"null", `null`, ""},
		{"an empty array", `[]`, ""},
	}
}

func TestAQuarantinedListThatIsNotAnArrayIsReported(t *testing.T) {
	const rel = ".config/Hermes/connections.json"
	const wrong = "parser: quarantined is not a JSON array"
	for _, test := range kindCases("quarantined", `{"reason": "x"}`) {
		t.Run(test.name, func(t *testing.T) {
			got := parseFile(t, rel, `{"connections": [], "quarantined": `+test.json+`}`)
			want := []string(nil)
			if test.wrong != "" {
				want = []string{wrong}
			}
			if !slices.Equal(got, want) {
				t.Errorf("rows %q, want %q", got, want)
			}
		})
	}
	t.Run("absent", func(t *testing.T) {
		if got := parseFile(t, rel, `{"connections": []}`); len(got) != 0 {
			t.Errorf("rows %q, want none", got)
		}
	})
}

// responseProblems runs responseRows on a stored response holding data and
// returns the problems it adds.
func responseProblems(t *testing.T, data string) parser.Problems {
	t.Helper()
	value, err := jsonvalue.DecodeString(data)
	if err != nil {
		t.Fatal(err)
	}
	artifact := &input.Artifact{Rel: ".hermes/response_store.db", Original: "/h/.hermes/response_store.db",
		Manifest: jsonvalue.NewNull()}
	reading := New()
	reader := reading.newMessageReader(artifact, parser.Options{}, nil, func(timeline.Row) {})
	var problems parser.Problems
	reading.responseRows(artifact, storedResponse{rowid: 5, responseID: "r", data: value}, nil,
		map[string]map[string]bool{}, reader, &problems)
	return problems
}

func TestAConversationHistoryThatIsNotAnArrayIsReported(t *testing.T) {
	for _, test := range kindCases("conversation_history", `{"role": "user"}`) {
		t.Run(test.name, func(t *testing.T) {
			problems := responseProblems(t, `{"session_id": "s", "conversation_history": `+test.json+`}`)
			want := parser.Problems(nil)
			if test.wrong != "" {
				want = parser.Problems{{Line: 5, Message: test.wrong}}
			}
			if !slices.Equal(problems, want) {
				t.Errorf("problems %v, want %v", problems, want)
			}
		})
	}
	t.Run("absent", func(t *testing.T) {
		if problems := responseProblems(t, `{"session_id": "s"}`); len(problems) != 0 {
			t.Errorf("problems %v, want none", problems)
		}
	})
}
