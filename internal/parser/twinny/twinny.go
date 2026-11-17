// Package twinny reads Twinny's conversations, standing grants, file
// changes and log channel.
//
// It reads:
//
//   - User/globalStorage/state.vscdb, the ItemTable row rjmacarthy.twinny:
//     twinny.conversations (keyed by id) and twinny.active-conversation
//     (only when its id is missing from the first), the standing grants
//     under twinny-global-context- and twinny.teamPolicy;
//   - User/workspaceStorage/<id>/state.vscdb: the review tab's owner and
//     repository, one "source host repository:" row;
//   - agent-changes/<uuid>.json, a secondary source repeating the
//     conversation's file changes;
//   - Twinny.log and its rotated copies, secondary: one system row per
//     entry, whose host-local time is read as UTC.
//
// The catalog gives the state database to the editor whose User directory
// holds it, so the parser reads every editor agent's. Messages carry no
// time: every row of a conversation takes its updatedAt, and source_line
// is 0. Provider API keys share the value and are never read. See
// research/twinny.md.
package twinny

import (
	"regexp"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/vscodestate"
)

// The keys of the extension's global state that are read.
const (
	itemKey          = "rjmacarthy.twinny"
	conversationsKey = "twinny.conversations"
	activeKey        = "twinny.active-conversation"
	globalPrefix     = "twinny-global-context-"
	teamPolicyKey    = "twinny.teamPolicy"
	workspacePrefix  = "twinny-workspace-context-"
	standingSource   = "(from state.vscdb rjmacarthy.twinny, file mtime)"
)

// standingKeys are the webview's global values that hold standing grants,
// in the order they are written.
func standingKeys() []string {
	return []string{"agentMode", "autoRunCommands", "alwaysRunCommands", "alwaysRunMcpTools"}
}

// The files read, by path relative to the home.
var (
	workspaceDatabase = regexp.MustCompile(`(?:^|/)User/workspaceStorage/[^/]+/state\.vscdb\z`)
	changesPattern    = regexp.MustCompile(`(?:^|/)User/(?:workspaceStorage/[^/]+|globalStorage)/rjmacarthy\.twinny/` +
		`agent-changes/[^/]+\.json\z`)
	logPattern = regexp.MustCompile(`(?:^|/)logs/(?:[^/]+/)+rjmacarthy\.twinny/Twinny\.log(?:_[0-9]+)?\z`)
)

// Parser reads Twinny's editor state, agent-changes records and log
// channel.
type Parser struct {
	parser.Base
}

// New returns the parser.
func New() *Parser {
	return &Parser{Base: parser.NewBase("twinny", "twinny")}
}

// ReadsAgents names the editors whose state.vscdb holds the extension's
// state.
func (p *Parser) ReadsAgents() []string {
	return vscodestate.EditorAgents()
}

// Wants reports whether the artifact is a state database, an agent-changes
// record or a Twinny log.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	rel := artifact.Rel
	return vscodestate.IsStateDatabase(rel) || workspaceDatabase.MatchString(rel) ||
		changesPattern.MatchString(rel) || logPattern.MatchString(rel)
}

// Secondary reports whether the artifact is an agent-changes record or a
// log file.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return changesPattern.MatchString(artifact.Rel) || logPattern.MatchString(artifact.Rel)
}

// Parse emits the rows of one Twinny file.
func (p *Parser) Parse(artifact *input.Artifact, options parser.Options, emit parser.Emit) error {
	rel := artifact.Rel
	switch {
	case changesPattern.MatchString(rel):
		p.changes(artifact, emit)
	case logPattern.MatchString(rel):
		p.log(artifact, emit)
	default:
		p.state(artifact, options, emit)
	}
	return nil
}

// state emits the rows of the extension's row of a state database, then a
// row per problem, which never echoes the value: it holds API keys.
func (p *Parser) state(artifact *input.Artifact, options parser.Options, emit parser.Emit) {
	state, problems := vscodestate.ReadItem(artifact.DiskPath, itemKey)
	switch {
	case state.IsObject() && workspaceDatabase.MatchString(artifact.Rel):
		p.workspaceRows(artifact, state, emit)
	case state.IsObject():
		p.globalRows(artifact, state, options, emit)
	case !state.IsNull() && len(problems) == 0:
		problems = []string{itemKey + " value is not a JSON object"}
	}
	for _, problem := range problems {
		emit(p.ProblemRow(artifact, vscodestate.ShownProblem(problem)))
	}
}

