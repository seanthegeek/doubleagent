package clinelegacy

import (
	"math"
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// decode returns JSON text as a value, failing the test when it is not JSON.
func decode(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	value, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatalf("%s: %v", text, err)
	}
	return value
}

// A mapUICase is one message kind, its message and the row it gives.
type mapUICase struct {
	name, kind, message string
	first               bool
	want                Mapped
	ok                  bool
}

// mapUICases returns the cases of TestMapUIGivesTheRowOfEachMessageKind.
func mapUICases() []mapUICase {
	return []mapUICase{
		{"the first text is the user's task with its images counted", "text",
			`{"text": "hello", "images": ["a", "b"]}`, true,
			Mapped{"user", "hello | [2 image(s)]", ""}, true},
		{"a later text with JSON gives its question and choices", "text",
			`{"text": "{\"question\": \"Q?\", \"options\": [\"a\", {\"answer\": \"b\"}, {\"x\": 1}, 3]}"}`,
			false, Mapped{"assistant", `Q? | a; b; {"x":1}; 3`, ""}, true},
		{"an empty followup is skipped", "followup", `{"text": "{\"response\": \"\", \"suggest\": []}"}`,
			false, Mapped{"assistant", "", ""}, false},
		{"a file tool names its paths and diff", "tool",
			`{"text": "{\"tool\": \"readFile\", \"path\": \"a.go\", \"batchFiles\": [{\"path\": \"b\"}, {}], \"diff\": \"d\"}"}`,
			false, Mapped{"tool_use", "a.go | b | d", "readFile"}, true},
		{"batch entries without a path are left out", "tool",
			`{"text": "{\"tool\": \"readFile\", \"batchFiles\": [{}, {\"path\": null}, {\"path\": \"\"}, {\"path\": 0}, \"x\"], \"content\": \"c\"}"}`,
			false, Mapped{"tool_use", "0 | c", "readFile"}, true},
		{"a tool without a diff shows its content", "tool",
			`{"text": "{\"tool\": \"\", \"diff\": \"\", \"content\": 0}"}`,
			false, Mapped{"tool_use", "0", "tool"}, true},
		{"a tool message that is not JSON keeps its text", "tool", `{"text": "plain"}`, false,
			Mapped{"tool_use", "plain", "tool"}, true},
		{"an MCP call shows its arguments as JSON", "use_mcp_server",
			`{"text": "{\"serverName\": \"s\", \"arguments\": {\"a\": 1}}"}`, false,
			Mapped{"tool_use", `s | {"a":1}`, "use_mcp_server"}, true},
		{"a browser action that is not JSON keeps its text", "browser_action", `{"text": "not json"}`,
			false, Mapped{"tool_use", "not json", "browser_action"}, true},
		{"subagent prompts are joined", "use_subagents", `{"text": "{\"prompts\": [\"a\", 2]}"}`, false,
			Mapped{"tool_use", "a ; 2", "use_subagents"}, true},
		{"a command is execute_command", "command", `{"text": "ls"}`, false,
			Mapped{"tool_use", "ls", "execute_command"}, true},
		{"a browser result shows its URL and screenshot", "browser_action_result",
			`{"text": "{\"currentUrl\": \"u\", \"screenshot\": \"x\"}"}`, false,
			Mapped{"tool_result", "u | [screenshot]", "browser_action"}, true},
		{"an empty MCP response is skipped", "mcp_server_response", `{"text": ""}`, false,
			Mapped{"tool_result", "", ""}, false},
		{"an API request names its counts", "api_req_started",
			`{"text": "{\"tokensIn\": 5, \"cost\": 0.5, \"cancelReason\": \"\"}"}`, false,
			Mapped{"system", "api_req_started: tokensIn=5 cost=0.5", ""}, true},
		{"an API request without JSON is its kind", "api_req_finished", `{"text": "x"}`, false,
			Mapped{"system", "api_req_finished", ""}, true},
		{"a condensation gives its counts and summary", "condense_context",
			`{"contextCondense": {"cost": 1, "summary": "s"}}`, false,
			Mapped{"system", "condense_context: cost=1 | s", ""}, true},
		{"an empty condensation is its kind", "condense_context", `{"contextCondense": {}}`, false,
			Mapped{"system", "condense_context", ""}, true},
		{"a checkpoint names its hash", "checkpoint_created", `{"lastCheckpointHash": "abc"}`, false,
			Mapped{"system", "checkpoint_created: abc", ""}, true},
		{"structured text is compact JSON", "error", `{"text": {"a": [1, 2.0]}}`, false,
			Mapped{"system", `error: {"a":[1,2.0]}`, ""}, true},
		{"empty reasoning is skipped", "reasoning", `{"text": ""}`, false, Mapped{"thinking", "", ""}, false},
	}
}

