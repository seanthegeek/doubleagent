package antigravity

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// summaries emits one "conversation summary" row per row of
// conversation_summaries.db, oldest change first. A database error met
// while reading them is one problem row in their place.
func (p *Parser) summaries(artifact *input.Artifact, emit parser.Emit) {
	database := p.openDatabase(artifact, "", emit)
	if database == nil {
		return
	}
	defer database.Close()
	rows, err := p.summaryRows(artifact, database)
	if err != nil {
		parser.EmitAll(emit, p.ErrorRows(artifact, parser.Problems{{Message: parser.DatabaseError(err)}}, "", ""))
		return
	}
	parser.EmitAll(emit, rows)
}

// summaryRows returns the rows of an open summaries database.
func (p *Parser) summaryRows(artifact *input.Artifact, database *sqlitefile.Database) ([]timeline.Row, error) {
	tables, err := database.TableNames()
	if err != nil || !tables["conversation_summaries"] {
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
		row.SessionID = parser.NonEmptyColumn(summary.Get("conversation_id"))
		row.TimestampUTC = parser.ColumnTime(summary.Get("last_modified_time"))
		row.ProjectPath = firstWorkspace(summary.Get("workspace_uris"))
		row.TurnType = timeline.TurnSystem
		steps := "0"
		if table.Has("step_count") {
			steps = summary.Get("step_count").String()
		}
		title := parser.NonEmptyColumn(summary.Get("title"))
		if title == "" {
			title = parser.NonEmptyColumn(summary.Get("preview"))
		}
		row.Text = timeline.Compact(parser.JoinNonEmpty(" | ", "conversation summary", title, parser.NonEmptyColumn(summary.Get("status")),
			steps+" steps", prefixed("agent=", parser.NonEmptyColumn(summary.Get("agent_name"))),
			prefixed("parent=", parser.NonEmptyColumn(summary.Get("parent_conversation_id")))))
		rows = append(rows, row)
	}
	return rows, nil
}

// firstWorkspace returns the path of the first workspace URI of a
// summary's JSON list, "" when there is none.
func firstWorkspace(value sqlitefile.Value) string {
	if value.IsEmpty() || (value.Type != sqlitefile.Text && value.Type != sqlitefile.Blob) {
		return ""
	}
	uris, err := jsonvalue.DecodeString(textlines.ValidUTF8(value.Bytes))
	if err != nil || uris.IsObject() {
		return ""
	}
	items := parser.Elements(uris)
	if len(items) == 0 || !items[0].IsString() {
		return ""
	}
	return uriPath(items[0].StringValue())
}
