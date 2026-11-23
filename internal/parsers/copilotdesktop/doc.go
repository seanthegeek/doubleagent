// Package copilotdesktop reads the app store of GitHub Copilot Desktop.
//
// The app runs its sessions through a bundled Copilot CLI that writes them
// into .copilot/session-state/<id>/ in the CLI's own format, which the
// copilot-cli parser reads. This parser reads the app's own store only, none
// of it a secondary source, and yields system rows:
//
//   - .copilot/data.db, SQLite in WAL mode, every column possibly NULL and
//     every time ISO 8601 UTC; a missing table gives no rows. The tables are
//     accounts, settings, model_providers, projects, worktrees, workspaces,
//     sessions (the project through workspace_session_aliases and
//     workspaces.project_id), activity_items, session_checkpoints and the
//     scheduling tables. A stored GitHub token is shown through
//     parserkit.SecretText; a missing one is written as absent.
//   - .copilot/repos/<repo>/.git/refs/copilot/checkpoints/<session>/<seq>/<event>,
//     the loose git ref the app writes into its clone of a project per
//     checkpoint: one row at the file's mtime.
//
// The model is sessions.model per session; data.db keeps no per-switch
// history. See research/copilot-desktop.md.
package copilotdesktop
