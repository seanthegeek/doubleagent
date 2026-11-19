package cursor

import (
	"cmp"
	"encoding/base64"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/evidencepath"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// The MCP OAuth and plugin keys. An MCP OAuth key's suffix is URL-safe
// base64 of "[<scope>] <field>"; the mcpOAuth.secret. values are
// ciphertext and are never selected, only their key names.
const (
	mcpGlobalPrefix = "mcpOAuth.global."
	mcpSecretPrefix = "mcpOAuth.secret."
	mcpTokens       = " tokens=stored (ciphertext, not read)"
	pluginsPrefix   = "cursor.plugins.installedIds."
	pluginsSource   = "(from state.vscdb cursor.plugins.installedIds, file mtime)"
)

// mcpKeyPattern matches a decoded MCP OAuth key name, "[<scope>] <field>".
var mcpKeyPattern = regexp.MustCompile(`^\[(.+)\] ([^` + parser.Space + `]+)\z`)

// mcpKeyName returns the scope and field an MCP OAuth key names, or false.
// The suffix is URL-safe base64 without padding (research/cursor.md); a
// padded suffix is read too. The name must be UTF-8.
func mcpKeyName(key, prefix string) (string, string, bool) {
	decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(key[len(prefix):], "="))
	if err != nil || !utf8.Valid(decoded) {
		return "", "", false
	}
	match := mcpKeyPattern.FindStringSubmatch(string(decoded))
	if match == nil {
		return "", "", false
	}
	return match[1], match[2], true
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
	for _, row := range i.store.rows(keyNames(i.table, func(row sqlitefile.Row) bool {
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
	text := "mcp server: " + name + " url=" + cmp.Or(timeline.Compact(url), "-") + " (from state.vscdb mcpOAuth.global, " +
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
// file: scope is the row's project. A value that does not decode, or is
// not a JSON array, is a problem.
func (i *items) plugins(emit parser.Emit) {
	if i.table == nil {
		return
	}
	for _, row := range i.store.rows(substrPrefixed(i.table, pluginsPrefix)) {
		suffix := row.key[len(pluginsPrefix):]
		scope := suffix[strings.LastIndex(suffix, "|")+1:]
		for _, entry := range objectEntries(row, i.problems) {
			var sources []string
			for _, source := range entry.Get("sources").Items() {
				sources = append(sources, source.Text())
			}
			made := i.system("plugin installed: id="+parser.SettingOrDash(entry.Get("id"))+" sources="+
				cmp.Or(strings.Join(sources, ","), "-")+" scope="+cmp.Or(scope, "-")+" "+pluginsSource, i.mtime, row.rowid)
			if strings.HasPrefix(scope, "file:") {
				made.ProjectPath = uriPath(scope)
			}
			emit(made)
		}
	}
}

// uriPath returns the path a file: URI names, a Windows drive path and a
// network path in the c:\Users\... and \\server\share\... forms fsPath
// writes; a URI net/url cannot read is kept as it is.
func uriPath(uri string) string {
	path, ok := evidencepath.FromFileURI(uri)
	switch {
	case !ok:
		return uri
	case evidencepath.IsDrivePath(path), strings.HasPrefix(path, "//"):
		return strings.ReplaceAll(path, "/", "\\")
	}
	return path
}