// The expected values are the row research/cline.md gives each message
// kind.
func TestMapUIGivesTheRowOfEachMessageKind(t *testing.T) {
	for _, test := range mapUICases() {
		t.Run(test.name, func(t *testing.T) {
			got, ok := MapUI(test.kind, decode(t, test.message), test.first)
			if ok != test.ok || (ok && got != test.want) {
				t.Errorf("MapUI = %+v, %v; want %+v, %v", got, ok, test.want, test.ok)
			}
		})
	}
}

// The expected values are the row research/cline.md gives each block.
func TestContentRowsGivesARowPerBlock(t *testing.T) {
	for _, test := range []struct {
		name, content string
		thinking      bool
		want          []ContentRow
	}{
		{"a string is one row", `"plain"`, false, []ContentRow{{"user", "plain", "", ""}}},
		{"an empty list gives none", `[]`, true, nil},
		{"text blocks and strings are rows, empty ones not", `[{"type": "text", "text": "t"}, "s", "", {"type": "text", "text": 5}]`,
			false, []ContentRow{{"user", "t", "", ""}, {"user", "s", "", ""}, {"user", "5", "", ""}}},
		{"a tool call takes its call_id", `[{"type": "tool_use", "name": "Bash", "input": {"command": "ls"}, "call_id": "c1"}]`,
			false, []ContentRow{{"tool_use", "ls", "Bash", "c1"}}},
		{"a failed result is marked", `[{"type": "tool_result", "content": {"a": 1}, "is_error": true, "tool_use_id": "t"}]`,
			false, []ContentRow{{"tool_result", `[error] {"a":1}`, "", "t"}}},
		{"thinking is kept when asked", `[{"type": "thinking", "thinking": "hm"}, {"type": "redacted_thinking"}, {"type": "image"}, {"type": "file", "path": "p"}, {"type": 7}, {"type": ""}]`,
			true, []ContentRow{{"thinking", "hm", "", ""}, {"thinking", "[redacted thinking]", "", ""},
				{"user", "[image]", "", ""}, {"user", "[file] | p", "", ""}, {"user", "[7]", "", ""}}},
		{"thinking is dropped otherwise", `[{"type": "thinking", "thinking": "hm"}, {"type": "image"}]`,
			false, []ContentRow{{"user", "[image]", "", ""}}},
		{"an object is its compact JSON", `{"x": 1}`, false, []ContentRow{{"user", `{"x":1}`, "", ""}}},
		{"false is written as JSON", `false`, false, []ContentRow{{"user", "false", "", ""}}},
		{"null gives nothing", `null`, true, nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := ContentRows(decode(t, test.content), "user", test.thinking)
			if !slices.Equal(got, test.want) {
				t.Errorf("ContentRows = %+v, want %+v", got, test.want)
			}
		})
	}
}

// ArrayAfterKey finds the bracket after the key, with JSON's four
// whitespace characters only.
func TestArrayAfterKeyTakesJSONWhitespaceOnly(t *testing.T) {
	for _, test := range []struct {
		name, text string
		want       int
		ok         bool
	}{
		{"JSON whitespace around the colon", "{\"messages\" \r\n:\t[", 16, true},
		{"no whitespace at all", `{"messages":[1]`, 12, true},
		{"a no-break space is not JSON whitespace", "{\"messages\"\u00a0: [", 0, false},
		{"a line separator is not JSON whitespace", "{\"messages\"\u2028: [", 0, false},
		{"a next-line character is not JSON whitespace", "{\"messages\"\u0085: [", 0, false},
		{"a later key that opens an array is found", `{"messages": 1, "messages": [`, 28, true},
		{"a key without an array is not found", `{"messages": {}}`, 0, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := ArrayAfterKey(test.text, KeyedArray("messages"))
			if got != test.want || ok != test.ok {
				t.Errorf("ArrayAfterKey = %d, %v; want %d, %v", got, ok, test.want, test.ok)
			}
		})
	}
}

