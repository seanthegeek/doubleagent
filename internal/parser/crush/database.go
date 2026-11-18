package crush

import (
	"strconv"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// mcpTables are the tables of MCP servers turned on and off per project.
var mcpTables = [...]string{"mcp_enabled_servers", "mcp_disabled_servers"}

// store reads one crush.db for the rows of one artifact.
type store struct {
	parser      *Parser
	artifact    *input.Artifact
	database    *sqlitefile.Database
	tables      map[string]bool
	projectPath string
	emit        parser.Emit
}

// parseDatabase emits the rows of crush.db: sessions, MCP settings,
// messages, then file versions and reads. A database error ends the rows
// with one `parser:` row; the rows before it stay.
func (p *Parser) parseDatabase(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	projectPath := p.projectOf(artifact.Original)
	problem := func(message string) {
		row := p.system(artifact, parser.ProblemPrefix+message, 0)
		row.ProjectPath = projectPath
		emit(row)
	}
	database, message := parser.OpenDatabase(artifact.DiskPath)
	if message != "" {
		problem(message)
		return
	}
	defer database.Close()
	reader := &store{parser: p, artifact: artifact, database: database, projectPath: projectPath,
		emit: emit}
	if err := reader.rows(options); err != nil {
		problem(parser.DatabaseError(err))
	}
}

// rows emits the database's rows in order, stopping at the first error.
func (s *store) rows(options parser.Options) error {
	var err error
	if s.tables, err = s.database.TableNames(); err != nil || !s.tables["messages"] {
		return err
	}
	if s.tables["sessions"] {
		if err := s.sessionRows(); err != nil {
			return err
		}
	}
	if err := s.mcpRows(); err != nil {
		return err
	}
	messages := &messageReader{store: s, options: options, models: map[string]modelState{},
		relayed: map[[2]string]bool{}}
	err = s.each("messages", []string{"created_at", "rowid"}, func(row sqlitefile.Row) {
		messages.message(row)
	})
	if err != nil {
		return err
	}
	return s.fileRows()
}

// each runs visit over the rows of table, with every column and the rowid,
// in the given order.
func (s *store) each(table string, order []string, visit func(sqlitefile.Row)) error {
	handle, err := s.database.Table(table)
	if err != nil {
		return err
	}
	columns := []string{"rowid"}
	for _, column := range handle.Columns() {
		columns = append(columns, column.Name)
	}
	return handle.Select(columns...).OrderBy(order...).Each(func(row sqlitefile.Row) error {
		visit(row)
		return nil
	})
}

// system returns a system row of the database at a rowid, with the
// project path.
func (s *store) system(rowid int64, sessionID, timestamp, text string) timeline.Row {
	row := s.parser.system(s.artifact, text, int(rowid))
	row.SessionID = sessionID
	row.ProjectPath = s.projectPath
	row.TimestampUTC = timestamp
	return row
}

// orText returns a column as row text, "" for an empty value (NULL, zero,
// or empty text or blob).
func orText(value sqlitefile.Value) string {
	if value.IsEmpty() {
		return ""
	}
	return value.String()
}

// sessionRows emits the session start, relay and deletion rows. A
// "session deleted:" row is written per parent a sub-agent session names
// that has no sessions row: the delete removes one id, and
// parent_session_id has no foreign key.
func (s *store) sessionRows() error {
	var sessions []sqlitefile.Row
	err := s.each("sessions", []string{"created_at", "rowid"}, func(row sqlitefile.Row) {
		sessions = append(sessions, row)
	})
	if err != nil {
		return err
	}
	ids := map[string]bool{}
	for _, session := range sessions {
		ids[orText(session.Get("id"))] = true
	}
	for _, session := range sessions {
		s.sessionStart(session)
	}
	orphans := map[string]bool{}
	for _, session := range sessions {
		parent := session.Get("parent_session_id")
		if parent.Type != sqlitefile.Text || len(parent.Bytes) == 0 || ids[parent.String()] ||
			orphans[parent.String()] {
			continue
		}
		orphans[parent.String()] = true
		s.emit(s.system(session.Rowid, parent.String(), updatedOrCreated(session),
			timeline.Compact("session deleted: "+parent.String()+" (parent_session_id of sub-agent "+
				"session "+session.Get("id").String()+" names no session in crush.db; time is the "+
				"sub-agent's updated_at)")))
	}
	return nil
}

// updatedOrCreated returns a session's updated_at, else its created_at.
func updatedOrCreated(session sqlitefile.Row) string {
	return timeutil.FirstNonEmpty(parser.ColumnTime(session.Get("updated_at")),
		parser.ColumnTime(session.Get("created_at")))
}

// sessionStart emits a session's start row and, for a session bound to an
// MCP channel, its relay row.
func (s *store) sessionStart(session sqlitefile.Row) {
	sessionID := orText(session.Get("id"))
	parent, count := "", ""
	if value := session.Get("parent_session_id"); !value.IsEmpty() {
		parent = "parent=" + value.String()
	}
	if value := session.Get("message_count"); !value.IsNull() {
		count = value.String() + " messages"
	}
	tokens := "tokens in=" + orZero(session.Get("prompt_tokens")) + " out=" +
		orZero(session.Get("completion_tokens"))
	cost := ""
	if value := session.Get("cost"); !value.IsEmpty() {
		cost = "cost=" + value.String()
	}
	s.emit(s.system(session.Rowid, sessionID, parser.ColumnTime(session.Get("created_at")),
		timeline.Compact(parser.JoinNonEmpty(" | ", "session start", orText(session.Get("title")), parent, count,
			tokens, cost))))
	if channel := session.Get("channel"); channel.Type == sqlitefile.Text && len(channel.Bytes) > 0 {
		s.emit(s.system(session.Rowid, sessionID, updatedOrCreated(session),
			timeline.Compact("session relayed: channel "+channel.String()+
				" (from crush.db sessions.channel, as of updated_at)")))
	}
}

// orZero returns a column as row text, "0" for an empty value.
func orZero(value sqlitefile.Value) string {
	if value.IsEmpty() {
		return "0"
	}
	return value.String()
}

// mcpRows emits one "permission setting:" row per MCP server turned on or
// off for the project, at the database file's mtime.
func (s *store) mcpRows() error {
	for _, table := range mcpTables {
		if !s.tables[table] {
			continue
		}
		handle, err := s.database.Table(table)
		if err != nil {
			return err
		}
		err = handle.Select("rowid", "name").OrderBy("rowid").Each(func(record sqlitefile.Row) error {
			s.emit(s.system(record.Rowid, "", parser.FileMtime(s.artifact),
				timeline.Compact("permission setting: "+table+"="+record.Get("name").String()+
					" (from "+s.artifact.Rel+", file mtime)")))
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// fileRows emits a "file version:" row per files snapshot, giving the
// stored content's length and never the content, and a "file read:" row per
// read.
func (s *store) fileRows() error {
	if s.tables["files"] {
		handle, err := s.database.Table("files")
		if err != nil {
			return err
		}
		err = handle.Select("rowid", "session_id", "path", "version", "created_at").
			Compute("size", func(row sqlitefile.Row) sqlitefile.Value {
				return sqlitefile.Length(row.Get("content"))
			}).OrderBy("created_at", "rowid").Each(func(record sqlitefile.Row) error {
			s.emit(s.system(record.Rowid, orText(record.Get("session_id")),
				parser.ColumnTime(record.Get("created_at")),
				timeline.Compact("file version: "+record.Get("path").String()+" version="+
					record.Get("version").String()+" bytes="+orZero(record.Get("size"))+
					" (from crush.db files)")))
			return nil
		})
		if err != nil {
			return err
		}
	}
	if !s.tables["read_files"] {
		return nil
	}
	handle, err := s.database.Table("read_files")
	if err != nil {
		return err
	}
	return handle.Select("rowid", "session_id", "path", "read_at").OrderBy("read_at", "rowid").
		Each(func(record sqlitefile.Row) error {
			s.emit(s.system(record.Rowid, orText(record.Get("session_id")),
				parser.ColumnTime(record.Get("read_at")),
				timeline.Compact("file read: "+record.Get("path").String()+
					" (from crush.db read_files, read_at)")))
			return nil
		})
}

// rowidText returns a rowid in decimal.
func rowidText(rowid int64) string {
	return strconv.FormatInt(rowid, 10)
}
