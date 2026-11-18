package vscode

import (
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/antigravity"
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
func (p *Parser) stateDatabase(artifact *input.Artifact, workspace bool, emit parser.Emit) {
	keys := append(grantKeys(), optInKey, allowlistKey)
	if !workspace {
		keys = append(keys, copilotStateKey)
	}
	values, problems := vscodestate.ReadItems(artifact.DiskPath, keys)
	var errors parser.Problems
	project := ""
	if workspace {
		project, _ = workspaceJSON(artifact, 0, &errors)
	}
	if state := values[copilotStateKey]; state.IsObject() {
		p.ownedTaskRows(artifact, state.Get(ownedTasksKey), emit)
		p.selectedRepoRows(artifact, state.Get(selectedReposKey), emit)
	}
	if !workspace {
		usages, more := usageItems(artifact.DiskPath)
		problems = append(problems, more...)
		p.sourceHostRows(artifact, usages, emit)
	}
	timestamp, source := parser.FileMtime(artifact), userRel(artifact.Rel)
	for _, text := range grantTexts(values, &problems) {
		row := p.systemRow(artifact, "", timestamp, text+" (from "+source+", file mtime)")
		row.ProjectPath = project
		emit(row)
	}
	for _, problem := range problems {
		row := p.systemRow(artifact, "", "", parser.ProblemPrefix+problem)
		row.ProjectPath = project
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, errors, "", project))
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
				texts = append(texts, "permission setting: "+key+"."+member.Key+"="+parser.SettingValue(member.Value))
			}
		case value.IsArray():
			for _, toolID := range value.Items() {
				texts = append(texts, "permission setting: "+key+"."+parser.SettingValue(toolID)+"=true")
			}
		default:
			*problems = append(*problems, vscodestate.ValueProblem+key+" is not a JSON object or array")
		}
	}
	if value, ok := values[optInKey]; ok {
		texts = append(texts, "permission setting: "+optInKey+"="+parser.SettingValue(value))
	}
	switch allow := values[allowlistKey]; {
	case allow.IsArray():
		for _, entry := range allow.Items() {
			text := parser.SettingValue(entry)
			if entry.IsString() {
				text = antigravity.URIToPath(entry.StringValue())
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
func usageItems(databasePath string) ([]usage, []string) {
	if !parser.RegularFile(databasePath) || sqlitefile.Problem(databasePath) != nil {
		return nil, nil
	}
	database, err := sqlitefile.OpenPath(databasePath)
	if err != nil {
		return nil, nil
	}
	defer database.Close()
	if tables, err := database.TableNames(); err != nil || !tables["ItemTable"] {
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
		decoded, err := jsonvalue.DecodeString(usageText(row.Get("value")))
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
// is the GitHub login; a GitHub Enterprise label ends " - <host><path>".
func (p *Parser) sourceHostRows(artifact *input.Artifact, usages []usage, emit parser.Emit) {
	for _, item := range usages {
		match := usageKey.FindStringSubmatch(item.key)
		if match == nil || !item.value.IsArray() {
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
		for _, entry := range item.value.Items() {
			if !entry.IsObject() {
				continue
			}
			user := entry.Get(idKey)
			if user.IsEmpty() {
				user = jsonvalue.NewString("-")
			}
			text := "source host: " + host + " account " + login + " " + kind + "=" + parser.SettingValue(user) +
				" scopes=" + scopesText(entry.Get("scopes")) + " (from state.vscdb " + item.key + ", lastUsed)"
			emit(p.systemRow(artifact, "", timeutil.FromValue(entry.Get("lastUsed")), text))
		}
	}
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
// row per owned cloud agent task.
func (p *Parser) ownedTaskRows(artifact *input.Artifact, tasks jsonvalue.Value, emit parser.Emit) {
	source := "(from state.vscdb " + copilotStateKey + " " + ownedTasksKey + ", recordedAt)"
	for _, task := range tasks.Members() {
		recorded, application := task.Value, jsonvalue.NewNull()
		if task.Value.IsObject() {
			recorded, application = task.Value.Get("recordedAt"), task.Value.Get("application")
		}
		sessionID, timestamp := "copilot-cloud-agent:/task/"+task.Key, timeutil.FromValue(recorded)
		emit(p.systemRow(artifact, sessionID, timestamp, "session synced: cloud agent task "+task.Key+
			" application="+orDash(parser.Text(application))+" "+source))
		emit(p.systemRow(artifact, sessionID, timestamp, "session origin: remote GitHub cloud agent "+source))
	}
}

// selectedRepoRows emits a "source host repository:" row per repository
// picked for a cloud agent task.
func (p *Parser) selectedRepoRows(artifact *input.Artifact, repositories jsonvalue.Value, emit parser.Emit) {
	source := "(from state.vscdb " + copilotStateKey + " " + selectedReposKey + ", timestamp)"
	for _, entry := range repositories.Items() {
		if name := entry.Get("name"); entry.IsObject() && !name.IsEmpty() {
			emit(p.systemRow(artifact, "", timeutil.FromValue(entry.Get("timestamp")),
				"source host repository: "+parser.SettingValue(name)+" type=cloud agent selection "+source))
		}
	}
}
