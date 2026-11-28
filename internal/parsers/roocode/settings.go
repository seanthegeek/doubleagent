package roocode

import (
	"cmp"
	"path"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parsers/clinelegacy"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
	"github.com/seanthegeek/doubleagent/internal/vscodestate"
)

// retryQueueKey is the workspace state key of the Roo Code Cloud client's
// failed uploads.
const retryQueueKey = "roo.retryQueue"

// extensionKeys are the ItemTable keys of the extension's state: its
// <publisher>.<name> (src/package.json), and the lower-case form in case an
// editor normalises it.
func extensionKeys() []string {
	return []string{"RooVeterinaryInc.roo-cline", "rooveterinaryinc.roo-cline"}
}

// standingKeys are the global state keys that grant or limit what runs
// without asking (packages/types/src/global-settings.ts:97-118 and :192),
// and the two VS Code settings the CLI's shim keeps in the same file.
func standingKeys() []string {
	return []string{"autoApprovalEnabled", "alwaysAllowReadOnly",
		"alwaysAllowReadOnlyOutsideWorkspace", "alwaysAllowWrite", "alwaysAllowWriteOutsideWorkspace",
		"alwaysAllowWriteProtected", "alwaysAllowMcp", "alwaysAllowModeSwitch", "alwaysAllowSubtasks",
		"alwaysAllowExecute", "alwaysAllowFollowupQuestions", "allowedCommands", "deniedCommands",
		"commandTimeoutAllowlist", "allowedMaxRequests", "allowedMaxCost", "mode",
		"roo-cline.allowedCommands", "roo-cline.deniedCommands"}
}

// orgSyncKeys are the Roo Code Cloud account settings, cached up to
// release 3.53.0, that decide whether task messages were uploaded.
func orgSyncKeys() []string {
	return []string{"recordTaskMessages", "enableTaskSharing", "allowPublicTaskSharing",
		"taskShareExpirationDays", "allowMembersViewAllTasks", "workspaceTaskVisibility"}
}

// cliEntries returns the entries of the CLI history: the document's entries
// array, else, for text that is not JSON, the array after its "entries"
// key walked as far as it parses.
func cliEntries(text string, data jsonvalue.Value, decoded bool, problems *parserkit.Problems) func(func(int, jsonvalue.Value) bool) {
	if entries := data.Get("entries"); entries.IsArray() {
		return func(yield func(int, jsonvalue.Value) bool) {
			for i, entry := range entries.Items() {
				if !yield(i+1, entry) {
					return
				}
			}
		}
	}
	if !decoded {
		if start, found := clinelegacy.ArrayAfterKey(text, entriesArray); found {
			return parserkit.ArrayAt(text, start, problems, nil)
		}
	}
	problems.Add(0, "cli-history.json has no entries array")
	return func(func(int, jsonvalue.Value) bool) {}
}

// entriesArray finds the entries array of a CLI history that is not JSON.
var entriesArray = clinelegacy.KeyedArray("entries")

