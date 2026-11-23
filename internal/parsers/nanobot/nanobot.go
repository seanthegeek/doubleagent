// Package nanobot reads nanobot's session stores, their copies, the memory
// and CLI histories, the WebUI mirror, settings and run records, under a
// state directory .nanobot or .nanobot-<instance>:
//
//   - sessions/<workspace id>/sessions.sqlite3, the store of record from
//     0.3.6: tables sessions (key, created_at, updated_at, metadata) and
//     messages (session_key, position, record: one JSONL message line).
//   - sessions/<workspace id>/<base64url key>.jsonl, its jsonl-backup/ and
//     .migration-conflicts/ copies, and legacy sessions/<key>.jsonl: a
//     metadata line, then OpenAI chat messages. The copies a database
//     imported are secondary. .workspace beside them is the project path.
//   - memory/history.jsonl, primary; webui/<key>.jsonl and its .segments/,
//     history/cli_history, cron/runs/*.json and triggers/runs/*.json under a
//     workspace, and .webui_session_index.json, secondary.
//   - config.json: the keys that grant or restrict the agent, as
//     "permission setting:" rows, and the provider keys, MCP server env
//     values and channel secrets it holds, as one "stored credentials:"
//     row; and pairing.json.
//
// The session files hold naive host local time, emitted as if UTC; the
// WebUI and run records carry true UTC, hence their wider matching window.
// A messaging session key gives relay rows. See research/nanobot.md.
package nanobot

import (
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
)

// state is the state directory below a home.
const state = `\.nanobot(?:-[^/]+)?`

// databaseName is the session database's file name.
const databaseName = "sessions.sqlite3"

// webuiTolerance is the matching window of the true-UTC sources, in
// seconds: the host's UTC offset (at most 14 hours) plus an hour of turn.
const webuiTolerance = 15 * 3600.0

// patterns holds the files the parser reads, by their path relative to the
// home, and the project directories in their original paths.
type patterns struct {
	session, database, history, webui, cliHistory *regexp.Regexp
	config, pairing, sidebar, run                 *regexp.Regexp
	originalRun, originalMemory                   *regexp.Regexp
}

// newPatterns compiles the patterns.
func newPatterns() patterns {
	return patterns{
		session: regexp.MustCompile(`\A` + state + `/sessions/(?:(?P<ws>[0-9a-f]{32})/` +
			`(?P<sub>\.migration-conflicts/|jsonl-backup/)?)?(?P<stem>[^/]+)\.jsonl\z`),
		database: regexp.MustCompile(`\A` + state + `/sessions/[0-9a-f]{32}/sessions\.sqlite3\z`),
		history:  regexp.MustCompile(`\A` + state + `/(?:.+/)?memory/history\.jsonl\z`),
		webui: regexp.MustCompile(`\A` + state +
			`/webui/(?:(?P<seg>[^/]+)\.segments/[0-9]{6}|(?P<stem>[^/]+))\.jsonl\z`),
		cliHistory: regexp.MustCompile(`\A` + state + `/history/cli_history\z`),
		config:     regexp.MustCompile(`\A` + state + `/config\.json\z`),
		pairing:    regexp.MustCompile(`\A` + state + `/pairing\.json\z`),
		sidebar:    regexp.MustCompile(`\A` + state + `/sessions/[0-9a-f]{32}/\.webui_session_index\.json\z`),
		run: regexp.MustCompile(`\A(?:` + state + `/(?P<ws>.+/)?)?(?P<kind>cron|triggers)/runs/` +
			`[^/]+\.json\z`),
		originalRun:    regexp.MustCompile(`\A(.*)[\\/](?:cron|triggers)[\\/]runs[\\/][^\\/]+\z`),
		originalMemory: regexp.MustCompile(`\A(.*)[\\/]memory[\\/]history\.jsonl\z`),
	}
}

// group returns the named group of match, "" when it did not take part.
func group(pattern *regexp.Regexp, match []string, name string) string {
	if match == nil {
		return ""
	}
	return match[pattern.SubexpIndex(name)]
}

// Parser reads nanobot's files.
type Parser struct {
	parserkit.Base
	files patterns
	text  textPatterns
	// markers holds each .workspace file read, by its state.
	markers map[markerState]*markerText
	// namespaces holds each sessions directory's namespaces, by its path,
	// listed once for the run, since every WebUI chat looks in all of them.
	namespaces map[string]*parserkit.ChildListings
}

// New returns the parser.
func New() *Parser {
	return &Parser{Base: parserkit.NewBase("nanobot", "nanobot"), files: newPatterns(),
		text: newTextPatterns()}
}

