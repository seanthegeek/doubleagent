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
		row.ProjectPath = uriPath(parser.Text(record.Get("workspace")))
		row.SessionID = parser.Text(record.Get("conversationId"))
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
// standing permission file, at the file's modification time, then a
// problem row per member that is not of the kind the format gives it.
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
	entries := standingEntries(artifact.Rel, document, &problems)
	for i, found := range entries {
		row := p.SystemRow(artifact, timeline.Compact("permission setting: "+found.key+"="+
			parser.SettingValue(found.value)+" (from "+artifact.Rel+", file mtime)"))
		row.SourceLine, row.TimestampUTC, row.ProjectPath = i+1, mtime, found.project
		emit(row)
	}
	for _, found := range problems {
		row := p.SystemRow(artifact, parser.ProblemPrefix+found.Message)
		row.SourceLine, row.TimestampUTC = 1, mtime
		emit(row)
	}
}

// listEntries returns an entry per item of an array, none for anything
// else; a list present but not an array adds a problem naming key.
func listEntries(key string, list jsonvalue.Value, project string, problems *parser.Problems) []entry {
	var out []entry
	for _, value := range parser.ArrayMember(list, key, 0, problems) {
		out = append(out, entry{key, value, project})
	}
	return out
}

// objectOf returns value, a member the format makes an object, adding a
// problem naming it when it is present but of another kind.
func objectOf(value jsonvalue.Value, member string, problems *parser.Problems) jsonvalue.Value {
	parser.ObjectMember(value, member, 0, problems)
	return value
}

// standingEntries returns the permission entries of a standing file. Only
// these keys are read: userSettings.globalPermissionGrants.allow and .deny,
// permissionGrantsV2Migrated, sandboxEnabledAtV2Migration and each
// sidecar's enabled and resolvedPermissions.accessGrants (config.json);
// permissionGrants and the first projectResources.resources[].folderUri (a
// project); agent_permissions.access_grants (a sidecar); permissions.allow,
// .deny and trustedWorkspaces (settings.json).
func standingEntries(rel string, document jsonvalue.Value, problems *parser.Problems) []entry {
	switch {
	case rel == configRel:
		return configEntries(document, problems)
	case projectPattern.MatchString(rel):
		return projectEntries(document, problems)
	case sidecarPattern.MatchString(rel):
		permissions := objectOf(document.Get("agent_permissions"), "agent_permissions", problems)
		return listEntries("agent_permissions.access_grants", permissions.Get("access_grants"), "", problems)
	case rel == cliSettingsRel:
		permissions := objectOf(document.Get("permissions"), "permissions", problems)
		out := listEntries("permissions.allow", permissions.Get("allow"), "", problems)
		out = append(out, listEntries("permissions.deny", permissions.Get("deny"), "", problems)...)
		for _, value := range parser.ArrayMember(document.Get("trustedWorkspaces"), "trustedWorkspaces", 0, problems) {
			path := ""
			if value.IsString() {
				path = uriPath(value.StringValue())
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
func configEntries(document jsonvalue.Value, problems *parser.Problems) []entry {
	settings := objectOf(document.Get("userSettings"), "userSettings", problems)
	grants := objectOf(settings.Get("globalPermissionGrants"), "userSettings.globalPermissionGrants", problems)
	out := listEntries("userSettings.globalPermissionGrants.allow", grants.Get("allow"), "", problems)
	out = append(out, listEntries("userSettings.globalPermissionGrants.deny", grants.Get("deny"), "", problems)...)
	for _, key := range []string{"permissionGrantsV2Migrated", "sandboxEnabledAtV2Migration"} {
		if value, ok := settings.Lookup(key); ok {
			out = append(out, entry{"userSettings." + key, value, ""})
		}
	}
	for _, sidecar := range parser.ObjectMember(document.Get("sidecars"), "sidecars", 0, problems) {
		objectOf(sidecar.Value, "sidecars."+sidecar.Key, problems)
		if value, ok := sidecar.Value.Lookup("enabled"); ok {
			out = append(out, entry{"sidecars." + sidecar.Key + ".enabled", value, ""})
		}
		resolved := objectOf(sidecar.Value.Get("resolvedPermissions"),
			"sidecars."+sidecar.Key+".resolvedPermissions", problems)
		out = append(out, listEntries("sidecars."+sidecar.Key+".resolvedPermissions.accessGrants",
			resolved.Get("accessGrants"), "", problems)...)
	}
	return out
}

// projectEntries returns the permission entries of a project file, each
// with the project's first folder.
func projectEntries(document jsonvalue.Value, problems *parser.Problems) []entry {
	folder := ""
	for _, resource := range document.Get("projectResources").Get("resources").Items() {
		if uri := resource.Get("folderUri"); uri.IsString() && uri.StringValue() != "" {
			folder = uriPath(uri.StringValue())
			break
		}
	}
	grants := objectOf(document.Get("permissionGrants"), "permissionGrants", problems)
	inner := objectOf(grants.Get("permissionGrants"), "permissionGrants.permissionGrants", problems)
	out := listEntries("permissionGrants.permissionGrants.allow", inner.Get("allow"), folder, problems)
	out = append(out, listEntries("permissionGrants.permissionGrants.deny", inner.Get("deny"), folder, problems)...)
	if value, ok := grants.Lookup("v2Migrated"); ok {
		out = append(out, entry{"permissionGrants.v2Migrated", value, folder})
	}
	return out
}
