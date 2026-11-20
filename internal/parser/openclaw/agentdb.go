package openclaw

import (
	"cmp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/zed"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// windowSet holds the session_windows rows by session id, a later row of an
// id replacing an earlier one in the earlier one's place.
type windowSet struct {
	order []string
	byID  map[string]sqlitefile.Row
}

// agentDatabase is one agent database being read: its tables, the rows the
// passes share, and the problems met, reported at the end.
type agentDatabase struct {
	*writer
	database *sqlitefile.Database
	names    parser.Tables
	windows  windowSet
	store    *databaseState
	problems parser.Problems
	// chunksBySession holds the cold archive chunks by session id once
	// they are read.
	chunksBySession map[string][]sqlitefile.Row
	// budget is what the database's compressed blobs may still decode to.
	budget *zed.Budget
	// carried cuts the values every generation of a session copies from
	// its window, and described holds them cut, by session id, so each is
	// cut once however many generations copy it.
	carried   *parser.Carried
	described map[string]windowText
}

// windowText is what each generation of a session copies from its window
// into its rows' text: the session key, the "<column>=<value>" words of the
// window's columns, the model and the model provider, each cut.
type windowText struct {
	key, columns, model, provider string
}

// table returns the rows a query over the named table gives.
func (a *agentDatabase) table(name string, query func(*sqlitefile.Table) sqlitefile.Query) ([]sqlitefile.Row, error) {
	a.Work.Add("queries", 1)
	table, err := a.database.Table(name)
	if err != nil {
		return nil, err
	}
	return query(table).All()
}

// parseAgentDatabase emits the rows of an agent database: the transcript
// events, the archived generations and the cold archives, each generation's
// rows of the other tables after its start, the tables read after the
// transcripts, the rows of generations with no transcript, and the
// problems met. A database error ends the file with its `parser:` row.
func (p *Parser) parseAgentDatabase(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	database, err := parser.OpenDatabase(artifact.DiskPath)
	if err != nil {
		emit(p.ProblemRow(artifact, parser.DatabaseError(err)))
		return
	}
	defer database.Close()
	reader := &agentDatabase{writer: &writer{Parser: p, artifact: artifact, options: options,
		emit: emit}, database: database, store: newDatabaseState(),
		windows: windowSet{byID: map[string]sqlitefile.Row{}},
		budget:  p.decoder.Budget(int64(database.PageCount())*int64(database.PageSize()), p.budgetFloor),
		carried: parser.NewCarried(artifact), described: map[string]windowText{}}
	if err := reader.read(); err != nil {
		emit(p.ProblemRow(artifact, parser.DatabaseError(err)))
		return
	}
	reader.carried.Report(0, &reader.problems)
	parser.EmitAll(emit, p.ErrorRows(artifact, reader.problems, "", ""))
}

// read runs the passes over the database in order.
func (a *agentDatabase) read() error {
	names, err := parser.TablesOf(a.database)
	if err != nil {
		return err
	}
	a.names = names
	if !names.Has("transcript_events") && names.Has("openclaw_engine_anchor") {
		a.problems.Add(0, anchorMessage(a.database, "agent"))
		return nil
	}
	nodes, err := a.prepare()
	if err != nil {
		return err
	}
	steps := []struct {
		table string
		run   func() error
	}{
		{"transcript_events", a.events},
		{"session_transcript_archives", a.archives},
		{"session_transcript_cold_archives", a.cold},
		{"session_pending_inputs", a.pending},
		{"session_transcript_fts_content", a.fullText},
		{"conversation_deliveries", func() error { return a.deliveries(nodes) }},
		{"memory_session_tombstones", a.tombstones},
	}
	for _, step := range steps {
		if names.Has(step.table) {
			if err := step.run(); err != nil {
				return err
			}
		}
	}
	a.leftovers()
	return nil
}

// prepare reads the windows, attaches the relay rows and the rows of each
// session's metadata entry, and returns the session_nodes rows read.
func (a *agentDatabase) prepare() ([]node, error) {
	if a.names.Has("session_windows") {
		rows, err := a.table("session_windows", func(table *sqlitefile.Table) sqlitefile.Query {
			return table.Select()
		})
		if err != nil {
			return nil, err
		}
		for _, window := range rows {
			sessionID := window.Get("session_id").String()
			if _, ok := a.windows.byID[sessionID]; !ok {
				a.windows.order = append(a.windows.order, sessionID)
			}
			a.windows.byID[sessionID] = window
		}
	}
	relayed, err := a.relays()
	if err != nil {
		return nil, err
	}
	var nodes []node
	if a.names.Has("session_nodes") {
		if nodes, err = a.nodes(); err != nil {
			return nil, err
		}
	}
	a.attachNodes(nodes, relayed)
	return nodes, nil
}

// leftovers emits the rows attached to generations that have no
// transcript rows in the database.
func (a *agentDatabase) leftovers() {
	for _, sessionID := range a.store.order {
		rows, ok := a.store.attached[sessionID]
		if !ok {
			continue
		}
		current := newSession(sessionID)
		current.cwd = a.store.cwds[sessionID]
		for _, attached := range rows {
			a.system(current, attached.line, attached.timestamp, attached.text, attached.model)
		}
	}
}

// anchorMessage returns the whole-file message of a PostgreSQL engine
// anchor: the openclaw_engine_anchor (store_id, engine, schema) table that
// replaces the store when database.engine is postgres
// (src/infra/database-engine/open.ts:149).
func anchorMessage(database *sqlitefile.Database, label string) string {
	store, schema := "-", "-"
	if table, err := database.Table("openclaw_engine_anchor"); err == nil {
		if row, found, err := table.Select("store_id", "schema").First(); err == nil && found {
			store, schema = row.Get("store_id").String(), row.Get("schema").String()
		}
	}
	where := "the Gateway state is"
	if label == "agent" {
		where = "transcripts are"
	}
	return label + " database is a PostgreSQL anchor (store " + store + ", schema " + schema +
		"): " + where + " on the database server"
}

// windowSession returns the session of a generation, described by its
// window: its channel, account, chat type, reason, previous session, parent
// key, spawner and display name, then extra, then its model provider. key,
// the generation's own session key, stands before the window's.
func (a *agentDatabase) windowSession(sessionID, key, extra string) *session {
	described := a.describe(sessionID)
	var parts []string
	for _, part := range []string{described.columns, extra} {
		if part != "" {
			parts = append(parts, part)
		}
	}
	if described.provider != "" {
		parts = append(parts, "model_provider="+described.provider)
	}
	current := newSession(sessionID)
	current.key = cmp.Or(key, described.key)
	current.extra = strings.Join(parts, " ")
	current.model = described.model
	current.provider = described.provider
	current.store = a.store
	return current
}

// describe returns what the generations of sessionID copy from its window,
// each value cut ("session_windows.<column>") the first time it is asked
// for.
func (a *agentDatabase) describe(sessionID string) windowText {
	if described, ok := a.described[sessionID]; ok {
		return described
	}
	window := a.windows.byID[sessionID]
	cut := func(column, value string) string { return a.carried.Cut("session_windows."+column, value) }
	described := windowText{key: cut("session_key", parser.NonEmptyColumn(window.Get("session_key")))}
	var columns []string
	for _, column := range []string{"channel", "account_id", "chat_type", "reason",
		"previous_session_id", "parent_session_key", "spawned_by", "display_name"} {
		value := window.Get(column)
		if !value.IsNull() && !(value.Type == sqlitefile.Text && len(value.Bytes) == 0) {
			columns = append(columns, column+"="+cut(column, value.String()))
		}
	}
	described.columns = strings.Join(columns, " ")
	described.provider = cut("model_provider", parser.NonEmptyColumn(window.Get("model_provider")))
	described.model = cut("model", parser.NonEmptyColumn(window.Get("model")))
	a.described[sessionID] = described
	return described
}

// contextRow returns a row of a table read after the transcripts, with its
// generation's working directory and no model.
func (a *agentDatabase) contextRow(sessionID string, line int, timestamp, turnType,
	text string) timeline.Row {
	current := newSession(sessionID)
	current.cwd = a.store.cwds[sessionID]
	return a.row(current, line, timestamp, turnType, text)
}
