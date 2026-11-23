// Package hermes reads Hermes Agent's sessions.
//
// Hermes keeps every conversation of a profile, whatever the front end, in
// one SQLite database in the OpenAI chat-completions message shape. Under
// <hermes> (.hermes<suffix> or AppData/Local/hermes<suffix>, then optionally
// profiles/<name>; empty when the base is a container volume's _data) it
// reads state.db (primary); the secondary copies
// state-snapshots/<id>/state.db, sessions/<id>.jsonl, .hermes_history,
// cron/output/<job>/<time>.md, response_store.db and a project's
// trajectory_samples.jsonl and failed_trajectories.jsonl; and
// pastes/pending.json, config.yaml (command_allowlist only),
// [platforms/]pairing/<platform>-approved.json and the desktop app's
// connections.json (in Hermes<suffix>).
//
// It yields transcript rows and model, permission, relay, deletion,
// scratch-directory, debug-upload and remote-backend system rows. Times
// without a zone (the history and cron files) are emitted as if UTC.
// config.yaml is read by a small reader of one flat block list, not a YAML
// library. See research/hermes.md.
package hermes
