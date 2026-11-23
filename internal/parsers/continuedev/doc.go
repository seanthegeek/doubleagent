// Package continuedev reads the sessions, events and logs of Continue, its
// IDE extensions and the cn CLI, under .continue/.
//
// It reads sessions/<sessionId>.json and the sessions/sessions.json index
// (rows only for what the index lost); dev_data/[<schema>/]
// chatInteraction, toolUsage, editInteraction, editOutcome and chatFeedback
// (secondary) .jsonl files; input_history.json (secondary);
// permissions.yaml, a flat subset of YAML read by a small reader of that
// shape alone; and logs/cn*.log.
//
// Session files carry no per-message time: every row of a session takes
// the index's dateCreated (empty when the index lacks the session), except
// thinking rows at reasoning.startAt, and source_line is the position in
// history[]. A cn.log time is host-local with no zone and is written as if
// it were UTC. See research/continue.md.
//
// Index, LoadSession and MessageText are shared with the PearAI parser,
// whose chat surface is a fork of Continue's.
package continuedev
