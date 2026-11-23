package parserregistry

import (
	"strconv"
	"testing"
)

// The Open Interpreter growth cases: Codex's cases under the fork's own
// home, and the fork's import ledger.

// openinterpreterLedger is the import ledger's path under the home.
const openinterpreterLedger = ".openinterpreter/external_agent_session_imports.json"

// openinterpreterRecord returns an import ledger record of the imported
// thread from the source file.
func openinterpreterRecord(thread, source string) string {
	return `{"source_path":"` + source + `","content_sha256":"abc","imported_thread_id":"` + thread +
		`","imported_at":1767312001,"source_modified_at":1767312000}`
}

// openinterpreterGrowth returns the open-interpreter growth set.
func openinterpreterGrowth() growthSet {
	ledger := []growthCase{
		{kind: "import ledger", shape: shapeSharedID, files: func(t *testing.T, n int) []growthFile {
			return one(openinterpreterLedger, []byte(`{"records":[`+joined(n, func(i int) string {
				return openinterpreterRecord("same", "/s"+strconv.Itoa(i)+".jsonl")
			})+`]}`))
		}},
		{kind: "import ledger", shape: shapeNested, n: 2048, files: func(t *testing.T, n int) []growthFile {
			return one(openinterpreterLedger, []byte(`{"records":[`+joined(n, func(i int) string {
				return strconv.Itoa(i)
			})+`,`+openinterpreterRecord("t", "/s.jsonl")+`,{"source_path":`+nestedArrays(n, `"x"`)+`}]}`))
		}},
	}
	return growthSet{parser: "open-interpreter", cases: append(codexCases(".openinterpreter"), ledger...)}
}
