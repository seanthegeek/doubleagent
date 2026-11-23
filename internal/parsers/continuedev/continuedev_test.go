package continuedev

import (
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

func TestYAMLScalarReadsOnlyAScalarOnItsLine(t *testing.T) {
	for _, test := range []struct {
		name, text, want string
		ok               bool
	}{
		{"a mapping value after a tab is no scalar", "Bash:\tls", "", false},
		{"a colon before a no-break space is kept", "Bash(ls:\u00a0)", "Bash(ls:\u00a0)", true},
		{"a double-quoted item is decoded", `"a\"b" # c`, `a"b`, true},
		{"a single-quoted item halves its quotes", "'it''s'", "it's", true},
		{"a plain item loses its comment", "plain # note", "plain", true},
		{"a mapping is no scalar", "a: b", "", false},
		{"a key alone is no scalar", "a:", "", false},
		{"a flow sequence is no scalar", "[x]", "", false},
		{"nothing is no scalar", "", "", false},
		{"a nested sequence is no scalar", "-x", "", false},
		{"a hash inside a word stays", "x#y", "x#y", true},
		{"an unclosed double quote is no scalar", `"unclosed`, "", false},
		{"an unclosed single quote is no scalar", "'bad", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := yamlScalar(test.text)
			if got != test.want || ok != test.ok {
				t.Errorf("yamlScalar = %q, %v; want %q, %v", got, ok, test.want, test.ok)
			}
		})
	}
}

func TestReadPermissionsReadsTheFlatShapeTheCLIWrites(t *testing.T) {
	for _, test := range []struct {
		name, text string
		want       []rule
		problem    string
	}{
		{"an item under an emptied list", "allow:\n  - Bash(ls)\nask: []\n  - x\n", nil,
			"unsupported YAML at line 4"},
		{"a byte-order mark, comments and a document marker", "\ufeffallow: # c\n  - 'a'\n---\n# c\nexclude: ~\n",
			[]rule{{2, "allow", "a"}}, ""},
		{"another key", "other:\n - a\n", nil, "unsupported YAML at line 1"},
		{"spaces around an item", "allow:\n  -   spaced   \n", []rule{{2, "allow", "spaced"}}, ""},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := readPermissions(test.text)
			if !slices.Equal(got, test.want) || errorText(err) != test.problem {
				t.Errorf("readPermissions = %v, %v; want %v, %q", got, err, test.want, test.problem)
			}
		})
	}
}

func TestTheMetadataPatternsTakeAWholeStatusWordAndTheLastURL(t *testing.T) {
	for _, test := range []struct {
		name, line string
		pattern    string
		want       []string
	}{
		{"a success", "[metadata] Success https://x/agents/a/metadata", "end",
			[]string{"Success", "https://x/agents/a/metadata"}},
		{"a longer word is no status", "[metadata] Successful https://x/metadata", "end", nil},
		{"a failure with its response", `[metadata] Failed 500 https://x/a/metadata {"e":1}`, "end",
			[]string{"Failed", "https://x/a/metadata"}},
		{"a letter outside ASCII continues the word", "[metadata] Error\u00e9 https://x/metadata", "end", nil},
		{"the last URL is taken", "[metadata] Error: boom https://y/metadata https://x/metadata", "end",
			[]string{"Error", "https://x/metadata"}},
		{"a no-break space is no separator", "[metadata] Error\u00a0https://x/metadata", "end", nil},
		{"a post with its fields", `[metadata] POST https://x/metadata {"a":1}`, "post",
			[]string{"https://x/metadata", `{"a":1}`}},
		{"a no-break space ends no URL", "[metadata] POST https://x/m\u00a0eta", "post", nil},
		{"fields cut short", "[metadata] POST u {bad", "post", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			pattern := metadataEnd
			if test.pattern == "post" {
				pattern = metadataPost
			}
			var got []string
			if match := pattern.FindStringSubmatch(test.line); match != nil {
				got = match[1:]
			}
			if !slices.Equal(got, test.want) {
				t.Errorf("groups = %q, want %q", got, test.want)
			}
		})
	}
}

func TestTheLogLinePatternTakesASCIIDigitsOnly(t *testing.T) {
	line := "2026-10-01 08:00:00.123 [0123abcd] [info]: x"
	if !logLine.MatchString(line) {
		t.Errorf("%q does not match", line)
	}
	if other := strings.Replace(line, "3", "\u0663", 1); logLine.MatchString(other) {
		t.Errorf("%q matches", other)
	}
}