// globalRows emits each conversation, then the standing grants.
func (p *Parser) globalRows(artifact *input.Artifact, state jsonvalue.Value, options parser.Options,
	emit parser.Emit) {
	var listed []jsonvalue.Value
	switch all := state.Get(conversationsKey); {
	case all.IsObject():
		for _, member := range all.Members() {
			listed = append(listed, member.Value)
		}
	case all.IsArray():
		listed = all.Items()
	}
	var conversations []jsonvalue.Value
	ids := map[string]bool{}
	for _, conversation := range listed {
		if conversation.IsObject() {
			conversations = append(conversations, conversation)
			ids[conversation.Get("id").Text()] = true
		}
	}
	active := state.Get(activeKey)
	if active.IsObject() && !active.Get("messages").IsEmpty() && !ids[active.Get("id").Text()] {
		conversations = append(conversations, active)
	}
	for _, conversation := range conversations {
		p.conversation(artifact, conversation, options, emit)
	}
	p.standingRows(artifact, state, emit)
}

// systemRow returns a system row of the artifact at timestamp.
func (p *Parser) systemRow(artifact *input.Artifact, timestamp, text string) timeline.Row {
	row := p.SystemRow(artifact, text)
	row.TimestampUTC = timestamp
	return row
}

// standingRows emits the standing grants and the team policy, at the
// file's modification time.
func (p *Parser) standingRows(artifact *input.Artifact, state jsonvalue.Value, emit parser.Emit) {
	timestamp := ""
	stamp := func() string {
		if timestamp == "" {
			timestamp = parser.FileMtime(artifact)
		}
		return timestamp
	}
	for _, key := range standingKeys() {
		value := state.Get(globalPrefix + key)
		if value.IsNull() {
			continue
		}
		stamp()
		label := "permission setting"
		if key == "agentMode" {
			label = "permission mode"
		}
		values := []jsonvalue.Value{value}
		if value.IsArray() {
			values = value.Items()
		}
		for _, item := range values {
			text := label + ": " + key + "=" + parser.SettingValue(item) + " " + standingSource
			emit(p.systemRow(artifact, timestamp, text))
		}
	}
	if policy := state.Get(teamPolicyKey); policy.IsObject() {
		stamp()
		flatten(teamPolicyKey, policy, func(key, value string) {
			emit(p.systemRow(artifact, timestamp, "permission setting: "+key+"="+value+" "+standingSource))
		})
	}
}

// flatten passes (dotted key, value text) for each leaf of value to yield,
// one pair per array entry; a null leaf outside an array gives nothing.
func flatten(prefix string, value jsonvalue.Value, yield func(key, value string)) {
	switch {
	case value.IsObject():
		for _, member := range value.Members() {
			flatten(prefix+"."+member.Key, member.Value, yield)
		}
	case value.IsArray():
		for _, item := range value.Items() {
			if item.IsObject() || item.IsArray() {
				yield(prefix, parser.JSONText(item))
			} else {
				yield(prefix, parser.SettingValue(item))
			}
		}
	case !value.IsNull():
		yield(prefix, parser.SettingValue(value))
	}
}

// workspaceRows emits the repository the pull request review tab last
// listed.
func (p *Parser) workspaceRows(artifact *input.Artifact, state jsonvalue.Value, emit parser.Emit) {
	owner := state.Get(workspacePrefix + "reviewOwner")
	repository := state.Get(workspacePrefix + "reviewRepo")
	if !owner.IsString() || owner.StringValue() == "" || !repository.IsString() || repository.StringValue() == "" {
		return
	}
	text := "source host repository: https://github.com/" + owner.StringValue() + "/" + repository.StringValue() +
		" type=review (from workspaceStorage state.vscdb rjmacarthy.twinny, file mtime)"
	emit(p.systemRow(artifact, parser.FileMtime(artifact), text))
}

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parser.Parser          = (*Parser)(nil)
	_ parser.AgentReader     = (*Parser)(nil)
	_ parser.SecondaryReader = (*Parser)(nil)
)
