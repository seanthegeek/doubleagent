package hermes

import (
	"path"
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// home is the Hermes home a path starts with: .hermes<suffix>/ or
// AppData/Local/hermes<suffix>/ under a user's home, or nothing when the
// collection base is a container volume holding the Hermes home itself;
// either one optionally followed by profiles/<name>/.
const home = `\A(?:(?:\.hermes[^/]*|AppData/Local/hermes[^/]*)/)?(?:profiles/[^/]+/)?`

// localTolerance is the matching window of the history and cron files, in
// seconds: they carry host-local times with no zone, emitted as if UTC, so
// the window spans every zone offset.
const localTolerance = 14*3600 + parserkit.DefaultTolerance

// Parser reads Hermes's session databases, their copies, histories and
// settings.
type Parser struct {
	parserkit.Base
	database, jsonl, snapshot, snapshotTime, pastes, config, pairing *regexp.Regexp
	history, cron, cronTime, responses, trajectory, connections      *regexp.Regexp
	worktree, promptFrame, responseFrame, yamlComment, yamlMapping   *regexp.Regexp
}

// New returns the parser.
func New() *Parser {
	return &Parser{
		Base:         parserkit.NewBase("hermes", "hermes"),
		database:     regexp.MustCompile(home + `state\.db\z`),
		jsonl:        regexp.MustCompile(home + `sessions/[^/]+\.jsonl\z`),
		snapshot:     regexp.MustCompile(home + `state-snapshots/([^/]+)/state\.db\z`),
		snapshotTime: regexp.MustCompile(`\A([0-9]{4})([0-9]{2})([0-9]{2})-([0-9]{2})([0-9]{2})([0-9]{2})(?:-|\z)`),
		pastes:       regexp.MustCompile(home + `pastes/pending\.json\z`),
		config:       regexp.MustCompile(home + `config\.yaml\z`),
		pairing:      regexp.MustCompile(home + `((?:platforms/)?pairing)/([^/]+)-approved\.json\z`),
		history:      regexp.MustCompile(home + `\.hermes_history\z`),
		cron:         regexp.MustCompile(home + `cron/output/([^/]+)/([^/]+)\.md\z`),
		cronTime:     regexp.MustCompile(`\A([0-9]{4}-[0-9]{2}-[0-9]{2})_([0-9]{2})-([0-9]{2})-([0-9]{2})\z`),
		responses:    regexp.MustCompile(home + `response_store\.db\z`),
		trajectory:   regexp.MustCompile(`\A(?:trajectory_samples|failed_trajectories)\.jsonl\z`),
		connections: regexp.MustCompile(`\A(?:\.config|Library/Application Support|AppData/Roaming)/` +
			`Hermes[^/]*/connections\.json\z`),
		worktree: regexp.MustCompile(`\A(.*)[/\\]\.worktrees[/\\][^/\\]+[/\\]?\z`),
		// The cron run document's length frames.
		promptFrame:   regexp.MustCompile(`(?m)^\*\*Prompt Characters:\*\* ([0-9]+)\n## Prompt\n\n`),
		responseFrame: regexp.MustCompile(`\A\*\*Response Characters:\*\* ([0-9]+)\n## Response\n\n`),
		// A YAML comment starts at # after a space or tab, a mapping value
		// at : before one.
		yamlComment: regexp.MustCompile(`[ \t]#`),
		yamlMapping: regexp.MustCompile(`:[ \t]`),
	}
}

// Marks says the parser records sessions relayed through a messaging
// platform.
func (p *Parser) Marks() parserkit.Marks {
	return parserkit.Marks{Relay: true}
}

// wanted returns the patterns of every file the parser reads.
func (p *Parser) wanted() []*regexp.Regexp {
	return []*regexp.Regexp{p.database, p.snapshot, p.jsonl, p.pastes, p.config, p.pairing,
		p.history, p.cron, p.responses, p.trajectory, p.connections}
}

// Wants reports whether the artifact is one of the files the parser reads.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the files the parser reads, one kind per pattern.
func (p *Parser) Kinds() []parserkit.Kind {
	names := []string{"state.db", "state snapshot", "session jsonl", "pending pastes", "config.yaml",
		"pairing approvals", ".hermes_history", "cron output", "response_store.db", "trajectory samples",
		"connections"}
	var kinds []parserkit.Kind
	for i, pattern := range p.wanted() {
		kinds = append(kinds, parserkit.PatternKind(names[i], pattern))
	}
	return kinds
}

// Secondary reports whether the artifact repeats the events of state.db.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	for _, pattern := range []*regexp.Regexp{p.snapshot, p.jsonl, p.history, p.cron, p.responses,
		p.trajectory} {
		if pattern.MatchString(artifact.Rel) {
			return true
		}
	}
	return false
}

