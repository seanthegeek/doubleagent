package analyze

import "testing"

// kiloTasks is the pre-fork Kilo Code extension's task directory.
const kiloTasks = ".config/Code/User/globalStorage/kilocode.kilo-code/tasks/"

// kiloRepeatFiles returns a pre-fork Kilo Code tasks/_index.json holding
// the task t1 twice, an old entry in /proj/a and a new one in /proj/b, and
// the task's API history.
func kiloRepeatFiles(*testing.T) []collectedFile {
	return []collectedFile{
		{"alice", "/home/alice", "kilo-code", kiloTasks + "_index.json", `{"entries":[` + repeatEntries("workspace") + `]}`},
		{"alice", "/home/alice", "kilo-code", kiloTasks + "t1/api_conversation_history.json", repeatTaskHistory}}
}

func TestTheRowsOfARepeatedKiloCodeTaskGoWithTheFirstIndexEntryTheFilterKeeps(t *testing.T) {
	t.Parallel()
	checkRepeatCases(t, "kilo-code", kiloRepeatFiles, familyCasesWith(
		"2026-01-01T00:00:00.000Z t1 /proj/a  task: TASK-A", "2026-06-01T00:00:00.000Z t1 /proj/b  task: TASK-B",
		"/proj/a ", "/proj/b "))
}
