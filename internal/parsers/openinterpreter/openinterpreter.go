// Package openinterpreter reads Open Interpreter, a fork of Codex CLI that
// keeps Codex's rollout format and stores under ~/.openinterpreter.
//
// The stores are Codex's (internal/parsers/codex reads every one): rollouts
// under sessions/ and archived_sessions/, plain or zstd-compressed, whose
// session_meta still names the Codex originator, so only the location tells
// the fork apart; history.jsonl, state_5.sqlite and
// thread_history_1.sqlite as secondary sources (Codex's schema up to its
// sixth migration, so every item takes its creation time); logs_2.sqlite,
// whose sent /feedback reports give "session synced:" rows; config.toml,
// rules/*.rules, memories/rollout_summaries/*.md and the state copies under
// db-backups/. What the fork adds is the /import ledger,
// external_agent_session_imports.json: each record gives one system row
// under the imported thread, whose turns were written in another agent and
// carry the import time. It has no automations and no desktop state. See
// research/open-interpreter.md.
package openinterpreter

import "github.com/seanthegeek/doubleagent/internal/parsers/codex"

// New returns the parser of Open Interpreter: the Codex parser with the
// fork's home and import ledger.
func New() *codex.Parser {
	return codex.NewRollout(codex.Settings{Agent: "open-interpreter",
		HomePrefix: ".openinterpreter", LedgerName: "external_agent_session_imports.json"})
}
