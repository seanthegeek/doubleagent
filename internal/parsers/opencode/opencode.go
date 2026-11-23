package opencode

import (
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// Config says where one OpenCode build keeps its files, each directory
// relative to the home.
type Config struct {
	// Agent is the catalog agent, and the parser's name.
	Agent string
	// DataDir holds the databases and the legacy JSON trees.
	DataDir string
	// DatabasePrefixes are the database file names before the optional
	// "-<channel>" and the ".db".
	DatabasePrefixes []string
	// StateDir holds the terminal UI's state files.
	StateDir string
	// NextDatabase is the file name of a pre-release database that is a
	// secondary source, "" when every database is primary.
	NextDatabase string
}

// patterns are the artifacts a Parser reads, by path relative to the home.
type patterns struct {
	history, stash, pinned, database         *regexp.Regexp
	session, message, oldSession, oldMessage *regexp.Regexp
}

// Parser reads the stores of one OpenCode build.
type Parser struct {
	parserkit.Base
	nextDatabase string
	patterns     patterns
	// trees holds the legacy tree files each home's files share.
	trees *parserkit.PerHome[*treeIndex]
}

// New returns the OpenCode parser.
func New() *Parser {
	return NewFor(Config{Agent: "opencode", DataDir: ".local/share/opencode",
		DatabasePrefixes: []string{"opencode"}, StateDir: ".local/state/opencode",
		NextDatabase: "opencode-next.db"})
}

// NewFor returns a parser of the build config describes.
func NewFor(config Config) *Parser {
	data, state := regexp.QuoteMeta(config.DataDir), regexp.QuoteMeta(config.StateDir)
	prefixes := make([]string, len(config.DatabasePrefixes))
	for i, prefix := range config.DatabasePrefixes {
		prefixes[i] = regexp.QuoteMeta(prefix)
	}
	compile := func(pattern string) *regexp.Regexp { return regexp.MustCompile(pattern) }
	return &Parser{
		Base:         parserkit.NewBase(config.Agent, config.Agent),
		nextDatabase: config.NextDatabase,
		trees:        parserkit.NewPerHome(newTreeIndex),
		patterns: patterns{
			history:    compile(`^` + state + `/prompt-history\.jsonl\z`),
			stash:      compile(`^` + state + `/prompt-stash\.jsonl\z`),
			pinned:     compile(`^` + state + `/session\.json\z`),
			database:   compile(`^` + data + `/(?:` + strings.Join(prefixes, "|") + `)(?:-[^/]+)?\.db\z`),
			session:    compile(`^` + data + `/storage/session/[^/]+/[^/]+\.json\z`),
			message:    compile(`^` + data + `/storage/message/[^/]+/[^/]+\.json\z`),
			oldSession: compile(`^` + data + `/project/[^/]+/storage/session/info/[^/]+\.json\z`),
			oldMessage: compile(`^` + data + `/project/[^/]+/storage/session/message/[^/]+/[^/]+\.json\z`),
		},
	}
}

// Marks says the parser records synced sessions (shares).
func (p *Parser) Marks() parserkit.Marks {
	return parserkit.Marks{Sync: true}
}

// Wants reports whether the artifact is a database, a legacy JSON tree file
// or a terminal UI state file.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the databases, the 1.x and older session and message
// files, the prompt history and stash, and the pinned session.
func (p *Parser) Kinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.PatternKind("database", p.patterns.database),
		parserkit.PatternKind("session file", p.patterns.session), parserkit.PatternKind("message file", p.patterns.message),
		parserkit.PatternKind("old session file", p.patterns.oldSession),
		parserkit.PatternKind("old message file", p.patterns.oldMessage),
		parserkit.PatternKind("prompt-history.jsonl", p.patterns.history),
		parserkit.PatternKind("prompt-stash.jsonl", p.patterns.stash), parserkit.PatternKind("session.json", p.patterns.pinned)}
}

// Secondary reports whether the artifact is the prompt history, the pinned
// sessions or the pre-release database.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	rel := artifact.Rel
	if p.patterns.history.MatchString(rel) || p.patterns.pinned.MatchString(rel) {
		return true
	}
	return p.patterns.database.MatchString(rel) && databaseLabel(artifact) == p.nextDatabase
}

// Parse emits the rows of one OpenCode file.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	rel := artifact.Rel
	switch {
	case p.patterns.history.MatchString(rel):
		p.history(artifact, emit)
	case p.patterns.stash.MatchString(rel):
		p.stash(artifact, emit)
	case p.patterns.pinned.MatchString(rel):
		p.pinned(artifact, emit)
	case p.patterns.database.MatchString(rel):
		p.database(artifact, options, emit)
	case p.patterns.session.MatchString(rel):
		p.sessionFile(artifact, false, emit)
	case p.patterns.oldSession.MatchString(rel):
		p.sessionFile(artifact, true, emit)
	case p.patterns.message.MatchString(rel):
		p.messageFile(artifact, options, false, emit)
	case p.patterns.oldMessage.MatchString(rel):
		p.messageFile(artifact, options, true, emit)
	}
	return nil
}

// databaseLabel returns the database's file name for row text
// (opencode.db, kilo.db, ...).
func databaseLabel(artifact *input.Artifact) string {
	return artifact.Rel[strings.LastIndex(artifact.Rel, "/")+1:]
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
	_ parserkit.Marker          = (*Parser)(nil)
)
