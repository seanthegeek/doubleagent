// Package vscode reads VS Code's chat sessions (GitHub Copilot Chat and
// other participants) and chat state.
//
// The store is VS Code's own, so the parser covers every data directory the
// catalog gives to vscode. It reads, under User/:
//
//   - workspaceStorage/<hash>/chatSessions/<id>.jsonl, a mutation log
//     replayed into the session (kind 0 sets the state, 1 sets at path k,
//     2 pushes after truncating to i, 3 deletes), or <id>.json, and the
//     same under globalStorage/emptyWindowChatSessions/ and
//     transferredChatSessions/; the project is workingDirectory, else the
//     sibling workspace.json's folder;
//   - globalStorage/github.copilot-chat/cloudSessions.json and the
//     GitHub.copilot-chat row of state.vscdb: "session synced:", "session
//     origin:" and "source host repository:" rows;
//   - every profile's and workspace's state.vscdb: the standing tool grants
//     and, in a profile, the GitHub account usage keys; secret:// rows are
//     never read;
//   - a profile's settings.json: the auto-approve keys and the default
//     model;
//   - chatEditingSessions/<key>/state.json, a secondary source repeating
//     which files a request edited, which survives the chat's delete.
//
// See research/vscode.md.
package vscode

import (
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// userDir matches the start of a path under an editor data directory's
// User/ that the catalog gives to vscode.
const userDir = `^(?:(?:\.config|Library/Application Support|AppData/Roaming)/(?:Code[^/]*|VSCodium[^/]*|Positron|Trae[^/]*)` +
	`|\.vscode-server[^/]*/data|\.vscodium-server[^/]*/data|\.positron-server/data)/User/`

// The files read, by path relative to the home. A state database is a
// profile's (globalStorage/, also the application scope, or
// profiles/<id>/globalStorage/) or a workspace's.
var (
	sessionPattern = regexp.MustCompile(userDir + `(?:workspaceStorage/[^/]+/chatSessions/[^/]+\.jsonl?` +
		`|globalStorage/emptyWindowChatSessions/[^/]+\.jsonl?|globalStorage/transferredChatSessions/[^/]+\.json)\z`)
	cloudSessionsPattern = regexp.MustCompile(userDir + `globalStorage/github\.copilot-chat/cloudSessions\.json\z`)
	statePattern         = regexp.MustCompile(userDir + `(globalStorage|profiles/(?:builtin/)?[^/]+/globalStorage` +
		`|workspaceStorage/[^/]+)/state\.vscdb\z`)
	settingsPattern = regexp.MustCompile(userDir + `(?:profiles/(?:builtin/)?[^/]+/)?settings\.json\z`)
	editingPattern  = regexp.MustCompile(userDir + `workspaceStorage/[^/]+/chatEditingSessions/([^/]+)/state\.json\z`)
)

// Parser reads VS Code's chat sessions, cloud session markers, state
// databases and settings.
type Parser struct {
	parser.Base
	// workspaces holds each workspace.json read, by its state.
	workspaces map[workspaceState]*workspaceFile
}

// New returns the parser.
func New() *Parser {
	return &Parser{Base: parser.NewBase("vscode", "vscode")}
}

// Marks says that VS Code records which sessions are synced to the cloud
// agent and which began there.
func (p *Parser) Marks() parser.Marks {
	return parser.Marks{Sync: true, Origin: true}
}

// Wants reports whether the artifact is a chat session, cloudSessions.json,
// a state database, settings or a chat editing state.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	rel := artifact.Rel
	return sessionPattern.MatchString(rel) || cloudSessionsPattern.MatchString(rel) ||
		statePattern.MatchString(rel) || settingsPattern.MatchString(rel) || editingPattern.MatchString(rel)
}

// Secondary reports whether the artifact is a chat editing state, which
// repeats which files a request edited.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return editingPattern.MatchString(artifact.Rel)
}

// Parse emits the rows of one VS Code file.
func (p *Parser) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	rel := artifact.Rel
	switch {
	case cloudSessionsPattern.MatchString(rel):
		p.cloudSessions(artifact, emit)
	case statePattern.MatchString(rel):
		scope := statePattern.FindStringSubmatch(rel)[1]
		p.stateDatabase(artifact, strings.HasPrefix(scope, "workspaceStorage/"), emit)
	case settingsPattern.MatchString(rel):
		p.settings(artifact, emit)
	case editingPattern.MatchString(rel):
		p.editing(artifact, editingPattern.FindStringSubmatch(rel)[1], emit)
	default:
		p.session(artifact, options, emit)
	}
	return nil
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parser.Parser          = (*Parser)(nil)
	_ parser.Marker          = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
)
