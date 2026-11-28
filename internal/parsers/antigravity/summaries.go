package antigravity

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// firstWorkspace returns the path of the first workspace URI of a
// summary's JSON list, "" when there is none, noting the list's repeated
// member names in the parse of artifact.
func firstWorkspace(artifact *input.Artifact, value sqlitefile.Value) string {
	if value.IsEmpty() || (value.Type != sqlitefile.Text && value.Type != sqlitefile.Blob) {
		return ""
	}
	uris, err := parserkit.DecodeJSON(artifact, textlines.ValidUTF8(value.Bytes))
	if err != nil || uris.IsObject() {
		return ""
	}
	items := uris.Items()
	if len(items) == 0 || !items[0].IsString() {
		return ""
	}
	return uriPath(items[0].StringValue())
}

// summaryRows returns the rows of an open summaries database.
func (p *Parser) summaryRows(artifact *input.Artifact, database *sqlitefile.Database) ([]timeline.Row, error) {
	tables, err := parserkit.TablesOf(database)
	if err != nil || !tables.Has("conversation_summaries") {
		return nil, err
	}
	table, err := database.Table("conversation_summaries")
	if err != nil {
		return nil, err
	}
	stored, err := table.Select().OrderBy("last_modified_time").All()
	if err != nil {
		return nil, err
	}
	rows := make([]timeline.Row, 0, len(stored))
	for position, summary := range stored {
		row := p.BaseRow(artifact)
		row.SourceLine = position + 1
		row.SessionID = parserkit.NonEmptyColumn(summary.Get("conversation_id"))
		row.TimestampUTC = parserkit.ColumnTime(summary.Get("last_modified_time"))
		row.ProjectPath = firstWorkspace(artifact, summary.Get("workspace_uris"))
		row.TurnType = timeline.TurnSystem
		steps := "0"
		if table.Has("step_count") {
			steps = summary.Get("step_count").String()
		}
		title := parserkit.NonEmptyColumn(summary.Get("title"))
		if title == "" {
			title = parserkit.NonEmptyColumn(summary.Get("preview"))
		}
		row.Text = timeline.Compact(parserkit.JoinNonEmpty(" | ", "conversation summary", title, parserkit.NonEmptyColumn(summary.Get("status")),
			steps+" steps", prefixed("agent=", parserkit.NonEmptyColumn(summary.Get("agent_name"))),
			prefixed("parent=", parserkit.NonEmptyColumn(summary.Get("parent_conversation_id")))))
		rows = append(rows, row)
	}
	return rows, nil
}

// summaries emits one "conversation summary" row per row of
// conversation_summaries.db, oldest change first. A database error met
// while reading them is one problem row in their place.
func (p *Parser) summaries(artifact *input.Artifact, emit parserkit.Emit) {
	database := p.openDatabase(artifact, "", emit)
	if database == nil {
		return
	}
	defer database.Close()
	rows, err := p.summaryRows(artifact, database)
	if err != nil {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, parserkit.Problems{{Message: parserkit.DatabaseError(err)}}, "", ""))
		return
	}
	parserkit.EmitAll(emit, rows)
}
