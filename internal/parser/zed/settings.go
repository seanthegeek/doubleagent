package zed

import (
	"bytes"
	"fmt"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
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

// parseSettings emits one `permission setting:` row per standing permission
// of a settings file, at the file's modification time.
func (p *Parser) parseSettings(artifact *input.Artifact, emit parser.Emit) {
	var problems parser.Problems
	raw, ok := parser.FileBytes(artifact.DiskPath, &problems)
	if !ok {
		parser.EmitAll(emit, p.ErrorRows(artifact, problems, "", ""))
		return
	}
	if len(bytes.Trim(raw, " \t\n\r\v\f")) == 0 {
		return
	}
	document, err := parser.ParseJSONC(raw)
	if err != nil || !document.IsObject() {
		emit(p.ProblemRow(artifact, artifact.Rel+" is not a JSON object"))
		return
	}
	timestamp := parser.FileMtime(artifact)
	for _, entry := range settingsEntries(document) {
		text := fmt.Sprintf("permission setting: %s=%s (from %s, file mtime)", entry.key,
			parser.SettingValue(entry.value), artifact.Rel)
		emit(p.stateRow(artifact, text, timestamp, 1, "", ""))
	}
}

// settingsEntries returns the standing permission entries of a settings
// object: those of agent.tool_permissions, agent.sandbox_permissions, the
// legacy agent.always_allow_tool_actions, agent_servers.<id>.default_mode
// and session.trust_all_worktrees, in the top level, each override block
// and each profiles.<name>.settings, the key prefixed with the block's.
func settingsEntries(document jsonvalue.Value) []setting {
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
				layers = append(layers, layer{"profiles." + member.Key + ".settings.", settings})
			}
		}
	}
	var entries []setting
	for _, each := range layers {
		entries = append(entries, blockEntries(each.prefix, each.block)...)
	}
	return entries
}

// blockEntries returns the standing permission entries of one settings
// block, each key prefixed with prefix.
func blockEntries(prefix string, block jsonvalue.Value) []setting {
	var entries []setting
	if agent := block.Get("agent"); agent.IsObject() {
		for _, section := range []string{"tool_permissions", "sandbox_permissions"} {
			if value, ok := agent.Lookup(section); ok {
				entries = flatten(entries, prefix+"agent."+section, value)
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

// flatten appends one entry per leaf of a settings value: an object's
// members under dotted keys, a list's items each under the list's key.
func flatten(entries []setting, key string, value jsonvalue.Value) []setting {
	switch {
	case value.IsObject():
		for _, member := range value.Members() {
			entries = flatten(entries, key+"."+member.Key, member.Value)
		}
	case value.IsArray():
		for _, item := range value.Items() {
			entries = append(entries, setting{key, item})
		}
	default:
		entries = append(entries, setting{key, value})
	}
	return entries
}
