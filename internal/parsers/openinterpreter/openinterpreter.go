package openinterpreter

import "github.com/seanthegeek/doubleagent/internal/parsers/codex"

// New returns the parser of Open Interpreter: the Codex parser with the
// fork's home and import ledger.
func New() *codex.Parser {
	return codex.NewRollout(codex.Settings{Agent: "open-interpreter",
		HomePrefix: ".openinterpreter", LedgerName: "external_agent_session_imports.json"})
}
