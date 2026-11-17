// Package opencode reads OpenCode's sessions: its SQLite databases, its
// legacy JSON trees and its terminal UI state.
//
// The databases are .local/share/opencode/opencode[-<channel>].db. A 2.x
// database keeps its transcript in session_message (with the session_part
// items of an assistant message) and its sessions in session_v2, and holds
// the 1.x session, message and part tables a migration left behind, which
// are read, marked recovered, only for a session the 2.x transcript lacks;
// a 1.x database is read the other way round. Beside the transcript: the
// session_share rows and the shares of session.updated.1 events (sync
// rows), archived sessions, per-session permission rules, saved permission
// grants and the worktrees OpenCode made (scratch directories).
// opencode-next.db, a pre-release 2.0 database, is a secondary source. The
// legacy storage/ and project/<slug>/storage/session/ trees hold one JSON
// file per session, message and part. The terminal UI's state directory
// holds prompt-history.jsonl (secondary), prompt-stash.jsonl and
// session.json (pinned sessions, secondary).
//
// The model and the agent (an OpenCode agent is a mode with its own
// permission rules) are tracked per session as "model:" and
// "permission mode:" rows. A row's project path is the session's
// directory, else its project's worktree unless "/", else the message's
// working directory. Kilo Code's CLI is a fork that keeps the same stores
// under its own directories: Config describes where a build keeps them,
// and the kilocode parser holds a Parser made for Kilo's. See
// research/opencode.md.
package opencode

import (
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
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
	parser.Base
	nextDatabase string
	patterns     patterns
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
		Base:         parser.NewBase(config.Agent, config.Agent),
		nextDatabase: config.NextDatabase,
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
func (p *Parser) Marks() parser.Marks {
	return parser.Marks{Sync: true}
}

// Wants reports whether the artifact is a database, a legacy JSON tree file
// or a terminal UI state file.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	for _, pattern := range []*regexp.Regexp{p.patterns.database, p.patterns.session,
		p.patterns.message, p.patterns.oldSession, p.patterns.oldMessage, p.patterns.history,
		p.patterns.stash, p.patterns.pinned} {
		if pattern.MatchString(artifact.Rel) {
			return true
		}
	}
	return false
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
func (p *Parser) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
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
	_ parser.Parser          = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
	_ parser.Marker          = (*Parser)(nil)
)
