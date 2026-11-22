package openclaw

import (
	"cmp"
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// notPlatforms are the channels that are not a messaging platform: the
// Control UI and the internal wake sources.
func notPlatforms() []string {
	return []string{"webchat", "heartbeat", "cron", "webhook", "voice", "sessions_send"}
}

// isPlatform reports whether a channel id names a messaging platform.
func isPlatform(channel string) bool {
	return channel != "" && !slices.Contains(notPlatforms(), channel)
}

// sessionPermissionKeys are the per-session permission state of
// session_nodes.entry_json.
func sessionPermissionKeys() []string {
	return []string{"elevatedLevel", "execHost", "execNode", "execCwd", "toolOverrides", "sendPolicy"}
}

// node is one session_nodes row with its entry_json decoded.
type node struct {
	rowid            int
	sessionKey       sqlitefile.Value
	currentSessionID sqlitefile.Value
	updatedAt        string
	entry            jsonvalue.Value
}

// nodes returns the session_nodes rows in rowid order. An entry_json that
// does not decode adds a problem and its row is left out.
func (a *agentDatabase) nodes() ([]node, error) {
	rows, err := a.table("session_nodes", func(table *sqlitefile.Table) sqlitefile.Query {
		return table.Select("session_key", "current_session_id", "entry_json", "updated_at")
	})
	var nodes []node
	for _, row := range rows {
		raw := parser.TextColumn(row.Get("entry_json"))
		entry, decodeErr := parser.DecodeJSON(a.artifact, raw)
		if decodeErr != nil {
			a.problems.AddRaw(int(row.Rowid), "session_nodes entry_json unparseable ("+
				decodeErr.Error()+")", raw)
			continue
		}
		if !entry.IsObject() {
			entry = jsonvalue.NewObject()
		}
		nodes = append(nodes, node{rowid: int(row.Rowid), sessionKey: row.Get("session_key"),
			currentSessionID: row.Get("current_session_id"),
			updatedAt:        parser.ColumnTime(row.Get("updated_at")), entry: entry})
	}
	return nodes, err
}

// attachNodes attaches the rows of each session's metadata entry to its
// current generation: the public share (to the generation it publishes),
// the external delivery route when no conversation or window recorded the
// relay, the model override, the per-session permission settings and the
// archive marker. The rows of a key stored more than once wait for its
// group instead (keyRepeat).
func (a *agentDatabase) attachNodes(nodes []node, relayed map[string]bool) {
	a.repeatKeys(nodes)
	for i, each := range nodes {
		attach := a.store.attach
		if repeat := a.keys[each.sessionKey.String()]; repeat != nil {
			attach = repeat.waiting(i)
		}
		nodeRows(each, relayed, attach)
	}
}

// nodeRows gives attach the rows of one session_nodes row's entry.
func nodeRows(each node, relayed map[string]bool, attach func(sessionID string, line int, timestamp, text,
	model string)) {
	entry, line, timestamp := each.entry, each.rowid, each.updatedAt
	sessionID := parser.NonEmptyColumn(each.currentSessionID)
	if grant := entry.Get("publicShare"); grant.IsObject() && !grant.Get("id").IsEmpty() {
		generation := parser.Text(grant.Get("sessionId"))
		attach(cmp.Or(generation, sessionID), line,
			timeutil.FromValue(grant.Get("createdAt")), "session synced: public share "+
				cmp.Or(parser.Text(grant.Get("id")), "-")+" generation "+cmp.Or(generation, "-")+
				" (from openclaw-agent.sqlite session_nodes.entry_json, publicShare.createdAt)", "")
	}
	if relay := deliveryRelay(entry.Get("delivery")); relay != "" && !relayed[sessionID] {
		attach(sessionID, line, timestamp, relay, "")
	}
	if override := entry.Get("modelOverride"); override.IsString() && override.StringValue() != "" {
		attach(sessionID, line, timestamp, overrideText(entry), override.StringValue())
	}
	for _, key := range sessionPermissionKeys() {
		if value := entry.Get(key); !isEmptySetting(value) {
			attach(sessionID, line, timestamp, "permission setting: "+key+"="+
				parser.SettingValue(value)+
				" (from openclaw-agent.sqlite session_nodes.entry_json, updated_at)", "")
		}
	}
	if archived, ok := entry.Lookup("archivedAt"); ok && !archived.IsNull() {
		attach(sessionID, line, timeutil.FromValue(archived), archiveText(entry), "")
	}
}

// isEmptySetting reports whether a permission setting is absent: null, an
// empty string, an empty object or an empty list.
func isEmptySetting(value jsonvalue.Value) bool {
	return !parser.Present(value) ||
		((value.IsObject() || value.IsArray()) && value.Len() == 0)
}

// archiveText returns the `session archived:` text of a session's entry,
// naming the reason and who archived it.
func archiveText(entry jsonvalue.Value) string {
	archivedBy := entry.Get("archivedBy")
	var actor []string
	for _, field := range []string{"type", "id"} {
		if value := archivedBy.Get(field); !value.IsEmpty() {
			actor = append(actor, value.Text())
		}
	}
	return "session archived: " + cmp.Or(parser.Text(entry.Get("archiveReason")), "-") + " by " +
		cmp.Or(strings.Join(actor, ":"), "-") +
		" (from openclaw-agent.sqlite session_nodes.entry_json, archivedAt)"
}

// deliveryRelay returns the `session relayed:` text of an external
// entry_json.delivery, else "": its kind is external and its route's
// channel a platform (src/config/sessions/types.ts:60-83; route is a
// ChannelRouteRef, src/plugin-sdk/channel-route.ts:18-25).
func deliveryRelay(delivery jsonvalue.Value) string {
	if !parser.IsText(delivery.Get("kind"), "external") {
		return ""
	}
	route, origin := delivery.Get("route"), delivery.Get("origin")
	channel := route.Get("channel")
	if !channel.IsString() || !isPlatform(channel.StringValue()) {
		return ""
	}
	field := func(value jsonvalue.Value) string { return cmp.Or(parser.Text(value), "-") }
	return "session relayed: " + channel.StringValue() + " account " +
		cmp.Or(firstStr(route.Get("accountId"), origin.Get("accountId")), "-") + " " +
		field(origin.Get("chatType")) + " from " + field(origin.Get("from")) + " to " +
		field(origin.Get("to")) +
		" (from openclaw-agent.sqlite session_nodes.entry_json delivery, updated_at)"
}

// firstStr returns the row text of the first value that is not empty, or
// "".
func firstStr(values ...jsonvalue.Value) string {
	return parser.Text(parser.FirstNonEmpty(values...))
}

// overrideText returns the `model:` text of a session's modelOverride
// (types.ts:502-516).
func overrideText(entry jsonvalue.Value) string {
	var origin []string
	for _, key := range []string{"modelOverrideFallbackOriginProvider", "modelOverrideFallbackOriginModel"} {
		if value := entry.Get(key); !value.IsEmpty() {
			origin = append(origin, value.Text())
		}
	}
	fallback := ""
	if len(origin) > 0 {
		fallback = " fallback_from=" + strings.Join(origin, "/")
	}
	return "model: " + entry.Get("modelOverride").Text() + " provider=" +
		cmp.Or(parser.Text(entry.Get("providerOverride")), "-") + " source=" +
		cmp.Or(parser.Text(entry.Get("modelOverrideSource")), "-") + fallback +
		" (from openclaw-agent.sqlite session_nodes.entry_json modelOverride, updated_at)"
}
