package vscode

import (
	"cmp"
	"regexp"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
	"github.com/seanthegeek/doubleagent/internal/vscodestate"
)

// The ItemTable keys read from a state database.
const (
	copilotStateKey  = "GitHub.copilot-chat"
	ownedTasksKey    = "github.copilot.cloudAgent.ownedTasks"
	selectedReposKey = "userSelectedRepositories"
	optInKey         = "chat.tools.global.autoApprove.optIn"
	allowlistKey     = "chat.externalPath.workspaceAllowlist"
	defaultModelKey  = "chat.defaultModel"
)

// grantKeys are the ToolConfirmStore keys (languageModelToolsConfirmationService.ts:240-244
// at 4d22f4f), each in workspace and profile scope.
func grantKeys() []string {
	return []string{"chat/autoconfirm", "chat/autoconfirm-post", "chat/servers/autoconfirm",
		"chat/servers/autoconfirm-post", "chat/autoconfirm-combination"}
}

// usageKey matches <providerId>-<account label>-usages and
// -mcpserver-usages, application scope (authenticationUsageService.ts:103,
// authenticationMcpUsageService.ts:101), and settingsPermission the
// settings.json keys of standing permissions.
var (
	usageKey           = regexp.MustCompile(`^(github(?:-enterprise)?)-(.+?)(-mcpserver)?-usages\z`)
	settingsPermission = regexp.MustCompile(`^chat\.tools\..*[Aa]utoApprov|^chat\.permissions\.default\z`)
)

// systemRow returns a system row of the artifact with a session id and
// time, its text compacted.
func (p *Parser) systemRow(artifact *input.Artifact, sessionID, timestamp, text string) timeline.Row {
	row := p.BaseRow(artifact)
	row.SessionID, row.TimestampUTC, row.TurnType = sessionID, timestamp, timeline.TurnSystem
	row.Text = timeline.Compact(text)
	return row
}

// stateDatabase emits the rows of a profile's or a workspace's
// state.vscdb: the cloud agent task, selected repository and source-host
// rows (profile scope only), a "permission setting:" row per standing grant
// at the file's modification time, then a row per problem.
func (p *Parser) stateDatabase(artifact *input.Artifact, workspace bool, emit parserkit.Emit) {
	keys := append(grantKeys(), optInKey, allowlistKey)
	if !workspace {
		keys = append(keys, copilotStateKey)
	}
	values, problems, repeats := vscodestate.ReadItems(artifact.Home.Listing, artifact.DiskPath, keys)
	parserkit.NoteRepeats(artifact, repeats)
	var errors parserkit.Problems
	project := ""
	if workspace {
		project, _ = p.workspaceJSON(artifact, 0, &errors)
	}
	p.copilotStateRows(artifact, values[copilotStateKey], &problems, emit)
	if !workspace {
		usages, more := usageItems(artifact, artifact.DiskPath)
		problems = append(problems, more...)
		p.sourceHostRows(artifact, usages, &problems, emit)
	}
	timestamp, source := parserkit.FileMtime(artifact), userRel(artifact.Rel)
	for _, text := range grantTexts(values, &problems) {
		row := p.systemRow(artifact, "", timestamp, text+" (from "+source+", file mtime)")
		row.ProjectPath = project
		emit(row)
	}
	for _, problem := range problems {
		row := p.systemRow(artifact, "", "", parserkit.ProblemPrefix+problem)
		row.ProjectPath = project
		emit(row)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, errors, "", project))
}

// copilotStateRows emits the cloud agent task and selected repository rows
// of the GitHub.copilot-chat value. A value, ownedTasks or
// userSelectedRepositories of the wrong kind adds a problem; a value that is
// not JSON is held as a string, which ReadItems has already reported.
func (p *Parser) copilotStateRows(artifact *input.Artifact, state jsonvalue.Value, problems *[]string,
	emit parserkit.Emit) {
	if !state.IsObject() {
		notJSON := vscodestate.ValueProblem + copilotStateKey + " is not valid JSON"
		reported := slices.ContainsFunc(*problems, func(problem string) bool {
			return strings.HasPrefix(problem, notJSON)
		})
		if !state.IsNull() && !reported {
			*problems = append(*problems, vscodestate.ValueProblem+parserkit.WrongKind(copilotStateKey, jsonvalue.Object))
		}
		return
	}
	p.ownedTaskRows(artifact, state.Get(ownedTasksKey), problems, emit)
	p.selectedRepoRows(artifact, state.Get(selectedReposKey), problems, emit)
}

