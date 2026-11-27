package tabby

import (
	"strconv"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// jsonColumn returns the JSON a column holds, or false for NULL, empty
// text and text that is not JSON: a blob is decoded as UTF-8 with
// replacement first, and a number is itself.
func jsonColumn(artifact *input.Artifact, value sqlitefile.Value) (jsonvalue.Value, bool) {
	var text string
	switch value.Type {
	case sqlitefile.Null:
		return jsonvalue.Value{}, false
	case sqlitefile.Integer:
		return jsonvalue.NewInt(value.Int), true
	case sqlitefile.Float:
		return jsonvalue.NewFloat(value.Float), true
	case sqlitefile.Blob:
		text = textlines.ValidUTF8(value.Bytes)
	default:
		text = value.Text()
		if text == "" {
			return jsonvalue.Value{}, false
		}
	}
	decoded, err := parserkit.DecodeJSON(artifact, text)
	if err != nil {
		return jsonvalue.Value{}, false
	}
	return decoded, true
}

// location returns "<file>:<line>", or the file alone when the line is
// null or empty text, or "" when there is no file.
func location(filepath, startLine jsonvalue.Value) string {
	path := parserkit.Text(filepath)
	if path == "" || !parserkit.Present(startLine) {
		return path
	}
	return path + ":" + startLine.Text()
}

// attachmentParts returns short descriptions of what was attached to a
// message: code from a repository, code from the client, documents and a
// file list, with each repository and document URL.
func attachmentParts(artifact *input.Artifact, attachment jsonvalue.Value) []string {
	var parts []string
	for _, code := range attachment.Get("code").Items() {
		if !code.IsObject() {
			continue
		}
		repository := parserkit.Text(code.Get("git_url"))
		if commit := code.Get("commit"); !commit.IsEmpty() {
			repository += "@" + commit.Text()
		}
		parts = append(parts, strings.ReplaceAll(parserkit.JoinNonEmpty(" | ", "code", repository,
			location(code.Get("filepath"), code.Get("start_line"))), " | ", " "))
	}
	for _, code := range attachment.Get("client_code").Items() {
		if code.IsObject() {
			parts = append(parts, strings.TrimSpace("client_code "+
				location(code.Get("filepath"), code.Get("start_line"))))
		}
	}
	for _, doc := range attachment.Get("doc").Items() {
		if !doc.IsObject() {
			continue
		}
		label := parserkit.Text(parserkit.FirstNonEmpty(doc.Get("title"), doc.Get("sha")))
		link := parserkit.Text(parserkit.FirstNonEmpty(doc.Get("link"), doc.Get("page_link")))
		parts = append(parts, strings.ReplaceAll(parserkit.JoinNonEmpty(" | ", "doc", label, link), " | ", " "))
	}
	if files := attachment.Get("code_file_list"); files.IsObject() && files.Get("file_list").IsArray() {
		parts = append(parts, "code_file_list "+strconv.Itoa(files.Get("file_list").Len())+" files")
	}
	return parts
}

// messageAttachment returns a thread message's attachment, built from the
// separate columns of a database older than 0.25 when it has none.
func messageAttachment(artifact *input.Artifact, message sqlitefile.Row) jsonvalue.Value {
	if attachment, ok := jsonColumn(artifact, message.Get("attachment")); ok && attachment.IsObject() {
		return attachment
	}
	member := func(key, column string) jsonvalue.Member {
		value, ok := jsonColumn(artifact, message.Get(column))
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
	for _, code := range attachment.Get("code").Items() {
		if url := code.Get("git_url"); code.IsObject() && !url.IsEmpty() {
			return url.Text()
		}
	}
	return ""
}

// projectOf returns the first git URL any of the attachments names.
func projectOf(artifact *input.Artifact, attachments []jsonvalue.Value) string {
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
func commandText(artifact *input.Artifact, item jsonvalue.Value) string {
	command := item
	if item.IsString() {
		command = jsonvalue.NewNull()
		if item.StringValue() != "" {
			if decoded, err := parserkit.DecodeJSON(artifact, item.StringValue()); err == nil {
				command = decoded
			}
		}
	}
	if command.IsObject() && !command.Get("command").IsEmpty() {
		text := command.Get("command").Text()
		var files []string
		for _, context := range command.Get("context").Items() {
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
	return parserkit.JSONText(item)
}
