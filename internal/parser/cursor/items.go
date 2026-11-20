package cursor

import (
	"cmp"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The ItemTable keys and key prefixes read, and the sources their rows
// name.
const (
	settingsKey = "src.vs.platform.reactivestorage.browser.reactiveStorageServiceImpl" +
		".persistentStorage.applicationUser"
	sandboxKey = "cursor/sandboxAutoRunSettingEnabled"
	// The editor's enrolment of this machine as a remote agent host for an
	// account; the secret:// computer key beside them is never read.
	enrolledPrefix = "remoteAgentHost.enrolled."
	displayPrefix  = "remoteAgentHost.displayName."
	// The settings blob is flushed late by the app, so its rows say so.
	composerSource  = "(from state.vscdb composerState, file mtime, written late by the app)"
	modelUsedSource = "(from state.vscdb aiSettings.modelLastUsedAt, written late by the app)"
)

// standingLists are the composerState lists of standing permissions.
func standingLists() []string {
	return []string{"yoloCommandAllowlist", "yoloCommandDenylist", "smartAllowlistDenylist",
		"webFetchDomainAllowlist", "autoApprovedModeTransitions"}
}

// switches are the composerState permission switches.
func switches() []string {
	return []string{"yoloEnableRunEverything", "yoloDeleteFileDisabled", "yoloMcpToolsDisabled",
		"yoloOutsideWorkspaceDisabled", "autoApproveModeTransitions", "autoAcceptWebSearchTool",
		"autoApplyFilesOutsideContext", "doNotShowYoloModeWarningAgain", "doNotShowFullYoloModeWarningAgain",
		"useLegacyTerminalTool", "playwrightProtection", "mcpAuthBlocking"}
}

// items reads the ItemTable rows of the primary store.
type items struct {
	owner    *Parser
	artifact *input.Artifact
	store    *store
	table    *sqlitefile.Table // nil without an ItemTable
	problems *parser.Problems
	mtime    string
}

// itemRows returns the reader of the store's ItemTable.
func (p *Parser) itemRows(artifact *input.Artifact, store *store, problems *parser.Problems) *items {
	return &items{owner: p, artifact: artifact, store: store, table: store.table("ItemTable"),
		problems: problems, mtime: parser.FileMtime(artifact)}
}

// system returns a system row of ItemTable at a formatted time.
func (i *items) system(text, timestamp string, rowid int64) timeline.Row {
	return i.owner.systemAt(i.artifact, text, timestamp, rowid)
}

// standing emits the standing permissions of the settings blob's
// composerState and the models' last use. A blob, composerState, list,
// aiSettings or modelLastUsedAt of the wrong kind is a problem.
func (i *items) standing(emit parser.Emit) {
	if i.table == nil {
		return
	}
	settings, ok := keyedValue(i.store, i.table, settingsKey)
	if !ok {
		return
	}
	document := objectRecord(settings, i.problems)
	sandbox := false
	if found, ok := keyedValue(i.store, i.table, sandboxKey); ok {
		var ignored parser.Problems
		sandbox = isTrue(decodeRecord(found.rowid, sandboxKey, found.value, &ignored))
	}
	line := int(settings.rowid)
	state, uses := settingsParts(document, line, i.problems)
	for _, text := range standingTexts(state, sandbox, line, i.problems) {
		emit(i.system(text, i.mtime, settings.rowid))
	}
	for _, used := range uses {
		emit(i.owner.row(i.artifact, timeline.TurnSystem, "model last used: "+used.Key+" "+modelUsedSource,
			timeutil.FromValue(used.Value), settings.rowid))
	}
}

// runMode returns the label the UI gives a modes4 entry's run mode, from
// its autoRun, smartModeAutoRun and fullAutoRun flags and the sandbox
// switch; no run-mode string is stored.
func runMode(mode jsonvalue.Value, sandbox bool) string {
	if isTrue(mode.Get("fullAutoRun")) {
		return "Run Everything"
	}
	if !isTrue(mode.Get("autoRun")) {
		return "Ask Every Time"
	}
	label := "Allowlist"
	if isTrue(mode.Get("smartModeAutoRun")) {
		label = "Auto-review"
	}
	if sandbox {
		label += " (with Sandbox)"
	}
	return label
}

// settingsParts returns the settings blob's composerState, an empty object
// when it is not one, and the members of aiSettings.modelLastUsedAt. A
// composerState, aiSettings or modelLastUsedAt of the wrong kind adds a
// problem at line.
func settingsParts(document jsonvalue.Value, line int, problems *parser.Problems) (jsonvalue.Value, []jsonvalue.Member) {
	state := document.Get("composerState")
	parser.ObjectMember(state, "composerState", line, problems)
	aiSettings := document.Get("aiSettings")
	parser.ObjectMember(aiSettings, "aiSettings", line, problems)
	uses := parser.ObjectMember(aiSettings.Get("modelLastUsedAt"), "aiSettings.modelLastUsedAt", line, problems)
	return parser.ObjectOrEmpty(state), uses
}

// standingTexts returns the "permission setting:" texts of composerState:
// one per standing-list entry, one per modes4 entry and one per switch. A
// standing list or modes4 that is not a JSON array adds a problem at line.
func standingTexts(state jsonvalue.Value, sandbox bool, line int, problems *parser.Problems) []string {
	var texts []string
	for _, key := range standingLists() {
		for _, entry := range parser.ArrayMember(state.Get(key), key, line, problems) {
			texts = append(texts, "permission setting: "+key+"="+parser.SettingValue(entry)+" "+composerSource)
		}
	}
	sandboxText := parser.SettingValue(jsonvalue.NewBool(sandbox))
	for _, mode := range parser.EntriesOfKind(state.Get("modes4"), "modes4", jsonvalue.Object, line, problems) {
		var flags []string
		for _, flag := range []string{"autoRun", "smartModeAutoRun", "fullAutoRun"} {
			value := "-"
			if found, ok := mode.Lookup(flag); ok {
				value = parser.SettingValue(found)
			}
			flags = append(flags, flag+"="+value)
		}
		texts = append(texts, "permission setting: run mode "+fieldOf(mode.Get("id"))+"="+runMode(mode, sandbox)+
			" "+strings.Join(flags, " ")+" sandbox="+sandboxText+" "+composerSource)
	}
	for _, key := range switches() {
		if value, ok := state.Lookup(key); ok {
			texts = append(texts, "permission setting: "+key+"="+parser.SettingValue(value)+" "+composerSource)
		}
	}
	return texts
}

// integrations emits the "source host:" rows of the GitHub accounts and
// the "remote agent host enrolled:" rows. Only the github- and
// remoteAgentHost. keys are selected, so no secret:// value is read.
func (i *items) integrations(emit parser.Emit) {
	if i.table == nil {
		return
	}
	rows := i.store.rows(keyedRows(i.table, func(row sqlitefile.Row) bool {
		return hasTextPrefix(row.Get("key"), "github-") || hasTextPrefix(row.Get("key"), "remoteAgentHost.")
	}))
	byKey := map[string]keyed{}
	for _, row := range rows {
		byKey[row.key] = row
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	for _, key := range keys {
		row := byKey[key]
		switch {
		case strings.HasPrefix(key, "github-") && !strings.HasSuffix(key, "-usages"):
			i.githubAccount(row, byKey, emit)
		case strings.HasPrefix(key, enrolledPrefix) && isTrue(plain(row.value)):
			emit(i.system(enrolledText(key, byKey), i.mtime, row.rowid))
		}
	}
}

// enrolledText returns the "remote agent host enrolled:" text of an
// enrolment key, with the account's display name.
func enrolledText(key string, byKey map[string]keyed) string {
	raw := byKey[displayPrefix+key[len(enrolledPrefix):]].value
	name := plain(raw)
	text := asText(raw)
	if name.IsString() {
		text = name.StringValue()
	}
	return "remote agent host enrolled: " + cmp.Or(timeline.Compact(text), "-") + " (from state.vscdb " + key +
		", file mtime; on by default, a paired device leaves no local record)"
}

// githubAccount emits the "source host: github account" row of a
// github-<login> key, at its latest use, else the file's modification
// time. Another extension's github- key that is not an account list is
// skipped; a cut account list is a bad record. A usages value that is not
// a list, and its entries that are not objects, are problems.
func (i *items) githubAccount(row keyed, byKey map[string]keyed, emit parser.Emit) {
	login := row.key[len("github-"):]
	if login == "" || !plain(row.value).IsArray() {
		if strings.HasPrefix(asText(row.value), "[") {
			decodeRecord(row.rowid, row.key, row.value, i.problems)
		}
		return
	}
	var usages []jsonvalue.Value
	if found, ok := byKey[row.key+"-usages"]; ok {
		for _, usage := range objectEntries(found, i.problems) {
			usages = append(usages, usage)
		}
	}
	text := githubText(login, row.key, usages, i.owner.Work)
	if latest := lastUsed(usages); !latest.IsEmpty() {
		emit(i.owner.row(i.artifact, timeline.TurnSystem, text, timeutil.FromValue(latest), row.rowid))
		return
	}
	emit(i.system(text, i.mtime, row.rowid))
}

// lastUsed returns the latest numeric lastUsed of a usages list (a
// boolean counts as 0 or 1), the first of equal ones, or null.
func lastUsed(usages []jsonvalue.Value) jsonvalue.Value {
	latest, best := jsonvalue.NewNull(), 0.0
	for _, usage := range usages {
		value := usage.Get("lastUsed")
		number, ok := value.Float()
		if flag, isBool := value.Bool(); isBool {
			number, ok = 0, true
			if flag {
				number = 1
			}
		}
		if ok && (latest.IsNull() || number > best) {
			latest, best = value, number
		}
	}
	return latest
}

// githubText returns the "source host: github account" text, the scopes
// of its usages without repeats, in the order first met; work counts the
// scopes looked up in the set of those already met.
func githubText(login, key string, usages []jsonvalue.Value, work parser.Work) string {
	var scopes []string
	seen := map[string]bool{}
	for _, usage := range usages {
		for _, scope := range usage.Get("scopes").Items() {
			work.Add("scope lookups", 1)
			if text := scope.Text(); !seen[text] {
				seen[text] = true
				scopes = append(scopes, text)
			}
		}
	}
	return "source host: github account " + login + " scopes=" + cmp.Or(strings.Join(scopes, ","), "-") +
		" (from state.vscdb key " + key + ")"
}
