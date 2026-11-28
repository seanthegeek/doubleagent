package twinny

import (
	"cmp"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
	workspaceDatabase = parserkit.MustRelPattern(`(?:^|/)User/workspaceStorage/[^/]+/state\.vscdb\z`)
	changesPattern    = parserkit.MustRelPattern(`(?:^|/)User/(?:workspaceStorage/[^/]+|globalStorage)/rjmacarthy\.twinny/` +
		`agent-changes/[^/]+\.json\z`)
	logPattern = parserkit.MustRelPattern(`(?:^|/)logs/(?:[^/]+/)+rjmacarthy\.twinny/Twinny\.log(?:_[0-9]+)?\z`)
)

// Parser reads Twinny's editor state, agent-changes records and log
// channel.
type Parser struct {
	parserkit.Base
	// shown holds, per home, the ids of the conversations each global state
	// database gives rows for, by the entry it opens, read once however
	// many workspaces look them up.
	shown *parserkit.PerHome[map[parserkit.EntryKey]map[string]bool]
}

// New returns the parser.
func New() *Parser {
	return &Parser{Base: parserkit.NewBase("twinny", "twinny"),
		shown: parserkit.NewPerHome(func() map[parserkit.EntryKey]map[string]bool {
			return map[parserkit.EntryKey]map[string]bool{}
		})}
}

// ReadsAgents names the editors whose state.vscdb holds the extension's
// state.
func (p *Parser) ReadsAgents() []string {
	return vscodestate.EditorAgents()
}

// Kinds returns the editors' global and workspace state databases, the
// agent changes and the log.
func (p *Parser) Kinds() []parserkit.Kind {
	return []parserkit.Kind{parserkit.RelFuncKind("state.vscdb", vscodestate.IsStateDatabase),
		parserkit.PatternKind("workspace state.vscdb", workspaceDatabase), parserkit.PatternKind("agent changes", changesPattern),
		parserkit.PatternKind("Twinny.log", logPattern)}
}

// Wants reports whether the artifact is a state database, an agent-changes
// record or a Twinny log.
func (p *Parser) Wants(artifact *input.Artifact) bool {
	return parserkit.WantsAny(p.Kinds(), artifact)
}

// Secondary reports whether the artifact is an agent-changes record or a
// log file.
func (p *Parser) Secondary(artifact *input.Artifact) bool {
	return changesPattern.MatchString(artifact.Rel) || logPattern.MatchString(artifact.Rel)
}

// systemRow returns a system row of the artifact at timestamp.
func (p *Parser) systemRow(artifact *input.Artifact, timestamp, text string) timeline.Row {
	row := p.SystemRow(artifact, text)
	row.TimestampUTC = timestamp
	return row
}

