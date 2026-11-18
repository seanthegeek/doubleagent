// Package pi reads the pi coding agent's sessions, trust settings, debug
// snapshot, crash records and session exports, and holds the readers of
// pi's session format that the letta, little-coder and openclaw parsers
// share.
//
// Session files are .pi/agent/sessions/--<cwd>--/<time>_<id>.jsonl and the
// legacy .pi/agent/*.jsonl: a "session" header (version, id, timestamp,
// cwd, parentSession), then entries with type, id and timestamp; message
// entries carry a role and its fields. Rows follow file order, not the
// active branch, and a fork's entries copied from a parent file in the same
// home are dropped. Other files read: trust.json and settings.json
// defaultProjectTrust (permission rows), crashes.json, pi-tui-crash.log and
// the 0.83.0 pi-crash.log, experimental/sessions/<id>/meta.json, and the
// debug log pi-debug.log, a secondary source.
//
// In a project directory, three exports are secondary sources: the HTML
// export pi-session-<name>.html, whose session-data element holds the
// session as base64 JSON; the JSONL export session-<time>.jsonl; and the
// bug report pi-bug-report-<id>.zip, read in memory and never extracted,
// at most maxInflated bytes decompressed. Copies give no `model:` rows. See
// research/pi.md.
package pi

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The files read at fixed paths relative to the home.
const (
	trustRel    = ".pi/agent/trust.json"
	settingsRel = ".pi/agent/settings.json"
	debugRel    = ".pi/agent/pi-debug.log"
	crashesRel  = ".pi/agent/crashes.json"
)

// debugTolerance is the matching window, in seconds, of the debug log's
// rows: its messages carry their own time, which for an assistant message
// is when its stream started, while the session file stamps the entry when
// the message ended.
const debugTolerance = 3600.0

// Parser reads pi's files. It remembers the entry identities of each parent
// session file it has read, for the forks of that parent.
type Parser struct {
	parser.Base
	session, sessionName, meta, tuiCrash *regexp.Regexp
	htmlExport, jsonlExport, bugZip      *regexp.Regexp
	parents                              map[string]map[entryKey]bool
}

// New returns the parser.
func New() *Parser {
	return &Parser{
		Base:        parser.NewBase("pi", "pi"),
		session:     regexp.MustCompile(`\A\.pi/agent/(?:sessions/[^/]+/)?[^/]+\.jsonl\z`),
		sessionName: regexp.MustCompile(`_([^_/]+)\.jsonl\z`),
		meta:        regexp.MustCompile(`\A\.pi/agent/experimental/sessions/([^/]+)/meta\.json\z`),
		tuiCrash:    regexp.MustCompile(`\A\.pi/agent/pi(?:-tui)?-crash\.log\z`),
		// The exports, relative to the project directory they were written in.
		htmlExport:  regexp.MustCompile(`\Api-session-[^/]+\.html\z`),
		jsonlExport: regexp.MustCompile(`\Asession-[0-9]{4}-[^/]*\.jsonl\z`),
		bugZip:      regexp.MustCompile(`\Api-bug-report-[^/]+\.zip\z`),
		parents:     map[string]map[entryKey]bool{},
	}
}

// Marks says the parser records synced sessions.
func (p *Parser) Marks() parser.Marks {
	return parser.Marks{Sync: true}
}

// Wants reports whether the artifact is a session, trust, settings, debug,
// crash, metadata or export file.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	rel := artifact.Rel
	return slices.Contains([]string{trustRel, settingsRel, debugRel, crashesRel}, rel) ||
		p.session.MatchString(rel) || p.meta.MatchString(rel) || p.tuiCrash.MatchString(rel) ||
		p.isExport(rel)
}

// isExport reports whether rel is one of the three project exports.
func (p *Parser) isExport(rel string) bool {
	return p.htmlExport.MatchString(rel) || p.jsonlExport.MatchString(rel) || p.bugZip.MatchString(rel)
}

// Secondary reports whether the artifact is the debug log or an export,
// each of which repeats entries of a session file.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return artifact.Rel == debugRel || p.isExport(artifact.Rel)
}

