// Package copilotcli reads the session state of the GitHub Copilot CLI.
//
// It reads, under .copilot (or .local/state/.copilot and .config/.copilot on
// installs from before the XDG move): session-state/<id>/events.jsonl (the
// primary store), session-state/<id>/workspace.yaml, and as secondary
// sources session-store.db, session-state/<id>/rewind-file-snapshots/index.json
// and command-history-state.json; the standing grants of
// permissions-config.json and settings.json (JSON with comments);
// sidebar-sessions-state/*.json (a session-to-cwd join, no rows); and
// logs/process-*.log. See research/copilot-cli.md for the fields each
// event, file and table carries; the Copilot Desktop app's bundled CLI
// writes session.permissions_changed with the allow-all keys instead of
// mode.
package copilotcli

import (
	"cmp"
	"regexp"
	"slices"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// turnTolerance is the pairing window of the store and the snapshots:
// turns.timestamp and the snapshot time are stamped at the end of a turn (or
// at a time not measured against the prompt's), so the prompt and the reply
// of the log are up to a turn's length earlier.
const turnTolerance = 3600.0

// notSpace matches one character that is not whitespace by
// unicode.IsSpace.
const notSpace = `[^` + parser.Space + `]`

// Parser reads the Copilot CLI's files. It keeps the session-to-directory
// joins of the sidebar directories it has read.
type Parser struct {
	parser.Base
	events, workspace, snapshots, store, history, permissions, settings, sidebar, log *regexp.Regexp
	homePrefix, signedIn, workspaceLine, logTime                                      *regexp.Regexp
	sidebarCache                                                                      map[string]map[string]string
}

// New returns the parser.
func New() *Parser {
	const home = `(?:\.copilot|\.local/state/\.copilot|\.config/\.copilot)`
	const session = `\A` + home + `/session-state/([0-9a-f-]{36})/`
	return &Parser{
		Base:          parser.NewBase("copilot-cli", "copilot-cli"),
		events:        regexp.MustCompile(session + `events\.jsonl\z`),
		workspace:     regexp.MustCompile(session + `workspace\.yaml\z`),
		snapshots:     regexp.MustCompile(session + `rewind-file-snapshots/index\.json\z`),
		store:         regexp.MustCompile(`\A` + home + `/session-store\.db\z`),
		history:       regexp.MustCompile(`\A` + home + `/command-history-state\.json\z`),
		permissions:   regexp.MustCompile(`\A` + home + `/permissions-config\.json\z`),
		settings:      regexp.MustCompile(`\A` + home + `/settings\.json\z`),
		sidebar:       regexp.MustCompile(`\A` + home + `/sidebar-sessions-state/[0-9a-f]{64}\.json\z`),
		log:           regexp.MustCompile(`\A` + home + `/logs/process-[0-9]+-[0-9]+\.log\z`),
		homePrefix:    regexp.MustCompile(`\A(` + home + `)/`),
		signedIn:      regexp.MustCompile(`Signed in to (` + notSpace + `+) as (.+?)\.?[ \t]*\z`),
		workspaceLine: regexp.MustCompile(`Workspace initialized: ([0-9a-f-]{36})`),
		logTime:       regexp.MustCompile(`\A([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9:.]+Z)[ \t]`),
		sidebarCache:  map[string]map[string]string{},
	}
}

// Marks says the parser records synced sessions.
func (p *Parser) Marks() parser.Marks {
	return parser.Marks{Sync: true}
}

// patterns returns the patterns of the files the parser reads.
func (p *Parser) patterns() []*regexp.Regexp {
	return []*regexp.Regexp{p.events, p.workspace, p.snapshots, p.store, p.history, p.permissions,
		p.settings, p.sidebar, p.log}
}

// Wants reports whether the artifact is one of the files the parser reads.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return slices.ContainsFunc(p.patterns(), func(pattern *regexp.Regexp) bool {
		return pattern.MatchString(artifact.Rel)
	})
}

// Secondary reports whether the artifact repeats events of the session
// logs.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	rel := artifact.Rel
	return p.store.MatchString(rel) || p.history.MatchString(rel) || p.snapshots.MatchString(rel)
}

// SecondaryTolerance returns the pairing window: a turn's length for the
// store and the snapshots.
func (p *Parser) SecondaryTolerance(artifact *input.Artifact) float64 {
	if p.store.MatchString(artifact.Rel) || p.snapshots.MatchString(artifact.Rel) {
		return turnTolerance
	}
	return parser.DefaultTolerance
}

// Parse emits the rows of one artifact, by the file it is.
func (p *Parser) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	rel := artifact.Rel
	switch {
	case p.events.MatchString(rel):
		p.parseEvents(artifact, p.events.FindStringSubmatch(rel)[1], options, emit)
	case p.workspace.MatchString(rel):
		p.parseWorkspace(artifact, p.workspace.FindStringSubmatch(rel)[1], emit)
	case p.snapshots.MatchString(rel):
		p.parseSnapshots(artifact, p.snapshots.FindStringSubmatch(rel)[1], emit)
	case p.store.MatchString(rel):
		p.parseStore(artifact, emit)
	case p.history.MatchString(rel):
		p.parseHistory(artifact, emit)
	case p.permissions.MatchString(rel):
		p.parsePermissions(artifact, emit)
	case p.settings.MatchString(rel):
		p.parseSettings(artifact, emit)
	case p.log.MatchString(rel):
		p.parseLog(artifact, emit)
	case p.sidebar.MatchString(rel):
		p.parseSidebar(artifact, emit)
	}
	return nil
}

