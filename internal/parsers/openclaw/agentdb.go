package openclaw

import (
	"cmp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers/zed"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// windowSet holds the session_windows rows by session id, in the order the
// ids are first met, each id's rows in table order, and the group of each
// id the table holds more than once (windowRepeat).
type windowSet struct {
	order   []string
	byID    map[string][]sqlitefile.Row
	repeats map[string]*windowRepeat
}

// agentDatabase is one agent database being read: its tables, the rows the
// passes share, and the problems met, reported at the end.
type agentDatabase struct {
	*writer
	database *sqlitefile.Database
	names    parserkit.Tables
	windows  windowSet
	store    *databaseState
	problems parserkit.Problems
	// chunksBySession holds the cold archive chunks by session id once
	// they are read.
	chunksBySession map[string][]sqlitefile.Row
	// budget is what the database's compressed blobs may still decode to.
	budget *zed.Budget
	// carried cuts the values every generation of a session copies from
	// its window, and described holds them cut, by session id, so each is
	// cut once however many generations copy it.
	carried   *parserkit.Carried
	described map[string]windowText
	// keys holds the group of each session_nodes key stored more than
	// once.
	keys map[string]*keyRepeat
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

// newAgentDatabase returns the reader of database, writing through out,
// its blobs decoded within budget.
func newAgentDatabase(out *writer, database *sqlitefile.Database, budget *zed.Budget) *agentDatabase {
	return &agentDatabase{writer: out, database: database, store: newDatabaseState(),
		windows: windowSet{byID: map[string][]sqlitefile.Row{}, repeats: map[string]*windowRepeat{}},
		budget:  budget, carried: parserkit.NewCarried(out.artifact), described: map[string]windowText{},
		keys: map[string]*keyRepeat{}}
}

// prepare reads the windows, making a group of each id stored more than
// once, attaches the relay rows and the rows of each session's metadata
// entry, and returns the session_nodes rows read.
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
			a.windows.byID[sessionID] = append(a.windows.byID[sessionID], window)
		}
		a.repeatWindows()
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
			a.emitAttached(current, attached)
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

// read runs the passes over the database in order.
func (a *agentDatabase) read() error {
	names, err := parserkit.TablesOf(a.database)
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
		{"session_nodes", func() error { a.keyNotices(nodes); return nil }},
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
	a.windowNotices()
	return nil
}

// parseAgentDatabase emits the rows of an agent database: the transcript
// events, the archived generations and the cold archives, each generation's
// rows of the other tables after its start, the tables read after the
// transcripts, the rows of generations with no transcript, and the
// problems met. A database error ends the file with its `parser:` row.
func (p *Parser) parseAgentDatabase(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) {
	database, err := parserkit.OpenDatabase(artifact, artifact.DiskPath)
	if err != nil {
		emit(p.ProblemRow(artifact, parserkit.DatabaseError(err)))
		return
	}
	defer database.Close()
	reader := newAgentDatabase(&writer{Parser: p, artifact: artifact, options: options, emit: emit},
		database, p.decoder.Budget(int64(database.PageCount())*int64(database.PageSize()), p.budgetFloor))
	if err := reader.read(); err != nil {
		emit(p.ProblemRow(artifact, parserkit.DatabaseError(err)))
		return
	}
	reader.carried.Report(0, &reader.problems)
	parserkit.EmitAll(emit, p.ErrorRows(artifact, reader.problems, "", ""))
}

// describeRow returns what a generation copies from the window row, each
// value cut ("session_windows.<column>").
func (a *agentDatabase) describeRow(window sqlitefile.Row) windowText {
	cut := func(column, value string) string { return a.carried.Cut("session_windows."+column, value) }
	described := windowText{key: cut("session_key", parserkit.NonEmptyColumn(window.Get("session_key")))}
	var columns []string
	for _, column := range []string{"channel", "account_id", "chat_type", "reason",
		"previous_session_id", "parent_session_key", "spawned_by", "display_name"} {
		value := window.Get(column)
		if !value.IsNull() && !(value.Type == sqlitefile.Text && len(value.Bytes) == 0) {
			columns = append(columns, column+"="+cut(column, value.String()))
		}
	}
	described.columns = strings.Join(columns, " ")
	described.provider = cut("model_provider", parserkit.NonEmptyColumn(window.Get("model_provider")))
	described.model = cut("model", parserkit.NonEmptyColumn(window.Get("model")))
	return described
}

// describe returns what the generations of sessionID copy from its window,
// each value cut ("session_windows.<column>") the first time it is asked
// for.
func (a *agentDatabase) describe(sessionID string) windowText {
	if described, ok := a.described[sessionID]; ok {
		return described
	}
	var window sqlitefile.Row
	if stored := a.windows.byID[sessionID]; len(stored) > 0 {
		window = stored[0]
	}
	described := a.describeRow(window)
	a.described[sessionID] = described
	return described
}

// windowSession returns the session of a generation, described by its
// window: its channel, account, chat type, reason, previous session, parent
// key, spawner and display name, then extra, then its model provider. key,
// the generation's own session key, stands before the window's. A window
// id stored more than once describes nothing: its rows take the model of
// the stored row their group goes with (windowRepeat).
func (a *agentDatabase) windowSession(sessionID, key, extra string) *session {
	if repeat := a.windows.repeats[sessionID]; repeat != nil {
		current := newSession(sessionID)
		current.key, current.extra, current.store = key, extra, a.store
		current.repeat, current.lent = repeat, true
		return current
	}
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

// contextRow returns a row of a table read after the transcripts, with its
// generation's working directory and no model.
func (a *agentDatabase) contextRow(sessionID string, line int, timestamp, turnType,
	text string) timeline.Row {
	current := newSession(sessionID)
	current.cwd = a.store.cwds[sessionID]
	return a.row(current, line, timestamp, turnType, text)
}
