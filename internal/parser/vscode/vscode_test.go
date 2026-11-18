package vscode

import (
	"fmt"
	"maps"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// The mutation log's rules are in research/vscode.md: kind 0 replaces the
// state, 1 sets the value at a path, 2 pushes onto the array at a path and
// 3 deletes the value at a path.

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

func TestApplyMutationChangesTheStateOrSaysWhyNot(t *testing.T) {
	for _, test := range []struct {
		name, records, failure, want string
	}{
		{"the test's sequence",
			`[{"kind":0,"v":{"a":[1,2,3],"b":{"c":1}}},{"kind":2,"k":["a"],"v":[9],"i":1},{"kind":1,"k":["b","c"],"v":2},{"kind":3,"k":["b"]},{"kind":2,"k":["new"],"v":[1]}]`,
			"", `{"a":[1,9],"new":[1]}`},
		{"a missing parent",
			`[{"kind":1,"k":["missing","x"],"v":1}]`,
			`path ["missing"] not found`, ""},
		{"a boolean kind is no kind",
			`[{"kind":true,"k":["b","c"],"v":5}]`,
			"unknown kind true", ""},
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
		{"a boolean index is not an index",
			`[{"kind":1,"k":["a",true],"v":7}]`,
			"cannot set true", ""},
		{"a boolean truncation truncates nothing",
			`[{"kind":2,"k":["a"],"i":true,"v":[8]}]`,
			"", `{"a":[1,2,3,8],"b":{"c":1},"s":"xyz","n":null}`},
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
			"cannot set 0", ""},
		{"a set into a string",
			`[{"kind":1,"k":["s",0],"v":1}]`,
			"cannot set 0", ""},
		{"a string is a leaf a walk cannot pass",
			`[{"kind":1,"k":["s",0,0],"v":1}]`,
			`path ["s",0] not found`, ""},
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
		{"a false kind is not kind zero",
			`[{"kind":false,"v":[1]}]`,
			"bad path null", ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := newNode(decoded(t, startState))
			var err error
			for _, record := range decoded(t, test.records).Items() {
				if state, err = applyMutation(state, record, parser.Work{}); err != nil {
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

func TestTrackGivesEachRequestTheLineThatMadeIt(t *testing.T) {
	for _, test := range []struct {
		name string
		log  []string
		want requestLines
	}{
		{"an array pushes each item", []string{`{"kind":0,"v":{"requests":[{}]}}`,
			`{"kind":2,"k":["requests"],"v":[{},{}]}`},
			requestLines{sessionStart: 1, 0: 1, 1: 2, 2: 2}},
		{"an object pushes itself", []string{`{"kind":0,"v":{"requests":[{}]}}`,
			`{"kind":2,"k":["requests"],"v":{"a":1,"b":2}}`},
			requestLines{sessionStart: 1, 0: 1, 1: 2}},
		{"a string pushes itself", []string{`{"kind":0,"v":{"requests":[]}}`,
			`{"kind":2,"k":["requests"],"v":"abc"}`},
			requestLines{sessionStart: 1, 0: 2}},
		{"a push to an absent list makes it", []string{`{"kind":0,"v":{}}`,
			`{"kind":2,"k":["requests"],"v":[{}]}`},
			requestLines{sessionStart: 1, 0: 2}},
		{"a truncation replaces the requests after it", []string{`{"kind":0,"v":{"requests":[{},{}]}}`,
			`{"kind":2,"k":["requests"],"i":1,"v":[{}]}`},
			requestLines{sessionStart: 1, 0: 1, 1: 2}},
		{"a truncation to zero replaces every request", []string{`{"kind":0,"v":{"requests":[{},{}]}}`,
			`{"kind":2,"k":["requests"],"i":0,"v":[{}]}`},
			requestLines{sessionStart: 1, 0: 2}},
		{"a negative truncation counts from the end", []string{`{"kind":0,"v":{"requests":[{},{},{}]}}`,
			`{"kind":2,"k":["requests"],"i":-1,"v":[{}]}`},
			requestLines{sessionStart: 1, 0: 1, 1: 1, 2: 2}},
		{"a negative truncation past the start replaces every request", []string{
			`{"kind":0,"v":{"requests":[{},{}]}}`, `{"kind":2,"k":["requests"],"i":-5,"v":[{}]}`},
			requestLines{sessionStart: 1, 0: 2}},
		{"a truncation beyond the end keeps every request", []string{`{"kind":0,"v":{"requests":[{}]}}`,
			`{"kind":2,"k":["requests"],"i":5,"v":[{}]}`},
			requestLines{sessionStart: 1, 0: 1, 1: 2}},
		{"a truncation beyond int64 keeps every request", []string{`{"kind":0,"v":{"requests":[{}]}}`,
			`{"kind":2,"k":["requests"],"i":99999999999999999999999,"v":[{}]}`},
			requestLines{sessionStart: 1, 0: 1, 1: 2}},
		{"a truncation that is not an integer is no truncation", []string{
			`{"kind":0,"v":{"requests":[{},{}]}}`, `{"kind":2,"k":["requests"],"i":1.0,"v":[{}]}`},
			requestLines{sessionStart: 1, 0: 1, 1: 1, 2: 2}},
		{"a set by index takes that request", []string{`{"kind":0,"v":{"requests":[{},{}]}}`,
			`{"kind":1,"k":["requests",1],"v":{}}`},
			requestLines{sessionStart: 1, 0: 1, 1: 2}},
		{"a set at the length adds a request", []string{`{"kind":0,"v":{"requests":[{}]}}`,
			`{"kind":1,"k":["requests",1],"v":{}}`},
			requestLines{sessionStart: 1, 0: 1, 1: 2}},
		{"a set at a negative index is not applied", []string{`{"kind":0,"v":{"requests":[{}]}}`,
			`{"kind":1,"k":["requests",-1],"v":{}}`},
			requestLines{sessionStart: 1, 0: 1}},
		{"a set of the whole list replaces every request", []string{`{"kind":0,"v":{"requests":[{},{}]}}`,
			`{"kind":1,"k":["requests"],"v":[{}]}`},
			requestLines{sessionStart: 1, 0: 2}},
		{"a delete of the list forgets every request", []string{`{"kind":0,"v":{"requests":[{}]}}`,
			`{"kind":3,"k":["requests"]}`},
			requestLines{sessionStart: 1}},
		{"a delete moves the later requests down", []string{`{"kind":0,"v":{"requests":[]}}`,
			`{"kind":2,"k":["requests"],"v":[{}]}`, `{"kind":2,"k":["requests"],"v":[{}]}`,
			`{"kind":2,"k":["requests"],"v":[{}]}`, `{"kind":3,"k":["requests",0]}`},
			requestLines{sessionStart: 1, 0: 3, 1: 4}},
		{"a negative delete counts from the end", []string{`{"kind":0,"v":{"requests":[]}}`,
			`{"kind":2,"k":["requests"],"v":[{}]}`, `{"kind":2,"k":["requests"],"v":[{}]}`,
			`{"kind":2,"k":["requests"],"v":[{}]}`, `{"kind":3,"k":["requests",-2]}`},
			requestLines{sessionStart: 1, 0: 2, 1: 4}},
		{"a delete out of range changes nothing", []string{`{"kind":0,"v":{"requests":[{}]}}`,
			`{"kind":3,"k":["requests",1]}`, `{"kind":3,"k":["requests",-2]}`},
			requestLines{sessionStart: 1, 0: 1}},
		{"a change inside a request keeps its line", []string{`{"kind":0,"v":{"requests":[{"a":[]}]}}`,
			`{"kind":1,"k":["requests",0,"b"],"v":1}`, `{"kind":2,"k":["requests",0,"a"],"v":[1]}`,
			`{"kind":3,"k":["requests",0,"b"]}`},
			requestLines{sessionStart: 1, 0: 1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "session.jsonl")
			if err := osfile.WriteFile(path, []byte(strings.Join(test.log, "\n")+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			lines := requestLines{}
			var problems parser.Problems
			state := replay(path, lines, &problems, parser.Work{})
			if !maps.Equal(lines, test.want) {
				t.Errorf("lines = %v, want %v (requests %s)", lines, test.want, state.Get("requests").JSON())
			}
			if got := int64(state.Get("requests").Len()); len(lines) != int(got)+1 {
				t.Errorf("%d request lines for %d requests", len(lines)-1, got)
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
	fuzz.Add(`[{"kind":0,"v":{"requests":[{},{},{}]}},{"kind":2,"k":["requests"],"i":-1,"v":[{}]},{"kind":3,"k":["requests",0]}]`)
	fuzz.Fuzz(func(t *testing.T, text string) {
		records, err := jsonvalue.DecodeString(text)
		if err != nil {
			return
		}
		var state *node
		lines := requestLines{}
		for i, record := range records.Items() {
			before := requestCount(state)
			next, err := applyMutation(state, record, parser.Work{})
			if err != nil {
				continue
			}
			state = next
			after := requestCount(state)
			track(record, before, after, i, lines, parser.Work{})
			// Every request has a line and no line outlives its request.
			if len(lines) != int(after)+1 {
				t.Fatalf("record %d: %d request lines for %d requests: %v", i, len(lines)-1, after, lines)
			}
			for position := range after {
				if _, ok := lines[position]; !ok {
					t.Fatalf("record %d: request %d has no line: %v", i, position, lines)
				}
			}
		}
		_ = state.value()
	})
}

// A URI's path keeps the slashes FromFileURI gives, a network share's
// server included; other text is kept as it is.
func TestURIToPathReadsFileURIsAndKeepsOtherText(t *testing.T) {
	for _, test := range []struct{ name, uri, want string }{
		{"a POSIX path", "file:///home/u/proj", "/home/u/proj"},
		{"a drive path", "file:///c%3A/Users/u/proj", "c:/Users/u/proj"},
		{"a network share keeps its server", "file://fileserver/share/proj", "//fileserver/share/proj"},
		{"a server alone names no folder", "file://fileserver", ""},
		{"localhost is this machine", "file://localhost/home/u", "/home/u"},
		{"a plain path is kept", "/home/u/proj", "/home/u/proj"},
		{"a URI net/url cannot read is kept", "file:///a%zz", "file:///a%zz"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := uriToPath(test.uri); got != test.want {
				t.Errorf("uriToPath(%q) = %q, want %q", test.uri, got, test.want)
			}
		})
	}
}

// growthLog returns a mutation log that pushes n requests one at a time,
// then sets n keys of one object and deletes them in the order set.
func growthLog(n int) string {
	var log strings.Builder
	log.WriteString(`{"kind":0,"v":{"requests":[],"settings":{}}}` + "\n")
	for range n {
		log.WriteString(`{"kind":2,"k":["requests"],"v":[{}]}` + "\n")
	}
	for i := range n {
		fmt.Fprintf(&log, `{"kind":1,"k":["settings","key%d"],"v":%d}`+"\n", i, i)
	}
	for i := range n {
		fmt.Fprintf(&log, `{"kind":3,"k":["settings","key%d"]}`+"\n", i)
	}
	return log.String()
}

func TestReplayWorkGrowsWithTheLogNotItsSquare(t *testing.T) {
	for _, n := range []int{100, 1000} {
		t.Run(fmt.Sprintf("%d pushes and %d deleted keys", n, n), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "session.jsonl")
			if err := osfile.WriteFile(path, []byte(growthLog(n)), 0o600); err != nil {
				t.Fatal(err)
			}
			work := parser.CountingWork()
			lines := requestLines{}
			var problems parser.Problems
			state := replay(path, lines, &problems, work)
			if state.Get("requests").Len() != n || state.Get("settings").Len() != 0 || len(problems) != 0 {
				t.Fatalf("replay gave %s with problems %v", state.JSON(), problems)
			}
			// Each push sets its one request's line and each delete
			// drops its one key.
			if got := work.Count("entries moved"); got != 2*n {
				t.Errorf("moved %d entries, want %d", got, 2*n)
			}
		})
	}
}

func TestObjectKeysKeepTheOrderAListOfKeysWouldKeep(t *testing.T) {
	for seed := range uint64(200) {
		t.Run(fmt.Sprintf("seed %d", seed), func(t *testing.T) {
			source := rand.New(rand.NewPCG(seed, 527))
			state := newNode(decoded(t, `{"o":{"b":0,"a":0}}`))
			want := []string{"b", "a"}
			for step := range 40 {
				key := string("abcd"[source.IntN(4)])
				record := fmt.Sprintf(`{"kind":1,"k":["o",%q],"v":%d}`, key, step)
				if source.IntN(2) == 0 {
					record = fmt.Sprintf(`{"kind":3,"k":["o",%q]}`, key)
					want = slices.DeleteFunc(want, func(each string) bool { return each == key })
				} else if !slices.Contains(want, key) {
					want = append(want, key)
				}
				if _, err := applyMutation(state, decoded(t, record), parser.Work{}); err != nil {
					t.Fatal(err)
				}
				if got := state.value().Get("o").Keys(); !slices.Equal(got, want) {
					t.Fatalf("after %s: keys %q, want %q", record, got, want)
				}
			}
		})
	}
}
