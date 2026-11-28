package cursor

import (
	"cmp"
	"slices"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
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
	problems *parserkit.Problems
	mtime    string
}

// itemRows returns the reader of the store's ItemTable.
func (p *Parser) itemRows(artifact *input.Artifact, store *store, problems *parserkit.Problems) *items {
	return &items{owner: p, artifact: artifact, store: store, table: store.table("ItemTable"),
		problems: problems, mtime: parserkit.FileMtime(artifact)}
}

// system returns a system row of ItemTable at a formatted time.
func (i *items) system(text, timestamp string, rowid int64) timeline.Row {
	return i.owner.systemAt(i.artifact, text, timestamp, rowid)
}

// sandboxes returns the stored values of the sandbox switch the run-mode
// rows are made with, false when there is none. A key stored more than
// once is a group (timeline.Group) whose parents are its stored rows, each
// giving a parser: row; only the first stored row of each value holds the
// run-mode rows, since a later one's would read the same, so they grow
// with the settings rows and not with the switch's.
func (i *items) sandboxes(emit parserkit.Emit) []sandbox {
	stored := keyedValues(i.store, i.table, sandboxKey)
	values := []sandbox{{}}
	if len(stored) > 0 {
		values = make([]sandbox, len(stored))
	}
	group := timeline.NewGroup()
	for n, found := range stored {
		var ignored parserkit.Problems
		values[n].enabled = isTrue(decodeRecord(i.artifact, found.rowid, sandboxKey, found.value, &ignored))
		if len(stored) == 1 {
			continue
		}
		notice := i.system("", i.mtime, found.rowid)
		values[n].parent = group.Parent(&notice)
		prefix := parserkit.ProblemPrefix + "ItemTable holds " + strconv.Itoa(len(stored)) +
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

// sandbox is one stored value of the sandbox switch that run-mode rows are
// made with, with its parent in the group of a repeated switch key (nil
// when the key is stored once).
type sandbox struct {
	enabled bool
	parent  *timeline.Parent
}

// settingsParts returns the settings blob's composerState, an empty object
// when it is not one, and the members of aiSettings.modelLastUsedAt. A
// composerState, aiSettings or modelLastUsedAt of the wrong kind adds a
// problem at line.
func settingsParts(document jsonvalue.Value, line int, problems *parserkit.Problems) (jsonvalue.Value, []jsonvalue.Member) {
	state := document.Get("composerState")
	parserkit.ObjectMember(state, "composerState", line, problems)
	aiSettings := document.Get("aiSettings")
	parserkit.ObjectMember(aiSettings, "aiSettings", line, problems)
	uses := parserkit.ObjectMember(aiSettings.Get("modelLastUsedAt"), "aiSettings.modelLastUsedAt", line, problems)
	return parserkit.ObjectOrEmpty(state), uses
}

// standingTexts returns the "permission setting:" texts of composerState's
// standing lists, one per entry, its modes4 entries and the texts of its
// switches. A standing list or modes4 that is not a JSON array adds a
// problem at line.
func standingTexts(state jsonvalue.Value, line int, problems *parserkit.Problems) ([]string, []jsonvalue.Value,
	[]string) {
	var lists, switched []string
	for _, key := range standingLists() {
		for _, entry := range parserkit.ArrayMember(state.Get(key), key, line, problems) {
			lists = append(lists, "permission setting: "+key+"="+parserkit.SettingValue(entry)+" "+composerSource)
		}
	}
	var modes []jsonvalue.Value
	for _, mode := range parserkit.EntriesOfKind(state.Get("modes4"), "modes4", jsonvalue.Object, line, problems) {
		modes = append(modes, mode)
	}
	for _, key := range switches() {
		if value, ok := state.Lookup(key); ok {
			switched = append(switched, "permission setting: "+key+"="+parserkit.SettingValue(value)+" "+composerSource)
		}
	}
	return lists, modes, switched
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

// runModeText returns the "permission setting: run mode" text of a modes4
// entry with the sandbox switch given.
func runModeText(mode jsonvalue.Value, sandbox bool) string {
	var flags []string
	for _, flag := range []string{"autoRun", "smartModeAutoRun", "fullAutoRun"} {
		value := "-"
		if found, ok := mode.Lookup(flag); ok {
			value = parserkit.SettingValue(found)
		}
		flags = append(flags, flag+"="+value)
	}
	return "permission setting: run mode " + parserkit.SettingOrDash(mode.Get("id")) + "=" + runMode(mode, sandbox) + " " +
		strings.Join(flags, " ") + " sandbox=" + parserkit.SettingValue(jsonvalue.NewBool(sandbox)) + " " + composerSource
}

// settingsRows emits the rows of one settings blob: its standing lists,
// its run modes with each stored sandbox switch (a row of the switch's
// parent shown only when the group goes with it), its switches and the
// models' last use.
func (i *items) settingsRows(settings keyed, sandboxes []sandbox, emit parserkit.Emit) {
	document := objectRecord(i.artifact, settings, i.problems)
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

// standing emits the standing permissions of each settings blob's
// composerState and the models' last use. A blob, composerState, list,
// aiSettings or modelLastUsedAt of the wrong kind is a problem. A repeated
// key gives a parser: row per stored row: each settings blob gives its own
// rows, and the run-mode rows, which take the sandbox switch, are shown
// once, with the stored switch the group goes with (sandboxes).
func (i *items) standing(emit parserkit.Emit) {
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

// shownOnceText returns the start of the parser: row of a stored row of a
// key ItemTable holds in n rows whose rows, named by what, are shown once.
func shownOnceText(what string, n int) string {
	return parserkit.ProblemPrefix + "ItemTable holds " + strconv.Itoa(n) + " rows of this row's key; its " + what +
		" shown once, with "
}

// heldOnce emits the parser: rows of the stored rows of a key whose row,
// named by what, is the one of the stored row held and would read the
// same from any other: with this row for the one held, with another of
// them for the rest.
func (i *items) heldOnce(what string, stored []keyed, held int, emit parserkit.Emit) {
	for n, row := range stored {
		which := "another of them"
		if n == held {
			which = "this row"
		}
		emit(i.system(shownOnceText(what, len(stored))+which, i.mtime, row.rowid))
	}
}

// lenders calls row once per stored row of a key whose value other rows
// take (a usages list, a display name), with the parent each stored row is
// in a group of the key (timeline.Group), or once with a nil parent and
// the one stored row, or none, when the key is stored at most once. Each
// parent gives a parser: row at the time row gives, the rows named by what
// shown once, with the parent the group goes with.
func (i *items) lenders(what string, stored []keyed, emit parserkit.Emit,
	row func(found *keyed) timeline.Row) {
	if len(stored) <= 1 {
		var found *keyed
		if len(stored) == 1 {
			found = &stored[0]
		}
		emit(row(found))
		return
	}
	group := timeline.NewGroup()
	prefix := shownOnceText(what, len(stored))
	for n := range stored {
		made := row(&stored[n])
		notice := made
		notice.TurnType, notice.Text, notice.SourceLine = timeline.TurnSystem, "", int(stored[n].rowid)
		notice.ToolName, notice.ToolUseID = "", ""
		parent := group.Parent(&notice)
		parent.Texts(&notice, timeline.RowText{Text: prefix + "this row"},
			timeline.RowText{Text: prefix + "another of them"})
		emit(notice)
		parent.Row(&made, timeline.WhenChosen)
		emit(made)
	}
}

// enrolledText returns the "remote agent host enrolled:" text of an
// enrolment key, with the account's display name raw.
func enrolledText(artifact *input.Artifact, key string, raw sqlitefile.Value) string {
	name := plain(artifact, raw)
	text := raw.UTF8Text()
	if name.IsString() {
		text = name.StringValue()
	}
	return "remote agent host enrolled: " + cmp.Or(timeline.Compact(text), "-") + " (from state.vscdb " + key +
		", file mtime; on by default, a paired device leaves no local record)"
}

// enrolment emits the "remote agent host enrolled:" row of an enrolment
// key whose first stored row that is true is held, with the display name
// of each stored name row (lenders).
func (i *items) enrolment(key string, stored, names []keyed, emit parserkit.Emit) {
	held := slices.IndexFunc(stored, func(row keyed) bool { return isTrue(plain(i.artifact, row.value)) })
	if held < 0 {
		return
	}
	if len(stored) > 1 {
		i.heldOnce("enrolment row is", stored, held, emit)
	}
	i.lenders("enrolment row is", names, emit, func(name *keyed) timeline.Row {
		var raw sqlitefile.Value
		if name != nil {
			raw = name.value
		}
		return i.system(enrolledText(i.artifact, key, raw), i.mtime, stored[held].rowid)
	})
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
func githubText(login, key string, usages []jsonvalue.Value, work parserkit.Work) string {
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

// githubAccount emits the "source host: github account" row of a
// github-<login> key, held by its first stored row that is a list, with
// the scopes of each stored usages row (lenders), at its latest use, else
// the file's modification time. Another extension's github- key that is
// not an account list is skipped; a cut account list is a bad record. A
// usages value that is not a list, and its entries that are not objects,
// are problems.
func (i *items) githubAccount(key string, stored, usages []keyed, emit parserkit.Emit) {
	login := key[len("github-"):]
	held := -1
	for n, row := range stored {
		if login != "" && plain(i.artifact, row.value).IsArray() {
			if held < 0 {
				held = n
			}
		} else if strings.HasPrefix(row.value.UTF8Text(), "[") {
			decodeRecord(i.artifact, row.rowid, row.key, row.value, i.problems)
		}
	}
	if held < 0 {
		return
	}
	if len(stored) > 1 {
		i.heldOnce("account row is", stored, held, emit)
	}
	i.lenders("account row is", usages, emit, func(found *keyed) timeline.Row {
		var entries []jsonvalue.Value
		if found != nil {
			for _, usage := range objectEntries(i.artifact, *found, i.problems) {
				entries = append(entries, usage)
			}
		}
		text := githubText(login, key, entries, i.owner.Work)
		if latest := lastUsed(entries); !latest.IsEmpty() {
			return i.owner.row(i.artifact, timeline.TurnSystem, text, timeutil.FromValue(latest), stored[held].rowid)
		}
		return i.system(text, i.mtime, stored[held].rowid)
	})
}

// integrations emits the "source host:" rows of the GitHub accounts and
// the "remote agent host enrolled:" rows. Only the github- and
// remoteAgentHost. keys are selected, so no secret:// value is read. A key
// stored more than once gives a parser: row per stored row
// (integrationRepeats).
func (i *items) integrations(emit parserkit.Emit) {
	if i.table == nil {
		return
	}
	github, remote := keyStarts("github-"), keyStarts("remoteAgentHost.")
	rows := i.store.rows(keyedRows(i.table, func(row sqlitefile.Row) bool { return github(row) || remote(row) }))
	byKey := map[string][]keyed{}
	for _, row := range rows {
		byKey[row.key] = append(byKey[row.key], row)
	}
	keys := make([]string, 0, len(byKey))
	for key, stored := range byKey {
		keys = append(keys, key)
		sortByRowid(stored)
	}
	slices.Sort(keys)
	for _, key := range keys {
		stored := byKey[key]
		switch {
		case strings.HasPrefix(key, "github-") && !strings.HasSuffix(key, "-usages"):
			i.githubAccount(key, stored, byKey[key+"-usages"], emit)
		case strings.HasPrefix(key, enrolledPrefix):
			i.enrolment(key, stored, byKey[displayPrefix+key[len(enrolledPrefix):]], emit)
		}
	}
}