// SecondaryTolerance returns the debug log's wider window, and the default
// one for the exports.
func (p *Parser) SecondaryTolerance(artifact *input.Artifact) float64 {
	if artifact.Rel == debugRel {
		return debugTolerance
	}
	return parser.DefaultTolerance
}

// Parse emits the rows of one pi file.
func (p *Parser) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	rel := artifact.Rel
	switch {
	case rel == trustRel:
		p.parseTrust(artifact, emit)
	case rel == settingsRel:
		p.parseSettings(artifact, emit)
	case rel == debugRel:
		p.parseDebugLog(artifact, options, emit)
	case rel == crashesRel:
		p.parseCrashes(artifact, emit)
	case p.meta.MatchString(rel):
		p.parseMeta(artifact, p.meta.FindStringSubmatch(rel)[1], emit)
	case p.tuiCrash.MatchString(rel):
		p.parseTUICrash(artifact, emit)
	case p.htmlExport.MatchString(rel):
		p.parseHTMLExport(artifact, options, emit)
	case p.jsonlExport.MatchString(rel):
		p.parseJSONLExport(artifact, options, emit)
	case p.bugZip.MatchString(rel):
		p.parseBugZip(artifact, options, emit)
	default:
		p.parseSession(artifact, options, emit)
	}
	return nil
}

// parseSession emits a session file's start row, its fork row when it
// copied entries from its parent, its entries' rows, its little-coder row
// and its bad lines. A file whose first record is not a header is not a
// session file to pi either: only its bad lines and the missing header are
// reported.
func (p *Parser) parseSession(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	session := ReadSession(artifact.DiskPath)
	if !session.HasHeader() {
		parser.EmitAll(emit, p.ErrorRows(artifact, session.headerless(artifact.Rel), "", ""))
		return
	}
	header := session.Header
	sessionID, cwd := parser.Text(header.Get("id")), parser.Text(header.Get("cwd"))
	parent := parser.Text(header.Get("parentSession"))
	headerRow := func(text string) timeline.Row {
		return p.systemRow(artifact, session.HeaderLine, timeutil.FromValue(header.Get("timestamp")),
			sessionID, cwd, text)
	}
	emit(headerRow(SessionStartText(header)))
	skip := p.parentKeys(artifact, parent)
	dropped := 0
	for _, entry := range session.Entries {
		if skip[keyOf(entry.Record)] {
			dropped++
		}
	}
	if dropped > 0 {
		emit(headerRow(fmt.Sprintf("fork: %d entries copied from the parent session %s omitted; "+
			"they are in the parent's rows", dropped, parent)))
	}
	entryRows(p.Base, artifact, session, options, sessionID, cwd, skip, emit)
	if why := littleCoderMarkers(artifact, session); len(why) > 0 {
		emit(headerRow("session driven by little-coder (pi launcher): " + strings.Join(why, "; ")))
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, session.Problems, sessionID, cwd))
}

// parentKeys returns the entry identities of the parent session file that
// parent, a path on the source host, names, when the home holds it and it
// is not the artifact itself; read once per parent file.
func (p *Parser) parentKeys(artifact *input.Artifact, parent string) map[entryKey]bool {
	if parent == "" {
		return nil
	}
	// Both paths are joins the input built, so one file has one spelling.
	path, ok := homeFile(artifact, parent, ".pi/")
	if !ok || path == artifact.DiskPath {
		return nil
	}
	if _, read := p.parents[path]; !read {
		p.parents[path] = entryKeys(path)
	}
	return p.parents[path]
}

