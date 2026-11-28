package zed

import (
	"regexp"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// The data and config directories below a home, on each platform and in
// the Flatpak sandbox.
const (
	dataDirectory   = `(?:\.local/share/zed|Library/Application Support/Zed|AppData/Local/Zed|\.var/app/dev\.zed\.Zed[A-Za-z]*/data/zed)`
	configDirectory = `(?:\.config/zed|AppData/Roaming/Zed|\.var/app/dev\.zed\.Zed[A-Za-z]*/config/zed)`
)

// patterns holds the three kinds of file the parser reads, by their path
// relative to the home.
type patterns struct {
	threads  *parserkit.RelPattern
	sidebar  *parserkit.RelPattern
	settings *parserkit.RelPattern
}

// newPatterns compiles the file patterns.
func newPatterns() patterns {
	return patterns{
		threads:  parserkit.MustRelPattern(`\A` + dataDirectory + `/threads/threads\.db\z`),
		sidebar:  parserkit.MustRelPattern(`\A` + dataDirectory + `/db/0-[^/]+/db\.sqlite\z`),
		settings: parserkit.MustRelPattern(`\A` + configDirectory + `/(?:global_)?settings\.json\z`),
	}
}

// Parser reads Zed's thread database, sidebar database and settings files.
type Parser struct {
	parserkit.Base
	files  patterns
	denial *regexp.Regexp
	// budgetFloor is the least decompression budget of a database
	// (BudgetFloor; a test lowers it).
	budgetFloor int
}

// New returns the parser.
func New() *Parser {
	return &Parser{Base: parserkit.NewBase("zed", "zed"), files: newPatterns(),
		denial: newDenialPattern(), budgetFloor: BudgetFloor}
}

// Kinds returns the threads database, the workspace database and the
// settings.
func (p *Parser) Kinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.PatternKind("threads.db", p.files.threads),
		parserkit.PatternKind("workspace db.sqlite", p.files.sidebar), parserkit.PatternKind("settings.json", p.files.settings)}
}

// Wants reports whether the artifact is threads.db, a channel's db.sqlite
// or a user settings file.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Parse emits the rows of one Zed file. A file that is not a database or
// cannot be opened is one `parser:` row.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	if p.files.settings.MatchString(artifact.Rel) {
		p.parseSettings(artifact, emit)
		return nil
	}
	database, err := parserkit.OpenDatabase(artifact, artifact.DiskPath)
	if err != nil {
		emit(p.ProblemRow(artifact, parserkit.DatabaseError(err)))
		return nil
	}
	defer database.Close()
	if p.files.threads.MatchString(artifact.Rel) {
		p.parseThreads(artifact, database, options, emit)
		return nil
	}
	p.parseSidebar(artifact, database, emit)
	return nil
}

// stateRow returns a system row of the artifact with the text compacted and
// the given time, source line, session and project.
func (p *Parser) stateRow(artifact *input.Artifact, text, timestamp string, sourceLine int64,
	sessionID, project string) timeline.Row {
	row := p.SystemRow(artifact, timeline.Compact(text))
	row.SourceLine = int(sourceLine)
	row.SessionID = sessionID
	row.ProjectPath = project
	row.TimestampUTC = timestamp
	return row
}

// The interface the registry looks for, checked when the package builds.
var _ parserkit.Parser = (*Parser)(nil)
