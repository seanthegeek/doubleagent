package codex

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// The rows of every store are checked by the golden output of the analysis
// commands (internal/analyze), over inputs that hold every parsed agent.
// These tests cover the helpers on their own.

func TestWantsAndSecondaryFollowTheAgentsHome(t *testing.T) {
	codex := New()
	fork := NewRollout(Settings{Agent: "fork", HomePrefix: ".fork", LedgerName: "imports.json"})
	tests := []struct {
		subject          *Parser
		rel              string
		wants, secondary bool
	}{
		{codex, ".codex/sessions/2025/01/01/rollout-x.jsonl", true, false},
		{codex, ".codex/archived_sessions/rollout-x.jsonl.zst", true, false},
		{codex, ".codex/sessions/rollout-x.jsonl.gz", false, false},
		{codex, ".codex/history.jsonl", true, true},
		{codex, ".codex/state_5.sqlite", true, true},
		{codex, ".codex/thread_history_1.sqlite", true, true},
		{codex, ".codex/db-backups/sqlite-1/state_5.sqlite", true, true},
		{codex, ".codex/.codex-global-state.json", true, true},
		{codex, ".codex/logs_2.sqlite", true, false},
		{codex, ".codex/logs_x.sqlite", false, false},
		{codex, ".codex/automations/a/automation.toml", true, false},
		{codex, ".codex/config.toml", true, false},
		{codex, ".codex/rules/default.rules", true, false},
		{codex, ".codex/memories/rollout_summaries/x.md", true, false},
		{codex, ".codex/imports.json", false, false},
		{codex, ".codex/sqlite/codex-dev.db", false, false},
		{fork, ".fork/imports.json", true, false},
		{fork, ".fork/automations/a/automation.toml", false, false},
		{fork, ".fork/.codex-global-state.json", false, false},
		{fork, ".fork/sessions/rollout-x.jsonl", true, false},
		{fork, ".codex/sessions/rollout-x.jsonl", false, false},
	}
	for _, test := range tests {
		t.Run(test.subject.Agent()+" "+test.rel, func(t *testing.T) {
			artifact := &input.Artifact{Rel: test.rel}
			if got := test.subject.Wants(artifact); got != test.wants {
				t.Errorf("wants: got %v, want %v", got, test.wants)
			}
			if got := test.subject.Secondary(artifact); got != test.secondary {
				t.Errorf("secondary: got %v, want %v", got, test.secondary)
			}
		})
	}
}

func TestRuleStatementsJoinLinesAndDropComments(t *testing.T) {
	tests := []struct {
		text     string
		want     []statement
		problems parser.Problems
	}{
		{"prefix_rule(pattern=[\"a b\"], decision=\"allow\")\n",
			[]statement{{1, "prefix_rule(pattern=[\"a b\"], decision=\"allow\")"}}, nil},
		{" \n", []statement{{1, " "}}, nil},
		{"prefix_rule(\r\n    pattern=[\"git\"],\r\n)\r\nnetwork_rule(host=\"x\")\r\n",
			[]statement{{1, "prefix_rule( pattern=[\"git\"], )"}, {4, "network_rule(host=\"x\")"}}, nil},
		{"prefix_rule(\n    pattern=[\"git\"],\n)\nnetwork_rule(host=\"x\")\n",
			[]statement{{1, "prefix_rule( pattern=[\"git\"], )"}, {4, "network_rule(host=\"x\")"}}, nil},
		{"prefix_rule(pattern=[\"\"\"a \nb\"\"\"])\n",
			[]statement{{1, "prefix_rule(pattern=[\"\"\"a  b\"\"\"])"}}, nil},
		{"# comment only\n\nprefix_rule(pattern=[\"#not\"]) # trailing\n",
			[]statement{{3, "prefix_rule(pattern=[\"#not\"])"}}, nil},
		{"prefix_rule(pattern=[\"a\\\"b\"])\n", []statement{{1, "prefix_rule(pattern=[\"a\\\"b\"])"}}, nil},
		{"prefix_rule(\n  pattern=[\"x\"\n", nil,
			parser.Problems{{Line: 1, Message: "unterminated rule", Raw: "prefix_rule(\n  pattern=[\"x\""}}},
		{"x = \"open\n", nil, parser.Problems{{Line: 1, Message: "unterminated rule", Raw: "x = \"open"}}},
		{")\nprefix_rule()\n", []statement{{1, ")"}, {2, "prefix_rule()"}}, nil},
		{"prefix_rule(pattern=['it\\'s'], decision='forbidden')\n",
			[]statement{{1, "prefix_rule(pattern=['it\\'s'], decision='forbidden')"}}, nil},
		{"a\\\n", []statement{{1, "a\\"}}, nil},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%q", test.text), func(t *testing.T) {
			var problems parser.Problems
			got := ruleStatements(test.text, &problems)
			if !reflect.DeepEqual(got, test.want) || !reflect.DeepEqual(problems, test.problems) {
				t.Errorf("got %+v and %+v, want %+v and %+v", got, problems, test.want, test.problems)
			}
		})
	}
}

