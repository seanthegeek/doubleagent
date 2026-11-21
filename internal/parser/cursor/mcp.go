package cursor

import (
	"cmp"
	"encoding/base64"
	"regexp"
	"slices"
	"strconv"
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
	scopes := map[string]map[string][]mcpField{}
	for _, row := range i.store.rows(substrPrefixed(i.table, mcpGlobalPrefix)) {
		scope, field, ok := mcpKeyName(row.key, mcpGlobalPrefix)
		if !ok || strings.HasPrefix(scope, "oauth-attempt:") {
			continue
		}
		if scopes[scope] == nil {
			scopes[scope] = map[string][]mcpField{}
		}
		scopes[scope][field] = append(scopes[scope][field],
			mcpField{rowid: row.rowid, value: plain(row.value), raw: asText(row.value)})
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

// mcpServer emits the "mcp server:" row of each stored server URL of one
// scope, at the scope's update time (serverTime). A field stored in more
// than one row, which only damage, an edit or a key spelled with and
// without its base64 padding writes, gives a parser: row per stored row:
// each URL gives its own row.
func (i *items) mcpServer(scope string, fields map[string][]mcpField, hasTokens bool, emit parser.Emit) {
	urls := byRowid(fields["mcp_server_url"])
	if len(urls) == 0 {
		return
	}
	updated, hasTime, group := i.serverTime(byRowid(fields["mcp_oauth_updated_at_ms"]), emit)
	source := "file mtime"
	if hasTime {
		source = "mcp_oauth_updated_at_ms"
	}
	name, _, _ := strings.Cut(scope, "::")
	for _, server := range urls {
		if len(urls) > 1 {
			emit(i.system(repeatedKeyText("ItemTable", len(urls)), i.mtime, server.rowid))
		}
		url := server.raw
		if server.value.IsString() {
			url = server.value.StringValue()
		}
		text := "mcp server: " + name + " url=" + cmp.Or(timeline.Compact(url), "-") +
			" (from state.vscdb mcpOAuth.global, " + source + ")"
		if hasTokens {
			text += mcpTokens
		}
		row := i.system(text, i.mtime, server.rowid)
		if hasTime {
			row = i.owner.row(i.artifact, timeline.TurnSystem, text, updated, server.rowid)
		}
		if group != nil {
			group.Member(&row, timeline.TakeTime)
		}
		emit(row)
	}
}

// byRowid returns the stored rows of a field in rowid order.
func byRowid(stored []mcpField) []mcpField {
	slices.SortStableFunc(stored, func(left, right mcpField) int { return cmp.Compare(left.rowid, right.rowid) })
	return stored
}

// serverTime returns the time of a scope's server rows, its
// mcp_oauth_updated_at_ms when that is a number (false when it is not,
// for the file's modification time), and the group the rows are
// members of. A field stored more than once gives a parser: row per stored
// row: one that is not a number holds no time; when two or more are, each
// is a parent of a group (timeline.Group) whose rows take the time of the
// one it goes with.
func (i *items) serverTime(updates []mcpField, emit parser.Emit) (string, bool, *timeline.Group) {
	if len(updates) <= 1 {
		if len(updates) == 1 && updates[0].value.IsNumber() {
			return timeutil.FromValue(updates[0].value), true, nil
		}
		return "", false, nil
	}
	numbers := 0
	for _, update := range updates {
		if update.value.IsNumber() {
			numbers++
		}
	}
	var group *timeline.Group
	if numbers > 1 {
		group = timeline.NewGroup()
	}
	when, first, prefix := "", true, shownOnceText("server rows are", len(updates))
	for _, update := range updates {
		if !update.value.IsNumber() {
			emit(i.system(parser.ProblemPrefix+"ItemTable holds "+strconv.Itoa(len(updates))+
				" rows of this row's key; it holds no time, so the server rows do not take it", i.mtime, update.rowid))
			continue
		}
		notice := i.owner.row(i.artifact, timeline.TurnSystem, prefix+"this row", timeutil.FromValue(update.value),
			update.rowid)
		if first {
			when, first = notice.TimestampUTC, false
		} else if group == nil {
			notice.Text = prefix + "another of them"
		}
		if group != nil {
			group.Parent(&notice).Texts(&notice, timeline.RowText{Text: prefix + "this row"},
				timeline.RowText{Text: prefix + "another of them"})
		}
		emit(notice)
	}
	if numbers == 0 {
		return "", false, nil
	}
	return when, true, group
}

// plugins emits a "plugin installed:" row per entry of each
// cursor.plugins.installedIds. list, at the file's modification time; a
// file: scope is the row's project. A value that does not decode, or is
// not a JSON array, is a problem.
func (i *items) plugins(emit parser.Emit) {
	if i.table == nil {
		return
	}
	carried := parser.NewCarried(i.artifact)
	defer carried.Report(0, i.problems)
	for _, row := range i.store.rows(substrPrefixed(i.table, pluginsPrefix)) {
		suffix := row.key[len(pluginsPrefix):]
		scope := suffix[strings.LastIndex(suffix, "|")+1:]
		// Each entry's row repeats the key's scope.
		shown := carried.Cut("plugin scope", scope)
		for _, entry := range objectEntries(row, i.problems) {
			var sources []string
			for _, source := range entry.Get("sources").Items() {
				sources = append(sources, source.Text())
			}
			made := i.system("plugin installed: id="+parser.SettingOrDash(entry.Get("id"))+" sources="+
				cmp.Or(strings.Join(sources, ","), "-")+" scope="+cmp.Or(shown, "-")+" "+pluginsSource, i.mtime, row.rowid)
			if strings.HasPrefix(scope, "file:") {
				made.ProjectPath = uriPath(scope)
			}
			emit(made)
		}
	}
}

// uriPath returns the path a file: URI names as its machine wrote it, a
// network path's leading // written \\ so the run reads it as one; the
// run gives every project path its one spelling, backslashes in a Windows
// path but in the text from a URL with a credential on, which is kept as
// written so the output still finds the credential. A URI net/url cannot
// read is kept as it is.
func uriPath(uri string) string {
	path, ok := evidencepath.FromFileURI(uri)
	if !ok {
		return uri
	}
	if rest, network := strings.CutPrefix(path, "//"); network {
		return `\\` + rest
	}
	return path
}
