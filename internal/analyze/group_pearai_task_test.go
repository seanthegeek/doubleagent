package analyze

import (
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile/sqlitetest"
)

// pearaiEditor is the PearAI editor's user directory.
const pearaiEditor = ".config/PearAI/User/globalStorage/"

// pearaiRepeatFiles returns a PearAI editor state whose agent extension's
// taskHistory holds the task t1 twice, and the task's API history.
func pearaiRepeatFiles(t *testing.T) []collectedFile {
	state := buildDatabase(t, sqlitetest.Table{Name: "ItemTable",
		SQL: "CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB)", Rows: [][]any{
			{"pearai.pearai-roo-cline", `{"taskHistory":[` + repeatEntries("workspace") + `]}`}}})
	return []collectedFile{
		{"alice", "/home/alice", "pearai", pearaiEditor + "state.vscdb", state},
		{"alice", "/home/alice", "pearai", pearaiEditor + "pearai.pearai-roo-cline/tasks/t1/api_conversation_history.json",
			repeatTaskHistory}}
}

func TestTheRowsOfARepeatedPearAITaskGoWithTheFirstHistoryEntryTheFilterKeeps(t *testing.T) {
	t.Parallel()
	checkRepeatCases(t, "pearai", pearaiRepeatFiles, familyCases(familyTaskA, familyTaskB))
}
