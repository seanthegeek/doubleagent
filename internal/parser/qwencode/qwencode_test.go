package qwencode

import (
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/dedup"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The rows each file gives are checked by the golden output of the analysis
// commands (internal/analyze), over inputs that hold every parsed agent;
// these tests cover the helpers on their own and the deduplication hooks.

func TestTheParserDeclaresItsMarksAndSecondarySources(t *testing.T) {
	subject := New()
	if subject.Agent() != "qwen-code" || subject.Name() != "qwen-code" {
		t.Errorf("agent %q, name %q", subject.Agent(), subject.Name())
	}
	if got := parser.MarksOf(subject); got != (parser.Marks{Relay: true}) {
		t.Errorf("marks %+v", got)
	}
	for rel, want := range map[string]bool{
		".qwen/tmp/abc/logs.json":                                              true,
		".qwen/managed-local/harness/tmp/abc/logs.json":                        true,
		".qwen/usage_record.jsonl":                                             true,
		".qwen/managed-local/worker-home/usage_record.jsonl":                   true,
		".qwen/managed-local/harness/usage_record.jsonl":                       false,
		".qwen/channels/telegram-group-history.jsonl":                          true,
		".qwen/projects/-srv/chats/e5f6a7b8-4444-4000-8000-000000000011.jsonl": false,
	} {
		if got := subject.Secondary(&input.Artifact{Rel: rel}); got != want {
			t.Errorf("%s: secondary %v, want %v", rel, got, want)
		}
	}
}

func TestArgsSummaryPicksTheMostIdentifyingArgument(t *testing.T) {
	tests := []struct{ arguments, want string }{
		{`{"command": "ls"}`, "ls"},
		{`{"other": 1}`, `{"other":1}`},
		{`null`, ""},
		{`""`, ""},
		{`"x"`, "x"},
		{`[1]`, "[1]"},
		{`{"path": ""}`, `{"path":""}`},
		{`{"path": 0, "url": "u"}`, "0"},
		{`{"query": {"b": 1, "a": 2}}`, `{"b":1,"a":2}`},
		{`0`, "0"},
		{`false`, "false"},
	}
	for _, test := range tests {
		if got := argsSummary(decode(t, test.arguments)); got != test.want {
			t.Errorf("argsSummary(%s) = %q, want %q", test.arguments, got, test.want)
		}
	}
}

func TestInjectedOriginNamesWhatTheUserDidNotType(t *testing.T) {
	tests := []struct{ record, want string }{
		{`{"type": "user", "subtype": "cron"}`, "cron"},
		{`{"type": "user"}`, ""},
		{`{"type": "user", "subtype": "goal_runtime", "provenance": "goal_runtime"}`,
			"goal_runtime (provenance=goal_runtime)"},
		{`{"provenance": "real_user", "subtype": "agent_message", "agentName": "", ` +
			`"systemPayload": {"author": {"name": "bob", "agentId": 7}}}`,
			"agent_message (provenance=real_user agent=bob agentId=7)"},
		{`{"provenance": "x", "agentName": 0, "agentId": false}`, "user (provenance=x)"},
		{`{"provenance": 5}`, "user (provenance=5)"},
	}
	for _, test := range tests {
		if got := injectedOrigin(decode(t, test.record)); got != test.want {
			t.Errorf("injectedOrigin(%s) = %q, want %q", test.record, got, test.want)
		}
	}
}

func TestSystemTextSummarisesThePayloadOfEachSubtype(t *testing.T) {
	tests := []struct{ subtype, payload, want string }{
		{"slash_command", `{"phase": "start", "rawCommand": "/x"}`, "slash_command: start /x"},
		{"rewind", `null`, "rewind: truncated record(s)"},
		{"rewind", `{"truncatedCount": 3}`, "rewind: truncated 3 record(s)"},
		{"custom_title", `{"customTitle": "  T "}`, "custom_title: T"},
		{"chat_compression", `{"info": {"b": 1}}`, `chat_compression: {"b":1}`},
		{"other", `{"z": 1}`, `other: {"z":1}`},
		{"other", `[]`, "other"},
		{"turn_result", `{}`, "turn_result"},
		{"session_model", `"x"`, "session_model: auth="},
	}
	for _, test := range tests {
		if got := systemText(test.subtype, decode(t, test.payload)); got != test.want {
			t.Errorf("systemText(%s, %s) = %q, want %q", test.subtype, test.payload, got, test.want)
		}
	}
}

func TestQuotedGroupMessagesReadOnlyTheChannelsBlock(t *testing.T) {
	block := "[Chat messages since your last reply - for context]\n- [a] b\n"
	tests := []struct {
		name, text string
		want       []string
	}{
		{"a block without its closing marker", block + "\nhi", nil},
		{"a block holding a line of another form", block + "x\n\n[Current message - respond to this]\nhi", nil},
		{"a block after other text", "ctx\n\n" + block + "\n[Current message - respond to this]\nhi",
			[]string{"group message: [a] b"}},
		{"a marker that does not start a line", "> " + block + "\n[Current message - respond to this]\nhi",
			nil},
		{"a marker typed inside a line before the real one", "say [Chat messages since your last " +
			"reply - for context]\nok\n" + block + "- [c] d e\n\n[Current message - respond to this]",
			[]string{"group message: [a] b", "group message: [c] d e"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := quotedGroupMessages(test.text); !slices.Equal(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestGroupHistoryRowsMatchAtTheirClearUnderTheirOwnForm(t *testing.T) {
	history := "/h/.qwen/channels/telegram-group-history.jsonl"
	tests := []struct {
		turn, text, file, formText string
		kind                       dedup.AnchorKind
		millis                     int64
	}{
		{"system", "group message: [Bob] a  b\n c (cleared 2026-10-03T08:06:00.000Z)", history,
			"group message: [Bob] a b c", dedup.AtTime, 1791014760000},
		{"system", "group message: [Bob] x (waiting)", history, "group message: [Bob] x",
			dedup.NeverMatch, 0},
		{"system", "group message: [Bob] x (cleared, time not recorded)", history,
			"group message: [Bob] x", dedup.NeverMatch, 0},
		{"system", "group message: [Bob]  x\t ", history, "group message: [Bob] x", dedup.OwnTime, 0},
		{"system", "group history cleared: k", history, "group history cleared: k", dedup.OwnTime, 0},
		{"system", "group message: [a] b (cleared 2026-10-03T08:06:00.000Z)", "/h/t.jsonl",
			"group message: [a] b (cleared 2026-10-03T08:06:00.000Z)", dedup.OwnTime, 0},
		{"user", "group message: [a] b (cleared 2026-10-03T08:06:00.000Z)", history,
			"group message: [a] b (cleared 2026-10-03T08:06:00.000Z)", dedup.OwnTime, 0},
		{"system", "group message: [a] b (cleared nonsense)", history, "group message: [a] b",
			dedup.NeverMatch, 0},
	}
	for _, test := range tests {
		t.Run(test.text, func(t *testing.T) {
			row := timeline.Row{Agent: "qwen-code", TurnType: test.turn, Text: test.text,
				SourceFile: test.file}
			text := dedup.NewTexts().NewText()
			if turn := MatchForm(row, text); turn != test.turn || text.String() != test.formText {
				t.Errorf("form %q %q, want %q", turn, text.String(), test.formText)
			}
			if kind, millis := MatchAnchor(row); kind != test.kind || millis != test.millis {
				t.Errorf("anchor %v %d, want %v %d", kind, millis, test.kind, test.millis)
			}
		})
	}
}

func TestSplitURLFindsTheHostAndPath(t *testing.T) {
	tests := []struct{ name, url, hostname, path string }{
		{"a pull request URL", "https://github.com/o/r/pull/1", "github.com", "/o/r/pull/1"},
		{"user, port, query and fragment are dropped", "HTTPS://User@GitHub.COM:443/o/r?q#f", "github.com", "/o/r"},
		{"a URL without a scheme", "//h/x", "h", "/x"},
		{"text without a host is all path", "github.com/o/r", "", "github.com/o/r"},
		{"an escape stays as written", "https://h/a%20b/c", "h", "/a%20b/c"},
		{"an IPv6 host loses its brackets and port", "https://[::1]:8/x", "::1", "/x"},
		{"an IPv6 zone is decoded", "https://[FE80::1%25en0]/x", "fe80::1%en0", "/x"},
		{"no URL gives nothing", "", "", ""},
		{"an opaque URL has no path", "mailto:x@y", "", ""},
		{"no path is empty", "https://h", "h", ""},
		{"a control character is refused", " \t\x01https://h/a/b", "", ""},
		{"a line break is refused", "ht\ntps://h/a", "", ""},
		{"an invalid escape is refused", "https://h/a%zz", "", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := splitURL(test.url); got != (urlParts{test.hostname, test.path}) {
				t.Errorf("splitURL(%q) = %+v, want %q %q", test.url, got, test.hostname, test.path)
			}
		})
	}
}

// decode returns the JSON text decoded.
func decode(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	value, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
