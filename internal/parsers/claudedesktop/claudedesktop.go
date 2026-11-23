// Package claudedesktop reads the Claude Desktop app's local agent
// sessions: Cowork, the Code tab and scheduled tasks.
//
// Under the app-data directory (Claude or Claude-3p under Library/
// Application Support, .config, AppData/Roaming or AppData/Local,
// AppData/Local/Claude-Data, or the MSIX build's AppData/Local/Packages/
// <package>/LocalCache/Roaming/Claude) it reads: the Claude Code
// transcripts under local-agent-mode-sessions/<account>/<org>/[agent/]
// <session dir>/.claude/projects/ and claude-code-sessions/<account>/<org>/
// (imported-staging/ included), in Claude Code's own format; each session
// dir's audit.jsonl and the session records local_<uuid>.json, both
// secondary sources; and scheduled-tasks.json, git-worktrees.json and
// remote-session-spaces.json.
//
// A transcript's rows take the session record's folder as project path.
// The audit log gives user, assistant, tool, model, permission, result and
// init rows; the records, scheduled tasks, worktrees and remote folder
// grants give system rows. Every createdAt, errorAt and fireAt is epoch
// milliseconds. The app is closed source: the shapes come from its bundle.
// See research/claude-desktop.md.
package claudedesktop

import (
	"regexp"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers/claudecode"
)

// The app-data directory, the two session trees under it, and the
// artifacts read, by path relative to the home. Group "sess" is a session
// dir, relative to the home.
const (
	appData = `^(?:Library/Application Support/Claude(?:-3p)?` +
		`|\.config/Claude(?:-3p)?` +
		`|AppData/Roaming/Claude(?:-3p)?` +
		`|AppData/Local/Claude(?:-3p)?` +
		`|AppData/Local/Claude-Data(?:/[^/]+)?` +
		`|AppData/Local/Packages/(?:AnthropicPBC\.)?Claude_[^/]+/LocalCache/Roaming/Claude)/`
	trees = `(?:local-agent-mode-sessions|claude-code-sessions)/[^/]+/[^/]+/`
)

var (
	transcriptPattern = regexp.MustCompile(`(?P<sess>` + appData + trees +
		`(?:agent/)?[^/]+)/\.claude/projects/[^/]+/` +
		`(?:[^/]+\.jsonl|[^/]+/subagents/(?:[^/]+/)*agent-[^/]+\.jsonl)\z`)
	stagedPattern = regexp.MustCompile(appData +
		`claude-code-sessions/[^/]+/[^/]+/imported-staging/[^/]+\.jsonl\z`)
	auditPattern = regexp.MustCompile(`(?P<sess>` + appData +
		`local-agent-mode-sessions/[^/]+/[^/]+/(?:agent/)?[^/]+)/audit\.jsonl\z`)
	recordPattern = regexp.MustCompile(appData + `(?P<tree>` + trees +
		`)(?:agent/)?local_[0-9a-f-]{36}\.json\z`)
	scheduledTasksPattern = regexp.MustCompile(appData + trees + `scheduled-tasks\.json\z`)
	worktreePattern       = regexp.MustCompile(appData + `git-worktrees\.json\z`)
	spacesPattern         = regexp.MustCompile(appData +
		`local-agent-mode-sessions/[^/]+/[^/]+/remote-session-spaces\.json\z`)
)

// Parser reads Claude Desktop's session transcripts, audit logs and session
// files.
type Parser struct {
	parserkit.Base
	code *claudecode.Reader
	// records holds the session record found for each session dir looked
	// up, by home and session dir.
	records map[recordKey]foundRecord
	// transcripts holds each .claude/projects directory a Code tab record
	// looked for its transcript in, indexed once for the run.
	transcripts map[string]*parserkit.ChildListings
	// shortNames holds, per org or agent/ directory a short-named session
	// dir looked its record up in, the record names of each short name.
	shortNames map[string]map[string][]string
	// sessionDirs holds, per home, whether each session dir a record named
	// holds an audit log or a transcript, by the entry it opens.
	sessionDirs *parserkit.PerHome[map[parserkit.EntryKey]bool]
}

// New returns the parser.
func New() *Parser {
	base := parserkit.NewBase("claude-desktop", "claude-desktop")
	return &Parser{Base: base, code: claudecode.NewReader(base),
		sessionDirs: parserkit.NewPerHome(func() map[parserkit.EntryKey]bool { return map[parserkit.EntryKey]bool{} })}
}

// Wants reports whether the artifact is a transcript, an audit log, a
// session record or one of the app's state files.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Kinds returns the session transcripts, the staged imports, the audit
// logs, the session records, the scheduled tasks, the worktree list and
// the remote session spaces.
func (p *Parser) Kinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.PatternKind("session transcript", transcriptPattern),
		parserkit.PatternKind("imported-staging transcript", stagedPattern),
		parserkit.PatternKind("audit.jsonl", auditPattern), parserkit.PatternKind("session record", recordPattern),
		parserkit.PatternKind("scheduled-tasks.json", scheduledTasksPattern),
		parserkit.PatternKind("git-worktrees.json", worktreePattern),
		parserkit.PatternKind("remote-session-spaces.json", spacesPattern)}
}

// Secondary reports whether the artifact is an audit log or a session
// record.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return auditPattern.MatchString(artifact.Rel) || recordPattern.MatchString(artifact.Rel)
}

// Parse emits the rows of one Claude Desktop file.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
	rel := artifact.Rel
	switch {
	case transcriptPattern.MatchString(rel):
		p.transcript(artifact, options, sessionRel(transcriptPattern, rel), emit)
	case stagedPattern.MatchString(rel):
		p.transcript(artifact, options, "", emit)
	case auditPattern.MatchString(rel):
		p.audit(artifact, options, sessionRel(auditPattern, rel), emit)
	case recordPattern.MatchString(rel):
		p.record(artifact, emit)
	case scheduledTasksPattern.MatchString(rel):
		p.scheduled(artifact, emit)
	case worktreePattern.MatchString(rel):
		p.worktrees(artifact, emit)
	case spacesPattern.MatchString(rel):
		p.spaces(artifact, emit)
	}
	return nil
}

// sessionRel returns the session dir, relative to the home, that
// pattern's group "sess" finds in rel.
func sessionRel(pattern *regexp.Regexp, rel string) string {
	return pattern.FindStringSubmatch(rel)[pattern.SubexpIndex("sess")]
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
)
