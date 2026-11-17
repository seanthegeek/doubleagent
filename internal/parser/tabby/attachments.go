package tabby

import (
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
	"github.com/seanthegeek/doubleagent/internal/pytext"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// jsonColumn returns the JSON a column holds, or false for NULL, empty
// text and text that is not JSON: a blob is decoded as UTF-8 with
// replacement first, and a number is itself.
func jsonColumn(value sqlitefile.Value) (pyjson.Value, bool) {
	var text string
	switch value.Type {
	case sqlitefile.Null:
		return pyjson.Value{}, false
	case sqlitefile.Integer:
		return pyjson.NewInt(value.Int), true
	case sqlitefile.Float:
		return pyjson.NewFloat(value.Float), true
	case sqlitefile.Blob:
		text = pytext.DecodeUTF8(value.Bytes)
	default:
		text = value.Text()
		if text == "" {
			return pyjson.Value{}, false
		}
	}
	decoded, err := pyjson.DecodeString(text)
	if err != nil {
		return pyjson.Value{}, false
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
func arrayItems(value pyjson.Value) []pyjson.Value {
	if value.IsArray() {
		return value.Items()
	}
	return nil
}

// location returns "<file>:<line>", or the file alone when the line is
// null or empty text, or "" when there is no file.
func location(filepath, startLine pyjson.Value) string {
	path := parser.Str(filepath)
	if path == "" || startLine.IsNull() || startLine.IsString() && startLine.Text() == "" {
		return path
	}
	return path + ":" + startLine.Str()
}

// attachmentParts returns short descriptions of what was attached to a
// message: code from a repository, code from the client, documents and a
// file list.
func attachmentParts(attachment pyjson.Value) []string {
	var parts []string
	for _, code := range arrayItems(attachment.Get("code")) {
		if !code.IsObject() {
			continue
		}
		repository := parser.Str(code.Get("git_url"))
		if commit := code.Get("commit"); commit.Truthy() {
			repository += "@" + commit.Str()
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
func firstTruthy(object pyjson.Value, first, second string) string {
	if value := object.Get(first); value.Truthy() {
		return value.Str()
	}
	return parser.Str(object.Get(second))
}

// messageAttachment returns a thread message's attachment, built from the
// separate columns of a database older than 0.25 when it has none.
func messageAttachment(message sqlitefile.Row) pyjson.Value {
	if attachment, ok := jsonColumn(message.Get("attachment")); ok && attachment.IsObject() {
		return attachment
	}
	member := func(key, column string) pyjson.Member {
		value, ok := jsonColumn(message.Get(column))
		if !ok {
			value = pyjson.NewNull()
		}
		return pyjson.Member{Key: key, Value: value}
	}
	return pyjson.NewObject(member("code", "code_attachments"),
		member("client_code", "client_code_attachments"), member("doc", "doc_attachments"))
}

// firstGitURL returns the first code[].git_url of an attachment, else "".
func firstGitURL(attachment pyjson.Value) string {
	for _, code := range arrayItems(attachment.Get("code")) {
		if url := code.Get("git_url"); code.IsObject() && url.Truthy() {
			return url.Str()
		}
	}
	return ""
}

// projectOf returns the first git URL any of the attachments names.
func projectOf(attachments []pyjson.Value) string {
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
func commandText(item pyjson.Value) string {
	command := item
	if item.IsString() {
		command = pyjson.NewNull()
		if item.Text() != "" {
			if decoded, err := pyjson.DecodeString(item.Text()); err == nil {
				command = decoded
			}
		}
	}
	if command.IsObject() && command.Get("command").Truthy() {
		text := command.Get("command").Str()
		var files []string
		for _, context := range arrayItems(command.Get("context")) {
			if uri := context.Get("uri"); context.IsObject() && uri.Truthy() {
				files = append(files, uri.Str())
			}
		}
		if len(files) > 0 {
			return text + " [context: " + strings.Join(files, "; ") + "]"
		}
		return text
	}
	switch {
	case item.IsNull(), item.IsString() && item.Text() == "":
		return ""
	case item.IsString():
		return item.Text()
	}
	return parser.CompactJSON(item)
}
