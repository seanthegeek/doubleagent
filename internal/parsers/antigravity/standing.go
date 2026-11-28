package antigravity

import (
	"github.com/seanthegeek/doubleagent/internal/evidencepath"
	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// history emits one user row per prompt of the CLI's history.jsonl.
func (p *Parser) history(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	for number, record := range parserkit.JSONLines(artifact.DiskPath, &problems) {
		row := p.BaseRow(artifact)
		row.SourceLine = number
		row.TimestampUTC = timeutil.FromValue(record.Get("timestamp"))
		row.ProjectPath = evidencepath.FileURIPath(parserkit.Text(record.Get("workspace")))
		row.SessionID = parserkit.Text(record.Get("conversationId"))
		row.TurnType = timeline.TurnUser
		row.Text = timeline.CompactValue(record.Get("display"))
		emit(row)
	}
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}

// entry is one permission entry of a standing file: its key, its value and
// the project it concerns.
type entry struct {
	key     string
	value   jsonvalue.Value
	project string
}

// listEntries returns an entry per item of an array, none for anything
// else; a list present but not an array adds a problem naming key.
func listEntries(key string, list jsonvalue.Value, project string, problems *parserkit.Problems) []entry {
	var out []entry
	for _, value := range parserkit.ArrayMember(list, key, 0, problems) {
		out = append(out, entry{key, value, project})
	}
	return out
}

// objectOf returns value, a member the format makes an object, adding a
// problem naming it when it is present but of another kind.
func objectOf(value jsonvalue.Value, member string, problems *parserkit.Problems) jsonvalue.Value {
	parserkit.ObjectMember(value, member, 0, problems)
	return value
}

// configEntries returns the permission entries of .gemini/config/config.json.
func configEntries(document jsonvalue.Value, problems *parserkit.Problems) []entry {
	settings := objectOf(document.Get("userSettings"), "userSettings", problems)
	grants := objectOf(settings.Get("globalPermissionGrants"), "userSettings.globalPermissionGrants", problems)
	out := listEntries("userSettings.globalPermissionGrants.allow", grants.Get("allow"), "", problems)
	out = append(out, listEntries("userSettings.globalPermissionGrants.deny", grants.Get("deny"), "", problems)...)
	for _, key := range []string{"permissionGrantsV2Migrated", "sandboxEnabledAtV2Migration"} {
		if value, ok := settings.Lookup(key); ok {
			out = append(out, entry{"userSettings." + key, value, ""})
		}
	}
	for _, sidecar := range parserkit.ObjectMember(document.Get("sidecars"), "sidecars", 0, problems) {
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
func projectEntries(document jsonvalue.Value, problems *parserkit.Problems) []entry {
	folder := ""
	for _, resource := range document.Get("projectResources").Get("resources").Items() {
		if uri := resource.Get("folderUri"); uri.IsString() && uri.StringValue() != "" {
			folder = evidencepath.FileURIPath(uri.StringValue())
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

// standingEntries returns the permission entries of a standing file. Only
// these keys are read: userSettings.globalPermissionGrants.allow and .deny,
// permissionGrantsV2Migrated, sandboxEnabledAtV2Migration and each
// sidecar's enabled and resolvedPermissions.accessGrants (config.json);
// permissionGrants and the first projectResources.resources[].folderUri (a
// project); agent_permissions.access_grants (a sidecar); permissions.allow,
// .deny and trustedWorkspaces (settings.json).
func standingEntries(rel string, document jsonvalue.Value, problems *parserkit.Problems) []entry {
	switch {
	case textlines.EqualFoldASCII(rel, configRel):
		return configEntries(document, problems)
	case projectPattern.MatchString(rel):
		return projectEntries(document, problems)
	case sidecarPattern.MatchString(rel):
		permissions := objectOf(document.Get("agent_permissions"), "agent_permissions", problems)
		return listEntries("agent_permissions.access_grants", permissions.Get("access_grants"), "", problems)
	case textlines.EqualFoldASCII(rel, cliSettingsRel):
		permissions := objectOf(document.Get("permissions"), "permissions", problems)
		out := listEntries("permissions.allow", permissions.Get("allow"), "", problems)
		out = append(out, listEntries("permissions.deny", permissions.Get("deny"), "", problems)...)
		for _, value := range parserkit.ArrayMember(document.Get("trustedWorkspaces"), "trustedWorkspaces", 0, problems) {
			path := ""
			if value.IsString() {
				path = evidencepath.FileURIPath(value.StringValue())
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

// standing emits a "permission setting:" row per permission entry of a
// standing permission file, at the file's modification time, then a
// problem row per member that is not of the kind the format gives it.
func (p *Parser) standing(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	content, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	mtime := parserkit.FileMtime(artifact)
	document, err := parserkit.DecodeJSON(artifact, textlines.ValidUTF8(content))
	if err != nil || !document.IsObject() {
		row := p.SystemRow(artifact, "parser: unreadable "+artifact.Rel)
		row.SourceLine, row.TimestampUTC = 1, mtime
		emit(row)
		return
	}
	entries := standingEntries(artifact.Rel, document, &problems)
	// The entries of a list share its key, a sidecar's name in it.
	carried := parserkit.NewCarried(artifact)
	for i, found := range entries {
		row := p.SystemRow(artifact, timeline.Compact("permission setting: "+carried.Cut("settings key", found.key)+"="+
			parserkit.SettingValue(found.value)+" (from "+artifact.Rel+", file mtime)"))
		row.SourceLine, row.TimestampUTC, row.ProjectPath = i+1, mtime, found.project
		emit(row)
	}
	carried.Report(0, &problems)
	for _, found := range problems {
		row := p.SystemRow(artifact, parserkit.ProblemPrefix+found.Message)
		row.SourceLine, row.TimestampUTC = 1, mtime
		emit(row)
	}
}
