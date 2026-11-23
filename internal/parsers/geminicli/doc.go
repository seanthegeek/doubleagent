// Package geminicli reads Gemini CLI's sessions, the past-use stores beside
// them and its standing permission files, under ~/.gemini or, under the
// macOS sandbox, ~/.cache/.gemini.
//
// The sessions are tmp/<slug>/chats/**/*.jsonl, a log of messages and of
// the mutations ($set, $patch, $rewindTo) the CLI's recording service
// writes, replayed here as the CLI replays them, and the legacy whole-JSON
// .json form. The secondary sources repeat some of their events:
// logs.json, shell_history, the /chat save checkpoint-<tag>.json, the /bug
// bug-report-history-<ms>.json, the tool checkpoints checkpoints/*.json and
// a copied gemini-client-error-<type>-<time>.json. The activity log
// logs/session-<id>.jsonl (its records, not the turns in their bodies), the
// memory/ files and the clipboard images images/clipboard-<ms>.<ext> are
// primary. The standing grants are trustedFolders.json, settings.json,
// policies/*.toml with sandbox.toml, trusted_hooks.json and
// acknowledgments/agents.json.
//
// A row's project is tmp/<slug>/.project_root, else the projects.json key
// of the slug, else the key whose sha256 is the hash directory or the
// session's projectHash, else the projectHash itself. See
// research/gemini-cli.md.
//
// TrustedFolderRows reads a trustedFolders.json for Qwen Code's parser too,
// whose fork keeps the file's form.
package geminicli