// errorText returns the text of err, "" for nil.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestLoadSessionRecoversWhatItCan(t *testing.T) {
	for _, test := range []struct {
		name, text, want, problem string
	}{
		{"trailing data", "{\"sessionId\": \"x\", \"history\": []}\n{\"junk\"", `{"sessionId":"x","history":[]}`,
			"trailing data after the session object at offset 33"},
		{"JSON whitespace around the colons", "{\"sessionId\" :\n\"s1\", \"history\"\r\n:\t[{\"a\": 1}, {\"b",
			`{"sessionId":"s1","history":[{"a":1}]}`, "session file did not parse"},
		{"a list", "[1]", `{}`, "session file is not a JSON object"},
		{"a trailing data offset counts characters", "{\"t\":\"\u00e9\"} x", `{"t":"é"}`,
			"trailing data after the session object at offset 9"},
		{"objects only, offsets counted by character", "x {\"title\": \"\u00e9\", \"history\": [{\"a\":1}, 2, {\"b\":2}]",
			`{"title":"é","history":[{"a":1},{"b":2}]}`, "session file did not parse"},
	} {
		t.Run(test.name, func(t *testing.T) {
			session, problem := LoadSession(nil, test.text)
			if got := session.JSON(); got != test.want {
				t.Errorf("session = %s, want %s", got, test.want)
			}
			if got, _, _ := strings.Cut(errorText(problem), " ("); got != test.problem {
				t.Errorf("problem = %v, want %q", problem, test.problem)
			}
		})
	}
}

func TestLoadSessionTakesJSONWhitespaceOnly(t *testing.T) {
	for _, space := range []string{"\u00a0", "\u2028", "\u0085"} {
		text := "{\"sessionId\"" + space + ":\"s1\", \"history\"" + space + ":[{\"a\": 1}, {\"b"
		session, _ := LoadSession(nil, text)
		if got := session.JSON(); got != `{"history":[]}` {
			t.Errorf("%q: session = %s", space, got)
		}
	}
}

func TestOutputTextAndStateDecision(t *testing.T) {
	output, err := jsonvalue.DecodeString(`[{"content": "a"}, {"content": {"b": 1}}, {"name": "n"}, {}, 5, null]`)
	if err != nil {
		t.Fatal(err)
	}
	if got := outputText(output); got != "a\n{\"b\":1}\nn\n5" {
		t.Errorf("outputText = %q", got)
	}
	violation, _ := jsonvalue.DecodeString(`[{"name": "Security Policy Violation"}]`)
	for _, test := range []struct {
		status, text string
		output       jsonvalue.Value
		want         string
	}{
		{"canceled", " Permission denied by user ", jsonvalue.NewNull(), "canceled reason=user"},
		{"canceled", "x", jsonvalue.NewNull(), "canceled"},
		{"errored", "", violation, "errored reason=policy"},
		{"errored", "", jsonvalue.NewArray(), ""},
	} {
		if got := stateDecision(test.status, test.output, test.text); got != test.want {
			t.Errorf("stateDecision(%q, %q) = %q, want %q", test.status, test.text, got, test.want)
		}
	}
}

func FuzzLoadSessionNeverPanics(fuzz *testing.F) {
	for _, seed := range []string{`{"sessionId": "x", "history": []}`, "{\"history\": [{\"a\": 1}, {\"b",
		`x {"title": "t"`, "[1]", ""} {
		fuzz.Add(seed)
	}
	fuzz.Fuzz(func(t *testing.T, text string) {
		session, _ := LoadSession(nil, text)
		if !session.IsObject() {
			t.Errorf("LoadSession(%q) is not an object", text)
		}
	})
}

func FuzzReadPermissionsNeverPanics(fuzz *testing.F) {
	for _, seed := range []string{"allow:\n  - Bash(ls)\n", "ask: []\n", "exclude:\n  - \"a\\\"b\" # c\n", "- x"} {
		fuzz.Add(seed)
	}
	fuzz.Fuzz(func(t *testing.T, text string) {
		rules, err := readPermissions(text)
		if err != nil && rules != nil {
			t.Errorf("readPermissions(%q) gave rules and a problem", text)
		}
	})
}

