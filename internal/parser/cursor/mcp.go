package cursor

import (
	"encoding/base64"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/parser/antigravity"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The MCP OAuth and plugin keys. An MCP OAuth key's suffix is URL-safe
// base64 of "[<scope>] <field>"; the mcpOAuth.secret. values are
// ciphertext and are never read, only their key names.
const (
	mcpGlobalPrefix = "mcpOAuth.global."
	mcpSecretPrefix = "mcpOAuth.secret."
	mcpTokens       = " tokens=stored (ciphertext, not read)"
	pluginsPrefix   = "cursor.plugins.installedIds."
	pluginsSource   = "(from state.vscdb cursor.plugins.installedIds, file mtime)"
)

// mcpKeyPattern matches a decoded MCP OAuth key name, "[<scope>] <field>";
// Python's $ also takes a final LF.
var mcpKeyPattern = regexp.MustCompile(`^\[(.+)\] ([^` + parser.PythonSpace + `]+)\n?\z`)

// base64Alphabet holds the characters a standard or URL-safe base64 text
// is made of, padding aside.
const base64Alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/-_"

// mcpKeyName returns the scope and field an MCP OAuth key names, or false.
// The suffix is decoded as Python's urlsafe_b64decode does: padded to a
// multiple of four, characters outside the alphabet dropped, the data
// characters needing the padding after them that completes their last
// group; then it is read as UTF-8.
func mcpKeyName(key, prefix string) (string, string, bool) {
	decoded, ok := lenientBase64(key[len(prefix):])
	if !ok || !utf8.Valid(decoded) {
		return "", "", false
	}
	match := mcpKeyPattern.FindStringSubmatch(string(decoded))
	if match == nil {
		return "", "", false
	}
	return match[1], match[2], true
}

// lenientBase64 decodes URL-safe base64 text as Python's urlsafe_b64decode
// does without validation, or returns false.
func lenientBase64(text string) ([]byte, bool) {
	var data strings.Builder
	pads := 0
	for i := range len(text) {
		switch char := text[i]; {
		case char >= utf8.RuneSelf:
			return nil, false
		case strings.IndexByte(base64Alphabet, char) >= 0:
			data.WriteByte(char)
			pads = 0
		case char == '=':
			pads++
		}
	}
	pads += (4 - len(text)%4) % 4
	count := data.Len() % 4
	if count == 1 || (count > 1 && pads < 4-count) {
		return nil, false
	}
	plain := strings.NewReplacer("-", "+", "_", "/").Replace(data.String())
	decoded, err := base64.RawStdEncoding.DecodeString(plain)
	return decoded, err == nil
}

// mcpField is one field of an MCP OAuth scope: its row and value.
type mcpField struct {
	rowid int64
	value jsonvalue.Value
	raw   string
}

// mcp emits an "mcp server:" row per MCP OAuth scope with an
// mcp_server_url, at its mcp_oauth_updated_at_ms, else the file's
// modification time, saying when an mcpOAuth.secret. key holds its tokens.
// The oauth-attempt: scopes are skipped.
func (i *items) mcp(emit parser.Emit) {
	if i.table == nil {
		return
	}
	tokens := map[string]bool{}
	for _, row := range i.store.rows(keyedRows(i.table, func(row sqlitefile.Row) bool {
		return hasTextPrefix(row.Get("key"), mcpSecretPrefix)
	})) {
		if scope, field, ok := mcpKeyName(row.key, mcpSecretPrefix); ok && field == "mcp_tokens" {
			tokens[scope] = true
		}
	}
	scopes := map[string]map[string]mcpField{}
	for _, row := range i.store.rows(substrPrefixed(i.table, mcpGlobalPrefix)) {
		scope, field, ok := mcpKeyName(row.key, mcpGlobalPrefix)
		if !ok || strings.HasPrefix(scope, "oauth-attempt:") {
			continue
		}
		if scopes[scope] == nil {
			scopes[scope] = map[string]mcpField{}
		}
		scopes[scope][field] = mcpField{rowid: row.rowid, value: plain(row.value), raw: asText(row.value)}
	}
	names := make([]string, 0, len(scopes))
	for name := range scopes {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, scope := range names {
		i.mcpServer(scope, scopes[scope], tokens[scope], emit)
	}
}

// mcpServer emits the "mcp server:" row of one scope with a server URL.
func (i *items) mcpServer(scope string, fields map[string]mcpField, hasTokens bool, emit parser.Emit) {
	server, ok := fields["mcp_server_url"]
	if !ok {
		return
	}
	url := server.raw
	if server.value.IsString() {
		url = server.value.StringValue()
	}
	updated := fields["mcp_oauth_updated_at_ms"].value
	hasTime := updated.IsNumber()
	source := "file mtime"
	if hasTime {
		source = "mcp_oauth_updated_at_ms"
	}
	name, _, _ := strings.Cut(scope, "::")
	text := "mcp server: " + name + " url=" + orDash(timeline.Compact(url)) + " (from state.vscdb mcpOAuth.global, " +
		source + ")"
	if hasTokens {
		text += mcpTokens
	}
	if hasTime {
		emit(i.owner.row(i.artifact, timeline.TurnSystem, text, timeutil.FromValue(updated), server.rowid))
		return
	}
	emit(i.system(text, i.mtime, server.rowid))
}

// plugins emits a "plugin installed:" row per entry of each
// cursor.plugins.installedIds. list, at the file's modification time; a
// file: scope is the row's project. A value that is not a JSON list is
// skipped, one that starts as a list and does not decode is a problem.
func (i *items) plugins(emit parser.Emit) {
	if i.table == nil {
		return
	}
	for _, row := range i.store.rows(substrPrefixed(i.table, pluginsPrefix)) {
		suffix := row.key[len(pluginsPrefix):]
		scope := suffix[strings.LastIndex(suffix, "|")+1:]
		if !lstripsToList(asText(row.value)) {
			continue
		}
		for _, entry := range decodeRecord(row.rowid, row.key, row.value, i.problems).Items() {
			if !entry.IsObject() {
				continue
			}
			var sources []string
			for _, source := range entry.Get("sources").Items() {
				sources = append(sources, source.Text())
			}
			made := i.system("plugin installed: id="+shown(entry.Get("id"))+" sources="+
				orDash(strings.Join(sources, ","))+" scope="+orDash(scope)+" "+pluginsSource, i.mtime, row.rowid)
			if strings.HasPrefix(scope, "file:") {
				made.ProjectPath = uriPath(scope)
			}
			emit(made)
		}
	}
}

// uriPath returns the percent-decoded path of a file: URI, a Windows drive
// path in the c:\Users\... form fsPath writes.
func uriPath(uri string) string {
	path := antigravity.URIPath(uri)
	if antigravity.IsDrivePath(path) {
		return strings.ReplaceAll(path[1:], "/", "\\")
	}
	return path
}