func TestTheTaskPatternMatchesTaskFilesUnderTheRoot(t *testing.T) {
	reader := NewTaskParser(parser.NewBase("x", "x"), `(?:(?:.*/)?User/globalStorage/saoudrizwan\.claude-dev|\.cline/data)`, "", nil)
	for _, test := range []struct {
		name, rel string
		want      bool
	}{
		{"a ui_messages.json of the extension", ".config/Code/User/globalStorage/saoudrizwan.claude-dev/tasks/1791021600000/ui_messages.json", true},
		{"an API history of the CLI", ".cline/data/tasks/1791021300000/api_conversation_history.json", true},
		{"a name in another case", ".cline/data/tasks/x/TASK_METADATA.JSON", true},
		{"a file of another name", ".cline/data/tasks/x/settings.json", false},
		{"a file one level deeper", ".cline/data/tasks/x/y/ui_messages.json", false},
		{"a trailing newline", ".cline/data/tasks/x/ui_messages.json\n", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := reader.taskPattern.MatchString(test.rel); got != test.want {
				t.Errorf("match = %v, want %v", got, test.want)
			}
		})
	}
}

// The expected starts follow the order the package comment gives.
func TestTaskStartPrefersTheIdThenTheMetadataThenTheIndex(t *testing.T) {
	for _, test := range []struct {
		name, taskID, item, metadata, want string
	}{
		{"a millisecond id", "1791021600000", `{"ts": 1}`, `{}`, "2026-10-03T10:00:00.000Z"},
		{"the earliest metadata time", "abc", `{"ts": 1}`,
			`{"model_usage": [{"ts": 1791021700000}], "environment_history": [{"ts": 1791021600000}, {"ts": "x"}]}`,
			"2026-10-03T10:00:00.000Z"},
		{"the index time", "abc", `{"ts": 1791021600000}`, `{}`, "2026-10-03T10:00:00.000Z"},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := taskStart(test.taskID, decode(t, test.item), decode(t, test.metadata))
			if got != test.want {
				t.Errorf("start = %q, want %q", got, test.want)
			}
		})
	}
}

func FuzzArrayAfterKeyFindsAnOpeningBracket(fuzz *testing.F) {
	for _, seed := range []string{"{\"messages\" \r\n:\t[", `"messages"`, `"messages":`, ""} {
		fuzz.Add(seed)
	}
	fuzz.Fuzz(func(t *testing.T, text string) {
		if offset, ok := ArrayAfterKey(text, KeyedArray("messages")); ok && text[offset] != '[' {
			t.Errorf("ArrayAfterKey(%q) = %d, not a bracket", text, offset)
		}
	})
}

func TestTaskPathsComeFromTheArtifactsDiskPath(t *testing.T) {
	home := filepath.Join(t.TempDir(), "home")
	file := &input.Artifact{Rel: "store/tasks/t1/ui_messages.json",
		DiskPath: filepath.Join(home, "store", "tasks", "t1", "ui_messages.json")}
	task := TaskOf(file)
	if task.TaskID != "t1" {
		t.Errorf("TaskID = %q", task.TaskID)
	}
	if got := task.InDir("x.json"); got != filepath.Join(home, "store", "tasks", "t1", "x.json") {
		t.Errorf("InDir = %q", got)
	}
	if got, ok := task.InRoot("state/taskHistory.json"); !ok || got != filepath.Join(home, "store", "state", "taskHistory.json") {
		t.Errorf("InRoot = %q, %v", got, ok)
	}
	if got, ok := task.InRootParent("state.vscdb"); !ok || got != filepath.Join(home, "state.vscdb") {
		t.Errorf("InRootParent = %q, %v", got, ok)
	}
	queue := &input.Artifact{Rel: "store/cache/sync-queue.json",
		DiskPath: filepath.Join(home, "store", "cache", "sync-queue.json")}
	if got, ok := (Task{Artifact: queue, RootLevels: 1}).InRoot("state/taskHistory.json"); !ok ||
		got != filepath.Join(home, "store", "state", "taskHistory.json") {
		t.Errorf("sync-queue InRoot = %q, %v", got, ok)
	}
	shallow := &input.Artifact{Rel: "tasks/t1/ui_messages.json",
		DiskPath: filepath.Join(home, "tasks", "t1", "ui_messages.json")}
	if _, ok := TaskOf(shallow).InRootParent("state.vscdb"); ok {
		t.Error("a root above the home was named")
	}
}

