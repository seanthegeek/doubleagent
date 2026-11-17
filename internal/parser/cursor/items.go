package cursor

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
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
// composerState and the models' last use.
func (i *items) standing(emit parser.Emit) {
	if i.table == nil {
		return
	}
	settings, ok := keyedValue(i.store, i.table, settingsKey)
	if !ok {
		return
	}
	document := objectOf(decodeRecord(settings.rowid, settingsKey, settings.value, i.problems))
	sandbox := false
	if found, ok := keyedValue(i.store, i.table, sandboxKey); ok {
		var ignored parser.Problems
		sandbox = isTrue(decodeRecord(found.rowid, sandboxKey, found.value, &ignored))
	}
	for _, text := range standingTexts(objectOf(document.Get("composerState")), sandbox) {
		emit(i.system(text, i.mtime, settings.rowid))
	}
	for _, used := range document.Get("aiSettings").Get("modelLastUsedAt").Members() {
		emit(i.owner.row(i.artifact, timeline.TurnSystem, "model last used: "+used.Key+" "+modelUsedSource,
			timeutil.FromValue(used.Value), settings.rowid))
	}
}

// runMode returns the label the UI gives a modes4 entry's run mode, from
// its autoRun, smartModeAutoRun and fullAutoRun flags and the sandbox
// switch; no run-mode string is stored.
func runMode(mode pyjson.Value, sandbox bool) string {
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

// standingTexts returns the "permission setting:" texts of composerState:
// one per standing-list entry, one per modes4 entry and one per switch.
func standingTexts(state pyjson.Value, sandbox bool) []string {
	var texts []string
	for _, key := range standingLists() {
		for _, entry := range state.Get(key).Items() {
			texts = append(texts, "permission setting: "+key+"="+parser.SettingValue(entry)+" "+composerSource)
		}
	}
	sandboxText := parser.SettingValue(pyjson.NewBool(sandbox))
	for _, mode := range state.Get("modes4").Items() {
		if !mode.IsObject() {
			continue
		}
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
		text = name.Text()
	}
	return "remote agent host enrolled: " + orDash(timeline.Compact(text)) + " (from state.vscdb " + key +
		", file mtime; on by default, a paired device leaves no local record)"
}

// githubAccount emits the "source host: github account" row of a
// github-<login> key, at its latest use, else the file's modification
// time. Another extension's github- key that is not an account list is
// skipped; a cut account list is a bad record.
func (i *items) githubAccount(row keyed, byKey map[string]keyed, emit parser.Emit) {
	login := row.key[len("github-"):]
	if login == "" || !plain(row.value).IsArray() {
		if strings.HasPrefix(asText(row.value), "[") {
			decodeRecord(row.rowid, row.key, row.value, i.problems)
		}
		return
	}
	var usages []pyjson.Value
	if found, ok := byKey[row.key+"-usages"]; ok {
		for _, usage := range decodeRecord(found.rowid, found.key, found.value, i.problems).Items() {
			if usage.IsObject() {
				usages = append(usages, usage)
			}
		}
	}
	text := githubText(login, row.key, usages)
	if latest := lastUsed(usages); latest.Truthy() {
		emit(i.owner.row(i.artifact, timeline.TurnSystem, text, timeutil.FromValue(latest), row.rowid))
		return
	}
	emit(i.system(text, i.mtime, row.rowid))
}

// lastUsed returns the latest numeric lastUsed of a usages list (a
// boolean counts as 0 or 1), the first of equal ones, or null.
func lastUsed(usages []pyjson.Value) pyjson.Value {
	latest, best := pyjson.NewNull(), 0.0
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
// of its usages without repeats.
func githubText(login, key string, usages []pyjson.Value) string {
	var scopes []string
	for _, usage := range usages {
		for _, scope := range usage.Get("scopes").Items() {
			if !slices.Contains(scopes, scope.Str()) {
				scopes = append(scopes, scope.Str())
			}
		}
	}
	return "source host: github account " + login + " scopes=" + orDash(strings.Join(scopes, ",")) +
		" (from state.vscdb key " + key + ")"
}
