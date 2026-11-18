package kiro

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// parseDatabase emits the rows of data.sqlite3, or one `parser:` row for a
// database that cannot be read; a failure part way gives only that row.
func (p *Parser) parseDatabase(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	database, err := parser.OpenDatabase(artifact.DiskPath)
	if err != nil {
		emit(p.ProblemRow(artifact, parser.DatabaseError(err)))
		return
	}
	defer database.Close()
	var rows []timeline.Row
	collect := func(row timeline.Row) { rows = append(rows, row) }
	if err := p.databaseRows(artifact, options, database, collect); err != nil {
		emit(p.ProblemRow(artifact, parser.DatabaseError(err)))
		return
	}
	parser.EmitAll(emit, rows)
}

// databaseRows emits the conversation rows, then the legacy shell history
// rows, of an open database.
func (p *Parser) databaseRows(artifact *input.Artifact, options parser.Options,
	database *sqlitefile.Database, emit parser.Emit) error {
	tables, err := database.TableNames()
	if err != nil {
		return err
	}
	if tables["conversations"] {
		table, err := database.Table("conversations")
		if err != nil {
			return err
		}
		rows, err := table.Select("key", "value").OrderBy("rowid").All()
		if err != nil {
			return err
		}
		for position, record := range rows {
			value := record.Get("value")
			text := parser.NonEmptyColumn(value)
			if value.Type == sqlitefile.Blob {
				text = textlines.ValidUTF8(value.Bytes)
			}
			p.conversation(artifact, options, text, parser.NonEmptyColumn(record.Get("key")), position+1, emit)
		}
	}
	if tables["history"] {
		return p.shellHistory(artifact, database, emit)
	}
	return nil
}

// shellHistory emits one "shell history:" row per row of the legacy Fig
// shell history table.
func (p *Parser) shellHistory(artifact *input.Artifact, database *sqlitefile.Database,
	emit parser.Emit) error {
	table, err := database.Table("history")
	if err != nil {
		return err
	}
	rows, err := table.Select().OrderBy("rowid").All()
	if err != nil {
		return err
	}
	for position, record := range rows {
		start := record.Get("time")
		if record.Has("start_time") {
			start = record.Get("start_time")
		}
		text := "shell history: " + parser.NonEmptyColumn(record.Get("command"))
		extras := ""
		for _, key := range []string{"exit_code", "shell", "hostname", "duration"} {
			value := record.Get(key)
			if value.IsNull() || value.Type == sqlitefile.Text && len(value.Bytes) == 0 {
				continue
			}
			if extras != "" {
				extras += " "
			}
			extras += key + "=" + value.String()
		}
		if extras != "" {
			text += " (" + extras + ")"
		}
		row := p.SystemRow(artifact, timeline.Compact(text))
		row.SourceLine = position + 1
		row.TimestampUTC = parser.ColumnTime(start)
		row.SessionID = parser.NonEmptyColumn(record.Get("session_id"))
		row.ProjectPath = parser.NonEmptyColumn(record.Get("cwd"))
		emit(row)
	}
	return nil
}
