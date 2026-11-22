package vscode

import (
	"cmp"
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/evidencepath"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
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
func enumName(value jsonvalue.Value, names []string) (string, int64, bool) {
	number, ok := value.Int()
	if !ok || number < 0 || number >= int64(len(names)) {
		return "", 0, false
	}
	return names[number], number, true
}

// mdText returns the text of an IMarkdownString ({value}), a plain string
// or an array of either: "" for null and for an object whose value is not
// a string, any other value as its row text (jsonvalue.Value.Text). An
// array's text is its items' texts, one per line, an item that is an array
// giving its own; the leaves are gathered first and joined once, so text
// inside many levels of arrays is copied once, not once per level.
func mdText(value jsonvalue.Value) string {
	if !value.IsArray() {
		return mdLeafText(value)
	}
	return strings.Join(mdLeaves(value, nil), "\n")
}

// mdLeaves appends to texts the text of each item of array that is not an
// array, in order, and the leaves of each item that is; an empty array
// stands as one "" in its place.
func mdLeaves(array jsonvalue.Value, texts []string) []string {
	if array.Len() == 0 {
		return append(texts, "")
	}
	for _, item := range array.Items() {
		if item.IsArray() {
			texts = mdLeaves(item, texts)
		} else {
			texts = append(texts, mdLeafText(item))
		}
	}
	return texts
}

// mdLeafText returns the text of a value of an IMarkdownString that is not
// an array.
func mdLeafText(value jsonvalue.Value) string {
	switch {
	case value.IsNull():
		return ""
	case value.IsString():
		return value.StringValue()
	case value.IsObject():
		return value.Get("value").StringValue()
	}
	return value.Text()
}

// uriText returns a URI as a path or URI string: a URI string as a path, a
// serialised URI's fsPath, path or external, a location's uri, and
// anything else as JSON.
func uriText(uri jsonvalue.Value) string {
	if uri.IsString() {
		return uriToPath(uri.StringValue())
	}
	if inner, ok := uri.Lookup("uri"); ok {
		return uriText(inner)
	}
	for _, key := range []string{"fsPath", "path", "external"} {
		if value := uri.Get(key); value.IsString() {
			return value.StringValue()
		}
	}
	if uri.IsNull() {
		return ""
	}
	return parser.JSONText(uri)
}

// uriToPath returns the path a file: URI names (evidencepath.FromFileURI),
// and any other text, a path already or a URI net/url cannot read, as it
// is.
func uriToPath(uri string) string {
	if path, ok := evidencepath.FromFileURI(uri); ok {
		return path
	}
	return uri
}

// toolCallText returns a tool invocation's command line or raw input, else
// its input, else its invocation message.
func toolCallText(part jsonvalue.Value) string {
	specific := part.Get("toolSpecificData")
	commandLine := specific.Get("commandLine")
	if edited := commandLine.Get("userEdited"); commandLine.IsObject() && !edited.IsEmpty() {
		return edited.Text()
	}
	if original := commandLine.Get("original"); commandLine.IsObject() && !original.IsEmpty() {
		return original.Text()
	}
	if raw, ok := specific.Lookup("rawInput"); ok {
		return parser.SettingValue(raw)
	}
	if input := part.Get("resultDetails").Get("input"); input.IsString() && input.StringValue() != "" {
		return input.StringValue()
	}
	return mdText(part.Get("invocationMessage"))
}

// confirmedText returns the "permission decision:" text of a tool
// invocation's isConfirmed, else "".
func confirmedText(part jsonvalue.Value) string {
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
	if source := part.Get("source"); source.IsObject() && !source.Get("type").IsEmpty() {
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
func confirmationPartText(part jsonvalue.Value) string {
	pairs := []string{"permission decision: confirmation"}
	if part.Has("isUsed") {
		pairs = append(pairs, "isUsed="+strconv.FormatBool(!part.Get("isUsed").IsEmpty()))
	}
	if buttons := part.Get("buttons"); buttons.IsArray() {
		pairs = append(pairs, "buttons="+parser.JSONText(buttons))
	}
	if title := part.Get("title"); !title.IsEmpty() {
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
func detailsOutput(output jsonvalue.Value) string {
	if !output.IsArray() {
		if output.IsNull() {
			return ""
		}
		return parser.SettingValue(output)
	}
	var values []string
	for _, item := range output.Items() {
		switch {
		case item.IsObject():
			value, ok := item.Lookup("value")
			if !ok {
				value = item.Get("text")
			}
			values = append(values, parser.SettingValue(value))
		case item.IsString():
			values = append(values, item.StringValue())
		}
	}
	return strings.Join(values, "\n")
}

// toolResultText returns a tool invocation's result text, "[error] "
// first when it failed.
func toolResultText(part jsonvalue.Value) string {
	details := part.Get("resultDetails")
	resultError := part.Get("resultError")
	text, isError := "", !resultError.IsEmpty()
	switch {
	case details.IsObject():
		text = detailsOutput(details.Get("output"))
		isError = isError || !details.Get("isError").IsEmpty()
	case details.IsArray():
		texts := make([]string, 0, details.Len())
		for _, uri := range details.Items() {
			texts = append(texts, uriText(uri))
		}
		text = strings.Join(texts, "\n")
	}
	if !resultError.IsEmpty() {
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
func editStateText(entry jsonvalue.Value) string {
	state := entry.Get("state")
	name, _, ok := enumName(state, editStates())
	if !ok {
		name = parser.SettingValue(state)
	}
	info := entry.Get("telemetryInfo")
	text := "edit state: " + name + " " + cmp.Or(uriToPath(parser.Text(entry.Get("resource"))), "-") +
		" request=" + cmp.Or(parser.Text(info.Get("requestId")), "-") + " model=" + cmp.Or(parser.Text(info.Get("modelId")), "-")
	if !entry.Get("isDeleted").IsEmpty() {
		text += " deleted=true"
	}
	return text + " (from chatEditingSessions state.json, file mtime)"
}
