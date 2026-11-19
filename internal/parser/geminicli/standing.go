package geminicli

import (
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/textlines"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/tomldoc"
)

// sandboxPolicy is the sandbox's standing grants, a policy file of the
// home only.
const sandboxPolicy = ".gemini/policies/sandbox.toml"

// settingLists and settingScalars are the settings.json keys that hold
// standing permission or sandbox state: one row per list entry, one row per
// scalar. retentionKeys are the keys of general.sessionRetention.
func settingLists() []string {
	return []string{"tools.sandboxAllowedPaths", "tools.core", "tools.allowed",
		"tools.confirmationRequired", "tools.exclude", "mcp.allowed", "mcp.excluded",
		"security.allowedExtensions"}
}

// settingScalars are the settings.json keys read as one row each.
func settingScalars() []string {
	return []string{"general.defaultApprovalMode", "tools.sandbox", "tools.sandboxNetworkAccess",
		"security.toolSandboxing", "security.disableYoloMode", "security.disableAlwaysAllow",
		"security.enablePermanentToolApproval", "security.autoAddToPolicyByDefault",
		"security.blockGitExtensions", "security.folderTrust.enabled", "security.enableConseca"}
}

// retentionKeys are the keys of general.sessionRetention read.
func retentionKeys() []string {
	return []string{"enabled", "maxAge", "maxCount", "minRetention"}
}

// standingRow returns a system row at line 1 for a standing setting or a
// whole-file problem, at the given time and in the given project.
func standingRow(base parser.Base, artifact *input.Artifact, timestamp, project,
	text string) timeline.Row {
	row := base.SystemRow(artifact, timeline.Compact(text))
	row.SourceLine = 1
	row.TimestampUTC = timestamp
	row.ProjectPath = project
	return row
}

// fromFile returns the "(from <rel>, file mtime)" note of a standing row.
func fromFile(artifact *input.Artifact) string {
	return "(from " + artifact.Rel + ", file mtime)"
}

// TrustedFolderRows emits one "permission setting:
// trustedFolders.<path>=<value>" row per key of a trustedFolders.json of
// Gemini CLI or of Qwen Code, as the parser of base, at the file's
// modification time and in the folder's project. The file is read as JSON
// with comments and trailing commas, which Qwen Code accepts; a file that
// cannot be read, or is not a JSON object, is one row saying so.
func TrustedFolderRows(base parser.Base, artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	data, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, base.ErrorRows(artifact, problems, "", ""))
		return
	}
	timestamp := parser.FileMtime(artifact)
	document, err := parser.ParseJSONC(data)
	if err != nil || !document.IsObject() {
		emit(standingRow(base, artifact, timestamp, "", "parser: "+artifact.Rel+
			" is not a JSON object"))
		return
	}
	for _, member := range document.Members() {
		emit(standingRow(base, artifact, timestamp, member.Key, "permission setting: trustedFolders."+
			member.Key+"="+parser.SettingValue(member.Value)+" "+fromFile(artifact)))
	}
}

// jsonObject returns the artifact's JSON object, read as JSON with
// comments when jsonc is set, or false with the row that says it cannot be
// read or is not one.
func (p *Parser) jsonObject(artifact *input.Artifact, jsonc bool) (jsonvalue.Value, timeline.Row,
	bool) {
	var problems parser.Problems
	data, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		return jsonvalue.NewNull(), p.ErrorRows(artifact, problems, "", "")[0], false
	}
	var document jsonvalue.Value
	var err error
	if jsonc {
		document, err = parser.ParseJSONC(data)
	} else {
		document, err = jsonvalue.Decode(data)
	}
	if err == nil && document.IsObject() {
		return document, timeline.Row{}, true
	}
	text := "parser: " + artifact.Rel + " is not a JSON object"
	return jsonvalue.NewNull(), standingRow(p.Base, artifact, parser.FileMtime(artifact), "", text), false
}

// dotted returns the value at a dotted key of nested objects, or null.
func dotted(document jsonvalue.Value, key string) jsonvalue.Value {
	value := document
	for part := range strings.SplitSeq(key, ".") {
		if !value.IsObject() {
			return jsonvalue.NewNull()
		}
		value = value.Get(part)
	}
	return value
}

// homeProject returns the project of a standing file: the project
// directory that holds it, or "" for a user's home.
func homeProject(artifact *input.Artifact) string {
	if artifact.Home.Project {
		return artifact.Home.Original
	}
	return ""
}

// settings emits the standing permission, sandbox and session retention
// rows of a settings.json, which the CLI reads with comments stripped.
func (p *Parser) settings(artifact *input.Artifact, emit parser.Emit) {
	document, bad, ok := p.jsonObject(artifact, true)
	if !ok {
		emit(bad)
		return
	}
	timestamp, project := parser.FileMtime(artifact), homeProject(artifact)
	setting := func(label, key string, value jsonvalue.Value) {
		emit(standingRow(p.Base, artifact, timestamp, project, label+": "+key+"="+
			parser.SettingValue(value)+" "+fromFile(artifact)))
	}
	for _, key := range settingLists() {
		switch value := dotted(document, key); {
		case value.IsArray():
			for _, entry := range value.Items() {
				setting("permission setting", key, entry)
			}
		case !value.IsNull():
			setting("permission setting", key, value)
		}
	}
	for _, key := range settingScalars() {
		if value := dotted(document, key); !value.IsNull() {
			setting("permission setting", key, value)
		}
	}
	for _, key := range retentionKeys() {
		key = "general.sessionRetention." + key
		if value := dotted(document, key); !value.IsNull() {
			setting("session retention", key, value)
		}
	}
}

