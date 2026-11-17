package vscode

import (
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/antigravity"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
)

// toolConfirmKinds are the names of ToolConfirmKind, a const enum saved as
// its number (chatService.ts:870-877 at d7622a529314).
func toolConfirmKinds() []string {
	return []string{"Denied", "ConfirmationNotNeeded", "Setting", "LmServicePerTool", "UserAction", "Skipped"}
}

// editStates are the names of ModifiedFileEntryState, a const enum saved
// as its index (chatEditingService.ts:353-357 at 4d22f4f).
func editStates() []string {
	return []string{"Modified", "Accepted", "Rejected"}
}

// enumName returns the name a const enum value stands for, and false for a
// value that is not an integer in range; a boolean is not one.
func enumName(value pyjson.Value, names []string) (string, int64, bool) {
	number, ok := value.Int()
	if !ok || number < 0 || number >= int64(len(names)) {
		return "", 0, false
	}
	return names[number], number, true
}

// mdText returns the text of an IMarkdownString ({value}), a plain string
// or an array of either: "" for null and for an object whose value is not
// a string, any other value as Python's str() writes it.
func mdText(value pyjson.Value) string {
	switch {
	case value.IsNull():
		return ""
	case value.IsString():
		return value.Text()
	case value.IsObject():
		return value.Get("value").Text()
	case value.IsArray():
		texts := make([]string, 0, value.Len())
		for _, item := range value.Items() {
			texts = append(texts, mdText(item))
		}
		return strings.Join(texts, "\n")
	}
	return value.Str()
}

// uriText returns a URI as a path or URI string: a URI string as a path, a
// serialised URI's fsPath, path or external, a location's uri, and
// anything else as JSON.
func uriText(uri pyjson.Value) string {
	if uri.IsString() {
		return antigravity.URIToPath(uri.Text())
	}
	if inner, ok := uri.Lookup("uri"); ok {
		return uriText(inner)
	}
	for _, key := range []string{"fsPath", "path", "external"} {
		if value := uri.Get(key); value.IsString() {
			return value.Text()
		}
	}
	if uri.IsNull() {
		return ""
	}
	return parser.CompactJSON(uri)
}

// textOrJSON returns a string as it is and any other value as JSON.
func textOrJSON(value pyjson.Value) string {
	if value.IsString() {
		return value.Text()
	}
	return parser.CompactJSON(value)
}

// toolCallText returns a tool invocation's command line or raw input, else
// its input, else its invocation message.
func toolCallText(part pyjson.Value) string {
	specific := part.Get("toolSpecificData")
	commandLine := specific.Get("commandLine")
	if edited := commandLine.Get("userEdited"); commandLine.IsObject() && edited.Truthy() {
		return edited.Str()
	}
	if original := commandLine.Get("original"); commandLine.IsObject() && original.Truthy() {
		return original.Str()
	}
	if raw, ok := specific.Lookup("rawInput"); ok {
		return textOrJSON(raw)
	}
	if input := part.Get("resultDetails").Get("input"); input.IsString() && input.Text() != "" {
		return input.Text()
	}
	return mdText(part.Get("invocationMessage"))
}

// confirmedText returns the "permission decision:" text of a tool
// invocation's isConfirmed, else "".
func confirmedText(part pyjson.Value) string {
	confirmed := part.Get("isConfirmed")
	if confirmed.IsNull() {
		return ""
	}
	value, reason := parser.SettingValue(confirmed), ""
	var pairs []string
	if confirmed.IsObject() {
		kind := confirmed.Get("type")
		value = parser.SettingValue(kind)
		if name, number, ok := enumName(kind, toolConfirmKinds()); ok {
			value = name + "(" + strconv.FormatInt(number, 10) + ")"
		}
		for _, member := range confirmed.Members() {
			if member.Key != "type" && member.Key != "reason" && !member.Value.IsNull() {
				pairs = append(pairs, member.Key+"="+parser.SettingValue(member.Value))
			}
		}
		reason = mdText(confirmed.Get("reason"))
	}
	if source := part.Get("source"); source.IsObject() && source.Get("type").Truthy() {
		pairs = append(pairs, "tool_source="+parser.SettingValue(source.Get("type")))
	}
	text := strings.Join(append([]string{"permission decision: confirmed=" + value}, pairs...), " ")
	if reason != "" {
		text += " reason=" + reason
	}
	return text
}

