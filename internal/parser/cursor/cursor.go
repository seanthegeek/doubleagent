// Package cursor reads the Cursor IDE's agent conversations and standing
// settings.
//
// Cursor's names: a composer is one conversation, a bubble one message,
// reasoning block or tool call in it, toolFormerData a tool call's record,
// yolo auto-run, and a cloud agent (bc-<uuid>) a conversation run on
// Cursor's servers. <ide> is Library/Application Support/Cursor,
// .config/Cursor or AppData/Roaming/Cursor. It reads:
//
//   - <ide>/User/globalStorage/state.vscdb, the primary store: the
//     cursorDiskKV composerData, composerConvHeaders, bubbleId,
//     composerDraft and (for deleted composers only) JSON agentKv:blob:
//     rows; the composerHeaders table; and ItemTable keys: the settings
//     blob, the sandbox switch, github-<login> and its -usages,
//     remoteAgentHost., the cloud agent lists and MCP authorisations, the
//     repository list, the key names of mcpOAuth. and the installed
//     plugins. No secret:// value is read;
//   - .cursor/projects/<cwd>/agent-transcripts/<id>/<id>.jsonl, a
//     secondary source whose user blocks carry a minute-precision time tag;
//   - .cursor/ai-tracking/ai-code-tracking.db, conversation-search.db (its
//     full-text content table read as a plain table) and
//     mcp-oauth-attempts/*.json under <ide>/User/globalStorage/,
//     .cursor/cli-config.json and .cursor/permissions.json.
//
// The project, session and model of a side database's or transcript's row
// come from the home's state.vscdb, read once per state of that file. See
// research/cursor.md.
package cursor

import (
	"regexp"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
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
	statePattern       = regexp.MustCompile(`^` + ide + `/User/globalStorage/state\.vscdb\z`)
	searchPattern      = regexp.MustCompile(`^` + ide + `/User/globalStorage/conversation-search\.db\z`)
	transcriptPattern  = regexp.MustCompile(`^\.cursor/projects/[^/]+/agent-transcripts/[^/]+/([^/]+)\.jsonl\z`)
	trackingPattern    = regexp.MustCompile(`^\.cursor/ai-tracking/ai-code-tracking\.db\z`)
	cliConfigPattern   = regexp.MustCompile(`^\.cursor/cli-config\.json\z`)
	permissionsPattern = regexp.MustCompile(`^\.cursor/permissions\.json\z`)
	attemptPattern     = regexp.MustCompile(`^` + ide + `/User/globalStorage/mcp-oauth-attempts/([^/]+\.json)\z`)
)

// transcriptTolerance is the matching window of a transcript's rows: its
// time tag has minute precision.
const transcriptTolerance = 60.0

// Parser reads the Cursor IDE's stores, transcripts and settings files. It
// keeps the projects and models of the last home state.vscdb read, and the
// group of each composer id a home state.vscdb repeats, by the database's
// path and the id, which its parse and its side files' parses share.
type Parser struct {
	parser.Base
	index  *stateIndex
	groups map[[2]string]*timeline.Group
}

// New returns the parser.
func New() *Parser {
	return &Parser{Base: parser.NewBase("cursor", "cursor")}
}

// Marks says that Cursor records which sessions are synced to the vendor
// and which began on its servers.
func (p *Parser) Marks() parser.Marks {
	return parser.Marks{Sync: true, Origin: true}
}

// Wants reports whether the artifact is one of the files the parser reads.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parser.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the state database, the search database, the agent
// transcripts, the tracking database, the CLI config, the permissions and
// the MCP sign-in attempts.
func (p *Parser) Kinds() []parser.Kind {
	return []parser.Kind{parser.PatternKind("state.vscdb", statePattern),
		parser.PatternKind("conversation-search.db", searchPattern),
		parser.PatternKind("agent transcript", transcriptPattern),
		parser.PatternKind("ai-code-tracking.db", trackingPattern), parser.PatternKind("cli-config.json", cliConfigPattern),
		parser.PatternKind("permissions.json", permissionsPattern),
		parser.PatternKind("mcp-oauth attempt", attemptPattern)}
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

// Parse emits the rows of one artifact, by the file it is.
func (p *Parser) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	rel := artifact.Rel
	switch {
	case statePattern.MatchString(rel):
		p.database(artifact, emit, func(store *store, add parser.Emit) { p.state(artifact, store, options, add) })
	case searchPattern.MatchString(rel):
		p.database(artifact, emit, func(store *store, add parser.Emit) { p.search(artifact, store, add) })
	case trackingPattern.MatchString(rel):
		p.database(artifact, emit, func(store *store, add parser.Emit) { p.tracking(artifact, store, add) })
	case transcriptPattern.MatchString(rel):
		p.transcript(artifact, emit)
	case attemptPattern.MatchString(rel):
		p.attempt(artifact, emit)
	default:
		p.settingsFile(artifact, emit)
	}
	return nil
}

// row returns a row of the artifact at a time and source line.
func (p *Parser) row(artifact *input.Artifact, turnType, text, timestamp string, line int64) timeline.Row {
	row := p.BaseRow(artifact)
	row.TurnType, row.Text, row.TimestampUTC, row.SourceLine = turnType, text, timestamp, int(line)
	return row
}

// systemAt returns a system row of the artifact whose time is a formatted
// timestamp.
func (p *Parser) systemAt(artifact *input.Artifact, text, timestamp string, line int64) timeline.Row {
	return p.row(artifact, timeline.TurnSystem, text, timeutil.FromText(timestamp), line)
}

// database emits the rows read reads from the artifact's database. The
// rows are gathered first: a database error met while reading them is one
// problem row in their place.
func (p *Parser) database(artifact *input.Artifact, emit parser.Emit, read func(*store, parser.Emit)) {
	database, err := parser.OpenDatabase(artifact.DiskPath)
	if err != nil {
		emit(p.row(artifact, timeline.TurnSystem, parser.ProblemPrefix+parser.DatabaseError(err), "", 0))
		return
	}
	defer database.Close()
	store := newStore(database, p.Work)
	var rows []timeline.Row
	read(store, func(row timeline.Row) { rows = append(rows, row) })
	if store.err != nil {
		emit(p.row(artifact, timeline.TurnSystem, parser.ProblemPrefix+parser.DatabaseError(store.err), "", 0))
		return
	}
	parser.EmitAll(emit, rows)
}

// store reads the tables of one database, keeping the first error, after
// which every read gives nothing.
type store struct {
	database *sqlitefile.Database
	tables   parser.Tables
	err      error
	// work counts the queries run, which a test measures.
	work parser.Work
}

// newStore returns the store of an open database, counting its queries in
// work.
func newStore(database *sqlitefile.Database, work parser.Work) *store {
	created := &store{database: database, work: work}
	created.tables, created.err = parser.TablesOf(database)
	return created
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
	_ parser.Parser          = (*Parser)(nil)
	_ parser.Marker          = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
	_ parser.ToleranceSetter = (*Parser)(nil)
)