// providerRows emits, at the file's modification time, one "model
// provider:" row per provider holding an API key, an entry of
// twinny.inference-providers (named by its id, else its key there) or an
// active provider, the key through parserkit.SecretText; then the problems
// of providers that are not objects.
func (p *Parser) providerRows(artifact *input.Artifact, state jsonvalue.Value, emit parserkit.Emit) {
	timestamp := ""
	row := func(text string, provider jsonvalue.Value, source string) {
		key := parserkit.Text(provider.Get("apiKey"))
		if key == "" {
			return
		}
		if timestamp == "" {
			timestamp = parserkit.FileMtime(artifact)
		}
		emit(p.systemRow(artifact, timestamp, timeline.Compact(text+" apiKey="+parserkit.SecretText(artifact, key)+
			" (from "+itemKey+" "+source+", file mtime)")))
	}
	var problems parserkit.Problems
	for providerKey, provider := range parserkit.MembersOfKind(state.Get(providersKey), providersKey, jsonvalue.Object,
		0, &problems) {
		text := "model provider: " + cmp.Or(parserkit.Text(provider.Get("id")), providerKey)
		for _, field := range [][2]string{{"label", "label"}, {"provider", "provider"}, {"type", "type"},
			{"modelName", "model"}} {
			if value := parserkit.Text(provider.Get(field[0])); value != "" {
				text += " " + field[1] + "=" + value
			}
		}
		row(text, provider, providersKey)
	}
	for _, kind := range []string{"chat", "fim", "embeddings"} {
		source := "twinny.active-" + kind + "-provider"
		provider := state.Get(source)
		parserkit.ObjectMember(provider, source, 0, &problems)
		row("model provider: active "+kind+" "+cmp.Or(parserkit.Text(provider.Get("id")), "-"), provider, source)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// flatten passes (dotted key, value text) for each leaf of value to yield,
// one pair per array entry; a null leaf outside an array gives nothing.
// The key written before each member or entry is cut by carried.
func flatten(prefix string, value jsonvalue.Value, carried *parserkit.Carried, yield func(key, value string)) {
	switch {
	case value.IsObject():
		for _, member := range value.Members() {
			flatten(carried.Cut("settings key", prefix)+"."+member.Key, member.Value, carried, yield)
		}
	case value.IsArray():
		for _, item := range value.Items() {
			if item.IsObject() || item.IsArray() {
				yield(carried.Cut("settings key", prefix), parserkit.JSONText(item))
			} else {
				yield(carried.Cut("settings key", prefix), parserkit.SettingValue(item))
			}
		}
	case !value.IsNull():
		yield(prefix, parserkit.SettingValue(value))
	}
}

// standingRows emits the standing grants and the team policy, at the
// file's modification time.
func (p *Parser) standingRows(artifact *input.Artifact, state jsonvalue.Value, emit parserkit.Emit) {
	timestamp := ""
	stamp := func() string {
		if timestamp == "" {
			timestamp = parserkit.FileMtime(artifact)
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
			text := label + ": " + key + "=" + parserkit.SettingValue(item) + " " + standingSource
			emit(p.systemRow(artifact, timestamp, text))
		}
	}
	switch policy := state.Get(teamPolicyKey); {
	case policy.IsObject():
		stamp()
		carried := parserkit.NewCarried(artifact)
		flatten(teamPolicyKey, policy, carried, func(key, value string) {
			emit(p.systemRow(artifact, timestamp, "permission setting: "+key+"="+value+" "+standingSource))
		})
		var problems parserkit.Problems
		carried.Report(0, &problems)
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
	case !policy.IsNull():
		emit(p.ProblemRow(artifact, parserkit.WrongKind(teamPolicyKey, jsonvalue.Object)))
	}
}

// opensUnsaved reports whether active, an open conversation, has messages
// (only an object has members) and an id that is none of shown.
func opensUnsaved(active jsonvalue.Value, shown map[string]bool) bool {
	return !active.Get("messages").IsEmpty() && !shown[active.Get("id").Text()]
}

// shownConversations returns the conversations a global state value gives
// rows for, and their ids: every member of twinny.conversations, then the
// open conversation when opensUnsaved says so. A member of the wrong kind
// adds a problem.
func shownConversations(state jsonvalue.Value, problems *parserkit.Problems) ([]jsonvalue.Value, map[string]bool) {
	var conversations []jsonvalue.Value
	shown := map[string]bool{}
	keep := func(conversation jsonvalue.Value) {
		conversations = append(conversations, conversation)
		shown[conversation.Get("id").Text()] = true
	}
	if all := state.Get(conversationsKey); all.IsArray() {
		for _, conversation := range parserkit.EntriesOfKind(all, conversationsKey, jsonvalue.Object, 0, problems) {
			keep(conversation)
		}
	} else {
		for _, conversation := range parserkit.MembersOfKind(all, conversationsKey, jsonvalue.Object, 0, problems) {
			keep(conversation)
		}
	}
	if active := state.Get(activeKey); opensUnsaved(active, shown) {
		keep(active)
	}
	return conversations, shown
}

// globalRows emits each conversation, then the standing grants and the
// providers.
func (p *Parser) globalRows(artifact *input.Artifact, state jsonvalue.Value, options parserkit.Options,
	emit parserkit.Emit) {
	var problems parserkit.Problems
	conversations, _ := shownConversations(state, &problems)
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
	for _, conversation := range conversations {
		p.conversation(artifact, conversation, "", options, emit)
	}
	p.standingRows(artifact, state, emit)
	p.providerRows(artifact, state, emit)
}

// globalShown returns the ids of the conversations the global state
// database of the User directory above a workspace's state database gives
// rows for, none when it is absent, unreadable or damaged. Each database is
// read once per home. Its problems and repeated member names are the parse
// of that database's to report, so the lookup passes them to no artifact.
func (p *Parser) globalShown(artifact *input.Artifact) map[string]bool {
	file, ok := parserkit.Above(artifact, 2, "globalStorage", "state.vscdb")
	if !ok {
		return nil
	}
	known := p.shown.For(artifact.Home)
	key := parserkit.KeyOf(file)
	if shown, ok := known[key]; ok {
		return shown
	}
	p.Work.Add("global state reads", 1)
	state, _, repeats := vscodestate.ReadItem(artifact.Home.Listing, file, itemKey, parserkit.ReaderErrors(artifact))
	parserkit.NoteRepeats(nil, repeats)
	var problems parserkit.Problems
	_, shown := shownConversations(state, &problems)
	known[key] = shown
	return shown
}

// openConversation emits the rows of a workspace row's open conversation
// when it shows: a workspace row's twinny.active-conversation gives a
// conversation's rows when it is an object with messages and its id is none
// of the ids the global row of the same User directory gives rows for, the
// members of twinny.conversations, and the global row's own open
// conversation when that one is shown. Each row takes the folder of the
// workspace.json beside the database as its project path.
func (p *Parser) openConversation(artifact *input.Artifact, state jsonvalue.Value, options parserkit.Options,
	emit parserkit.Emit) {
	active := state.Get(activeKey)
	// Without messages the global database is not read.
	if !opensUnsaved(active, nil) || !opensUnsaved(active, p.globalShown(artifact)) {
		return
	}
	var problems parserkit.Problems
	project := ""
	if file, ok := parserkit.Beside(artifact, "workspace.json"); ok {
		project, _ = parserkit.ReadWorkspace(artifact, file, &problems)
	}
	p.conversation(artifact, active, project, options, emit)
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, parserkit.Text(active.Get("id")), project))
}

