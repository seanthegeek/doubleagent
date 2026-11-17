package vscode

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// The expected values in this file come from running the Python helpers
// of the same names on the same inputs.

// startState is the state each mutation case starts from.
const startState = `{"a":[1,2,3],"b":{"c":1},"s":"xyz","n":null}`

// decoded returns text decoded as JSON, failing the test when it is not.
func decoded(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	value, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatalf("decode %s: %v", text, err)
	}
	return value
}

func TestApplyMutationChangesTheStateAsPythonDoes(t *testing.T) {
	for _, test := range []struct {
		name, records, failure, want string
	}{
		{"the test's sequence",
			`[{"kind":0,"v":{"a":[1,2,3],"b":{"c":1}}},{"kind":2,"k":["a"],"v":[9],"i":1},{"kind":1,"k":["b","c"],"v":2},{"kind":3,"k":["b"]},{"kind":2,"k":["new"],"v":[1]}]`,
			"", `{"a":[1,9],"new":[1]}`},
		{"a missing parent",
			`[{"kind":1,"k":["missing","x"],"v":1}]`,
			`path ["missing"] not found`, ""},
		{"a true kind sets",
			`[{"kind":true,"k":["b","c"],"v":5}]`,
			"", `{"a":[1,2,3],"b":{"c":5},"s":"xyz","n":null}`},
		{"a float kind pushes",
			`[{"kind":2.0,"k":["a"],"v":4}]`,
			"", `{"a":[1,2,3,4],"b":{"c":1},"s":"xyz","n":null}`},
		{"a negative index sets nothing",
			`[{"kind":1,"k":["a",-1],"v":7}]`,
			"index -1 out of range", ""},
		{"a negative index walks",
			`[{"kind":1,"k":["a",-1,"x"],"v":7}]`,
			`cannot set "x"`, ""},
		{"an index one past the end appends",
			`[{"kind":1,"k":["a",3],"v":7}]`,
			"", `{"a":[1,2,3,7],"b":{"c":1},"s":"xyz","n":null}`},
		{"an index beyond the end",
			`[{"kind":1,"k":["a",9],"v":7}]`,
			"index 9 out of range", ""},
		{"a true index",
			`[{"kind":1,"k":["a",true],"v":7}]`,
			"", `{"a":[1,7,3],"b":{"c":1},"s":"xyz","n":null}`},
		{"a true truncation",
			`[{"kind":2,"k":["a"],"i":true,"v":[8]}]`,
			"", `{"a":[1,8],"b":{"c":1},"s":"xyz","n":null}`},
		{"a negative truncation",
			`[{"kind":2,"k":["a"],"i":-1,"v":[8]}]`,
			"", `{"a":[1,2,8],"b":{"c":1},"s":"xyz","n":null}`},
		{"a delete by negative index",
			`[{"kind":3,"k":["a",-3]}]`,
			"", `{"a":[2,3],"b":{"c":1},"s":"xyz","n":null}`},
		{"a delete of nothing",
			`[{"kind":3,"k":["zz"]}]`,
			"", `{"a":[1,2,3],"b":{"c":1},"s":"xyz","n":null}`},
		{"a push to null",
			`[{"kind":2,"k":["n"],"v":null}]`,
			"", `{"a":[1,2,3],"b":{"c":1},"s":"xyz","n":[null]}`},
		{"a push to an object",
			`[{"kind":2,"k":["b"],"v":1}]`,
			`push to non-list at ["b"]`, ""},
		{"a push to the root",
			`[{"kind":2,"k":[],"v":1}]`,
			"push to non-list at []", ""},
		{"a push into a string",
			`[{"kind":2,"k":["s",0],"v":1}]`,
			`push to non-list at ["s",0]`, ""},
		{"a set into a string",
			`[{"kind":1,"k":["s",0],"v":1}]`,
			"cannot set 0", ""},
		{"a walk through a string",
			`[{"kind":1,"k":["s",0,0],"v":1}]`,
			"cannot set 0", ""},
		{"an empty path",
			`[{"kind":1,"k":[],"v":1}]`,
			"bad path []", ""},
		{"no path",
			`[{"kind":1,"v":1}]`,
			"bad path null", ""},
		{"an unknown kind",
			`[{"kind":7,"k":["a"]}]`,
			"unknown kind 7", ""},
		{"no kind",
			`[{"k":["a"]}]`,
			"unknown kind null", ""},
		{"a string kind",
			`[{"kind":"1","k":["a"]}]`,
			`unknown kind "1"`, ""},
		{"a set on a list by name",
			`[{"kind":1,"k":["a","x"],"v":1}]`,
			`cannot set "x"`, ""},
		{"a set on null",
			`[{"kind":1,"k":["n","x"],"v":1}]`,
			`cannot set "x"`, ""},
		{"a big index",
			`[{"kind":1,"k":["a",100000000000000000000000],"v":1}]`,
			"index 100000000000000000000000 out of range", ""},
		{"a big truncation",
			`[{"kind":2,"k":["a"],"i":-100000000000000000000000,"v":[1]}]`,
			"", `{"a":[1],"b":{"c":1},"s":"xyz","n":null}`},
		{"a float index",
			`[{"kind":1,"k":["a",1.0],"v":1}]`,
			"cannot set 1.0", ""},
		{"kind zero without a value",
			`[{"kind":0}]`,
			"", `null`},
		{"a false kind replaces",
			`[{"kind":false,"v":[1]}]`,
			"", `[1]`},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := newNode(decoded(t, startState))
			var err error
			for _, record := range decoded(t, test.records).Items() {
				if state, err = applyMutation(state, record); err != nil {
					break
				}
			}
			switch {
			case test.failure != "" && (err == nil || err.Error() != test.failure):
				t.Errorf("applyMutation error = %v, want %q", err, test.failure)
			case test.failure == "" && err != nil:
				t.Errorf("applyMutation error = %v", err)
			case test.failure == "" && state.value().JSON() != test.want:
				t.Errorf("state = %s, want %s", state.value().JSON(), test.want)
			}
		})
	}
}

