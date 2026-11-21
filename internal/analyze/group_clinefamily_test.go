package analyze

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parsers"
)

// familyRunOf runs timeline over a collection holding the files, for the
// agent given, with the options given, and returns each row with a session
// of timeline.jsonl as "time session project model text".
func familyRunOf(t *testing.T, agent string, files []collectedFile, options ...string) []string {
	t.Helper()
	work := t.TempDir()
	collection := writeCollection(t, filepath.Join(work, "col"),
		[]jsonvalue.Member{member("hostname", jsonvalue.NewString("h1"))}, files)
	env := Environment{Now: func() time.Time { return time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC) },
		Parsers: parsers.New, Version: "0.0.0-test"}
	output := filepath.Join(work, "out")
	args := append(append([]string{"timeline", "--agent", agent}, options...), collection, output)
	code, _, stderr := runCommand(env, args...)
	if code != ExitFound {
		t.Fatalf("%q exited %d: %s", args, code, stderr)
	}
	return recordsOf(t, filepath.Join(output, TimelineFile), "session_id", "timestamp_utc", "session_id",
		"project_path", "model", "text")
}

// checkRepeatCases runs each case over the files files gives, in the order
// given and reversed, and compares the rows with the case's, each sorted:
// rows of one time come in the order of their files, which is not what the
// cases test.
func checkRepeatCases(t *testing.T, agent string, files func(t *testing.T) []collectedFile, cases []repeatCase) {
	t.Helper()
	for _, order := range []string{"in path order", "in reverse order"} {
		for _, test := range cases {
			t.Run(test.name+" "+order, func(t *testing.T) {
				listed := files(t)
				if order == "in reverse order" {
					slices.Reverse(listed)
				}
				rows, want := familyRunOf(t, agent, listed, test.options...), slices.Clone(test.want)
				slices.Sort(rows)
				slices.Sort(want)
				if !slices.Equal(rows, want) {
					t.Errorf("rows\n%s\nwant\n%s", strings.Join(rows, "\n"), strings.Join(want, "\n"))
				}
			})
		}
	}
}

// The storage roots of Cline's and Roo Code's task files in an editor.
const (
	clineRoot = ".config/Code/User/globalStorage/saoudrizwan.claude-dev/"
	rooRoot   = ".config/Code/User/globalStorage/rooveterinaryinc.roo-cline/"
)

// repeatEntries returns two task index entries of the task t1: an old one in
// /proj/a with the model m-a, and a new one in /proj/b with m-b, the project
// under the key given.
func repeatEntries(projectKey string) string {
	return `{"id":"t1","ts":1767225600000,"task":"TASK-A","` + projectKey + `":"/proj/a","modelId":"m-a"},` +
		`{"id":"t1","ts":1780272000000,"task":"TASK-B","` + projectKey + `":"/proj/b","modelId":"m-b"}`
}

// repeatTaskHistory returns the API history of the task t1: a user message
// a day after the new entry, and an assistant reply with no time of its own.
const repeatTaskHistory = `[{"role":"user","ts":1780358400000,"content":[{"type":"text","text":"USER-TEXT"}]},` +
	`{"role":"assistant","content":[{"type":"text","text":"REPLY-TEXT"}]}]`

// clineRepeatFiles returns a Cline task index holding the task t1 twice and
// the task's API history.
func clineRepeatFiles(*testing.T) []collectedFile {
	return []collectedFile{
		{"alice", "/home/alice", "cline", clineRoot + "state/taskHistory.json",
			"[" + repeatEntries("cwdOnTaskInitialization") + "]"},
		{"alice", "/home/alice", "cline", clineRoot + "tasks/t1/api_conversation_history.json", repeatTaskHistory}}
}

// The rows of the stores above: each entry's task row, and the task's rows
// with the attributes of one entry (the reply at that entry's time).
const (
	familyTaskA = "2026-01-01T00:00:00.000Z t1 /proj/a m-a task: TASK-A |"
	familyTaskB = "2026-06-01T00:00:00.000Z t1 /proj/b m-b task: TASK-B |"
)

// familyMembers returns the task's rows with the project and model of one
// entry, the reply at that entry's time.
func familyMembers(entryTime, attributes string) []string {
	return []string{entryTime + " t1 " + attributes + " REPLY-TEXT",
		"2026-06-02T00:00:00.000Z t1 " + attributes + " USER-TEXT"}
}

// familyCases returns the runs over a store whose index repeats the task t1,
// and the rows each gives, the task's rows taking each entry's project and
// model.
func familyCases(taskA, taskB string) []repeatCase {
	return familyCasesWith(taskA, taskB, "/proj/a m-a", "/proj/b m-b")
}

// familyCasesWith returns the runs familyCases does, the task's rows taking
// the attributes given of each entry.
func familyCasesWith(taskA, taskB, attributesA, attributesB string) []repeatCase {
	first := familyMembers("2026-01-01T00:00:00.000Z", attributesA)
	second := familyMembers("2026-06-01T00:00:00.000Z", attributesB)
	return []repeatCase{
		{"without a filter they go with the first entry of the id", nil,
			[]string{taskA, first[0], taskB, first[1]}},
		{"a window past the first entry moves them to the second", []string{"--since", "2026-03-01"},
			[]string{taskB, second[0], second[1]}},
		{"a pattern only the second entry and the rows match moves them to the second",
			[]string{"--match", "TASK-B|-TEXT"}, []string{taskB, second[0], second[1]}},
		{"a pattern only the first entry and the rows match keeps them with the first",
			[]string{"--match", "TASK-A|-TEXT"}, []string{taskA, first[0], first[1]}},
		{"a pattern neither entry matches keeps them with the first", []string{"--match", "-TEXT"}, first},
	}
}

func TestTheRowsOfARepeatedClineTaskGoWithTheFirstIndexEntryTheFilterKeeps(t *testing.T) {
	checkRepeatCases(t, "cline", clineRepeatFiles, familyCases(familyTaskA, familyTaskB))
}

// rooRepeatFiles returns a Roo Code task index holding the task t1 twice,
// with no history_item.json, and the task's API history.
func rooRepeatFiles(*testing.T) []collectedFile {
	return []collectedFile{
		{"alice", "/home/alice", "roo-code", rooRoot + "tasks/_index.json",
			`{"version":1,"entries":[` + repeatEntries("workspace") + `]}`},
		{"alice", "/home/alice", "roo-code", rooRoot + "tasks/t1/api_conversation_history.json", repeatTaskHistory}}
}

func TestTheRowsOfARepeatedRooCodeTaskGoWithTheFirstIndexEntryTheFilterKeeps(t *testing.T) {
	checkRepeatCases(t, "roo-code", rooRepeatFiles, familyCases(familyTaskA, familyTaskB))
}
