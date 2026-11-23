// Package littlecoder reads little-coder's own files. little-coder is a
// launcher and extension set for pi, so its transcripts are pi session
// files that the pi parser reads; this parser reads what little-coder
// writes itself:
//
//   - .pi/agent/little-coder-prompt-history.json, a JSON array of prompts,
//     oldest first, with no time or session; a row's source line is the
//     array position.
//   - .little-coder/checkpoints/<session file name>/<sanitised path>, the
//     pre-edit copy of each file a session first edited, or an empty
//     <name>.absent marker. One row per directory, emitted on its first file
//     by name, at the earliest file modification time; the session id comes
//     from the directory name and the project from the pi session's header
//     when it was collected. Only the directory's listing and the files'
//     times are read, never their content.
//
// The retired v0.0.x releases, a separate implementation, kept their state
// in .little-coder/:
// saved sessions sessions/**/*.json (saved_at in host local time, written
// as if UTC), sessions/history.json and sessions/mr_sessions/
// session_latest.json (secondary copies), input_history.txt (secondary),
// config.json (the gist id, the Telegram chat id, the permission mode and
// the provider keys and tokens, these through parserkit.SecretText) and
// checkpoints/<session
// id>/snapshots.json. A file cut mid-write keeps the messages before the
// cut. See research/little-coder.md.
package littlecoder
