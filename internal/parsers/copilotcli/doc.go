// Package copilotcli reads the session state of the GitHub Copilot CLI.
//
// It reads, under .copilot (or .local/state/.copilot and .config/.copilot on
// installs from before the XDG move): session-state/<id>/events.jsonl (the
// primary store), session-state/<id>/workspace.yaml, and as secondary
// sources session-store.db, session-state/<id>/rewind-file-snapshots/index.json
// and command-history-state.json; the standing grants of
// permissions-config.json and settings.json (JSON with comments);
// sidebar-sessions-state/*.json (a session-to-cwd join, no rows); and
// logs/process-*.log. See research/copilot-cli.md for the fields each
// event, file and table carries; the Copilot Desktop app's bundled CLI
// writes session.permissions_changed with the allow-all keys instead of
// mode.
package copilotcli
