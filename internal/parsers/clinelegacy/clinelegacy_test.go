package clinelegacy

import (
	"math"
	"path/filepath"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
	want                uiRow
	ok                  bool
}

// mapUICases returns the cases of TestMapUIGivesTheRowOfEachMessageKind.
func mapUICases() []mapUICase {
	return []mapUICase{
		{"the first text is the user's task with its images counted", "text",
			`{"text": "hello", "images": ["a", "b"]}`, true,
			uiRow{"user", "hello | [2 image(s)]", ""}, true},
		{"a later text with JSON gives its question and choices", "text",
			`{"text": "{\"question\": \"Q?\", \"options\": [\"a\", {\"answer\": \"b\"}, {\"x\": 1}, 3]}"}`,
			false, uiRow{"assistant", `Q? | a; b; {"x":1}; 3`, ""}, true},
		{"an empty followup is skipped", "followup", `{"text": "{\"response\": \"\", \"suggest\": []}"}`,
			false, uiRow{"assistant", "", ""}, false},
		{"a file tool names its paths and diff", "tool",
			`{"text": "{\"tool\": \"readFile\", \"path\": \"a.go\", \"batchFiles\": [{\"path\": \"b\"}, {}], \"diff\": \"d\"}"}`,
			false, uiRow{"tool_use", "a.go | b | d", "readFile"}, true},
		{"batch entries without a path are left out", "tool",
			`{"text": "{\"tool\": \"readFile\", \"batchFiles\": [{}, {\"path\": null}, {\"path\": \"\"}, {\"path\": 0}, \"x\"], \"content\": \"c\"}"}`,
			false, uiRow{"tool_use", "0 | c", "readFile"}, true},
		{"a tool without a diff shows its content", "tool",
			`{"text": "{\"tool\": \"\", \"diff\": \"\", \"content\": 0}"}`,
			false, uiRow{"tool_use", "0", "tool"}, true},
		{"a tool message that is not JSON keeps its text", "tool", `{"text": "plain"}`, false,
			uiRow{"tool_use", "plain", "tool"}, true},
		{"an MCP call shows its arguments as JSON", "use_mcp_server",
			`{"text": "{\"serverName\": \"s\", \"arguments\": {\"a\": 1}}"}`, false,
			uiRow{"tool_use", `s | {"a":1}`, "use_mcp_server"}, true},
		{"a browser action that is not JSON keeps its text", "browser_action", `{"text": "not json"}`,
			false, uiRow{"tool_use", "not json", "browser_action"}, true},
		{"subagent prompts are joined", "use_subagents", `{"text": "{\"prompts\": [\"a\", 2]}"}`, false,
			uiRow{"tool_use", "a ; 2", "use_subagents"}, true},
		{"a command is execute_command", "command", `{"text": "ls"}`, false,
			uiRow{"tool_use", "ls", "execute_command"}, true},
		{"a browser result shows its URL and screenshot", "browser_action_result",
			`{"text": "{\"currentUrl\": \"u\", \"screenshot\": \"x\"}"}`, false,
			uiRow{"tool_result", "u | [screenshot]", "browser_action"}, true},
		{"an empty MCP response is skipped", "mcp_server_response", `{"text": ""}`, false,
			uiRow{"tool_result", "", ""}, false},
		{"an API request names its counts", "api_req_started",
			`{"text": "{\"tokensIn\": 5, \"cost\": 0.5, \"cancelReason\": \"\"}"}`, false,
			uiRow{"system", "api_req_started: tokensIn=5 cost=0.5", ""}, true},
		{"an API request without JSON is its kind", "api_req_finished", `{"text": "x"}`, false,
			uiRow{"system", "api_req_finished", ""}, true},
		{"a condensation gives its counts and summary", "condense_context",
			`{"contextCondense": {"cost": 1, "summary": "s"}}`, false,
			uiRow{"system", "condense_context: cost=1 | s", ""}, true},
		{"an empty condensation is its kind", "condense_context", `{"contextCondense": {}}`, false,
			uiRow{"system", "condense_context", ""}, true},
		{"a checkpoint names its hash", "checkpoint_created", `{"lastCheckpointHash": "abc"}`, false,
			uiRow{"system", "checkpoint_created: abc", ""}, true},
		{"structured text is compact JSON", "error", `{"text": {"a": [1, 2.0]}}`, false,
			uiRow{"system", `error: {"a":[1,2.0]}`, ""}, true},
		{"empty reasoning is skipped", "reasoning", `{"text": ""}`, false, uiRow{"thinking", "", ""}, false},
	}
}