// cliHistory emits one user row per entry of .roo/cli-history.json. The
// CLI rewrites the whole file per prompt, so a file cut mid-write is walked
// along its entries array as far as it parses.
func (p *Parser) cliHistory(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	text, ok := parserkit.ReadText(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	data, err := parserkit.DecodeJSON(artifact, text)
	// The CLI's own loadHistory keeps only non-blank strings, so an entry of
	// another kind is passed over as it passes it over (research/roo-code.md).
	for position, entry := range cliEntries(text, data, err == nil, &problems) {
		if !entry.IsString() || strings.TrimSpace(entry.StringValue()) == "" {
			continue
		}
		row := p.BaseRow(artifact)
		row.SourceLine = position
		row.TurnType = timeline.TurnUser
		row.Text = entry.StringValue()
		emit(row)
	}
	if err != nil && !problems.Damaged() {
		problems.Add(0, "cli-history.json unparseable after its entries array")
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// unusable emits the unreadable-file rows of problems, else message as a
// problem row.
func (p *Parser) unusable(artifact *input.Artifact, problems parserkit.Problems, message string, emit parserkit.Emit) {
	if len(problems) > 0 {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	emit(p.ProblemRow(artifact, message))
}

// stateFile returns a state file as a JSON object, or false after emitting
// the row that says why it is not one.
func (p *Parser) stateFile(artifact *input.Artifact, what string, emit parserkit.Emit) (jsonvalue.Value, bool) {
	var problems parserkit.Problems
	document, _ := parserkit.LoadJSON(artifact, artifact.DiskPath, &problems)
	if document.IsObject() {
		return document, true
	}
	p.unusable(artifact, problems, what+" is not a JSON object", emit)
	return jsonvalue.NewNull(), false
}

// settingRow returns "<label>: <key>=<value> (from <source>, file mtime)"
// at line 1 and the file's modification time.
func (p *Parser) settingRow(artifact *input.Artifact, label, key string, value jsonvalue.Value,
	source, project string) timeline.Row {
	row := p.SystemRow(artifact, timeline.Compact(label+": "+key+"="+parserkit.SettingValue(value)+
		" (from "+source+", file mtime)"))
	row.SourceLine = 1
	row.TimestampUTC = parserkit.FileMtime(artifact)
	row.ProjectPath = project
	return row
}

// settingsRows emits a permission setting row per scalar of keys, or per
// entry of a list.
func (p *Parser) settingsRows(artifact *input.Artifact, document jsonvalue.Value, keys []string,
	source string, emit parserkit.Emit) {
	for _, key := range keys {
		value := document.Get(key)
		if value.IsNull() {
			continue
		}
		items := []jsonvalue.Value{value}
		if value.IsArray() {
			items = value.Items()
		}
		for _, item := range items {
			emit(p.settingRow(artifact, "permission setting", key, item, source, ""))
		}
	}
}

// globalStateRows emits the standing permission settings, then the Roo Code
// Cloud settings cache, which exists only while the extension was signed in
// and says whether task messages were uploaded.
func (p *Parser) globalStateRows(artifact *input.Artifact, state jsonvalue.Value, source string, emit parserkit.Emit) {
	p.settingsRows(artifact, state, standingKeys(), source, emit)
	cloudSource := source + ", Roo Code Cloud settings cache"
	if user := state.Get("user-settings"); user.IsObject() {
		value := user.Get("settings").Get("taskSyncEnabled")
		if value.IsNull() {
			value = jsonvalue.NewString("unset (defaults to true)")
		}
		emit(p.settingRow(artifact, "sync setting", "user-settings.settings.taskSyncEnabled", value,
			cloudSource, ""))
	}
	cloud := state.Get("organization-settings").Get("cloudSettings")
	if !cloud.IsObject() {
		return
	}
	for _, key := range orgSyncKeys() {
		if value := cloud.Get(key); !value.IsNull() {
			emit(p.settingRow(artifact, "sync setting", "organization-settings.cloudSettings."+key,
				value, cloudSource, ""))
		}
	}
}

// cliSettings emits the permission settings of .roo/cli-settings.json.
func (p *Parser) cliSettings(artifact *input.Artifact, emit parserkit.Emit) {
	if document, ok := p.stateFile(artifact, "cli-settings.json", emit); ok {
		p.settingsRows(artifact, document, []string{"requireApproval", "dangerouslySkipPermissions",
			"mode"}, artifact.Rel, emit)
	}
}

// mcp emits one row per tool name of mcpServers.<name>.alwaysAllow.
// Nothing else is read: the server entries can hold env and headers.
func (p *Parser) mcp(artifact *input.Artifact, emit parserkit.Emit) {
	document, ok := p.stateFile(artifact, path.Base(artifact.Rel), emit)
	if !ok {
		return
	}
	project := ""
	if artifact.Home.Project {
		project = artifact.Home.Original
	}
	carried := parserkit.NewCarried(artifact)
	for _, member := range document.Get("mcpServers").Members() {
		// Each tool's row repeats its server's name.
		key := "mcpServers." + carried.Cut("server name", member.Key) + ".alwaysAllow"
		for _, tool := range member.Value.Get("alwaysAllow").Items() {
			emit(p.settingRow(artifact, "permission setting", key, tool, artifact.Rel, project))
		}
	}
	var problems parserkit.Problems
	carried.Report(0, &problems)
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
}

// queuedEvent returns the JSON event body of a queued request, or null
// when it is not a JSON object.
func queuedEvent(artifact *input.Artifact, request jsonvalue.Value) jsonvalue.Value {
	body := request.Get("options").Get("body")
	if !body.IsString() {
		return jsonvalue.NewNull()
	}
	event, err := parserkit.DecodeJSON(artifact, body.StringValue())
	if err != nil || !event.IsObject() {
		return jsonvalue.NewNull()
	}
	return event
}

// retryQueueRows emits one row per request the Roo Code Cloud client queued
// after a failed upload: the event type, the task id, the retry state, the
// URL and the Authorization header, the session's
// bearer token, through parserkit.SecretText; the body, the event, is not
// written. A retry queue that is not an array emits no row and returns the
// problem text saying so; otherwise it returns "".
func (p *Parser) retryQueueRows(artifact *input.Artifact, state jsonvalue.Value, source string,
	emit parserkit.Emit) string {
	var problems parserkit.Problems
	for i, request := range parserkit.EntriesOfKind(state.Get(retryQueueKey), retryQueueKey, jsonvalue.Object, 0,
		&problems) {
		event := queuedEvent(artifact, request)
		taskID := event.Get("properties").Get("taskId")
		retries := "-"
		if value, found := request.Lookup("retryCount"); found {
			retries = value.Text()
		}
		parts := []string{
			"cloud upload queued: " + cmp.Or(parserkit.Text(event.Get("type")), parserkit.Text(request.Get("operation")), "-"),
			"task=" + cmp.Or(parserkit.Text(taskID), "-"),
			"type=" + cmp.Or(parserkit.Text(request.Get("type")), "-"),
			"retries=" + retries,
			"url=" + cmp.Or(parserkit.Text(request.Get("url")), "-"),
		}
		if authorization := parserkit.Text(request.Get("options").Get("headers").Get("Authorization")); authorization != "" {
			parts = append(parts, "Authorization="+parserkit.SecretText(artifact, authorization))
		}
		if lastError := request.Get("lastError"); !lastError.IsEmpty() {
			parts = append(parts, "lastError="+lastError.Text())
		}
		row := p.SystemRow(artifact, timeline.Compact(strings.Join(parts, " ")+
			" (Roo Code Cloud retry queue, from "+source+" "+retryQueueKey+", request timestamp)"))
		row.SourceLine = i + 1
		row.TimestampUTC = timeutil.FromValue(request.Get("timestamp"))
		row.SessionID = parserkit.Text(taskID)
		emit(row)
	}
	if len(problems) > 0 {
		return problems[0].Message
	}
	return ""
}

// stateDatabase emits the rows of the extension's state in an editor's
// global or workspace state.vscdb, then a row per problem.
func (p *Parser) stateDatabase(artifact *input.Artifact, emit parserkit.Emit) {
	values, problems, repeats := vscodestate.ReadItems(artifact.Home.Listing, artifact.DiskPath, extensionKeys(),
		parserkit.ReaderErrors(artifact))
	parserkit.NoteRepeats(artifact, repeats)
	workspace := workspaceDatabase.MatchString(artifact.Rel)
	for _, key := range extensionKeys() {
		state, found := values[key]
		if !found || state.IsNull() {
			continue
		}
		if !state.IsObject() {
			if !slices.ContainsFunc(problems, func(problem string) bool { return strings.Contains(problem, key) }) {
				problems = append(problems, key+" value is not a JSON object")
			}
			continue
		}
		source := artifact.Rel + " ItemTable " + key
		if workspace {
			if problem := p.retryQueueRows(artifact, state, source, emit); problem != "" {
				problems = append(problems, problem)
			}
		} else {
			p.globalStateRows(artifact, state, source, emit)
		}
	}
	for _, problem := range problems {
		emit(p.ProblemRow(artifact, problem))
	}
}