func TestAResultLineFindsItsPostWithoutAScanOfTheOthers(t *testing.T) {
	for _, count := range []int{1000, 2000, 4000} {
		t.Run(strconv.Itoa(count)+" posts", func(t *testing.T) {
			pending := newPendingPosts(parserkit.CountingWork())
			for i := range count {
				pending.add(&post{processID: "0000abcd", url: "https://x/agents/a" + strconv.Itoa(i) + "/metadata"})
			}
			for range count {
				pending.settle("0000abcd", "https://x/agents/none/metadata", "Success")
			}
			if settled := pending.work.Count("posts settled"); settled != 0 {
				t.Errorf("%d posts examined for results no post waits for", settled)
			}
		})
	}
}

func TestAResultSettlesTheOldestPostOfItsProcessAndURL(t *testing.T) {
	const url = "https://x/agents/a/metadata"
	first, second := &post{processID: "0000abcd", url: url, status: "-"}, &post{processID: "0000abcd", url: url, status: "-"}
	other := &post{processID: "0000beef", url: url, status: "-"}
	pending := newPendingPosts(parserkit.CountingWork())
	for _, entry := range []*post{first, other, second} {
		pending.add(entry)
	}
	tests := []struct {
		name, processID, url, status string
		want                         [3]string
	}{
		{"another URL settles none", "0000abcd", "https://x/agents/b/metadata", "Failed", [3]string{"-", "-", "-"}},
		{"the oldest of the process first", "0000abcd", url, "Success", [3]string{"Success", "-", "-"}},
		{"then the next", "0000abcd", url, "Error", [3]string{"Success", "Error", "-"}},
		{"a third result finds none", "0000abcd", url, "Failed", [3]string{"Success", "Error", "-"}},
		{"another process has its own", "0000beef", url, "Auth required", [3]string{"Success", "Error", "Auth required"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pending.settle(test.processID, test.url, test.status)
			if got := [3]string{first.status, second.status, other.status}; got != test.want {
				t.Errorf("statuses %q, want %q", got, test.want)
			}
		})
	}
	if settled := pending.work.Count("posts settled"); settled != 3 {
		t.Errorf("%d posts settled, want 3", settled)
	}
}

// The sessions index reports a damaged array in the kit's wording.
func TestReadIndexReportsADamagedArrayInTheKitsWording(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		entries  int
		problems parserkit.Problems
	}{
		{"data after the array", `[{"sessionId": "a"}] x`, 1,
			parserkit.Problems{{Line: 2, Message: "unexpected data after the array"}}},
		{"data after an empty array", `[] x`, 0,
			parserkit.Problems{{Line: 1, Message: "unexpected data after the array"}}},
		{"cut after a comma", `[{"sessionId": "a"},`, 1,
			parserkit.Problems{{Line: 2, Message: "array cut off after record 1"}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "sessions.json")
			if err := osfile.WriteFile(path, []byte(test.text), 0o600); err != nil {
				t.Fatal(err)
			}
			entries, problems := ReadIndex(path)
			if len(entries) != test.entries || !slices.Equal(problems, test.problems) {
				t.Errorf("got %d entries and %+v, want %d and %+v", len(entries), problems,
					test.entries, test.problems)
			}
		})
	}
}

func TestTheIndexKeepsTheFirstEntryOfASessionIDAndGroupsOnlyARepeatedOne(t *testing.T) {
	path := filepath.Join(t.TempDir(), IndexName)
	text := `[{"sessionId":"a","title":"first"},{"sessionId":"b"},{"sessionId":"a","title":"second"}]`
	if err := osfile.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	indexes := NewIndexes()
	if got := indexes.Index(path)["a"].Get("title").Text(); got != "first" {
		t.Errorf("the entry of a is titled %q, want the first", got)
	}
	for _, test := range []struct {
		name, sessionID string
		entries         int
	}{
		{"an id held twice has both entries", "a", 2},
		{"an id held once has no repeat", "b", 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := 0
			if found := indexes.Repeated(path, test.sessionID); found != nil {
				got = len(found.Entries)
			}
			if got != test.entries {
				t.Errorf("%d entries, want %d", got, test.entries)
			}
		})
	}
	if rows := indexes.Notices(path, timeline.Row{}); len(rows) != 2 || rows[0].SourceLine != 1 ||
		rows[1].SourceLine != 3 {
		t.Errorf("notices %+v, want one at each entry of a", rows)
	}
}
