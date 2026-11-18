// Package zed reads the Zed editor's agent panel threads, its sidebar
// database and the standing permission entries of its user settings files.
//
// Paths are under the data directory (.local/share/zed, Library/Application
// Support/Zed, AppData/Local/Zed or the Flatpak data/zed) or the config
// directory (.config/zed, AppData/Roaming/Zed or the Flatpak config/zed):
//
//   - threads/threads.db, table threads (id, summary, updated_at, data_type,
//     data, parent_id, folder_paths, created_at): data is the thread as JSON,
//     zstd-compressed when data_type is "zstd". Its messages are externally
//     tagged User, Agent, Resume or Compaction; the older role, segments,
//     tool_uses and tool_results shape is read too.
//   - db/0-<channel>/db.sqlite: sidebar_threads, trusted_worktrees, the
//     agent_draft_prompts and created_git_worktrees namespaces of
//     scoped_kv_store, archived_git_worktrees with thread_archived_worktrees,
//     and sidebar_terminal_threads, each as system rows.
//   - settings.json and global_settings.json (JSON with comments): the
//     standing permission keys, in every override block.
//
// Messages carry no time: each row takes the thread's updated_at, and the
// thread-start, `git remote:` and `model:` rows the project snapshot's time.
// Denials are read from failed tool results' texts; an approval leaves no
// record. The source line is the SQLite rowid. See research/zed.md.
package zed

import (
	"regexp"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
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
	threads  *regexp.Regexp
	sidebar  *regexp.Regexp
	settings *regexp.Regexp
}

// newPatterns compiles the file patterns.
func newPatterns() patterns {
	return patterns{
		threads:  regexp.MustCompile(`\A` + dataDirectory + `/threads/threads\.db\z`),
		sidebar:  regexp.MustCompile(`\A` + dataDirectory + `/db/0-[^/]+/db\.sqlite\z`),
		settings: regexp.MustCompile(`\A` + configDirectory + `/(?:global_)?settings\.json\z`),
	}
}

// Parser reads Zed's thread database, sidebar database and settings files.
type Parser struct {
	parser.Base
	files  patterns
	denial *regexp.Regexp
	// budgetFloor is the least decompression budget of a database
	// (BudgetFloor; a test lowers it).
	budgetFloor int
}

// New returns the parser.
func New() *Parser {
	return &Parser{Base: parser.NewBase("zed", "zed"), files: newPatterns(),
		denial: newDenialPattern(), budgetFloor: BudgetFloor}
}

// Wants reports whether the artifact is threads.db, a channel's db.sqlite
// or a user settings file.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	rel := artifact.Rel
	return p.files.threads.MatchString(rel) || p.files.sidebar.MatchString(rel) ||
		p.files.settings.MatchString(rel)
}

// Parse emits the rows of one Zed file. A file that is not a database or
// cannot be opened is one `parser:` row.
func (p *Parser) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	if p.files.settings.MatchString(artifact.Rel) {
		p.parseSettings(artifact, emit)
		return nil
	}
	database, message := parser.OpenDatabase(artifact.DiskPath)
	if message != "" {
		emit(p.ProblemRow(artifact, message))
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
var _ parser.Parser = (*Parser)(nil)
