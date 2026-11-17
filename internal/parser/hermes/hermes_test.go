package hermes

import (
	"fmt"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// The rows of every store are proved against the Python analyzer by the
// parser replay (internal/parity/replay); these tests port what the Python
// tests call directly and cover the helpers, with expected values from
// running the analyzer's functions on the same inputs.

func TestYAMLScalarsReadOnlyOneLineStrings(t *testing.T) {
	tests := []struct {
		value string
		want  string
		ok    bool
	}{
		{"a", "a", true},
		{"a\t# note", "a", true},
		{"a:\tb", "", false},
		{"a: b", "", false},
		{"a:", "", false},
		{`"x\u00e9" # c`, "xé", true},
		{`"x" y`, "", false},
		{"'it''s'", "it's", true},
		{"'open", "", false},
		{"[1]", "", false},
		{"-x", "-x", true},
		{"x\u00a0", "x\u00a0", true},
		{"'x'\u00a0", "", false},
		{`"a\`, "", false},
		{"a #b", "a", true},
		{"&anchor", "", false},
		{"\"1\"\t#", "1", true},
		{"", "", false},
		{"a\u00a0# b", "a\u00a0# b", true},
		{"a:\u2028b", "a:\u2028b", true},
	}
	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			got, ok := New().yamlScalar(test.value)
			if got != test.want || ok != test.ok {
				t.Errorf("got %q, %v; want %q, %v", got, ok, test.want, test.ok)
			}
		})
	}
}

func TestFlatListReadsOneTopLevelBlockList(t *testing.T) {
	tests := []struct {
		text, key string
		want      string
	}{
		{"key:\t# note\n  - x\t# note\n", "key", `[{2 x}] ""`},
		{"command_allowlist:\n  - one\n\u00a0- two\n", "command_allowlist", `[{2 one}] ""`},
		{"command_allowlist:\u00a0\n  - one\n", "command_allowlist",
			`[] "line 1: command_allowlist is not a block list"`},
		{"a: 1\ncommand_allowlist: [x]\n", "command_allowlist",
			`[] "line 2: command_allowlist is not a block list"`},
		{"command_allowlist:\n- a\n  - \"b\"\n# c\n\n  - c: d\n", "command_allowlist",
			`[{2 a} {3 b}] "line 6: not a string item of command_allowlist"`},
		{"command_allowlist: ~\nother:\n  - z\n", "command_allowlist", `[] ""`},
		{"x: 1\n", "command_allowlist", `[] ""`},
	}
	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			entries, problem := New().readFlatList(textlines.Split(test.text), test.key)
			if got := fmt.Sprintf("%v %q", entries, problem); got != test.want {
				t.Errorf("got %s, want %s", got, test.want)
			}
		})
	}
}

func TestCronDocumentsSplitByFramesThenHeadings(t *testing.T) {
	tests := []struct {
		document, prompt, reply string
		ok                      bool
	}{
		{"head\n**Prompt Characters:** 5\n## Prompt\n\nhéllo\n\n**Response Characters:** 3\n## Response\n\nwör\n",
			"héllo", "wör", true},
		{"## Prompt\n\nask\n\n## Response\n\nmid\n## Response\n\nend\n", "ask\n\n## Response\n\nmid\n", "end", true},
		{"nothing here", "", "", false},
		{"## Response\n\nr\n## Prompt\n\np", "", "", false},
		{"**Prompt Characters:** 99\n## Prompt\n\nshort\n\n## Response\n\nr\n", "short", "r", true},
		{"## Prompt\n\n## Prompt\n\nx\n**Prompt Characters:** 1\n## Prompt\n\ny\n\n**Response Characters:** 1\n## Response\n\nz",
			"## Prompt\n\nx\n**Prompt Characters:** 1\n## Prompt\n\ny\n\n**Response Characters:** 1\n", "z", true},
	}
	for _, test := range tests {
		t.Run(test.document, func(t *testing.T) {
			prompt, reply, ok := New().splitCronDocument(test.document)
			if prompt != test.prompt || reply != test.reply || ok != test.ok {
				t.Errorf("got %q, %q, %v", prompt, reply, ok)
			}
		})
	}
}

func TestModelSwitchMarkersNameTheModelAndProvider(t *testing.T) {
	prefix := "[System: The active model for this chat has changed to "
	tests := []struct {
		content         pyjson.Value
		model, provider string
		ok              bool
	}{
		{pyjson.NewString(prefix + "gpt-5 via provider openai. From this point forward, x]"), "gpt-5", "openai", true},
		{pyjson.NewString(prefix + "m. From this point forward,"), "m", "", true},
		{pyjson.NewString(prefix + ". From this point forward,"), "", "", false},
		{pyjson.NewString("[System: model changed]"), "", "", false},
		{pyjson.NewInt(5), "", "", false},
	}
	for _, test := range tests {
		t.Run(test.content.Str(), func(t *testing.T) {
			model, provider, ok := modelSwitch(test.content)
			if model != test.model || provider != test.provider || ok != test.ok {
				t.Errorf("got %q, %q, %v", model, provider, ok)
			}
		})
	}
}

func TestStringHelpersMatchTheAnalyzer(t *testing.T) {
	tests := []struct {
		name  string
		got   func(pyjson.Value) string
		input pyjson.Value
		want  string
	}{
		{"approval", terminalDecision, pyjson.NewString(`{"approval":"once"}`), "approved (once)"},
		{"blocked with summary", terminalDecision,
			pyjson.NewString(`{"status":"blocked","error":"no","user_summary":"s"}`), "blocked no user_summary=s"},
		{"blocked bare", terminalDecision, pyjson.NewString(`{"status":"blocked"}`), "blocked -"},
		{"pending", terminalDecision,
			pyjson.NewString(`{"status":"pending_approval","pattern_key":"rm","smart_denied":1}`),
			"pending pattern=rm smart_denied=true"},
		{"sentinel approval", terminalDecision, pyjson.NewString("\x00json:{\"approval\":\"a\"}"), "approved (a)"},
		{"not JSON", terminalDecision, pyjson.NewString("nope"), ""},
		{"other status", terminalDecision, pyjson.NewString(`{"status":"ok"}`), ""},
		{"command first", callSummary, pyjson.NewString(`{"command":"ls","path":"p"}`), "ls"},
		{"empty path skipped", callSummary, pyjson.NewString(`{"path":"","url":"u"}`), "u"},
		{"other arguments", callSummary, pyjson.NewString(`{"x":1}`), `{"x":1}`},
		{"text", callSummary, pyjson.NewString("not json"), "not json"},
		{"empty", callSummary, pyjson.NewString(""), ""},
		{"null", callSummary, pyjson.NewNull(), "null"},
		{"number", callSummary, pyjson.NewInt(5), "5"},
		{"array", callSummary, pyjson.NewString("[1, 2]"), "[1,2]"},
		{"zero command", callSummary, pyjson.NewString(`{"command":0}`), "0"},
		{"false command", callSummary, pyjson.NewString(`{"command":false}`), "False"},
		{"image parts", contentText, pyjson.NewString("\x00json:[{\"type\":\"text\",\"text\":\"a\"}," +
			"{\"type\":\"image_url\"},{\"type\":\"input_image\",\"text\":\"t\"}]"), "a\n[image]\n[image]"},
		{"bad sentinel JSON", contentText, pyjson.NewString("\x00json:bad"), "bad"},
		{"plain content", contentText, pyjson.NewString("plain"), "plain"},
		{"null content", contentText, pyjson.NewNull(), ""},
		{"number content", contentText, pyjson.NewInt(5), "5"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := test.got(test.input); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestToolCallsReportWhatIsNotAListOfObjects(t *testing.T) {
	tests := []struct {
		value pyjson.Value
		calls int
		want  parser.Problems
	}{
		{pyjson.NewString(`[{"id":"a"}]`), 1, nil},
		{pyjson.NewString(`{"id":"b"}`), 1, nil},
		{pyjson.NewString("42"), 0, parser.Problems{{Line: 7,
			Message: "unparseable record 7 (tool_calls is not a list)", Raw: "42"}}},
		{pyjson.NewString(`["x",{"id":"c"}]`), 1, parser.Problems{{Line: 7,
			Message: "unparseable record 7 (tool_calls entry is not an object)", Raw: `["x",{"id":"c"}]`}}},
		{pyjson.NewString(""), 0, nil},
		{pyjson.NewNull(), 0, nil},
	}
	for _, test := range tests {
		t.Run(test.value.Str(), func(t *testing.T) {
			message := pyjson.NewObject(pyjson.Member{Key: "tool_calls", Value: test.value})
			calls, problems := toolCalls(message, pyjson.NewInt(7))
			if len(calls) != test.calls || fmt.Sprint(problems) != fmt.Sprint(test.want) {
				t.Errorf("got %d calls, %v; want %d, %v", len(calls), problems, test.calls, test.want)
			}
		})
	}
}
