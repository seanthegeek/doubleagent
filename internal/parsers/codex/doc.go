// Package codex reads the stores of Codex CLI and the Codex Desktop app
// under ~/.codex, and of the agents that fork Codex and keep its formats
// under a home of their own (Open Interpreter).
//
// The store of record is the rollout: one JSON line per event under
// sessions/ and archived_sessions/ (rollout-*.jsonl, or zstd-compressed
// .jsonl.zst, where a cut frame keeps the lines before the cut). Beside it
// the parser reads history.jsonl, the SQLite projections state_5.sqlite (and
// its copies under db-backups/) and thread_history_1.sqlite, the desktop
// app's .codex-global-state.json, all secondary sources that repeat what the
// rollout records; the tracing logs logs_<N>.sqlite (sent /feedback reports
// and approval decisions); and the standing settings of config.toml,
// rules/*.rules, memories/rollout_summaries/*.md, a fork's import ledger and
// Codex's automations/*/automation.toml.
//
// One package holds the rollout reader and the readers of the SQLite and
// JSON stores, which need each other: the store readers need the rollout's turn-item rows and guardian texts,
// and the rollout reader needs the log databases for a guardian review's
// governed call. A fork is the same parser built with other Settings. See
// research/codex-cli.md, Parser plan, for every row.
package codex
