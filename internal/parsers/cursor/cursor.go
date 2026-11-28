package cursor

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// ide matches the IDE's data directory relative to the home.
const ide = `(?:Library/Application Support|\.config|AppData/Roaming)/Cursor`

// ideParents are the IDE's data directories, in the order the home's
// state.vscdb is looked for.
func ideParents() []string {
	return []string{"Library/Application Support/Cursor", ".config/Cursor", "AppData/Roaming/Cursor"}
}

// The files read, by path relative to the home.
var (
	statePattern       = parserkit.MustRelPattern(`^` + ide + `/User/globalStorage/state\.vscdb\z`)
	searchPattern      = parserkit.MustRelPattern(`^` + ide + `/User/globalStorage/conversation-search\.db\z`)
	transcriptPattern  = parserkit.MustRelPattern(`^\.cursor/projects/[^/]+/agent-transcripts/[^/]+/([^/]+)\.jsonl\z`)
	trackingPattern    = parserkit.MustRelPattern(`^\.cursor/ai-tracking/ai-code-tracking\.db\z`)
	cliConfigPattern   = parserkit.MustRelPattern(`^\.cursor/cli-config\.json\z`)
	permissionsPattern = parserkit.MustRelPattern(`^\.cursor/permissions\.json\z`)
	attemptPattern     = parserkit.MustRelPattern(`^` + ide + `/User/globalStorage/mcp-oauth-attempts/([^/]+\.json)\z`)
)

// transcriptTolerance is the matching window of a transcript's rows: its
// time tag has minute precision.
const transcriptTolerance = 60.0

// Parser reads the Cursor IDE's stores, transcripts and settings files. It
// keeps the projects and models of the last home state.vscdb read, and the
// group of each composer id a home state.vscdb repeats, by the database's
// path and the id, which its parse and its side files' parses share.
type Parser struct {
	parserkit.Base
	index  *stateIndex
	groups map[[2]string]*timeline.Group
	// cloudGroups is the group of each cloud agent bcId a home's
	// state.vscdb repeats, kept as groups is.
	cloudGroups map[[2]string]*timeline.Group
}

// New returns the parser.
func New() *Parser {
	return &Parser{Base: parserkit.NewBase("cursor", "cursor")}
}

// Marks says that Cursor records which sessions are synced to the vendor
// and which began on its servers.
func (p *Parser) Marks() parserkit.Marks {
	return parserkit.Marks{Sync: true, Origin: true}
}

// Kinds returns the state database, the search database, the agent
// transcripts, the tracking database, the CLI config, the permissions and
// the MCP sign-in attempts.
func (p *Parser) Kinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.PatternKind("state.vscdb", statePattern),
		parserkit.PatternKind("conversation-search.db", searchPattern),
		parserkit.PatternKind("agent transcript", transcriptPattern),
		parserkit.PatternKind("ai-code-tracking.db", trackingPattern), parserkit.PatternKind("cli-config.json", cliConfigPattern),
		parserkit.PatternKind("permissions.json", permissionsPattern),
		parserkit.PatternKind("mcp-oauth attempt", attemptPattern)}
}

// Wants reports whether the artifact is one of the files the parser reads.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Secondary reports whether the artifact is an agent transcript, which
// repeats the store's events.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return transcriptPattern.MatchString(artifact.Rel)
}

// SecondaryTolerance returns 60 seconds: the transcript's tag has minute
// precision.
func (p *Parser) SecondaryTolerance(*input.Artifact) float64 {
	return transcriptTolerance
}

// row returns a row of the artifact at a time and source line.
func (p *Parser) row(artifact *input.Artifact, turnType, text, timestamp string, line int64) timeline.Row {
	row := p.BaseRow(artifact)
	row.TurnType, row.Text, row.TimestampUTC, row.SourceLine = turnType, text, timestamp, int(line)
	return row
}

// newStore returns the store of an open database, counting its queries in
// work.
func newStore(artifact *input.Artifact, database *sqlitefile.Database, work parserkit.Work) *store {
	created := &store{artifact: artifact, database: database, work: work}
	created.tables, created.err = parserkit.TablesOf(database)
	return created
}

// database emits the rows read reads from the artifact's database. The
// rows are gathered first: a database error met while reading them is one
// problem row in their place.
func (p *Parser) database(artifact *input.Artifact, emit parserkit.Emit, read func(*store, parserkit.Emit)) {
	database, err := parserkit.OpenDatabase(artifact, artifact.DiskPath)
	if err != nil {
		emit(p.row(artifact, timeline.TurnSystem, parserkit.ProblemPrefix+parserkit.DatabaseError(err), "", 0))
		return
	}
	defer database.Close()
	store := newStore(artifact, database, p.Work)
	var rows []timeline.Row
	read(store, func(row timeline.Row) { rows = append(rows, row) })
	if store.err != nil {
		emit(p.row(artifact, timeline.TurnSystem, parserkit.ProblemPrefix+parserkit.DatabaseError(store.err), "", 0))
		return
	}
	parserkit.EmitAll(emit, rows)
}

// Parse emits the rows of one artifact, by the file it is.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	rel := artifact.Rel
	switch {
	case statePattern.MatchString(rel):
		p.database(artifact, emit, func(store *store, add parserkit.Emit) { p.state(artifact, store, options, add) })
	case searchPattern.MatchString(rel):
		p.database(artifact, emit, func(store *store, add parserkit.Emit) { p.search(artifact, store, add) })
	case trackingPattern.MatchString(rel):
		p.database(artifact, emit, func(store *store, add parserkit.Emit) { p.tracking(artifact, store, add) })
	case transcriptPattern.MatchString(rel):
		p.transcript(artifact, emit)
	case attemptPattern.MatchString(rel):
		p.attempt(artifact, emit)
	default:
		p.settingsFile(artifact, emit)
	}
	return nil
}

// store reads the tables of one database, keeping the first error, after
// which every read gives nothing.
type store struct {
	// artifact is the parse the JSON of the database notes its repeated
	// member names in, nil for a lookup in a database that is parsed as
	// its own artifact.
	artifact *input.Artifact
	database *sqlitefile.Database
	tables   parserkit.Tables
	err      error
	// work counts the queries run, which a test measures.
	work parserkit.Work
}

// systemAt returns a system row of the artifact whose time is a formatted
// timestamp.
func (p *Parser) systemAt(artifact *input.Artifact, text, timestamp string, line int64) timeline.Row {
	return p.row(artifact, timeline.TurnSystem, text, timeutil.FromText(timestamp), line)
}

// keep records err when it is the first, and reports whether the store has
// failed.
func (s *store) keep(err error) bool {
	if s.err == nil {
		s.err = err
	}
	return s.err != nil
}

// table returns the named table, nil when it is absent or the store has
// failed.
func (s *store) table(name string) *sqlitefile.Table {
	if s.err != nil || !s.tables.Has(name) {
		return nil
	}
	table, err := s.database.Table(name)
	if s.keep(err) {
		return nil
	}
	return table
}

// rows returns rows read, nothing when the read failed.
func (s *store) rows(found []keyed, err error) []keyed {
	s.work.Add("queries", 1)
	if s.keep(err) {
		return nil
	}
	return found
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.Marker          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
	_ parserkit.ToleranceSetter = (*Parser)(nil)
)