// SecondaryTolerance returns the wide window of the history and cron files,
// else the default one.
func (p *Parser) SecondaryTolerance(artifact *input.Artifact) float64 {
	if p.history.MatchString(artifact.Rel) || p.cron.MatchString(artifact.Rel) {
		return localTolerance
	}
	return parserkit.DefaultTolerance
}

// Parse emits the rows of one file.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	rel := artifact.Rel
	if snapshot := p.snapshot.FindStringSubmatch(rel); snapshot != nil {
		emit(p.snapshotRow(artifact, snapshot[1]))
		p.parseDatabase(artifact, options, emit)
		return nil
	}
	switch {
	case p.database.MatchString(rel):
		p.parseDatabase(artifact, options, emit)
	case p.pastes.MatchString(rel):
		p.parsePastes(artifact, emit)
	case p.config.MatchString(rel):
		p.parseConfig(artifact, emit)
	case p.pairing.MatchString(rel):
		p.parsePairing(artifact, p.pairing.FindStringSubmatch(rel), emit)
	case p.history.MatchString(rel):
		p.parseHistory(artifact, emit)
	case p.cron.MatchString(rel):
		p.parseCron(artifact, p.cron.FindStringSubmatch(rel), emit)
	case p.responses.MatchString(rel):
		p.parseResponses(artifact, options, emit)
	case p.trajectory.MatchString(rel):
		p.parseTrajectory(artifact, emit)
	case p.connections.MatchString(rel):
		p.parseConnections(artifact, emit)
	default:
		p.parseJSONL(artifact, options, emit)
	}
	return nil
}

// snapshotRow returns the system row naming a quick snapshot, at the UTC
// time its id starts with (%Y%m%d-%H%M%S[-label]).
func (p *Parser) snapshotRow(artifact *input.Artifact, snapshotID string) timeline.Row {
	row := p.system(artifact, 0, noSession(), "state snapshot "+snapshotID+": older copy of state.db")
	if match := p.snapshotTime.FindStringSubmatch(snapshotID); match != nil {
		row.TimestampUTC = timeutil.FromText(match[1] + "-" + match[2] + "-" + match[3] + "T" +
			match[4] + ":" + match[5] + ":" + match[6] + "Z")
	}
	return row
}

// noSession returns the empty session a row with no session is built from.
func noSession() jsonvalue.Value {
	return jsonvalue.NewObject()
}

// sessionOf returns the stand-in session of an id alone.
func sessionOf(sessionID string) jsonvalue.Value {
	return jsonvalue.NewObject(jsonvalue.Member{Key: "id", Value: jsonvalue.NewString(sessionID)})
}

// displayName returns a session's display_name, or origin's chat_name in
// place of an empty one.
func displayName(session, origin jsonvalue.Value) jsonvalue.Value {
	if name := session.Get("display_name"); !name.IsEmpty() {
		return name
	}
	return origin.Get("chat_name")
}

// projectOf returns a session's project: its cwd, else its git repository
// root.
func projectOf(session jsonvalue.Value) string {
	return parserkit.Text(parserkit.FirstNonEmpty(session.Get("cwd"), session.Get("git_repo_root")))
}

// row returns a row of the artifact attributed to the session.
func (p *Parser) row(artifact *input.Artifact, line int, session jsonvalue.Value, timestamp string) timeline.Row {
	row := p.BaseRow(artifact)
	row.SourceLine = line
	row.SessionID = parserkit.Text(session.Get("id"))
	row.ProjectPath = projectOf(session)
	row.GitBranch = parserkit.Text(session.Get("git_branch"))
	row.Model = parserkit.Text(session.Get("model"))
	row.TimestampUTC = timestamp
	return row
}

// system returns a system row of the session with the text as it is.
func (p *Parser) system(artifact *input.Artifact, line int, session jsonvalue.Value, text string) timeline.Row {
	row := p.row(artifact, line, session, "")
	row.TurnType = timeline.TurnSystem
	row.Text = text
	return row
}

// copyRow returns a system row with base's attribution and time, the
// model and the text as it is.
func copyRow(base timeline.Row, text, model string) timeline.Row {
	base.TurnType = timeline.TurnSystem
	base.Model = model
	base.Text = text
	return base
}

// parseJSONL emits the rows of a sessions/<id>.jsonl copy, whose name is
// the session id.
func (p *Parser) parseJSONL(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) {
	var problems parserkit.Problems
	session := sessionOf(strings.TrimSuffix(path.Base(artifact.Rel), ".jsonl"))
	reader := p.newMessageReader(artifact, options, nil, emit)
	for number, record := range parserkit.JSONLines(artifact.DiskPath, &problems) {
		reader.message(record, session, jsonvalue.NewInt(int64(number)))
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, parserkit.Text(session.Get("id")),
		projectOf(session)))
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
	_ parserkit.ToleranceSetter = (*Parser)(nil)
	_ parserkit.Marker          = (*Parser)(nil)
)