// workspaceRows emits the open conversation, then the repository the pull
// request review tab last listed.
func (p *Parser) workspaceRows(artifact *input.Artifact, state jsonvalue.Value, options parserkit.Options,
	emit parserkit.Emit) {
	p.openConversation(artifact, state, options, emit)
	owner := state.Get(workspacePrefix + "reviewOwner")
	repository := state.Get(workspacePrefix + "reviewRepo")
	if !owner.IsString() || owner.StringValue() == "" || !repository.IsString() || repository.StringValue() == "" {
		return
	}
	text := "source host repository: https://github.com/" + owner.StringValue() + "/" + repository.StringValue() +
		" type=review (from workspaceStorage state.vscdb rjmacarthy.twinny, file mtime)"
	emit(p.systemRow(artifact, parserkit.FileMtime(artifact), text))
}

// state emits the rows of the extension's row of a state database, then a
// row per problem.
func (p *Parser) state(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) {
	state, problems, repeats := vscodestate.ReadItem(artifact.Home.Listing, artifact.DiskPath, itemKey,
		parserkit.ReaderErrors(artifact))
	parserkit.NoteRepeats(artifact, repeats)
	switch {
	case state.IsObject() && workspaceDatabase.MatchString(artifact.Rel):
		p.workspaceRows(artifact, state, options, emit)
	case state.IsObject():
		p.globalRows(artifact, state, options, emit)
	case !state.IsNull() && len(problems) == 0:
		problems = []string{itemKey + " value is not a JSON object"}
	}
	for _, problem := range problems {
		emit(p.ProblemRow(artifact, problem))
	}
}

// Parse emits the rows of one Twinny file.
func (p *Parser) Parse(artifact *input.Artifact, options parserkit.Options, emit parserkit.Emit) error {
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

// The interfaces the registry looks for, checked when the package builds.
var (
	_ parserkit.Parser          = (*Parser)(nil)
	_ parserkit.AgentReader     = (*Parser)(nil)
	_ parserkit.SecondaryReader = (*Parser)(nil)
)