// columns are the fields of a row that a call sets beyond its turn type and
// text.
type columns struct {
	timestamp, sessionID, project, branch, model, toolName, toolUseID string
}

// row returns a row of the artifact.
func (p *Parser) row(artifact *input.Artifact, line int, turn, text string, set columns) timeline.Row {
	row := p.BaseRow(artifact)
	row.SourceLine = line
	row.TurnType = turn
	row.Text = timeline.Compact(text)
	row.TimestampUTC, row.SessionID, row.ProjectPath = set.timestamp, set.sessionID, set.project
	row.GitBranch, row.Model, row.ToolName, row.ToolUseID = set.branch, set.model, set.toolName,
		set.toolUseID
	return row
}

// problem returns the row of a problem with a whole file: a system row at
// line 1.
func (p *Parser) problem(artifact *input.Artifact, text, timestamp string) timeline.Row {
	return p.row(artifact, 1, timeline.TurnSystem, text, columns{timestamp: timestamp})
}

// argumentKeys are the arguments of a tool call that stand for the call, in
// the order they are tried.
var argumentKeys = []string{"command", "url", "query", "path", "input"}

// argsText returns the tool_use text of a call's arguments: a string
// argument (a custom grammar tool such as apply_patch) as it is, else the
// first of argumentKeys, else compact JSON.
func argsText(arguments jsonvalue.Value) string {
	switch {
	case arguments.IsString():
		return arguments.StringValue()
	case arguments.IsObject():
		for _, key := range argumentKeys {
			if value := arguments.Get(key); parser.Present(value) {
				return value.Text()
			}
		}
		if arguments.Len() > 0 {
			return parser.JSONText(arguments)
		}
		return ""
	}
	return arguments.Text()
}

// exitSucceeded reports whether an exit code is no failure: null, a number
// equal to zero or the text "0".
func exitSucceeded(value jsonvalue.Value) bool {
	if value.IsNull() || parser.IsText(value, "0") {
		return true
	}
	if value.IsBool() {
		return false
	}
	number, ok := value.Float()
	return value.IsNumber() && ok && number == 0
}

// resultText returns the tool_result text of a tool.execution_complete
// event. detailedContent is the complete result and content the concise one
// sent to the model, except that a successful skill invocation puts the
// concise text in detailedContent, so the longer of the two is taken,
// detailedContent on a tie.
func resultText(data jsonvalue.Value) string {
	result := data.Get("result")
	detailed, concise := result.Get("detailedContent").Text(), result.Get("content").Text()
	body := detailed
	if utf8.RuneCountInString(concise) > utf8.RuneCountInString(detailed) {
		body = concise
	}
	if code := data.Get("shellExecution").Get("exitCode"); !exitSucceeded(code) && !code.IsBool() {
		body = "[exit " + code.Text() + "] " + body
	}
	if success, ok := data.Get("success").Bool(); ok && !success {
		if message := data.Get("error").Get("message").Text(); message != "" {
			if body != "" {
				body += "\n"
			}
			body += message
		}
	}
	return body
}

// requestTarget returns what a permissionRequest asks about, or "": the
// command, URL, file or path, or "<server>/<tool>" for an MCP tool.
func requestTarget(request jsonvalue.Value) string {
	for _, key := range []string{"fullCommandText", "url", "fileName", "path"} {
		if value := request.Get(key); parser.Present(value) {
			return value.Text()
		}
	}
	server, tool := request.Get("serverName").Text(), request.Get("toolName").Text()
	switch {
	case server != "":
		return server + "/" + tool
	case tool != "":
		return tool
	}
	return ""
}

// decisionKeys are the permission.completed result keys appended to the
// decision row, in order, with the label each is written under.
var decisionKeys = [][2]string{{"locationKey", "location"}, {"approval", "approval"},
	{"directories", "directories"}, {"feedback", "feedback"}, {"rules", "rules"},
	{"reason", "reason"}, {"path", "path"}, {"message", "message"}}

// decisionText returns "permission decision: <kind> source=<decisionSource
// or -> ..." for a permission.completed event.
func decisionText(data jsonvalue.Value) string {
	result := data.Get("result")
	line := "permission decision: " + cmp.Or(result.Get("kind").Text(), "-") + " source=" +
		cmp.Or(data.Get("decisionSource").Text(), "-")
	for _, key := range decisionKeys {
		value := result.Get(key[0])
		empty := !parser.Present(value) || (value.IsArray() && value.Len() == 0) ||
			(value.IsObject() && value.Len() == 0)
		if !empty {
			line += " " + key[1] + "=" + parser.SettingValue(value)
		}
	}
	return line
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parser.Parser          = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
	_ parser.ToleranceSetter = (*Parser)(nil)
	_ parser.Marker          = (*Parser)(nil)
)
