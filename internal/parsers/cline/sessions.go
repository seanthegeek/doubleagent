package cline

import (
	"path"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers/clinelegacy"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// blobUploadKey is the metadata key that marks a transcript uploaded to an
// enterprise's own storage.
const blobUploadKey = "enterprise.blobUpload"

// importedKeys are the metadata.importedFrom keys named on a session's
// start row, each with the name the row gives it.
func importedKeys() [][2]string {
	return [][2]string{{"tool", "imported_from"}, {"sourceSessionId", "imported_session"},
		{"sourcePath", "imported_path"}, {"importedAt", "imported_at"},
		{"sourceProvider", "imported_provider"}, {"sourceModel", "imported_model"}}
}

// startKeys are the session fields named on a session's start row.
func startKeys() []string {
	keys := []string{"source", "provider", "status", "interactive", "pid", "parent_session_id",
		"is_subagent", "title", "prompt"}
	for _, pair := range importedKeys() {
		keys = append(keys, pair[1])
	}
	return keys
}

// addMetadata copies a session's metadata title and importedFrom fields
// onto the session under the names its start row uses.
func addMetadata(session, meta jsonvalue.Value) {
	if meta.IsObject() && !meta.Get("title").IsEmpty() {
		session.Set("title", meta.Get("title"))
	}
	source := meta.Get("importedFrom")
	if !source.IsObject() {
		return
	}
	for _, pair := range importedKeys() {
		if value := source.Get(pair[0]); parserkit.Present(value) {
			session.Set(pair[1], value)
		}
	}
}

// sessionStart returns a session's start, end or index row: its time from
// timeKey, its id, project and model, and "<label>: <key=value ...>".
func (p *Parser) sessionStart(artifact *input.Artifact, session jsonvalue.Value, position int,
	timeKey, label string, keys []string) timeline.Row {
	row := p.BaseRow(artifact)
	row.SourceLine = position
	row.TimestampUTC = timeutil.FromValue(session.Get(timeKey))
	row.SessionID = parserkit.Text(session.Get("session_id"))
	row.ProjectPath = parserkit.Text(parserkit.FirstNonEmpty(session.Get("cwd"), session.Get("workspace_root")))
	row.Model = parserkit.Text(session.Get("model"))
	row.TurnType = timeline.TurnSystem
	row.Text = timeline.Compact(label + ": " + clinelegacy.FormatCounts(session, keys...))
	return row
}

// startRows emits a start row, then its model row and its blob upload row
// when the session has them.
func (p *Parser) startRows(artifact *input.Artifact, start timeline.Row, session, meta jsonvalue.Value,
	source string, emit parserkit.Emit) {
	emit(start)
	if model := parserkit.Text(session.Get("model")); model != "" {
		row := start
		row.Model = model
		row.Text = "model: " + model
		if provider := parserkit.Text(session.Get("provider")); provider != "" {
			row.Text += " provider=" + provider
		}
		row.Text = timeline.Compact(row.Text)
		emit(row)
	}
	if row, ok := blobUpload(artifact, start, meta, source); ok {
		emit(row)
	}
}

// blobUpload returns the "session synced:" row of a metadata
// enterprise.blobUpload object, with the start row's time, session and
// project and the endpoint, or false when the
// metadata has none.
func blobUpload(artifact *input.Artifact, start timeline.Row, meta jsonvalue.Value, source string) (timeline.Row, bool) {
	blob := meta.Get(blobUploadKey)
	if !blob.IsObject() {
		return timeline.Row{}, false
	}
	storage := blob.Get("storage")
	user := "unknown"
	if value := blob.Get("userDistinctId"); parserkit.Present(value) {
		user = value.Text()
	}
	row := start
	row.Text = "session synced: enterprise prompt upload to " + parserkit.SettingOrDash(storage.Get("adapterType")) +
		" bucket " + parserkit.SettingOrDash(storage.Get("bucket")) + " object sessions/" + user + "/" +
		start.SessionID + "/messages.json endpoint=" + parserkit.SettingOrDash(storage.Get("endpoint")) +
		" region=" + parserkit.SettingOrDash(storage.Get("region")) + " account=" + parserkit.SettingOrDash(storage.Get("accountId")) +
		" (from " + source + " " + blobUploadKey + ", started_at)"
	return row, true
}

// manifest emits the start, model, blob upload and end rows of a session
// manifest.
func (p *Parser) manifest(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	sessionID := path.Base(path.Dir(artifact.Rel))
	document, _ := parserkit.LoadJSON(artifact, artifact.DiskPath, &problems)
	if !document.IsObject() {
		if len(problems) == 0 {
			problems.Add(0, "session manifest is not a JSON object")
		}
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, sessionID, ""))
		return
	}
	session := parserkit.CopyObject(document)
	if !session.Has("session_id") {
		session.Set("session_id", jsonvalue.NewString(sessionID))
	}
	meta := session.Get("metadata")
	addMetadata(session, meta)
	start := p.sessionStart(artifact, session, 0, "started_at", "session start", startKeys())
	p.startRows(artifact, start, session, meta, path.Base(artifact.Rel)+" metadata", emit)
	if !session.Get("ended_at").IsEmpty() {
		emit(p.sessionStart(artifact, session, 0, "ended_at", "session end",
			[]string{"status", "exit_code"}))
	}
}