// grantTexts returns the "permission setting:" texts, without the source,
// of one state database's grants: one per entry of each ToolConfirmStore
// key (an object keyed by tool or server id, or the older array of ids),
// the global auto-approve opt-in, and one per external path allowed in the
// workspace. A value of another JSON type adds a problem.
func grantTexts(values map[string]jsonvalue.Value, problems *[]string) []string {
	var texts []string
	for _, key := range grantKeys() {
		value := values[key]
		switch {
		case value.IsNull() || value.IsString(): // absent, or not JSON (ReadItems reported it)
		case value.IsObject():
			for _, member := range value.Members() {
				texts = append(texts, "permission setting: "+key+"."+member.Key+"="+parserkit.SettingValue(member.Value))
			}
		case value.IsArray():
			for _, toolID := range value.Items() {
				texts = append(texts, "permission setting: "+key+"."+parserkit.SettingValue(toolID)+"=true")
			}
		default:
			*problems = append(*problems, vscodestate.ValueProblem+key+" is not a JSON object or array")
		}
	}
	if value, ok := values[optInKey]; ok {
		texts = append(texts, "permission setting: "+optInKey+"="+parserkit.SettingValue(value))
	}
	switch allow := values[allowlistKey]; {
	case allow.IsArray():
		for _, entry := range allow.Items() {
			text := parserkit.SettingValue(entry)
			if entry.IsString() {
				text = uriToPath(entry.StringValue())
			}
			texts = append(texts, "permission setting: "+allowlistKey+"="+text)
		}
	case !allow.IsNull() && !allow.IsString():
		*problems = append(*problems, vscodestate.ValueProblem+allowlistKey+" is not a JSON array")
	}
	return texts
}

// usage is one GitHub authentication usage row: its key and decoded value.
type usage struct {
	key   string
	value jsonvalue.Value
}

// usageItems returns the GitHub authentication usage rows of a state
// database in row order, and the problems of values that are not JSON. A
// database ReadItems could not open gives nothing here, since it reported
// it; secret:// rows never match.
func usageItems(artifact *input.Artifact, databasePath string) ([]usage, []string) {
	if !parserkit.RegularFile(databasePath) {
		return nil, nil
	}
	database, err := parserkit.OpenDatabase(artifact, databasePath)
	if err != nil {
		return nil, nil
	}
	defer database.Close()
	if tables, err := parserkit.TablesOf(database); err != nil || !tables.Has("ItemTable") {
		return nil, nil
	}
	table, err := database.Table("ItemTable")
	if err != nil {
		return nil, nil
	}
	rows, err := table.Select("key", "value").Where(func(row sqlitefile.Row) bool {
		return sqlitefile.Like(row.Get("key"), "github%-usages")
	}).All()
	if err != nil {
		return nil, nil
	}
	var found []usage
	var problems []string
	for _, row := range rows {
		key := row.Get("key")
		if key.Type != sqlitefile.Text || !usageKey.MatchString(key.Text()) {
			continue
		}
		decoded, err := parserkit.DecodeJSON(artifact, usageText(row.Get("value")))
		if err != nil {
			problems = append(problems, vscodestate.ValueProblem+key.Text()+" is not valid JSON ("+err.Error()+")")
			continue
		}
		found = append(found, usage{key: key.Text(), value: decoded})
	}
	return found, problems
}

// usageText returns a usage value as the text its JSON is read from: a
// blob decoded as UTF-8 with replacement, anything else as its row text
// (sqlitefile.Value.String).
func usageText(value sqlitefile.Value) string {
	if value.Type == sqlitefile.Blob {
		return textlines.ValidUTF8(value.Bytes)
	}
	return value.String()
}

