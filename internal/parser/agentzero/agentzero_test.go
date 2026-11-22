package agentzero

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The rows of every file kind are checked by the golden output of the
// analysis commands (internal/analyze), over inputs that hold every parsed
// agent; these tests cover the helpers the rows are built from.

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
		if got := subject.recoverLogs(nil, text); len(got) != 1 || got[0].JSON() != `{"a":1}` {
			t.Errorf("%q gave %d items", space, len(got))
		}
	}
	for _, space := range []string{"\u00a0", "\u2028", "\u0085"} {
		if got := subject.recoverLogs(nil, `{"logs"`+space+`: [{"a": 1}`); len(got) != 0 {
			t.Errorf("%q gave %d items", space, len(got))
		}
	}
	got := subject.recoverLogs(nil, `{"logs": [{"no": 0}, {"no": 1`)
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
			got := parseAI(nil, decoded(t, test.json))
			if text := got.JSON(); (got.IsNull() && test.want != "") ||
				(!got.IsNull() && text != test.want) {
				t.Errorf("got %q, want %q", text, test.want)
			}
		})
	}
}

func TestOverrideTextNamesAPresetOrTheSlots(t *testing.T) {
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
			if got := overrideText(nil, decoded(t, test.json)); got != test.want {
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
	wrong := 0
	walkHistory(document, true, &wrong, func(summary bool, node jsonvalue.Value) {
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
	walkHistory(document, false, &wrong, func(_ bool, node jsonvalue.Value) {
		seen = append(seen, node.Get("id").StringValue())
	})
	if !slices.Equal(seen, []string{"1", "2", "3"}) || wrong != 0 {
		t.Errorf("without summaries got %q and %d entries that are not objects", seen, wrong)
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
	tests := []struct {
		name    string
		heading string
		want    string
	}{
		{"an ASCII space ends the marker", "icon://abc x", "x"},
		{"a no-break space ends the marker", "icon://q\u00a0z", "z"},
		{"an ideographic space ends the marker", "icon://q\u3000z", "z"},
		{"every run of whitespace after the marker goes", "icon://q \t\u2028z", "z"},
		{"an information separator is part of the marker", "icon://q\x1cz y", "y"},
		{"two markers both go", "icon://abc x icon://q\u00a0z", "x z"},
	}
	subject := New()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := subject.icon.ReplaceAllString(test.heading, ""); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestTheConnectorEnvLosesEveryQuoteAtEitherEndOfAValue(t *testing.T) {
	const rel = ".agent-zero/.env"
	tests := []struct{ name, value, want string }{
		{"a plain value", `host`, "host"},
		{"double quotes", `"host"`, "host"},
		{"single quotes", `'host'`, "host"},
		{"two layers", `""host""`, "host"},
		{"mixed layers", `'"host"'`, "host"},
		{"unmatched quotes", `"host'`, "host"},
		{"a quote at one end", `host"`, "host"},
		{"a quote inside stays", `ho"st`, `ho"st`},
		{"spaces around the quotes", `  "host"  `, "host"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			writeFile(t, home, rel, "AGENT_ZERO_LAST_CONTEXT_HOST="+test.value+"\nAGENT_ZERO_LAST_CONTEXT_ID=c1\n")
			artifact := &input.Artifact{DiskPath: filepath.Join(home, filepath.FromSlash(rel)), Original: "/a/" + rel, Rel: rel, Home: &input.Home{DiskPath: home}}
			var texts []string
			if err := New().Parse(artifact, parser.Options{}, func(row timeline.Row) { texts = append(texts, row.Text) }); err != nil {
				t.Fatal(err)
			}
			want := "session origin: remote agent-zero instance " + test.want + " context c1 (from .agent-zero/.env, file mtime)"
			if len(texts) != 2 || texts[1] != want {
				t.Errorf("rows %q, want %q", texts, want)
			}
		})
	}
}
