package claudecode

import (
	"fmt"
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/dedup"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The rows each file gives are checked by the golden output of the analysis
// commands (internal/analyze), over inputs that hold every parsed agent;
// these tests cover the helpers on their own.

func TestWantsTheClaudeCodeFilesOnly(t *testing.T) {
	tests := []struct {
		rel  string
		want bool
	}{
		{".claude/projects/-srv-proj/11111111-2222-4333-8444-555555555555.jsonl", true},
		{".claude/projects/-srv-proj/s1/subagents/agent-a1.jsonl", true},
		{".claude-work/projects/-srv-proj/s1.jsonl", true},
		{".claude/history.jsonl", true},
		{".claude-work/history.jsonl", true},
		{".claude/sessions/4242.json", true},
		{".claude/sessions/pid.json", false},
		{".claude/settings.json", true},
		{".claude/settings.local.json", true},
		{".claude.json", true},
		{".claude-work/.claude.json", true},
		{".claude/projects/-srv-proj/s1/ccr-tip.json", true},
		{".claude/projects/-srv-proj/bridge-pointer.json", true},
		{".claude/projects/-srv-proj/s1/tool-results/b1.txt", false},
		{".claude/paste-cache/abc.txt", false},
		{".claude-code-router/config.json", false},
		{".claude-code-router/history.json", false},
		{"proj/.claude/history.jsonl", false},
	}
	for _, test := range tests {
		t.Run(test.rel, func(t *testing.T) {
			if got := New().Wants(&input.Artifact{Rel: test.rel}); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestOnlyTheHistoryIsASecondarySourceAndTheParserDeclaresItsMarks(t *testing.T) {
	subject := New()
	if subject.Agent() != "claude-code" || subject.Name() != "claude-code" {
		t.Errorf("agent %q, name %q", subject.Agent(), subject.Name())
	}
	if got := parserkit.MarksOf(subject); got != (parserkit.Marks{Sync: true, Origin: true}) {
		t.Errorf("marks %+v", got)
	}
	for rel, want := range map[string]bool{".claude/history.jsonl": true,
		".claude-x/history.jsonl": true, ".claude/projects/p/s.jsonl": false} {
		if got := parserkit.IsSecondary(subject, &input.Artifact{Rel: rel}); got != want {
			t.Errorf("secondary %s: got %v", rel, got)
		}
	}
}

func TestToolSummaryNamesTheCallsAction(t *testing.T) {
	tests := []struct {
		name, tool, input, want string
	}{
		{"an edit names its file", "Edit", `{"file_path": "/a", "old_string": "x", "new_string": "y"}`, "/a"},
		{"a search joins pattern and path", "Grep", `{"pattern": "foo", "path": "/src"}`, "foo | /src"},
		{"an unknown tool is compact JSON in key order", "Unknown", `{"z": 1, "a": 2}`, `{"z":1,"a":2}`},
		{"an input that is not an object is JSON", "Unknown", `"raw"`, `"raw"`},
		{"an empty field is not present", "Bash", `{"command": ""}`, `{"command":""}`},
		{"a null field falls back to the common ones", "Bash", `{"command": null, "description": "d"}`, "d"},
		{"a number is written as the evidence wrote it", "Read", `{"file_path": 5}`, "5"},
		{"a task joins description and prompt", "Agent", `{"description": "x", "prompt": "y"}`, "x | y"},
		{"the common fields are tried in order", "Mystery", `{"query": "q", "url": "u"}`, "u"},
		{"a null input is null", "Bash", `null`, "null"},
		{"one of two named fields is enough", "Grep", `{"path": "/only"}`, "/only"},
		{"a boolean is true or false", "Skill", `{"skill": true}`, "true"},
		{"floats and non-ASCII are kept as written", "X", `{"é": [1.0, 1e16]}`, `{"é":[1.0,1e16]}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ToolSummary(test.tool, decode(t, test.input)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestResultTextReadsToolReferencesAndBlocks(t *testing.T) {
	tests := []struct {
		name, content, want string
	}{
		{"a list of blocks", `[{"type": "tool_reference", "tool_name": "Read"}, {"type": "text", "text": "a"},
			"s", {"type": "image"}, {"type": "tool_reference"}]`, "Read\na\ns\n[image]"},
		{"a string", `"plain"`, "plain"},
		{"null", `null`, ""},
		{"an object with text", `{"text": "t"}`, "t"},
		{"a number", `5`, "5"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ResultText(decode(t, test.content)); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

// commandBlock returns a slash command's tag form in a transcript, which
// holds the command name twice and then its arguments.
func commandBlock(name, arguments string) string {
	return "<command-name>/" + name + "</command-name>\n            <command-message>" + name +
		"</command-message>\n            <command-args>" + arguments + "</command-args>"
}

// matchForm returns the turn type and the text MatchForm writes for a row.
func matchForm(row timeline.Row) (string, string) {
	text := dedup.NewTexts().NewText()
	turn := MatchForm(row, text)
	return turn, text.String()
}

func TestMatchFormPairsCommandsAndPastesWithTheirHistoryLines(t *testing.T) {
	tests := []struct {
		name           string
		turn, text     string
		wantTurn, want string
	}{
		{"a slash command record", "system", "slash command: " + commandBlock("model", ""),
			"user", "/model"},
		{"a local command record", "system", "local_command: " + commandBlock("help", ""),
			"user", "/help"},
		{"a command with arguments", "system", "slash command: " + commandBlock("a", "x"),
			"user", "/a x"},
		{"arguments collapse their whitespace", "system",
			"slash command: <command-name>model</command-name><command-args>  a  b </command-args>",
			"user", "/model a b"},
		{"a name without its slash", "system", "local_command: <command-name>/help</command-name>",
			"user", "/help"},
		{"a command label without tags", "system", "slash command: none", "system", "slash command: none"},
		{"another system row", "system", "other", "system", "other"},
		{"a paste wrapper goes", "user", "<pasted_content id=\"0bb3\">\none\ntwo\n</pasted_content> Summarise it.",
			"user", "one two Summarise it."},
		{"a paste wrapper beside a line break", "user", "<pasted_content id=\"1\">x</pasted_content>\n y",
			"user", "x y"},
		{"whitespace runs collapse", "user", "echo  a\tb", "user", "echo a b"},
		{"only ASCII whitespace collapses", "user", " a  b\x1c", "user", " a  b\x1c"},
		{"an assistant row is as it is", "assistant", " a  b ", "assistant", " a  b "},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			turn, text := matchForm(timeline.Row{TurnType: test.turn, Text: test.text})
			if turn != test.wantTurn || text != test.want {
				t.Errorf("got (%q, %q), want (%q, %q)", turn, text, test.wantTurn, test.want)
			}
		})
	}
}

func TestMatchFormKeepsWhitespaceDistinctions(t *testing.T) {
	pairs := [][2]timeline.Row{
		{{TurnType: "user", Text: "echo a b"}, {TurnType: "user", Text: "echo ab"}},
		{{TurnType: "system", Text: "slash command: " + commandBlock("a", "x y")},
			{TurnType: "user", Text: "/a xy"}},
	}
	for _, pair := range pairs {
		leftTurn, left := matchForm(pair[0])
		rightTurn, right := matchForm(pair[1])
		if leftTurn == rightTurn && left == right {
			t.Errorf("%q and %q match as %q", pair[0].Text, pair[1].Text, left)
		}
	}
}

func TestMatchWindowWidensOnlyATypedCommand(t *testing.T) {
	tests := []struct {
		turn, text string
		want       bool
	}{
		{"user", "/model", true},
		{"user", "/rename x", true},
		{"user", "/a:b-c_d\tx", true},
		{"user", "/9", false},
		{"user", "/model\nx", false},
		{"user", "/mo.del", false},
		{"user", "run /bin/ls", false},
		{"user", " /model", false},
		{"system", "/model", false},
	}
	for _, test := range tests {
		t.Run(fmt.Sprintf("%s %q", test.turn, test.text), func(t *testing.T) {
			seconds, ok := MatchWindow(timeline.Row{TurnType: test.turn, Text: test.text}, dedup.NewTexts())
			if ok != test.want || (ok && seconds != 600) {
				t.Errorf("got %v, %v", seconds, ok)
			}
		})
	}
}

// commandRecord is the transcript row of /model at a second past 11:00.
func commandRecord(second int, session, project, source string) timeline.Row {
	return timeline.Row{Host: "h", User: "u", Agent: "claude-code", TurnType: "system",
		Text: "slash command: <command-name>/model</command-name>\n<command-message>model" +
			"</command-message>\n<command-args></command-args>",
		TimestampUTC: fmt.Sprintf("2026-10-01T11:%02d:%02d.000Z", second/60, second%60),
		ProjectPath:  project, SessionID: session, SourceFile: source}
}

// historyLine is the history row of /model typed at a second past 11:00.
func historyLine(second int, session, project, source string) timeline.Row {
	row := commandRecord(second, session, project, source)
	row.TurnType, row.Text = "user", "/model"
	return row
}

func TestHistoryCommandsMatchTheirRecordsUnderTheHooks(t *testing.T) {
	tests := []struct {
		name      string
		primary   []timeline.Row
		secondary []timeline.Row
		kept      []int
	}{
		{"bounds follow the matching scope",
			[]timeline.Row{commandRecord(40, "s1", "/a", "t")},
			[]timeline.Row{historyLine(0, "", "/a", "h"), historyLine(30, "", "/b", "h")}, []int{1}},
		{"bounds cover narrower scopes",
			[]timeline.Row{commandRecord(50, "s1", "/srv/proj", "t")},
			[]timeline.Row{historyLine(7, "", "/srv/proj", "h"), historyLine(23, "s1", "/srv/proj", "h")},
			[]int{0}},
		{"bounds cover broader scopes",
			[]timeline.Row{commandRecord(50, "s1", "/srv/proj", "t")},
			[]timeline.Row{historyLine(7, "s1", "/srv/proj", "h"), historyLine(23, "", "/srv/proj", "h")},
			[]int{0}},
		{"a line of another session does not bound",
			[]timeline.Row{commandRecord(50, "s1", "/srv/proj", "t")},
			[]timeline.Row{historyLine(7, "s1", "/srv/proj", "h"), historyLine(23, "s2", "/srv/proj", "h")},
			[]int{1}},
		{"bounds follow the candidate session",
			[]timeline.Row{commandRecord(50, "s1", "/srv/proj", "t1"), commandRecord(30, "s2", "/srv/proj", "t2")},
			[]timeline.Row{historyLine(7, "", "/srv/proj", "h"), historyLine(23, "s2", "/srv/proj", "h")},
			nil},
		{"bounds follow the candidate project",
			[]timeline.Row{commandRecord(50, "s", "/srv/Q", "t")},
			[]timeline.Row{historyLine(7, "s", "/srv/P", "h"), historyLine(23, "", "/srv/Q", "h")}, []int{0}},
		{"the widened window stays between history files",
			[]timeline.Row{commandRecord(80, "s1", "/srv/proj", ".claude/projects/-srv-proj/s1.jsonl")},
			[]timeline.Row{historyLine(7, "s1", "/srv/proj", ".claude/history.jsonl"),
				historyLine(67, "s1", "/srv/proj", ".claude-work/history.jsonl")}, []int{0}},
	}
	hooks := dedup.Hooks{"claude-code": New().DedupHooks()}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			secondary := make([]dedup.Secondary, 0, len(test.secondary))
			for _, row := range test.secondary {
				secondary = append(secondary, dedup.Secondary{Row: row, Tolerance: 2})
			}
			kept, dropped := dedup.Deduplicate(test.primary, secondary, hooks)
			var want []timeline.Row
			for _, index := range test.kept {
				row := test.secondary[index]
				row.Recovered = "yes"
				want = append(want, row)
			}
			if got := kept[len(test.primary):]; !slices.Equal(got, want) {
				t.Errorf("kept %+v, want %+v", got, want)
			}
			if dropped["claude-code"] != len(test.secondary)-len(test.kept) {
				t.Errorf("dropped %v", dropped)
			}
		})
	}
}

func TestSidecarPathsAreTheSessionsOwnDirectory(t *testing.T) {
	tests := []struct {
		rel, want string
	}{
		{".claude/projects/p/s1.jsonl", ".claude/projects/p/s1"},
		{".claude/projects/p/a.b.jsonl", ".claude/projects/p/a.b"},
		{".claude/projects/p/.jsonl", ".claude/projects/p/.jsonl"},
		{".claude/projects/p/s1/subagents/agent-a.jsonl", ".claude/projects/p/s1"},
		{".claude/projects/p/s1/subagents/x/y/agent-a.jsonl", ".claude/projects/p/s1"},
	}
	for _, test := range tests {
		home := filepath.Join(t.TempDir(), "home")
		artifact := &input.Artifact{Rel: test.rel, DiskPath: filepath.Join(home, filepath.FromSlash(test.rel))}
		got, ok := sessionDir(artifact)
		if want := filepath.Join(home, filepath.FromSlash(test.want)); !ok || got != want {
			t.Errorf("sessionDir(%q) = %q, %v, want %q", test.rel, got, ok, want)
		}
	}
	for hash, want := range map[string]bool{"abc123XYZ": true, "": false, "a.b": false, "a\n": false,
		"../x": false} {
		if got := isPasteHash(hash); got != want {
			t.Errorf("isPasteHash(%q) = %v", hash, got)
		}
	}
}

func TestPastePlaceholdersExpandFromInlineTextOrTheCache(t *testing.T) {
	home := t.TempDir()
	if err := osfile.MkdirAll(filepath.Join(home, "paste-cache"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(filepath.Join(home, "paste-cache", "abc.txt"), []byte("cached\xff"), 0o600); err != nil {
		t.Fatal(err)
	}
	pasted := decode(t, `{"1": {"type": "text", "content": "inline"}, "2": {"contentHash": "abc"},
		"3": {"contentHash": "missing"}, "4": {"type": "image", "content": "x"}, "5": {"type": null, "content": "x"}}`)
	display := "a [Pasted text #1 +3 lines] b [Pasted text #2] c [Pasted text #3] [Pasted text #4] " +
		"[Pasted text #5] [Pasted text #6]"
	want := "a inline b cached� c [Pasted text #3] [Pasted text #4] [Pasted text #5] [Pasted text #6]"
	var problems parserkit.Problems
	var out timeline.TextBuilder
	New().expandPastes(&out, &input.Home{}, home, display, pasted, &problems)
	var row timeline.Row
	out.Set(&row)
	if got := row.FullText(); got != want || len(problems) != 0 {
		t.Errorf("got %q, %v; want %q", got, problems, want)
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
