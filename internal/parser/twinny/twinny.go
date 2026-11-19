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
// is 0. Each provider holding an API key gives a "model provider:" row with
// the key through parser.SecretText. See research/twinny.md.
package twinny

import (
	"cmp"
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
	providersKey     = "twinny.inference-providers"
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

// globalRows emits each conversation, then the standing grants and the
// providers. A conversations value that is neither an object nor a list,
// or that holds conversations that are not objects, adds a problem.
func (p *Parser) globalRows(artifact *input.Artifact, state jsonvalue.Value, options parser.Options,
	emit parser.Emit) {
	var problems parser.Problems
	var conversations []jsonvalue.Value
	ids := map[string]bool{}
	keep := func(conversation jsonvalue.Value) {
		conversations = append(conversations, conversation)
		ids[conversation.Get("id").Text()] = true
	}
	if all := state.Get(conversationsKey); all.IsArray() {
		for _, conversation := range parser.EntriesOfKind(all, conversationsKey, jsonvalue.Object, 0, &problems) {
			keep(conversation)
		}
	} else {
		for _, conversation := range parser.MembersOfKind(all, conversationsKey, jsonvalue.Object, 0, &problems) {
			keep(conversation)
		}
	}
	p.problemRows(artifact, &problems, emit)
	active := state.Get(activeKey)
	if active.IsObject() && !active.Get("messages").IsEmpty() && !ids[active.Get("id").Text()] {
		conversations = append(conversations, active)
	}
	for _, conversation := range conversations {
		p.conversation(artifact, conversation, options, emit)
	}
	p.standingRows(artifact, state, emit)
	p.providerRows(artifact, state, emit)
}

// problemRows emits a row per problem and empties the list.
func (p *Parser) problemRows(artifact *input.Artifact, problems *parser.Problems, emit parser.Emit) {
	for _, problem := range *problems {
		emit(p.ProblemRow(artifact, problem.Message))
	}
	*problems = nil
}

// providerRows emits, at the file's modification time, one "model
// provider:" row per provider that holds an API key, of either kind: one
// of twinny.inference-providers, named by its id, else by its key there,
// or an active provider, the key through parser.SecretText; then one row
// for a provider list that is not an object, one for all of its providers
// that are not objects, and one per active provider that is not an object.
func (p *Parser) providerRows(artifact *input.Artifact, state jsonvalue.Value, emit parser.Emit) {
	timestamp := ""
	row := func(text string, provider jsonvalue.Value, source string) {
		key := parser.Text(provider.Get("apiKey"))
		if key == "" {
			return
		}
		if timestamp == "" {
			timestamp = parser.FileMtime(artifact)
		}
		emit(p.systemRow(artifact, timestamp, timeline.Compact(text+" apiKey="+parser.SecretText(artifact, key)+
			" (from "+itemKey+" "+source+", file mtime)")))
	}
	var problems parser.Problems
	for providerKey, provider := range parser.MembersOfKind(state.Get(providersKey), providersKey, jsonvalue.Object,
		0, &problems) {
		text := "model provider: " + cmp.Or(parser.Text(provider.Get("id")), providerKey)
		for _, field := range [][2]string{{"label", "label"}, {"provider", "provider"}, {"type", "type"},
			{"modelName", "model"}} {
			if value := parser.Text(provider.Get(field[0])); value != "" {
				text += " " + field[1] + "=" + value
			}
		}
		row(text, provider, providersKey)
	}
	for _, kind := range []string{"chat", "fim", "embeddings"} {
		source := "twinny.active-" + kind + "-provider"
		provider := state.Get(source)
		parser.ObjectMember(provider, source, 0, &problems)
		row("model provider: active "+kind+" "+cmp.Or(parser.Text(provider.Get("id")), "-"), provider, source)
	}
	p.problemRows(artifact, &problems, emit)
}

// systemRow returns a system row of the artifact at timestamp.
func (p *Parser) systemRow(artifact *input.Artifact, timestamp, text string) timeline.Row {
	row := p.SystemRow(artifact, text)
	row.TimestampUTC = timestamp
	return row
}

// standingRows emits the standing grants and the team policy, at the
// file's modification time; a team policy that is not an object adds a
// problem.
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
	switch policy := state.Get(teamPolicyKey); {
	case policy.IsObject():
		stamp()
		flatten(teamPolicyKey, policy, func(key, value string) {
			emit(p.systemRow(artifact, timestamp, "permission setting: "+key+"="+value+" "+standingSource))
		})
	case !policy.IsNull():
		emit(p.ProblemRow(artifact, parser.WrongKind(teamPolicyKey, jsonvalue.Object)))
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
