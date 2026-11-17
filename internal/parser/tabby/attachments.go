package tabby

import (
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pytext"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// jsonColumn returns the JSON a column holds, or false for NULL, empty
// text and text that is not JSON: a blob is decoded as UTF-8 with
// replacement first, and a number is itself.
func jsonColumn(value sqlitefile.Value) (jsonvalue.Value, bool) {
	var text string
	switch value.Type {
	case sqlitefile.Null:
		return jsonvalue.Value{}, false
	case sqlitefile.Integer:
		return jsonvalue.NewInt(value.Int), true
	case sqlitefile.Float:
		return jsonvalue.NewFloat(value.Float), true
	case sqlitefile.Blob:
		text = pytext.DecodeUTF8(value.Bytes)
	default:
		text = value.Text()
		if text == "" {
			return jsonvalue.Value{}, false
		}
	}
	decoded, err := jsonvalue.DecodeString(text)
	if err != nil {
		return jsonvalue.Value{}, false
	}
	return decoded, true
}

// joinParts joins the non-empty parts with " | ".
func joinParts(parts ...string) string {
	var kept []string
	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, " | ")
}

// arrayItems returns the items of an array, nothing for any other value.
func arrayItems(value jsonvalue.Value) []jsonvalue.Value {
	if value.IsArray() {
		return value.Items()
	}
	return nil
}

// location returns "<file>:<line>", or the file alone when the line is
// null or empty text, or "" when there is no file.
func location(filepath, startLine jsonvalue.Value) string {
	path := parser.Str(filepath)
	if path == "" || startLine.IsNull() || startLine.IsString() && startLine.StringValue() == "" {
		return path
	}
	return path + ":" + startLine.Text()
}

// attachmentParts returns short descriptions of what was attached to a
// message: code from a repository, code from the client, documents and a
// file list.
func attachmentParts(attachment jsonvalue.Value) []string {
	var parts []string
	for _, code := range arrayItems(attachment.Get("code")) {
		if !code.IsObject() {
			continue
		}
		repository := parser.Str(code.Get("git_url"))
		if commit := code.Get("commit"); !commit.IsEmpty() {
			repository += "@" + commit.Text()
		}
		parts = append(parts, strings.ReplaceAll(joinParts("code", repository,
			location(code.Get("filepath"), code.Get("start_line"))), " | ", " "))
	}
	for _, code := range arrayItems(attachment.Get("client_code")) {
		if code.IsObject() {
			parts = append(parts, pytext.Strip("client_code "+
				location(code.Get("filepath"), code.Get("start_line"))))
		}
	}
	for _, doc := range arrayItems(attachment.Get("doc")) {
		if !doc.IsObject() {
			continue
		}
		label := firstTruthy(doc, "title", "sha")
		link := firstTruthy(doc, "link", "page_link")
		parts = append(parts, strings.ReplaceAll(joinParts("doc", label, link), " | ", " "))
	}
	if files := attachment.Get("code_file_list"); files.IsObject() && files.Get("file_list").IsArray() {
		parts = append(parts, "code_file_list "+strconv.Itoa(files.Get("file_list").Len())+" files")
	}
	return parts
}

// firstTruthy returns str(object[first] or object[second] or "").
func firstTruthy(object jsonvalue.Value, first, second string) string {
	if value := object.Get(first); !value.IsEmpty() {
		return value.Text()
	}
	return parser.Str(object.Get(second))
}

// messageAttachment returns a thread message's attachment, built from the
// separate columns of a database older than 0.25 when it has none.
func messageAttachment(message sqlitefile.Row) jsonvalue.Value {
	if attachment, ok := jsonColumn(message.Get("attachment")); ok && attachment.IsObject() {
		return attachment
	}
	member := func(key, column string) jsonvalue.Member {
		value, ok := jsonColumn(message.Get(column))
		if !ok {
			value = jsonvalue.NewNull()
		}
		return jsonvalue.Member{Key: key, Value: value}
	}
	return jsonvalue.NewObject(member("code", "code_attachments"),
		member("client_code", "client_code_attachments"), member("doc", "doc_attachments"))
}

// firstGitURL returns the first code[].git_url of an attachment, else "".
func firstGitURL(attachment jsonvalue.Value) string {
	for _, code := range arrayItems(attachment.Get("code")) {
		if url := code.Get("git_url"); code.IsObject() && !url.IsEmpty() {
			return url.Text()
		}
	}
	return ""
}

// projectOf returns the first git URL any of the attachments names.
func projectOf(attachments []jsonvalue.Value) string {
	for _, attachment := range attachments {
		if url := firstGitURL(attachment); url != "" {
			return url
		}
	}
	return ""
}

// commandText returns one edit.recentlyCommand entry of the VS Code
// extension as row text, its context files' URIs appended. An entry is the
// JSON of {command, context} or a bare command string.
func commandText(item jsonvalue.Value) string {
	command := item
	if item.IsString() {
		command = jsonvalue.NewNull()
		if item.StringValue() != "" {
			if decoded, err := jsonvalue.DecodeString(item.StringValue()); err == nil {
				command = decoded
			}
		}
	}
	if command.IsObject() && !command.Get("command").IsEmpty() {
		text := command.Get("command").Text()
		var files []string
		for _, context := range arrayItems(command.Get("context")) {
			if uri := context.Get("uri"); context.IsObject() && !uri.IsEmpty() {
				files = append(files, uri.Text())
			}
		}
		if len(files) > 0 {
			return text + " [context: " + strings.Join(files, "; ") + "]"
		}
		return text
	}
	switch {
	case item.IsNull(), item.IsString() && item.StringValue() == "":
		return ""
	case item.IsString():
		return item.StringValue()
	}
	return parser.CompactJSON(item)
}