// policy emits one row per [[rule]] table of a policy file, and for the
// home's sandbox.toml one per sandbox mode and command grant. A rule
// array holding entries that are not tables gives one row for them all
// after the rules (wrongTables).
func (p *Parser) policy(artifact *input.Artifact, emit parser.Emit) {
	timestamp, project := parser.FileMtime(artifact), homeProject(artifact)
	var problems parser.Problems
	data, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", project))
		return
	}
	row := func(text string) {
		emit(standingRow(p.Base, artifact, timestamp, project, text))
	}
	document, err := tomldoc.Parse(textlines.ValidUTF8(data))
	if err != nil {
		row("parser: unreadable " + artifact.Rel + ": " + parser.ReaderErrorText(artifact, err))
		return
	}
	source := fromFile(artifact)
	if rules, ok := document.Get("rule"); ok && rules.Kind() == tomldoc.KindArray {
		wrong := 0
		for _, rule := range rules.Items() {
			if rule.Kind() != tomldoc.KindTable {
				wrong++
				continue
			}
			row(ruleText(rule.Table(), source))
		}
		if wrong > 0 {
			row("parser: " + artifact.Rel + " " + wrongTables("rule", wrong))
		}
	} else if ok {
		row("parser: " + artifact.Rel + " rule is not an array")
	}
	if artifact.Rel == sandboxPolicy && !artifact.Home.Project {
		for _, text := range sandboxTexts(document, artifact.Rel, source) {
			row(text)
		}
	}
}

// wrongTables returns the problem text of a TOML array whose entries
// include some that are not tables: "<member> holds <n> entries that are
// not tables", or "holds 1 entry that is not a table", the TOML form of
// parser.WrongEntries.
func wrongTables(member string, n int) string {
	if n == 1 {
		return member + " holds 1 entry that is not a table"
	}
	return member + " holds " + strconv.Itoa(n) + " entries that are not tables"
}

// ruleText returns the row text of one [[rule]] table: its decision and
// tool, "-" when absent, then its other keys in file order.
func ruleText(rule *tomldoc.Table, source string) string {
	fields := []string{"permission setting: policy", tableSetting(rule, "decision"),
		tableSetting(rule, "toolName")}
	for _, entry := range rule.Entries() {
		if entry.Key != "decision" && entry.Key != "toolName" {
			fields = append(fields, entry.Key+"="+tomldoc.SettingText(entry.Value))
		}
	}
	return strings.Join(append(fields, source), " ")
}

// tableSetting returns a table's key as setting text, "-" when absent.
func tableSetting(table *tomldoc.Table, key string) string {
	if value, ok := table.Get(key); ok {
		return tomldoc.SettingText(value)
	}
	return "-"
}

// sandboxTexts returns the row texts of sandbox.toml: one per table of
// modes, then one per table of commands, each table's keys in file order
// as the CLI's sandbox policy manager wrote them. A modes or commands, or
// a grant in one, that is present but not a table is a "parser: <file>
// <key> is not a table" text in its place, as a rule that is not an array.
func sandboxTexts(document *tomldoc.Table, rel, source string) []string {
	var out []string
	for _, section := range []struct{ label, key string }{{"mode", "modes"}, {"command", "commands"}} {
		table, ok := document.Get(section.key)
		if !ok {
			continue
		}
		if table.Kind() != tomldoc.KindTable {
			out = append(out, "parser: "+rel+" "+section.key+" is not a table")
			continue
		}
		for _, grant := range table.Table().Entries() {
			if grant.Value.Kind() != tomldoc.KindTable {
				out = append(out, "parser: "+rel+" "+section.key+"."+grant.Key+" is not a table")
				continue
			}
			fields := []string{"permission setting: sandbox " + section.label, grant.Key}
			for _, entry := range grant.Value.Table().Entries() {
				fields = append(fields, entry.Key+"="+tomldoc.SettingText(entry.Value))
			}
			out = append(out, strings.Join(append(fields, source), " "))
		}
	}
	return out
}

// projectGrants emits the rows of trusted_hooks.json (one per trusted hook
// of each project) or acknowledgments/agents.json (one per acknowledged
// agent of each project). Both are plain JSON.
func (p *Parser) projectGrants(artifact *input.Artifact, emit parser.Emit) {
	document, bad, ok := p.jsonObject(artifact, false)
	if !ok {
		emit(bad)
		return
	}
	timestamp := parser.FileMtime(artifact)
	hooks := hooksPattern.MatchString(artifact.Rel)
	for _, project := range document.Members() {
		var keys []string
		var entries []jsonvalue.Value
		switch value := project.Value; {
		case hooks && value.IsArray():
			for _, hook := range value.Items() {
				keys, entries = append(keys, "trustedHooks"), append(entries, hook)
			}
		case hooks:
			keys, entries = []string{"trustedHooks"}, []jsonvalue.Value{value}
		case value.IsObject():
			for _, agent := range value.Members() {
				keys = append(keys, "acknowledgedAgents."+agent.Key)
				entries = append(entries, agent.Value)
			}
		default:
			keys, entries = []string{"acknowledgedAgents"}, []jsonvalue.Value{value}
		}
		for i, key := range keys {
			emit(standingRow(p.Base, artifact, timestamp, project.Key, "permission setting: "+key+"="+
				parser.SettingValue(entries[i])+" "+fromFile(artifact)))
		}
	}
}