func TestConfirmedTextReadsEachForm(t *testing.T) {
	for _, test := range []struct{ name, part, want string }{
		{"a boolean", `{"isConfirmed": false}`, "permission decision: confirmed=false"},
		{"a denial with its source", `{"isConfirmed": {"type": 0, "source": "user"}, "source": {"type": "mcp"}}`,
			"permission decision: confirmed=Denied(0) source=user tool_source=mcp"},
		{"a reason", `{"isConfirmed": {"type": 1, "reason": {"value": "read only"}}}`,
			"permission decision: confirmed=ConfirmationNotNeeded(1) reason=read only"},
		{"a boolean type is not an enum value", `{"isConfirmed": {"type": true}}`,
			"permission decision: confirmed=true"},
		{"a type out of range", `{"isConfirmed": {"type": 6}}`, "permission decision: confirmed=6"},
		{"null", `{"isConfirmed": null}`, ""},
		{"absent", `{}`, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := confirmedText(decoded(t, test.part)); got != test.want {
				t.Errorf("confirmedText = %q, want %q", got, test.want)
			}
		})
	}
}

func FuzzApplyMutation(fuzz *testing.F) {
	fuzz.Add(`[{"kind":0,"v":{"requests":[]}},{"kind":2,"k":["requests"],"v":[{"a":1}]},{"kind":1,"k":["requests",0,"a"],"v":2}]`)
	fuzz.Fuzz(func(t *testing.T, text string) {
		records, err := jsonvalue.DecodeString(text)
		if err != nil {
			return
		}
		var state *node
		lines := requestLines{}
		for i, record := range records.Items() {
			if next, err := applyMutation(state, record); err == nil {
				state = next
				track(record, state, i, lines)
			}
		}
		_ = state.value()
	})
}
