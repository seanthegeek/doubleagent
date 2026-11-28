package zed

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// userDenial is the text of a tool result when the user denied the call.
const userDenial = "Permission to run tool denied by user"

// newDenialPattern returns the pattern of the texts Zed's permission rules
// give a call they deny with no prompt (crates/agent/src/tool_permissions.rs
// at 9cd14da). The original is a Rust pattern, whose \S and \d are
// Unicode classes: a run of non-space excludes every character of
// parserkit.Space, and a digit is any decimal digit.
func newDenialPattern() *regexp.Regexp {
	word := `[^` + parserkit.Space + `]+`
	return regexp.MustCompile(`\A(?s:Blocked by global default: deny` +
		`|Command blocked by security rule for ` + word + ` tool` +
		`|` + word + ` tool is disabled` +
		`|Blocked by built-in security rule\..*` +
		`|The terminal command could not be approved because .*` +
		`|The ` + word + ` tool cannot run because \p{Nd}+ regex patterns? failed to compile\..*` +
		`|The .+ shell does not support "always allow" patterns .*)\z`)
}

// asList returns what a loop over content visits when a single string or
// object stands for a list of one: the items of an array, or the value
// alone.
func asList(content jsonvalue.Value) []jsonvalue.Value {
	if content.IsArray() {
		return content.Items()
	}
	return []jsonvalue.Value{content}
}

// contentText returns the text of a tool result's content: a list of
// {"Text": ...} or {"Image": ...} items, one such item, or a string.
func contentText(content jsonvalue.Value) string {
	switch {
	case content.IsNull():
		return ""
	case content.IsString():
		return content.StringValue()
	case !content.IsObject() && !content.IsArray():
		return parserkit.JSONText(content)
	}
	var parts []string
	for _, item := range asList(content) {
		switch {
		case item.IsString():
			parts = append(parts, item.StringValue())
		case !item.IsObject():
		case item.Get("Text").IsString():
			parts = append(parts, item.Get("Text").StringValue())
		case item.Has("Image"):
			parts = append(parts, "[image]")
		case item.Get("text").IsString():
			parts = append(parts, item.Get("text").StringValue())
		default:
			parts = append(parts, parserkit.JSONText(item))
		}
	}
	return strings.Join(parts, "\n")
}

// mentionText returns a Mention content item as "[mention] <uri>".
func mentionText(mention jsonvalue.Value) string {
	if !mention.IsObject() {
		return "[mention]"
	}
	return "[mention] " + parserkit.SettingValue(mention.Get("uri"))
}

// onlyKeys reports whether every key of the object is one of keys.
func onlyKeys(object jsonvalue.Value, keys ...string) bool {
	for _, key := range object.Keys() {
		found := false
		for _, allowed := range keys {
			found = found || key == allowed
		}
		if !found {
			return false
		}
	}
	return true
}

// toolInputText returns a tool use's input as text, unwrapping a
// {"type", "value"} wrapper, else its raw_input string. A null, empty
// string or empty object input counts as absent.
func toolInputText(toolUse jsonvalue.Value) string {
	toolInput := toolUse.Get("input")
	if toolInput.IsObject() && toolInput.Has("value") && onlyKeys(toolInput, "type", "value") {
		toolInput = toolInput.Get("value")
	}
	empty := !parserkit.Present(toolInput) ||
		(toolInput.IsObject() && toolInput.Len() == 0)
	if !empty {
		return parserkit.SettingValue(toolInput)
	}
	if raw := toolUse.Get("raw_input"); raw.IsString() {
		return raw.StringValue()
	}
	return ""
}

// firstFolder returns the first non-blank path of a newline-joined
// folder_paths column, else "".
func firstFolder(folderPaths sqlitefile.Value) string {
	if folderPaths.Type != sqlitefile.Text {
		return ""
	}
	for _, path := range strings.Split(folderPaths.Text(), "\n") {
		if strings.TrimSpace(path) != "" {
			return path
		}
	}
	return ""
}

// acpBlockText returns the text of one ACP content block.
func acpBlockText(block jsonvalue.Value) string {
	kind := block.Get("type")
	switch {
	case block.IsString():
		return block.StringValue()
	case parserkit.IsText(kind, "text") && block.Get("text").IsString():
		return block.Get("text").StringValue()
	case parserkit.IsText(kind, "resource_link") && block.Get("uri").IsString():
		return "[mention] " + block.Get("uri").StringValue()
	case parserkit.IsText(kind, "resource") && block.Get("resource").IsObject():
		return "[mention] " + parserkit.SettingValue(block.Get("resource").Get("uri"))
	case parserkit.IsText(kind, "image"), parserkit.IsText(kind, "audio"):
		return "[" + kind.StringValue() + "]"
	}
	return parserkit.JSONText(block)
}

// acpBlocksText returns the text of a draft prompt's ACP content blocks:
// a text block's text, a resource link's or resource's URI as a mention,
// a marker for an image or audio block, and anything else as CompactJSON.
func acpBlocksText(blocks jsonvalue.Value) string {
	if !blocks.IsObject() && !blocks.IsArray() {
		return parserkit.JSONText(blocks)
	}
	var parts []string
	for _, block := range asList(blocks) {
		parts = append(parts, acpBlockText(block))
	}
	return strings.Join(parts, "\n")
}

// denialText returns the `permission decision:` text of a failed tool
// result Zed wrote for a denial, and "" for any other result: an approval
// leaves no record.
func (p *Parser) denialText(result jsonvalue.Value) string {
	if result.Get("is_error").IsEmpty() {
		return ""
	}
	text := strings.TrimSpace(contentText(result.Get("content")))
	switch {
	case text == userDenial:
		return "permission decision: denied"
	case p.denial.MatchString(text):
		return "permission decision: auto_denied reason=" + text
	}
	return ""
}

// threadKey returns a sidebar thread_id, 16 raw UUID bytes, as the
// hyphenated UUID the key-value store keys drafts by; any other value as
// its text, "" when it is empty.
func threadKey(value sqlitefile.Value) string {
	if value.Type == sqlitefile.Blob && len(value.Bytes) == 16 {
		raw := value.Bytes
		return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16])
	}
	return parserkit.NonEmptyColumn(value)
}

// firstColumn returns the first of the values that is not empty, as
// parserkit.NonEmptyColumn gives it, or "".
func firstColumn(values ...sqlitefile.Value) string {
	for _, value := range values {
		if !value.IsEmpty() {
			return value.String()
		}
	}
	return ""
}