func TestMemorySummaryHeaderRunsToTheFirstBlankLine(t *testing.T) {
	tests := []struct {
		text   string
		header map[string]string
		body   string
	}{
		{"cwd: /srv/a b\n\nsummary\n", map[string]string{"cwd": "/srv/a b"}, "summary"},
		{"cwd: /srv/proj \n\nsummary\n", map[string]string{"cwd": "/srv/proj "}, "summary"},
		{"thread_id: t\n \nbody\n", map[string]string{"thread_id": "t"}, " \nbody"},
		{"cwd : /x\n\nbody\n", map[string]string{}, "cwd : /x\n\nbody"},
		{"thread_id: t\r\ncwd: /srv/proj\r\n\r\nline one\r\nline two\r\n",
			map[string]string{"thread_id": "t", "cwd": "/srv/proj"}, "line one\nline two"},
		{"thread_id: t\nupdated_at: 2025-01-01T00:00:00Z",
			map[string]string{"thread_id": "t", "updated_at": "2025-01-01T00:00:00Z"}, ""},
		{"", map[string]string{}, ""},
		{"random: x\nbody", map[string]string{}, "random: x\nbody"},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%q", test.text), func(t *testing.T) {
			header, body := memorySummary(test.text)
			if !maps.Equal(header, test.header) || body != test.body {
				t.Errorf("got %q and %q, want %q and %q", header, body, test.header, test.body)
			}
		})
	}
}

func TestLogFieldsAreSeparatedBySpaceOrTabOnly(t *testing.T) {
	tests := []struct {
		body string
		want map[string]string
	}{
		{"a=1 b=2\tc=3", map[string]string{"a": "1", "b": "2", "c": "3"}},
		{"a=1 b=2 c=x", map[string]string{"a": "1 b=2", "c": "x"}},
		{"a=1 b=2 c=x", map[string]string{"a": "1 b=2", "c": "x"}},
		{"a=1\u0085b=2 c=x", map[string]string{"a": "1\u0085b=2", "c": "x"}},
		{`k="quoted value" x=y`, map[string]string{"k": "quoted value", "x": "y"}},
		{`k="unclosed x=y`, map[string]string{"k": `"unclosed`, "x": "y"}},
		{`k="esc\"aped" z=1`, map[string]string{"k": `esc\"aped`, "z": "1"}},
		{"a.b=1 a.b=2", map[string]string{"a.b": "2"}},
		{" =x _a=1", map[string]string{"_a": "1"}},
		{`"k"=1 k2="" k3="\\"`, map[string]string{"k2": "", "k3": `\\`}},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%q", test.body), func(t *testing.T) {
			if got := logFields(test.body); !maps.Equal(got, test.want) {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestContextMarkersIgnoreCase(t *testing.T) {
	tests := []struct{ name, text, want string }{
		{"an upper-case open marker", "<SKILL>x</skill>", "skill"},
		{"an upper-case close marker with spaces around", "  <environment_context>\n</ENVIRONMENT_CONTEXT>  ",
			"environment_context"},
		{"the Kelvin sign lowers to k", "<s\u212aill>x</skill>", "skill"},
		{"a capital I with dot lowers to i", "<sk\u0130ll>x</skill>", "skill"},
		{"a dotless i is not i", "<sk\u0131ll>x</skill>", ""},
		{"the AGENTS.md heading", "# agents.md INSTRUCTIONS\n</instructions>", "agents_md_instructions"},
		{"text before the marker", "hello <skill>x</skill>", ""},
		{"an empty block", " <skill></skill> ", "skill"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := contextMarker(test.text); got != test.want {
				t.Errorf("contextMarker(%q) = %q, want %q", test.text, got, test.want)
			}
		})
	}
}

func TestUUID7TimeReadsTheFirst48Bits(t *testing.T) {
	tests := []struct {
		value sqlitefile.Value
		want  string
	}{
		{sqlitefile.TextValue("0190f5a2-5c3e-7d4f-8a1b-2c3d4e5f6a7b"), "2024-07-27T19:19:36.766Z"},
		{sqlitefile.TextValue("0190f5a2-5c3e-4d4f-8a1b-2c3d4e5f6a7b"), ""},
		{sqlitefile.TextValue("xyz"), ""},
		{sqlitefile.Value{}, ""},
		{sqlitefile.TextValue("01-90f5a25c3e7d4f8a1b2c3d4e5f6a7b"), "2024-07-27T19:19:36.766Z"},
	}
	for _, test := range tests {
		t.Run(test.value.String(), func(t *testing.T) {
			if got := uuid7Time(test.value); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestOpenInterpreterSettingsAreAForkOfTheSameParser(t *testing.T) {
	fork := NewRollout(Settings{Agent: "open-interpreter", HomePrefix: ".openinterpreter"})
	if fork.Agent() != "open-interpreter" || fork.Name() != "open-interpreter" {
		t.Errorf("got %s and %s", fork.Agent(), fork.Name())
	}
	if !slices.Equal([]bool{fork.Marks().Sync, New().Marks().Sync}, []bool{true, true}) {
		t.Error("both record synced sessions")
	}
}

func FuzzRuleStatements(fuzz *testing.F) {
	fuzz.Add("prefix_rule(pattern=[\"git\"],\n decision='allow') # x\n")
	fuzz.Add("x = \"open\\\n")
	fuzz.Fuzz(func(t *testing.T, text string) {
		var problems parser.Problems
		for _, found := range ruleStatements(text, &problems) {
			if found.line < 1 {
				t.Fatalf("statement at line %d", found.line)
			}
		}
		if len(problems) > 1 {
			t.Fatalf("%d problems; at most the last statement stays open", len(problems))
		}
	})
}