// The expected values are the row research/cline.md gives each message
// kind.
func TestMapUIGivesTheRowOfEachMessageKind(t *testing.T) {
	for _, test := range mapUICases() {
		t.Run(test.name, func(t *testing.T) {
			got, ok := mapUI(nil, test.kind, decode(t, test.message), test.first)
			if ok != test.ok || (ok && got != test.want) {
				t.Errorf("mapUI = %+v, %v; want %+v, %v", got, ok, test.want, test.ok)
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
	reader := NewTaskParser(parserkit.NewBase("x", "x"), `(?:(?:.*/)?User/globalStorage/saoudrizwan\.claude-dev|\.cline/data)`, "", nil)
	for _, test := range []struct {
		name, rel string
		want      bool
	}{
		{"a ui_messages.json of the extension", ".config/Code/User/globalStorage/saoudrizwan.claude-dev/tasks/1791021600000/ui_messages.json", true},
		{"an API history of the CLI", ".cline/data/tasks/1791021300000/api_conversation_history.json", true},
		{"a name in another case", ".cline/data/tasks/x/TASK_METADATA.JSON", true},
		{"a LONG S for an s", ".cline/data/taſks/x/ui_messages.json", false},
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
		fromItem                           bool
	}{
		{"a millisecond id", "1791021600000", `{"ts": 1}`, `{}`, "2026-10-03T10:00:00.000Z", false},
		{"the earliest metadata time", "abc", `{"ts": 1}`,
			`{"model_usage": [{"ts": 1791021700000}], "environment_history": [{"ts": 1791021600000}, {"ts": "x"}]}`,
			"2026-10-03T10:00:00.000Z", false},
		{"a boolean ts is not a time", "u1", `{}`,
			`{"environment_history": [{"ts": true, "os_name": "x"}, {"ts": 1767000000000, "os_name": "y"}]}`,
			"2025-12-29T09:20:00.000Z", false},
		{"the index time", "abc", `{"ts": 1791021600000}`, `{}`, "2026-10-03T10:00:00.000Z", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, fromItem := taskStart(test.taskID, decode(t, test.item), decode(t, test.metadata))
			if got != test.want || fromItem != test.fromItem {
				t.Errorf("start = %q from the index %v, want %q from the index %v", got, fromItem, test.want,
					test.fromItem)
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
			if got := context.modelAt(decode(t, test.timestamp)); got != test.want {
				t.Errorf("modelAt(%s) = %q, want %q", test.timestamp, got, test.want)
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
		{"a time between entries gives the one before it", `250`, "a2"},
		{"a tie in time is ordered by model", `100`, "b"},
		{"after every entry gives the last", `999`, "c"},
		{"a time that is not a number gives the earliest", `"x"`, "a"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := context.modelAt(decode(t, test.timestamp)); got != test.want {
				t.Errorf("modelAt(%s) = %q, want %q", test.timestamp, got, test.want)
			}
		})
	}
}

func TestTimeOfReadsAJSONNumberOnly(t *testing.T) {
	tests := []struct {
		name, json string
		want       float64
		isNumber   bool
	}{
		{"a number", "1790000000000", 1790000000000, true},
		{"true is not a number", "true", 0, false},
		{"false is not a number", "false", 0, false},
		{"text is zero", `"1790000000000"`, 0, false},
		{"null is zero", "null", 0, false},
		{"a number beyond the float64 range sorts last", "1e999", math.Inf(1), true},
		{"a negative one sorts first", "-1e999", math.Inf(-1), true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := decode(t, test.json)
			if got, ok := TimeOf(value); got != test.want || ok != test.isNumber {
				t.Errorf("TimeOf(%s) = %v %v, want %v %v", test.json, got, ok, test.want, test.isNumber)
			}
		})
	}
}
