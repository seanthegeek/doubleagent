package cursor

import (
	"cmp"
	"slices"
	"strconv"
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

// standing emits the standing permissions of each settings blob's
// composerState and the models' last use. A blob, composerState, list,
// aiSettings or modelLastUsedAt of the wrong kind is a problem. A key
// ItemTable holds in more than one row, which only damage or an edit
// writes, the key being UNIQUE, gives a parser: row per stored row: each
// settings blob gives its own rows, and the run-mode rows, which take the
// sandbox switch, are shown once, with the stored switch the group goes
// with (sandboxes).
func (i *items) standing(emit parser.Emit) {
	if i.table == nil {
		return
	}
	settings := keyedValues(i.store, i.table, settingsKey)
	if len(settings) == 0 {
		return
	}
	sandboxes := i.sandboxes(emit)
	for _, stored := range settings {
		if len(settings) > 1 {
			emit(i.system(repeatedKeyText("ItemTable", len(settings)), i.mtime, stored.rowid))
		}
		i.settingsRows(stored, sandboxes, emit)
	}
}

// sandbox is one stored value of the sandbox switch that run-mode rows are
// made with, with its parent in the group of a repeated switch key (nil
// when the key is stored once).
type sandbox struct {
	enabled bool
	parent  *timeline.Parent
}

// sandboxes returns the stored values of the sandbox switch the run-mode
// rows are made with, false when there is none. A key stored more than
// once is a group (timeline.Group) whose parents are its stored rows, each
// giving a parser: row; only the first stored row of each value holds the
// run-mode rows, since a later one's would read the same, so they grow
// with the settings rows and not with the switch's.
func (i *items) sandboxes(emit parser.Emit) []sandbox {
	stored := keyedValues(i.store, i.table, sandboxKey)
	values := []sandbox{{}}
	if len(stored) > 0 {
		values = make([]sandbox, len(stored))
	}
	group := timeline.NewGroup()
	for n, found := range stored {
		var ignored parser.Problems
		values[n].enabled = isTrue(decodeRecord(found.rowid, sandboxKey, found.value, &ignored))
		if len(stored) == 1 {
			continue
		}
		notice := i.system("", i.mtime, found.rowid)
		values[n].parent = group.Parent(&notice)
		prefix := parser.ProblemPrefix + "ItemTable holds " + strconv.Itoa(len(stored)) +
			" rows of this row's key; its run mode rows are shown once, with "
		values[n].parent.Texts(&notice, timeline.RowText{Text: prefix + "this row"},
			timeline.RowText{Text: prefix + "another of them"})
		emit(notice)
	}
	var held []sandbox
	seen := map[bool]bool{}
	for _, value := range values {
		if !seen[value.enabled] {
			seen[value.enabled] = true
			held = append(held, value)
		}
	}
	return held
}

// settingsRows emits the rows of one settings blob: its standing lists,
// its run modes with each stored sandbox switch (a row of the switch's
// parent shown only when the group goes with it), its switches and the
// models' last use.
func (i *items) settingsRows(settings keyed, sandboxes []sandbox, emit parser.Emit) {
	document := objectRecord(settings, i.problems)
	line := int(settings.rowid)
	state, uses := settingsParts(document, line, i.problems)
	lists, modes, switched := standingTexts(state, line, i.problems)
	for _, text := range lists {
		emit(i.system(text, i.mtime, settings.rowid))
	}
	for _, value := range sandboxes {
		for _, mode := range modes {
			row := i.system(runModeText(mode, value.enabled), i.mtime, settings.rowid)
			if value.parent != nil {
				value.parent.Row(&row, timeline.WhenChosen)
			}
			emit(row)
		}
	}
	for _, text := range switched {
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

// standingTexts returns the "permission setting:" texts of composerState's
// standing lists, one per entry, its modes4 entries and the texts of its
// switches. A standing list or modes4 that is not a JSON array adds a
// problem at line.
func standingTexts(state jsonvalue.Value, line int, problems *parser.Problems) ([]string, []jsonvalue.Value,
	[]string) {
	var lists, switched []string
	for _, key := range standingLists() {
		for _, entry := range parser.ArrayMember(state.Get(key), key, line, problems) {
			lists = append(lists, "permission setting: "+key+"="+parser.SettingValue(entry)+" "+composerSource)
		}
	}
	var modes []jsonvalue.Value
	for _, mode := range parser.EntriesOfKind(state.Get("modes4"), "modes4", jsonvalue.Object, line, problems) {
		modes = append(modes, mode)
	}
	for _, key := range switches() {
		if value, ok := state.Lookup(key); ok {
			switched = append(switched, "permission setting: "+key+"="+parser.SettingValue(value)+" "+composerSource)
		}
	}
	return lists, modes, switched
}

// runModeText returns the "permission setting: run mode" text of a modes4
// entry with the sandbox switch given.
func runModeText(mode jsonvalue.Value, sandbox bool) string {
	var flags []string
	for _, flag := range []string{"autoRun", "smartModeAutoRun", "fullAutoRun"} {
		value := "-"
		if found, ok := mode.Lookup(flag); ok {
			value = parser.SettingValue(found)
		}
		flags = append(flags, flag+"="+value)
	}
	return "permission setting: run mode " + fieldOf(mode.Get("id")) + "=" + runMode(mode, sandbox) + " " +
		strings.Join(flags, " ") + " sandbox=" + parser.SettingValue(jsonvalue.NewBool(sandbox)) + " " + composerSource
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
