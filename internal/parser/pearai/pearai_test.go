package pearai

import (
	"math/bits"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/clinelegacy"
)

// rowOf returns a content row of a turn type, text, tool name and call id.
func rowOf(turnType, text, toolName, toolUseID string) clinelegacy.ContentRow {
	return clinelegacy.ContentRow{TurnType: turnType, Text: text, ToolName: toolName, ToolUseID: toolUseID}
}

func TestXMLBlocksSplitsAssistantTextAtItsToolTags(t *testing.T) {
	for _, test := range []struct {
		name, text string
		thinking   bool
		want       []clinelegacy.ContentRow
	}{
		{"text, thinking and a tool", "Let me look.\n<thinking>\nplan\n</thinking>\n<read_file>\n<path>src/a.go</path>\n</read_file>",
			true, []clinelegacy.ContentRow{rowOf("assistant", "Let me look.", "", ""), rowOf("thinking", "plan", "", ""),
				rowOf("tool_use", "src/a.go", "read_file", "")}},
		{"thinking dropped", "Let me look.\n<thinking>\nplan\n</thinking>\n<read_file>\n<path>src/a.go</path>\n</read_file>",
			false, []clinelegacy.ContentRow{rowOf("assistant", "Let me look.", "", ""), rowOf("tool_use", "src/a.go", "read_file", "")}},
		{"an unclosed tool and parameter", "<execute_command>\n<command>ls -la</command>\n<cwd>/srv</cwd>",
			false, []clinelegacy.ContentRow{rowOf("tool_use", "ls -la | /srv", "execute_command", "")}},
		{"text after a tool", "<attempt_completion><result>done</result></attempt_completion> tail\n",
			false, []clinelegacy.ContentRow{rowOf("tool_use", "done", "attempt_completion", ""), rowOf("assistant", "tail", "", "")}},
		{"a body without parameters and an unknown tag", "<write_to_file>raw body</write_to_file><unknown>x</unknown>",
			false, []clinelegacy.ContentRow{rowOf("tool_use", "raw body", "write_to_file", ""), rowOf("assistant", "<unknown>x</unknown>", "", "")}},
		{"a parameter holding a tag", "<new_task><mode>code</mode><message>a <b>c</b></message></new_task>",
			false, []clinelegacy.ContentRow{rowOf("tool_use", "code | a <b>c</b>", "new_task", "")}},
		{"empty parameters keep the body", "<read_file><path></path></read_file>",
			false, []clinelegacy.ContentRow{rowOf("tool_use", "<path></path>", "read_file", "")}},
		{"no tags", "no tags", false, []clinelegacy.ContentRow{rowOf("assistant", "no tags", "", "")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := xmlBlocks(test.text, test.thinking); !slices.Equal(got, test.want) {
				t.Errorf("xmlBlocks = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestPromptTurnTakesTheLastUserBlock(t *testing.T) {
	for _, test := range []struct {
		name, prompt, turnType, text string
	}{
		{"a chat prompt", "<system>\nsys\n<user>\nhi\n<assistant>\nyo\n<user>\nlast\n", "user", "last\n"},
		{"plain text", "hello", "system", "model request: hello"},
		{"blocks not at the start", "x<user>\nhi\n", "system", "model request: x<user>\nhi\n"},
		{"no user block", "<system>\nonly\n", "system", "model request: <system>\nonly\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			turnType, text := promptTurn(test.prompt)
			if turnType != test.turnType || text != test.text {
				t.Errorf("promptTurn = %q, %q; want %q, %q", turnType, text, test.turnType, test.text)
			}
		})
	}
}

func TestTheResultPatternTakesAWholeToolNameAndTheTaskItsTags(t *testing.T) {
	for _, test := range []struct {
		name, text, want string
		task             bool
	}{
		{"a result with its argument", "[read_file for 'a.go'] Result:", "read_file", false},
		{"a bare result", "[read_file] Result:", "read_file", false},
		{"a letter outside ASCII continues the name", "[read_fileé] Result:", "", false},
		{"a digit continues the name", "[read_file1] Result:", "", false},
		{"an upper-case name", "[Read_file] Result:", "", false},
		{"a result over two lines", "[read_file]\nx] Result:", "read_file", false},
		{"a task with spaces", "<task>\n do it \n</task>\n", "do it", true},
		{"a task between Unicode spaces", " <task>x</task> ", "x", true},
		{"the last closing tag", "<task>a</task> b </task>", "a</task> b", true},
		{"text before the task", "pre <task>x</task>", "", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			pattern := resultPattern
			if test.task {
				pattern = taskPattern
			}
			got := ""
			if match := pattern.FindStringSubmatch(test.text); match != nil {
				got = match[1]
			}
			if got != test.want {
				t.Errorf("group = %q, want %q", got, test.want)
			}
		})
	}
}

func TestAPIContentRowsReadsRooToolResultsAndCalls(t *testing.T) {
	user, err := jsonvalue.DecodeString(`{"role": "user", "content": [
		{"type": "text", "text": "[read_file for 'a.go'] Result:"},
		{"type": "text", "text": "The user denied this operation."},
		{"type": "image"},
		{"type": "text", "text": "Skipping tool [write_to_file] because a tool was already used"},
		{"type": "text", "text": "<environment_details>\nenv\n</environment_details>"},
		{"type": "text", "text": "<task>\nhello\n</task>"},
		"plain"]}`)
	if err != nil {
		t.Fatal(err)
	}
	want := []clinelegacy.ContentRow{rowOf("tool_result", "The user denied this operation.\n[image]", "read_file", "2"),
		rowOf("system", "permission decision: denied", "read_file", "2"),
		rowOf("tool_result", "Skipping tool [write_to_file] because a tool was already used", "write_to_file", "2"),
		rowOf("system", "<environment_details>\nenv\n</environment_details>", "", ""),
		rowOf("user", "hello", "", ""), rowOf("user", "plain", "", "")}
	parser := New()
	if got := parser.APIContentRows(user, 3, "user", false); !slices.Equal(got, want) {
		t.Errorf("user rows = %+v, want %+v", got, want)
	}
	assistant, err := jsonvalue.DecodeString(`{"role": "assistant", "content": [
		{"type": "text", "text": "Let me look.\n<thinking>\nplan\n</thinking>\n<read_file>\n<path>src/a.go</path>\n</read_file><list_files><path>.</path></list_files>"},
		{"type": "thinking", "thinking": "t"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	want = []clinelegacy.ContentRow{rowOf("assistant", "Let me look.", "", ""), rowOf("thinking", "plan", "", ""),
		rowOf("tool_use", "src/a.go", "read_file", "2"), rowOf("tool_use", ".", "list_files", "2.2"),
		rowOf("thinking", "t", "", "")}
	if got := parser.APIContentRows(assistant, 2, "assistant", true); !slices.Equal(got, want) {
		t.Errorf("assistant rows = %+v, want %+v", got, want)
	}
}

func TestPermissionDecisionReadsRoosTexts(t *testing.T) {
	for _, test := range []struct{ text, want string }{
		{"  The user denied this operation.", "denied"},
		{"The user denied this operation and provided the following feedback: no", "denied with feedback"},
		{"x", ""},
	} {
		if got := permissionDecision(test.text); got != test.want {
			t.Errorf("permissionDecision(%q) = %q, want %q", test.text, got, test.want)
		}
	}
}

func FuzzXMLBlocksKeepsEveryTag(fuzz *testing.F) {
	for _, seed := range []string{"<read_file><path>a</path></read_file>", "<thinking>x", "<<execute_command>>", ""} {
		fuzz.Add(seed)
	}
	fuzz.Fuzz(func(t *testing.T, text string) {
		for _, block := range xmlBlocks(text, true) {
			if block.TurnType == "tool_use" && block.ToolName == "" {
				t.Errorf("xmlBlocks(%q) gave a tool row without a tool", text)
			}
		}
	})
}

func TestTheOutputLogCountsEachByteOnceForItsLineNumbers(t *testing.T) {
	separator := strings.Repeat("=", 74) + "\n" + strings.Repeat("=", 74) + "\n"
	for _, writes := range []int{1000, 2000, 4000} {
		t.Run(strconv.Itoa(writes)+" writes", func(t *testing.T) {
			text := strings.Repeat(separator+"Completion:\nok\n", writes)
			lines := &lineCounter{text: text, line: 1, work: parser.CountingWork()}
			for i, location := range logSeparator.FindAllStringIndex(text, -1) {
				if line, want := lines.lineAt(location[1]), 4*i+3; line != want {
					t.Fatalf("write %d at line %d, want %d", i+1, line, want)
				}
			}
			if counted := lines.work.Count("bytes counted"); counted == 0 || counted > len(text) {
				t.Errorf("%d bytes counted in a text of %d", counted, len(text))
			}
		})
	}
}

// decoded returns JSON text decoded, failing the test on an error.
func decoded(t *testing.T, text string) jsonvalue.Value {
	t.Helper()
	value, err := jsonvalue.DecodeString(text)
	if err != nil {
		t.Fatal(err)
	}
	return value
}

func TestALookupOfAnAskTimeIsABinarySearch(t *testing.T) {
	asks := &askIndex{}
	for _, stamp := range []string{"10", "20", "20", "30"} {
		asks.times = append(asks.times, decoded(t, stamp))
	}
	tests := []struct{ name, timestamp, want string }{
		{"before every ask", "5", ""},
		{"at an ask", "10", "10"},
		{"between two asks", "25", "20"},
		{"after every ask", "99", "30"},
		{"a time that is not a number", `"25"`, ""},
		{"a time past the float range is after every ask", `1e400`, "30"},
		{"a negative time past the float range is before every ask", `-1e400`, ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := asks.lastBefore(decoded(t, test.timestamp)); got != test.want {
				t.Errorf("lastBefore(%s) = %q, want %q", test.timestamp, got, test.want)
			}
		})
	}
	for _, count := range []int{1024, 2048, 4096} {
		t.Run(strconv.Itoa(count)+" asks", func(t *testing.T) {
			many := &askIndex{work: parser.CountingWork()}
			for i := range count {
				many.times = append(many.times, jsonvalue.NewInt(int64(i)))
			}
			for i := range count {
				many.lastBefore(jsonvalue.NewInt(int64(i)))
			}
			compared, limit := many.work.Count("asks compared"), count*(bits.Len(uint(count))+1)
			if compared == 0 || compared > limit {
				t.Errorf("%d asks compared over %d lookups, want at most %d", compared, count, limit)
			}
		})
	}
}

// testdata/state.vscdb is built by testdata/gen_state.py.
func TestATaskFindsItsHistoryEntryThroughOneIndexOfTheState(t *testing.T) {
	home := t.TempDir()
	data, err := osfile.ReadFile(filepath.Join("testdata", "state.vscdb"))
	if err != nil {
		t.Fatal(err)
	}
	state := filepath.Join(home, "User", "globalStorage", "state.vscdb")
	if err := osfile.MkdirAll(filepath.Dir(state), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := osfile.WriteFile(state, data, 0o600); err != nil {
		t.Fatal(err)
	}
	subject := New()
	subject.Work = parser.CountingWork()
	task := func(taskID string) clinelegacy.Task {
		rel := "User/globalStorage/pearai.pearai-roo-cline/tasks/" + taskID + "/ui_messages.json"
		return clinelegacy.TaskOf(&input.Artifact{Rel: rel, DiskPath: filepath.Join(home, filepath.FromSlash(rel)),
			Home: &input.Home{DiskPath: home}})
	}
	tests := []struct{ name, taskID, want string }{
		{"a task in the history", "t5", "task 5"},
		{"the first entry of an id stands", "t7", "task 7"},
		{"a task missing from the history", "t999", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			entry, _ := subject.HistoryItem(task(test.taskID))
			if got := entry.Get("task").Text(); got != test.want {
				t.Errorf("task %q, want %q", got, test.want)
			}
		})
	}
	for i := range 200 {
		subject.HistoryItem(task("t" + strconv.Itoa(i)))
	}
	if read := subject.Work.Count("history entries read"); read != 202 {
		t.Errorf("%d history entries examined for 203 tasks, want the 202 entries once", read)
	}
}
