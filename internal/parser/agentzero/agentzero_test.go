package agentzero

import (
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// The rows of every file kind are proved against the Python analyzer by the
// parser replay (internal/parity/replay), over the recorded fixtures; these
// tests port what the Python tests call directly. Expected values come from
// running the analyzer's functions.

// decoded returns the value of JSON text.
func decoded(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	value, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestRecoverLogsTakesJSONWhitespaceOnly(t *testing.T) {
	subject := New()
	for _, space := range []string{" ", "\t", "\n", "\r\n"} {
		text := `{"logs"` + space + ":" + space + `[{"a": 1}`
		if got := subject.recoverLogs(text); len(got) != 1 || got[0].JSON() != `{"a":1}` {
			t.Errorf("%q gave %d items", space, len(got))
		}
	}
	for _, space := range []string{"\u00a0", "\u2028", "\u0085"} {
		if got := subject.recoverLogs(`{"logs"` + space + `: [{"a": 1}`); len(got) != 0 {
			t.Errorf("%q gave %d items", space, len(got))
		}
	}
	got := subject.recoverLogs(`{"logs": [{"no": 0}, {"no": 1`)
	if len(got) != 1 || got[0].JSON() != `{"no":0}` {
		t.Errorf("a cut log gave %d items", len(got))
	}
}

func TestArgsSummaryPicksTheFirstKnownVisibleArgument(t *testing.T) {
	tests := []struct {
		json string
		want string
	}{
		{`{"a":1,"_b":2,"command":"ls"}`, "ls"},
		{`{"_x":1}`, "{}"},
		{`{"code":"","query":"q"}`, "q"},
		{`{"url":0}`, "0"},
		{`"text"`, "text"},
		{`null`, "null"},
		{`[1]`, "[1]"},
		{`5`, "5"},
		{`{"path":{"k":1}}`, `{"k":1}`},
	}
	for _, test := range tests {
		t.Run(test.json, func(t *testing.T) {
			if got := argsSummary(decoded(t, test.json)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestHeadlineTextJoinsTheHeadlineAndThoughts(t *testing.T) {
	tests := []struct {
		json string
		want string
	}{
		{`{"headline":"H","thoughts":["a","",3]}`, "H\na\n3"},
		{`{"thoughts":"t"}`, "t"},
		{`{"headline":"","thoughts":[]}`, ""},
		{`{"headline":5}`, "5"},
	}
	for _, test := range tests {
		t.Run(test.json, func(t *testing.T) {
			if got := headlineText(decoded(t, test.json)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestParseAIReadsAnObjectOrTextHoldingOne(t *testing.T) {
	tests := []struct {
		json string
		want string
	}{
		{`"{\"a\":1}"`, `{"a":1}`},
		{`" {\"a\": 2} "`, `{"a":2}`},
		{`"[1]"`, ""},
		{`"{bad"`, ""},
		{`"x"`, ""},
		{`null`, ""},
		{`{"z":1}`, `{"z":1}`},
		{`5`, ""},
	}
	for _, test := range tests {
		t.Run(test.json, func(t *testing.T) {
			got := parseAI(decoded(t, test.json))
			if text := got.JSON(); (got.IsNull() && test.want != "") ||
				(!got.IsNull() && text != test.want) {
				t.Errorf("got %q, want %q", text, test.want)
			}
		})
	}
}

func TestOverrideTextNamesAPresetOrTheSlotsWithoutKeys(t *testing.T) {
	tests := []struct {
		json string
		want string
	}{
		{`{"provider":"openai","name":"gpt"}`, "openai/gpt"},
		{`{"provider":" a "}`, "a"},
		{`{}`, ""},
		{`{"preset_name":"P"}`, "preset P"},
		{`{"chat":{"provider":"x","name":"y"},"utility":{"name":"z"},"embedding":{}}`, "chat=x/y utility=z"},
		{`{"foo":1}`, "custom configuration"},
		{`"s"`, ""},
		{`{"chat":{}}`, "custom configuration"},
	}
	for _, test := range tests {
		t.Run(test.json, func(t *testing.T) {
			if got := overrideText(decoded(t, test.json)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestContentTextReadsBlocksTextAndObjects(t *testing.T) {
	tests := []struct {
		json string
		want string
	}{
		{`""`, ""},
		{`null`, ""},
		{`{"a":1}`, `{"a":1}`},
		{`[1,"x"]`, "x"},
		{`{"text":"t"}`, "t"},
		{`{"text":5}`, `{"text":5}`},
		{`[{"text":"a"},{"type":"image"},"s"]`, "a\n[image]\ns"},
		{`5`, "5"},
		{`true`, "true"},
	}
	for _, test := range tests {
		t.Run(test.json, func(t *testing.T) {
			if got := contentText(decoded(t, test.json)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestWalkHistoryVisitsBulksTopicsThenTheCurrentTopic(t *testing.T) {
	document := decoded(t, `{"_cls":"History","bulks":[{"_cls":"Bulk","summary":"S",`+
		`"records":[{"_cls":"Message","id":"1"}]}],"topics":[{"messages":[{"_cls":"Message","id":"2"}],`+
		`"summary":"T"}],"current":{"_cls":"Topic","messages":[{"_cls":"Message","id":"3"}]}}`)
	var seen []string
	walkHistory(document, true, func(summary bool, node jsonvalue.Value) {
		if summary {
			seen = append(seen, "summary "+node.StringValue())
		} else {
			seen = append(seen, "message "+node.Get("id").StringValue())
		}
	})
	want := []string{"summary S", "message 1", "summary T", "message 2", "message 3"}
	if !slices.Equal(seen, want) {
		t.Errorf("got %q, want %q", seen, want)
	}
	seen = nil
	walkHistory(document, false, func(_ bool, node jsonvalue.Value) { seen = append(seen, node.Get("id").StringValue()) })
	if !slices.Equal(seen, []string{"1", "2", "3"}) {
		t.Errorf("without summaries got %q", seen)
	}
}

func TestPythonLinesSplitAtPythonsBoundaries(t *testing.T) {
	tests := []struct {
		text string
		want []string
	}{
		{"a\r\nb\vc\x1cd\u2028e\u0085f", []string{"a", "b", "c", "d", "e", "f"}},
		{"x\n", []string{"x"}},
		{"a\n\nb", []string{"a", "", "b"}},
		{"a\rb", []string{"a", "b"}},
		{"", nil},
	}
	for _, test := range tests {
		if got := pythonLines(test.text); !slices.Equal(got, test.want) {
			t.Errorf("pythonLines(%q) = %q, want %q", test.text, got, test.want)
		}
	}
}

func TestStemDropsOnlyARealSuffix(t *testing.T) {
	for name, want := range map[string]string{"abc.json": "abc", "a.b.json": "a.b", ".json": ".json",
		"abc.": "abc.", "abc": "abc"} {
		if got := stem(name); got != want {
			t.Errorf("stem(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestWantsTheChatAndFilesBesideIt(t *testing.T) {
	tests := []struct {
		rel       string
		wants     bool
		secondary bool
		tolerance float64
	}{
		{"agent-zero/usr/chats/ctx/chat.json", true, false, 2},
		{"Desktop/agent-zero/inst/usr/chats/ctx/chat.json", true, false, 2},
		{"chats/ctx/chat.json", true, false, 2},
		{"agent-zero/tmp/chats/ctx.json", true, false, 2},
		{"agent-zero/usr/scheduler/tasks.json", true, true, 60},
		{"scheduler/tasks.json", true, true, 60},
		{"plugins/_telegram_integration/state.json", true, true, 2},
		{"usr/plugins/_telegram_integration/state.json", false, false, 2},
		{"agent-zero/usr/plugins/_tool_access/config.json", true, false, 2},
		{"agent-zero/usr/projects/p/.a0proj/agents/x/plugins/_orchestrator/config.json", true, false, 2},
		{"plugins/_infection_check/.toggle-1", true, false, 2},
		{"plugins/_infection_check/.toggle-2", false, false, 2},
		{".agent-zero/.env", true, false, 2},
		{"agent-zero/usr/chats/ctx/other.json", false, false, 2},
	}
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			artifact := &input.Artifact{Rel: test.rel}
			subject := New()
			if subject.Wants(artifact) != test.wants || subject.Secondary(artifact) != test.secondary ||
				subject.SecondaryTolerance(artifact) != test.tolerance {
				t.Errorf("wants %v secondary %v tolerance %v", subject.Wants(artifact),
					subject.Secondary(artifact), subject.SecondaryTolerance(artifact))
			}
		})
	}
}

func TestTheIconMarkerIsRemovedWithTheSpaceAfterIt(t *testing.T) {
	got := New().icon.ReplaceAllString("icon://abc x icon://q\u00a0z", "")
	if got != "x z" {
		t.Errorf("got %q", got)
	}
	if strings.Contains(New().icon.String(), `\S`) {
		t.Error("the pattern must use Python's whitespace set, not Go's")
	}
}
