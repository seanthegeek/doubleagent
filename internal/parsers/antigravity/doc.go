// Package antigravity reads the Antigravity CLI's (agy) and desktop app's
// conversations, histories and permissions.
//
// Under each data directory .gemini/<dir>/ (antigravity-cli for the CLI,
// antigravity for the desktop app, antigravity-ide, antigravity-backup) it
// reads the conversation databases conversations/<id>.db (SQLite holding
// protobuf blobs) and conversation_summaries.db; the brain mirror
// brain/<id>/.system_generated/logs/ (transcript_full.jsonl,
// transcript.jsonl or their chunks/, one copy per conversation; a secondary
// source); the CLI's history.jsonl (a secondary source); and the standing
// permission files .gemini/config/config.json, config/projects/<id>.json,
// config/sidecars/<name>/sidecar.json and antigravity-cli/settings.json.
//
// The protobuf messages come from the descriptors embedded in the agy
// binary; schemaText lists every field read. File: URIs are read as paths
// by evidencepath.FromFileURI. See research/antigravity.md.
package antigravity