func TestEpochOfReadsANumberOrANumericString(t *testing.T) {
	tests := []struct {
		name, value string
		want        float64
	}{
		{"a number", `1791021600000`, 1791021600000},
		{"true counts as 1", `true`, 1},
		{"a numeric string with spaces", `" 12.5\n"`, 12.5},
		{"a string that is not a number is 0", `"soon"`, 0},
		{"NaN is 0", `"nan"`, 0},
		{"a string past the float range is infinite", `"1e999"`, math.Inf(1)},
		{"a number past the float range is infinite", `1e400`, math.Inf(1)},
		{"a negative number past the float range is minus infinity", `-1e400`, math.Inf(-1)},
		{"a string of a negative number past the float range", `"-1e400"`, math.Inf(-1)},
		{"NaN text in another case is 0", `"NaN"`, 0},
		{"a negative number", `-5`, -5},
		{"zero", `0`, 0},
		{"false counts as 0", `false`, 0},
		{"null is 0", `null`, 0},
		{"an object is 0", `{"a": 1}`, 0},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := epochOf(decode(t, test.value)); got != test.want {
				t.Errorf("epochOf(%s) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}

func TestATimePastTheFloatRangeFallsAfterOrBeforeEveryRealTime(t *testing.T) {
	metadata := decode(t, `{"model_usage": [{"ts": 1000, "model_id": "early"}, {"ts": 1e400, "model_id": "never"},
		{"ts": -1e400, "model_id": "always"}]}`)
	context := newTaskContext("t", jsonvalue.NewNull(), metadata, "cwd")
	tests := []struct {
		name, timestamp, want string
	}{
		{"before the earliest real switch", `500`, "always"},
		{"after it", `5000`, "early"},
		{"zero", `0`, "always"},
		{"a negative time", `-5`, "always"},
		{"a time past the float range", `1e400`, "never"},
		{"a negative time past the float range", `-1e400`, "always"},
		{"text of a time past the float range is not a time", `"1e400"`, "always"},
		{"NaN text is not a time", `"NaN"`, "always"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := context.ModelAt(decode(t, test.timestamp)); got != test.want {
				t.Errorf("ModelAt(%s) = %q, want %q", test.timestamp, got, test.want)
			}
		})
	}
}

func TestModelAtTakesTheLastModelInUse(t *testing.T) {
	metadata := decode(t, `{"model_usage": [{"ts": 300, "model_id": "c"}, {"ts": "200", "model_id": "b"},
		{"model_id": "a"}, {"ts": 200, "model_id": "a2"}, {"ts": 400}]}`)
	context := newTaskContext("t", decode(t, `{"modelId": "m"}`), metadata, "cwd")
	tests := []struct {
		name, timestamp, want string
	}{
		{"before every entry gives the earliest", `-1`, "a"},
		{"a tie in time is ordered by model", `250`, "b"},
		{"after every entry gives the last", `999`, "c"},
		{"a time that is not a number gives the earliest", `"x"`, "a"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := context.ModelAt(decode(t, test.timestamp)); got != test.want {
				t.Errorf("ModelAt(%s) = %q, want %q", test.timestamp, got, test.want)
			}
		})
	}
}

func TestOrderOfSortsATimeBeyondTheRangeLast(t *testing.T) {
	tests := []struct {
		name, json string
		want       float64
		isNumber   bool
	}{
		{"a number", "1790000000000", 1790000000000, true},
		{"true is one", "true", 1, true},
		{"text is zero", `"1790000000000"`, 0, false},
		{"null is zero", "null", 0, false},
		{"a number beyond the float64 range sorts last", "1e999", math.Inf(1), false},
		{"a negative one sorts first", "-1e999", math.Inf(-1), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := decode(t, test.json)
			if got := OrderOf(value); got != test.want {
				t.Errorf("OrderOf(%s) = %v, want %v", test.json, got, test.want)
			}
			if _, ok := NumberOf(value); ok != test.isNumber {
				t.Errorf("NumberOf(%s) gave %v, want %v", test.json, ok, test.isNumber)
			}
		})
	}
}