// sourceHostRows emits a "source host:" row per extension or MCP server
// that used a GitHub account, at the usage's lastUsed. The account label
// is the GitHub login; a GitHub Enterprise label ends " - <host><path>",
// shown in the host and in the key the row names. A
// usage value that is not an array, or that holds entries that are not
// objects, adds a problem.
func (p *Parser) sourceHostRows(artifact *input.Artifact, usages []usage, problems *[]string, emit parserkit.Emit) {
	carried := parserkit.NewCarried(artifact)
	for _, item := range usages {
		match := usageKey.FindStringSubmatch(item.key)
		if match == nil {
			continue
		}
		provider, label, mcp := match[1], match[2], match[3] != ""
		host, login := "github.com", label
		if provider == "github-enterprise" {
			if cut := strings.LastIndex(label, " - "); cut > 0 {
				login, host = label[:cut], label[cut+len(" - "):]
			} else {
				host = provider
			}
		}
		kind, idKey := "extension", "extensionId"
		if mcp {
			kind, idKey = "mcp_server", "mcpServerId"
		}
		var found parserkit.Problems
		entries := parserkit.EntriesOfKind(item.value, vscodestate.ValueProblem+item.key, jsonvalue.Object, 0, &found)
		// Each entry's row repeats the usage's key and account.
		key, host, login := carried.Cut("usage key", item.key), carried.Cut("usage account", host),
			carried.Cut("usage account", login)
		for _, entry := range entries {
			user := entry.Get(idKey)
			if user.IsEmpty() {
				user = jsonvalue.NewString("-")
			}
			text := "source host: " + host + " account " + login + " " + kind + "=" + parserkit.SettingValue(user) +
				" scopes=" + scopesText(entry.Get("scopes")) + " (from state.vscdb " + key +
				", lastUsed)"
			emit(p.systemRow(artifact, "", timeutil.FromValue(entry.Get("lastUsed")), text))
		}
		*problems = append(*problems, messagesOf(found)...)
	}
	var cut parserkit.Problems
	carried.Report(0, &cut)
	*problems = append(*problems, messagesOf(cut)...)
}

// messagesOf returns the messages of problems, for a list of problem
// texts that carry no line.
func messagesOf(problems parserkit.Problems) []string {
	texts := make([]string, 0, len(problems))
	for _, problem := range problems {
		texts = append(texts, problem.Message)
	}
	return texts
}

// scopesText returns a usage's scopes joined by commas, "-" when they are
// not an array.
func scopesText(scopes jsonvalue.Value) string {
	if !scopes.IsArray() {
		return "-"
	}
	texts := make([]string, 0, scopes.Len())
	for _, scope := range scopes.Items() {
		texts = append(texts, scope.Text())
	}
	return strings.Join(texts, ",")
}

// ownedTaskRows emits a "session synced:" and a "session origin: remote"
// row per owned cloud agent task; tasks that is not an object adds a
// problem.
func (p *Parser) ownedTaskRows(artifact *input.Artifact, tasks jsonvalue.Value, problems *[]string,
	emit parserkit.Emit) {
	source := "(from state.vscdb " + copilotStateKey + " " + ownedTasksKey + ", recordedAt)"
	if !tasks.IsObject() && !tasks.IsNull() {
		*problems = append(*problems, parserkit.WrongKind(ownedTasksKey, jsonvalue.Object))
	}
	for _, task := range tasks.Members() {
		recorded, application := task.Value, jsonvalue.NewNull()
		if task.Value.IsObject() {
			recorded, application = task.Value.Get("recordedAt"), task.Value.Get("application")
		}
		sessionID, timestamp := "copilot-cloud-agent:/task/"+task.Key, timeutil.FromValue(recorded)
		emit(p.systemRow(artifact, sessionID, timestamp, "session synced: cloud agent task "+task.Key+
			" application="+cmp.Or(parserkit.Text(application), "-")+" "+source))
		emit(p.systemRow(artifact, sessionID, timestamp, "session origin: remote GitHub cloud agent "+source))
	}
}

// selectedRepoRows emits a "source host repository:" row per repository
// picked for a cloud agent task; repositories that is not an array, or
// that holds entries that are not objects, adds a problem.
func (p *Parser) selectedRepoRows(artifact *input.Artifact, repositories jsonvalue.Value, problems *[]string,
	emit parserkit.Emit) {
	source := "(from state.vscdb " + copilotStateKey + " " + selectedReposKey + ", timestamp)"
	var found parserkit.Problems
	for _, entry := range parserkit.EntriesOfKind(repositories, selectedReposKey, jsonvalue.Object, 0, &found) {
		if name := entry.Get("name"); !name.IsEmpty() {
			emit(p.systemRow(artifact, "", timeutil.FromValue(entry.Get("timestamp")),
				"source host repository: "+parserkit.SettingValue(name)+" type=cloud agent selection "+source))
		}
	}
	*problems = append(*problems, messagesOf(found)...)
}