// homeFile returns where the home holds original, a file's path on the
// source host: relative to the home's own original path, else from the
// last marker segment (loose input whose home path is not the host's). A
// path through a symbolic link is never taken, and only a regular file is.
func homeFile(artifact *input.Artifact, original, marker string) (string, bool) {
	home := artifact.Home
	if home == nil {
		return "", false
	}
	path := strings.ReplaceAll(original, `\`, "/")
	var rels []string
	homePath := strings.TrimRight(strings.ReplaceAll(home.Original, `\`, "/"), "/")
	if homePath != "" && strings.HasPrefix(path, homePath+"/") {
		rels = append(rels, path[len(homePath)+1:])
	}
	if found := strings.LastIndex(path, "/"+marker); found >= 0 {
		rels = append(rels, path[found+1:])
	}
	for _, rel := range rels {
		parts := slices.DeleteFunc(strings.Split(rel, "/"), func(part string) bool { return part == "" })
		if len(parts) == 0 {
			continue
		}
		if current, ok := home.PathOf(strings.Join(parts, "/")); ok && parser.RegularFile(current) {
			return current, true
		}
	}
	return "", false
}

// SessionStartText returns the `session start:` row text of a session
// header: its version (1 when unset), cwd and, for a fork, parent session.
func SessionStartText(header jsonvalue.Value) string {
	version := "1"
	if value := header.Get("version"); !value.IsEmpty() {
		version = value.Text()
	}
	text := "session start: version=" + version + " cwd=" + parser.Text(header.Get("cwd"))
	if parent := parser.Text(header.Get("parentSession")); parent != "" {
		text += " parentSession=" + parent
	}
	return text
}

// littleCoderMarkers returns why a pi session looks driven by
// little-coder: its lc- custom message types, an assistant on the llamacpp
// provider, a checkpoint directory named after the session file. It is
// empty when nothing points at little-coder.
func littleCoderMarkers(artifact *input.Artifact, session *Session) []string {
	types := newFirstSeen()
	llamacpp := false
	for _, entry := range session.Entries {
		record := entry.Record
		switch kind := record.Get("type"); {
		case parser.IsText(kind, "custom_message"):
			if custom := parser.Text(record.Get("customType")); strings.HasPrefix(custom, "lc-") {
				types.add(custom)
			}
		case parser.IsText(kind, "message"):
			message := record.Get("message")
			llamacpp = llamacpp || (parser.IsText(message.Get("role"), "assistant") &&
				parser.IsText(message.Get("provider"), "llamacpp"))
		}
	}
	var why []string
	if len(types.order) > 0 {
		why = append(why, "custom_message types "+strings.Join(types.order, ", "))
	}
	if llamacpp {
		why = append(why, "assistant provider llamacpp")
	}
	if artifact.Home != nil {
		name := artifact.Rel[strings.LastIndex(artifact.Rel, "/")+1:]
		checkpoint, found := artifact.Home.PathOf(".little-coder/checkpoints/" + name)
		if info, err := osfile.Lstat(checkpoint); found && err == nil && info.IsDir() {
			why = append(why, "checkpoints in ~/.little-coder/checkpoints/"+filepath.Base(checkpoint))
		}
	}
	return why
}

// firstSeen keeps distinct texts in the order they are first met, through a
// set, so each text added costs one lookup whatever came before. lookups
// counts them, a measure of the work that a test reads.
type firstSeen struct {
	seen    map[string]bool
	order   []string
	lookups int
}

// newFirstSeen returns an empty firstSeen.
func newFirstSeen() *firstSeen {
	return &firstSeen{seen: map[string]bool{}}
}

// add keeps text when it has not been met before.
func (f *firstSeen) add(text string) {
	f.lookups++
	if !f.seen[text] {
		f.seen[text] = true
		f.order = append(f.order, text)
	}
}

// systemRow returns a system row of the artifact at line and timestamp,
// with its session id, project path and compacted text.
func (p *Parser) systemRow(artifact *input.Artifact, line int, timestamp, sessionID, cwd,
	text string) timeline.Row {
	row := p.SystemRow(artifact, timeline.Compact(text))
	row.SourceLine = line
	row.TimestampUTC = timestamp
	row.SessionID = sessionID
	row.ProjectPath = cwd
	return row
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parser.Parser          = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
	_ parser.ToleranceSetter = (*Parser)(nil)
	_ parser.Marker          = (*Parser)(nil)
)
