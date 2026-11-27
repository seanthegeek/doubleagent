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
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/textlines"
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
// session file it has read, per home, for the forks of that parent.
type Parser struct {
	parserkit.Base
	session, meta, tuiCrash         *parserkit.RelPattern
	htmlExport, jsonlExport, bugZip *parserkit.RelPattern
	sessionName                     *regexp.Regexp
	parents                         *parserkit.PerHome[parentFiles]
}

// parentFiles maps each parent session file of a home read, by its entry,
// to the entry identities it holds.
type parentFiles map[parserkit.EntryKey]map[entryKey]bool

// New returns the parser.
func New() *Parser {
	return &Parser{
		Base:        parserkit.NewBase("pi", "pi"),
		session:     parserkit.MustRelPattern(`\A\.pi/agent/(?:sessions/[^/]+/)?[^/]+\.jsonl\z`),
		sessionName: regexp.MustCompile(`_([^_/]+)\.jsonl\z`),
		meta:        parserkit.MustRelPattern(`\A\.pi/agent/experimental/sessions/([^/]+)/meta\.json\z`),
		tuiCrash:    parserkit.MustRelPattern(`\A\.pi/agent/pi(?:-tui)?-crash\.log\z`),
		// The exports, relative to the project directory they were written in.
		htmlExport:  parserkit.MustRelPattern(`\Api-session-[^/]+\.html\z`),
		jsonlExport: parserkit.MustRelPattern(`\Asession-[0-9]{4}-[^/]*\.jsonl\z`),
		bugZip:      parserkit.MustRelPattern(`\Api-bug-report-[^/]+\.zip\z`),
		parents:     parserkit.NewPerHome(func() parentFiles { return parentFiles{} }),
	}
}

// Marks says the parser records synced sessions.
func (p *Parser) Marks() parserkit.Marks {
	return parserkit.Marks{Sync: true}
}

// Wants reports whether the artifact is a session, trust, settings, debug,
// crash, metadata or export file.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the trust, settings, debug log and crashes files, the
// sessions, the experimental session metadata, the crash logs and the
// three export forms.
func (p *Parser) Kinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.RelKind("trust.json", trustRel), parserkit.RelKind("settings.json", settingsRel),
		parserkit.RelKind("pi-debug.log", debugRel), parserkit.RelKind("crashes.json", crashesRel),
		parserkit.PatternKind("session", p.session), parserkit.PatternKind("session meta.json", p.meta),
		parserkit.PatternKind("crash log", p.tuiCrash), parserkit.PatternKind("HTML export", p.htmlExport),
		parserkit.PatternKind("JSONL export", p.jsonlExport), parserkit.PatternKind("bug report zip", p.bugZip)}
}

// isExport reports whether rel is one of the three project exports.
func (p *Parser) isExport(rel string) bool {
	return p.htmlExport.MatchString(rel) || p.jsonlExport.MatchString(rel) || p.bugZip.MatchString(rel)
}

// Secondary reports whether the artifact is the debug log or an export,
// each of which repeats entries of a session file.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return textlines.EqualFoldASCII(artifact.Rel, debugRel) || p.isExport(artifact.Rel)
}

// SecondaryTolerance returns the debug log's wider window, and the default
// one for the exports.
func (p *Parser) SecondaryTolerance(artifact *input.Artifact) float64 {
	if textlines.EqualFoldASCII(artifact.Rel, debugRel) {
		return debugTolerance
	}
	return parserkit.DefaultTolerance
}

// Parse emits the rows of one pi file.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	rel := artifact.Rel
	switch {
	case textlines.EqualFoldASCII(rel, trustRel):
		p.parseTrust(artifact, emit)
	case textlines.EqualFoldASCII(rel, settingsRel):
		p.parseSettings(artifact, emit)
	case textlines.EqualFoldASCII(rel, debugRel):
		p.parseDebugLog(artifact, options, emit)
	case textlines.EqualFoldASCII(rel, crashesRel):
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
func (p *Parser) parseSession(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) {
	session := ReadSession(artifact.DiskPath)
	if !session.HasHeader() {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, session.headerless(artifact.Rel), "", ""))
		return
	}
	header := session.Header
	sessionID, cwd := parserkit.Text(header.Get("id")), parserkit.Text(header.Get("cwd"))
	parent := parserkit.Text(header.Get("parentSession"))
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
	parserkit.EmitAll(emit, p.ErrorRows(artifact, session.Problems, sessionID, cwd))
}

// parentKeys returns the entry identities of the parent session file that
// parent, a path on the source host, names, when the home holds it and it
// is not the artifact itself; read once per parent file, however the
// evidence spells its name.
func (p *Parser) parentKeys(artifact *input.Artifact, parent string) map[entryKey]bool {
	if parent == "" {
		return nil
	}
	path, ok := homeFile(artifact, parent, ".pi/")
	if !ok {
		return nil
	}
	// The evidence may spell the parent's name in any case the disk opens,
	// so the file is known by its entry, the artifact's own included.
	key := parserkit.KeyOf(path)
	if key == parserkit.KeyOf(artifact.DiskPath) {
		return nil
	}
	parents := p.parents.For(artifact.Home)
	if _, read := parents[key]; !read {
		p.Work.Add("parent files read", 1)
		parents[key] = entryKeys(path)
	}
	return parents[key]
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
		if current, ok := home.PathOf(strings.Join(parts, "/")); ok && parserkit.RegularFile(current) {
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
	text := "session start: version=" + version + " cwd=" + parserkit.Text(header.Get("cwd"))
	if parent := parserkit.Text(header.Get("parentSession")); parent != "" {
		text += " parentSession=" + parent
	}
	return text
}

// littleCoderMarkers returns why a pi session looks driven by
// little-coder: its lc- custom message types, an assistant on the llamacpp
// provider, a checkpoint directory named after the session file. It is
// empty when nothing points at little-coder.
func littleCoderMarkers(artifact *input.Artifact, session *Session) []string {
	types := newFirstSeen(parserkit.Work{})
	llamacpp := false
	for _, entry := range session.Entries {
		record := entry.Record
		switch kind := record.Get("type"); {
		case parserkit.IsText(kind, "custom_message"):
			if custom := parserkit.Text(record.Get("customType")); strings.HasPrefix(custom, "lc-") {
				types.add(custom)
			}
		case parserkit.IsText(kind, "message"):
			message := record.Get("message")
			llamacpp = llamacpp || (parserkit.IsText(message.Get("role"), "assistant") &&
				parserkit.IsText(message.Get("provider"), "llamacpp"))
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
// set, so each text added costs one lookup whatever came before. work
// counts the lookups, which a test reads.
type firstSeen struct {
	seen  map[string]bool
	order []string
	work  parserkit.Work
}

// newFirstSeen returns an empty firstSeen that counts its lookups in work.
func newFirstSeen(work parserkit.Work) *firstSeen {
	return &firstSeen{seen: map[string]bool{}, work: work}
}

// add keeps text when it has not been met before.
func (f *firstSeen) add(text string) {
	f.work.Add("lookups", 1)
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
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
	_ parserkit.ToleranceSetter = (*Parser)(nil)
	_ parserkit.Marker          = (*Parser)(nil)
)