// confirmationPartText returns the "permission decision:" text of a
// confirmation response part.
func confirmationPartText(part pyjson.Value) string {
	pairs := []string{"permission decision: confirmation"}
	if part.Has("isUsed") {
		pairs = append(pairs, "isUsed="+strconv.FormatBool(part.Get("isUsed").Truthy()))
	}
	if buttons := part.Get("buttons"); buttons.IsArray() {
		pairs = append(pairs, "buttons="+parser.CompactJSON(buttons))
	}
	if title := part.Get("title"); title.Truthy() {
		pairs = append(pairs, "title="+parser.SettingValue(title))
	}
	text := strings.Join(pairs, " ")
	if message := mdText(part.Get("message")); message != "" {
		text += "\n" + message
	}
	return text
}

// detailsOutput returns the text of a tool result's resultDetails.output:
// each string or each object's value (else its text) of an array, joined by
// LF, or a single value.
func detailsOutput(output pyjson.Value) string {
	if !output.IsArray() {
		if output.IsNull() {
			return ""
		}
		return textOrJSON(output)
	}
	var values []string
	for _, item := range output.Items() {
		switch {
		case item.IsObject():
			value, ok := item.Lookup("value")
			if !ok {
				value = item.Get("text")
			}
			values = append(values, textOrJSON(value))
		case item.IsString():
			values = append(values, item.Text())
		}
	}
	return strings.Join(values, "\n")
}

// toolResultText returns a tool invocation's result text, "[error] "
// first when it failed.
func toolResultText(part pyjson.Value) string {
	details := part.Get("resultDetails")
	resultError := part.Get("resultError")
	text, isError := "", resultError.Truthy()
	switch {
	case details.IsObject():
		text = detailsOutput(details.Get("output"))
		isError = isError || details.Get("isError").Truthy()
	case details.IsArray():
		texts := make([]string, 0, details.Len())
		for _, uri := range details.Items() {
			texts = append(texts, uriText(uri))
		}
		text = strings.Join(texts, "\n")
	}
	if resultError.Truthy() {
		var chunks []string
		if chunk := mdText(resultError); !resultError.IsBool() && chunk != "" {
			chunks = append(chunks, chunk)
		}
		if text != "" {
			chunks = append(chunks, text)
		}
		text = strings.Join(chunks, "\n")
	}
	if text == "" {
		text = mdText(part.Get("pastTenseMessage"))
	}
	if complete, ok := part.Get("isComplete").Bool(); text == "" && ok && !complete {
		text = "[incomplete]"
	}
	if isError {
		return "[error] " + text
	}
	return text
}

// userRel returns rel from its User/ component on, as row text names the
// file.
func userRel(rel string) string {
	if position := strings.Index(rel, "/User/"); position >= 0 {
		return rel[position+1:]
	}
	return rel
}

// editStateText returns the "edit state:" text of one snapshot entry of a
// chat editing state.json.
func editStateText(entry pyjson.Value) string {
	state := entry.Get("state")
	name, _, ok := enumName(state, editStates())
	if !ok {
		name = parser.SettingValue(state)
	}
	info := entry.Get("telemetryInfo")
	text := "edit state: " + name + " " + orDash(antigravity.URIToPath(parser.Str(entry.Get("resource")))) +
		" request=" + orDash(parser.Str(info.Get("requestId"))) + " model=" + orDash(parser.Str(info.Get("modelId")))
	if entry.Get("isDeleted").Truthy() {
		text += " deleted=true"
	}
	return text + " (from chatEditingSessions state.json, file mtime)"
}

// orDash returns text, or "-" when it is empty.
func orDash(text string) string {
	if text == "" {
		return "-"
	}
	return text
}
