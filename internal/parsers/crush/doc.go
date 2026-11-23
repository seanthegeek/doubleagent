// Package crush reads Crush's per-project SQLite database, its project
// registry and its permission settings.
//
// It reads <project>/.crush/crush.db (WAL mode; the artifact's rel is
// .crush/crush.db under a project or a home, and the project path is the
// directory holding .crush, since the database records no working
// directory), projects.json under .local/share/crush or AppData/Local/crush,
// every crush.json scope and the crushrc shell configs.
//
// The database's tables read are sessions, messages (whose parts column is
// a JSON array of {type, data}), mcp_enabled_servers, mcp_disabled_servers,
// files (the content's length only) and read_files. Parts carry no time:
// they take the message's created_at, except tool calls and results, at
// finished_at or the finish part's time. A finish part is a row only when
// the turn ended abnormally or carries a message. Beside the turns the
// database gives "model:", "permission decision:", "session relayed:",
// "session deleted:", "file version:" and "file read:" rows. projects.json
// gives one row per project; crush.json and the crushrc "permissions allow|deny"
// lines give "permission setting:" rows at the file's mtime, and crush.json
// a "stored credentials:" row of its providers' API keys. No git branch
// is recorded. See research/crush.md.
package crush
