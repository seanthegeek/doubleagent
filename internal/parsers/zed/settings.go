package zed

import (
	"bytes"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// settingsOverrides are the release-channel and platform override blocks of
// a user settings file, each a whole settings object of its own.
func settingsOverrides() []string {
	return []string{"dev", "nightly", "preview", "stable", "macos", "linux", "windows"}
}

// setting is one standing permission entry: its dotted key and its value.
type setting struct {
	key   string
	value jsonvalue.Value
}

// flatten appends one entry per leaf of a settings value: an object's
// members under dotted keys, a list's items each under the list's key; the
// key written before each member or item is cut by carried.
func flatten(entries []setting, key string, value jsonvalue.Value, carried *parserkit.Carried) []setting {
	switch {
	case value.IsObject():
		for _, member := range value.Members() {
			entries = flatten(entries, carried.Cut("settings key", key)+"."+member.Key, member.Value, carried)
		}
	case value.IsArray():
		for _, item := range value.Items() {
			entries = append(entries, setting{carried.Cut("settings key", key), item})
		}
	default:
		entries = append(entries, setting{key, value})
	}
	return entries
}

// blockEntries returns the standing permission entries of one settings
// block, each key prefixed with prefix.
func blockEntries(prefix string, block jsonvalue.Value, carried *parserkit.Carried) []setting {
	var entries []setting
	if agent := block.Get("agent"); agent.IsObject() {
		for _, section := range []string{"tool_permissions", "sandbox_permissions"} {
			if value, ok := agent.Lookup(section); ok {
				entries = flatten(entries, prefix+"agent."+section, value, carried)
			}
		}
		if legacy, ok := agent.Lookup("always_allow_tool_actions"); ok {
			entries = append(entries, setting{prefix + "agent.always_allow_tool_actions", legacy})
		}
	}
	if servers := block.Get("agent_servers"); servers.IsObject() {
		for _, member := range servers.Members() {
			if mode, ok := member.Value.Lookup("default_mode"); ok && member.Value.IsObject() {
				key := prefix + "agent_servers." + member.Key + ".default_mode"
				entries = append(entries, setting{key, mode})
			}
		}
	}
	if session := block.Get("session"); session.IsObject() {
		if trust, ok := session.Lookup("trust_all_worktrees"); ok {
			entries = append(entries, setting{prefix + "session.trust_all_worktrees", trust})
		}
	}
	return entries
}

// settingsEntries returns the standing permission entries of a settings
// object: those of agent.tool_permissions, agent.sandbox_permissions, the
// legacy agent.always_allow_tool_actions, agent_servers.<id>.default_mode
// and session.trust_all_worktrees, in the top level, each override block
// and each profiles.<name>.settings, the key prefixed with the block's; a
// profile's name or a key written before each of its leaves is cut by
// carried.
func settingsEntries(document jsonvalue.Value, carried *parserkit.Carried) []setting {
	type layer struct {
		prefix string
		block  jsonvalue.Value
	}
	layers := []layer{{"", document}}
	for _, name := range settingsOverrides() {
		if block := document.Get(name); block.IsObject() {
			layers = append(layers, layer{name + ".", block})
		}
	}
	if profiles := document.Get("profiles"); profiles.IsObject() {
		for _, member := range profiles.Members() {
			if settings := member.Value.Get("settings"); settings.IsObject() {
				layers = append(layers, layer{"profiles." + carried.Cut("settings profile", member.Key) +
					".settings.", settings})
			}
		}
	}
	var entries []setting
	for _, each := range layers {
		entries = append(entries, blockEntries(each.prefix, each.block, carried)...)
	}
	return entries
}

// parseSettings emits one `permission setting:` row per standing permission
// of a settings file, at the file's modification time.
func (p *Parser) parseSettings(artifact *input.Artifact, emit parserkit.Emit) {
	var problems parserkit.Problems
	raw, ok := parserkit.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	if len(bytes.Trim(raw, textlines.ASCIIWhitespace)) == 0 {
		return
	}
	document, err := parserkit.ParseJSONC(artifact, raw)
	if err != nil || !document.IsObject() {
		emit(p.ProblemRow(artifact, artifact.Rel+" is not a JSON object"))
		return
	}
	timestamp := parserkit.FileMtime(artifact)
	carried := parserkit.NewCarried(artifact)
	for _, entry := range settingsEntries(document, carried) {
		text := "permission setting: " + entry.key + "=" + parserkit.SettingValue(entry.value) +
			" (from " + artifact.Rel + ", file mtime)"
		emit(p.stateRow(artifact, text, timestamp, 1, "", ""))
	}
	carried.Report(1, &problems)
	parserkit.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
}