// Marks says the parser records sessions relayed through a messaging
// platform.
func (p *Parser) Marks() parserkit.Marks {
	return parserkit.Marks{Relay: true}
}

// Wants reports whether the artifact is one of the nanobot files the parser
// reads.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the session files, the memory history, the WebUI mirror,
// the session database, the CLI history, the config, the pairing, the
// sidebar index and the run records.
func (p *Parser) Kinds() []parserkit.Kind {
	files := p.files
	return []parserkit.Kind{parserkit.PatternKind("session jsonl", files.session),
		parserkit.PatternKind("memory history.jsonl", files.history), parserkit.PatternKind("WebUI mirror", files.webui),
		parserkit.PatternKind("sessions.sqlite3", files.database), parserkit.PatternKind("cli_history", files.cliHistory),
		parserkit.PatternKind("config.json", files.config), parserkit.PatternKind("pairing.json", files.pairing),
		parserkit.PatternKind("sidebar index", files.sidebar), {Name: "run record", Wants: p.isRun}}
}

// isRun reports whether the artifact is a cron or trigger run record. The
// bare cron/runs/ form counts only at the root of a project base, where the
// project catalog put it.
func (p *Parser) isRun(artifact *input.Artifact) bool {
	if !p.files.run.MatchString(artifact.Rel) {
		return false
	}
	return strings.HasPrefix(artifact.Rel, ".nanobot") || (artifact.Home != nil && artifact.Home.Project)
}

// Secondary reports whether the artifact repeats the session store's
// events: the WebUI mirror, the CLI history, the sidebar index, a run
// record, or a session file the namespace's database imported.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	rel := artifact.Rel
	return p.files.webui.MatchString(rel) || p.files.cliHistory.MatchString(rel) ||
		p.files.sidebar.MatchString(rel) || p.isRun(artifact) || p.frozen(artifact)
}

// SecondaryTolerance returns the wide window of the true-UTC sources, the
// WebUI mirror and the run records, else the default one.
func (p *Parser) SecondaryTolerance(artifact *input.Artifact) float64 {
	if p.files.webui.MatchString(artifact.Rel) || p.isRun(artifact) {
		return webuiTolerance
	}
	return parserkit.DefaultTolerance
}

// namespaceFile returns the path of name in the sessions/<workspace id>
// directory of a namespaced session file: the file's own directory, or its
// parent for a .migration-conflicts/ or jsonl-backup/ copy. It is false for
// a legacy global file.
func (p *Parser) namespaceFile(artifact *input.Artifact, name string) (string, bool) {
	match := p.files.session.FindStringSubmatch(artifact.Rel)
	if group(p.files.session, match, "ws") == "" {
		return "", false
	}
	if group(p.files.session, match, "sub") != "" {
		return parserkit.Above(artifact, 1, name)
	}
	return parserkit.Beside(artifact, name)
}

// frozen reports whether the artifact is a session file the namespace's
// sessions.sqlite3 imported: a file of the namespace or a jsonl-backup/
// copy, with the database beside it. A .migration-conflicts/ copy and a
// legacy global file were never imported.
func (p *Parser) frozen(artifact *input.Artifact) bool {
	match := p.files.session.FindStringSubmatch(artifact.Rel)
	if match == nil || group(p.files.session, match, "sub") == ".migration-conflicts/" {
		return false
	}
	database, ok := p.namespaceFile(artifact, databaseName)
	return ok && parserkit.RegularFile(database)
}

// Parse emits the rows of one nanobot file.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	rel := artifact.Rel
	switch {
	case p.files.session.MatchString(rel):
		p.parseSession(artifact, options, emit)
	case p.files.database.MatchString(rel):
		p.parseDatabase(artifact, options, emit)
	case p.files.webui.MatchString(rel):
		p.parseWebUI(artifact, options, emit)
	case p.files.cliHistory.MatchString(rel):
		p.parseCLIHistory(artifact, emit)
	case p.files.config.MatchString(rel):
		p.parseConfig(artifact, emit)
	case p.files.pairing.MatchString(rel):
		p.parsePairing(artifact, emit)
	case p.files.sidebar.MatchString(rel):
		p.parseSidebar(artifact, emit)
	case p.files.run.MatchString(rel):
		p.parseRun(artifact, emit)
	default:
		p.parseHistory(artifact, emit)
	}
	return nil
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
	_ parserkit.ToleranceSetter = (*Parser)(nil)
	_ parserkit.Marker          = (*Parser)(nil)
)
