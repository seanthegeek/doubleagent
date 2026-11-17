package antigravity

import (
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// history emits one user row per prompt of the CLI's history.jsonl.
func (p *Parser) history(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	for number, record := range parser.JSONLines(artifact.DiskPath, &problems) {
		row := p.BaseRow(artifact)
		row.SourceLine = number
		row.TimestampUTC = timeutil.FromValue(record.Get("timestamp"))
		row.ProjectPath = URIToPath(parser.Str(record.Get("workspace")))
		row.SessionID = parser.Str(record.Get("conversationId"))
		row.TurnType = timeline.TurnUser
		row.Text = timeline.CompactValue(record.Get("display"))
		emit(row)
	}
	parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// entry is one permission entry of a standing file: its key, its value and
// the project it concerns.
type entry struct {
	key     string
	value   jsonvalue.Value
	project string
}

// standing emits a "permission setting:" row per permission entry of a
// standing permission file, at the file's modification time.
func (p *Parser) standing(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	content, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	mtime := parser.FileMtime(artifact)
	document, err := jsonvalue.DecodeString(textlines.ValidUTF8(content))
	if err != nil || !document.IsObject() {
		row := p.SystemRow(artifact, "parser: unreadable "+artifact.Rel)
		row.SourceLine, row.TimestampUTC = 1, mtime
		emit(row)
		return
	}
	for i, found := range standingEntries(artifact.Rel, document) {
		row := p.SystemRow(artifact, timeline.Compact("permission setting: "+found.key+"="+
			parser.SettingValue(found.value)+" (from "+artifact.Rel+", file mtime)"))
		row.SourceLine, row.TimestampUTC, row.ProjectPath = i+1, mtime, found.project
		emit(row)
	}
}

// listEntries returns an entry per item of an array, none for anything
// else.
func listEntries(key string, list jsonvalue.Value, project string) []entry {
	var out []entry
	for _, value := range list.Items() {
		out = append(out, entry{key, value, project})
	}
	return out
}

// standingEntries returns the permission entries of a standing file. Only
// these keys are read: userSettings.globalPermissionGrants.allow and .deny,
// permissionGrantsV2Migrated, sandboxEnabledAtV2Migration and each
// sidecar's enabled and resolvedPermissions.accessGrants (config.json);
// permissionGrants and the first projectResources.resources[].folderUri (a
// project); agent_permissions.access_grants (a sidecar); permissions.allow,
// .deny and trustedWorkspaces (settings.json).
func standingEntries(rel string, document jsonvalue.Value) []entry {
	switch {
	case rel == configRel:
		return configEntries(document)
	case projectPattern.MatchString(rel):
		return projectEntries(document)
	case sidecarPattern.MatchString(rel):
		return listEntries("agent_permissions.access_grants", document.Get("agent_permissions").Get("access_grants"), "")
	case rel == cliSettingsRel:
		permissions := document.Get("permissions")
		out := listEntries("permissions.allow", permissions.Get("allow"), "")
		out = append(out, listEntries("permissions.deny", permissions.Get("deny"), "")...)
		for _, value := range document.Get("trustedWorkspaces").Items() {
			path := ""
			if value.IsString() {
				path = URIToPath(value.StringValue())
			}
			shown := value
			if path != "" {
				shown = jsonvalue.NewString(path)
			}
			out = append(out, entry{"trustedWorkspace", shown, path})
		}
		return out
	}
	return nil
}

// configEntries returns the permission entries of .gemini/config/config.json.
func configEntries(document jsonvalue.Value) []entry {
	settings := document.Get("userSettings")
	grants := settings.Get("globalPermissionGrants")
	out := listEntries("userSettings.globalPermissionGrants.allow", grants.Get("allow"), "")
	out = append(out, listEntries("userSettings.globalPermissionGrants.deny", grants.Get("deny"), "")...)
	for _, key := range []string{"permissionGrantsV2Migrated", "sandboxEnabledAtV2Migration"} {
		if value, ok := settings.Lookup(key); ok {
			out = append(out, entry{"userSettings." + key, value, ""})
		}
	}
	for _, sidecar := range document.Get("sidecars").Members() {
		if value, ok := sidecar.Value.Lookup("enabled"); ok {
			out = append(out, entry{"sidecars." + sidecar.Key + ".enabled", value, ""})
		}
		out = append(out, listEntries("sidecars."+sidecar.Key+".resolvedPermissions.accessGrants",
			sidecar.Value.Get("resolvedPermissions").Get("accessGrants"), "")...)
	}
	return out
}

// projectEntries returns the permission entries of a project file, each
// with the project's first folder.
func projectEntries(document jsonvalue.Value) []entry {
	folder := ""
	for _, resource := range document.Get("projectResources").Get("resources").Items() {
		if uri := resource.Get("folderUri"); uri.IsString() && uri.StringValue() != "" {
			folder = URIToPath(uri.StringValue())
			break
		}
	}
	grants := document.Get("permissionGrants")
	inner := grants.Get("permissionGrants")
	out := listEntries("permissionGrants.permissionGrants.allow", inner.Get("allow"), folder)
	out = append(out, listEntries("permissionGrants.permissionGrants.deny", inner.Get("deny"), folder)...)
	if value, ok := grants.Lookup("v2Migrated"); ok {
		out = append(out, entry{"permissionGrants.v2Migrated", value, folder})
	}
	return out
}
