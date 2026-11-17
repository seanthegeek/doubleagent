package openclaw

import (
	"fmt"
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

// columnStr returns a column value as Python's str(value or "") gives it.
func columnStr(value sqlitefile.Value) string {
	if value.IsEmpty() {
		return ""
	}
	return value.String()
}

// orDash returns text, or "-" when it is empty.
func orDash(text string) string {
	return orText(text, "-")
}

// columnText returns a column value as text, "" for NULL: the value of a
// column the analyzer reads as a string when it is one.
func columnText(value sqlitefile.Value) string {
	if value.Type == sqlitefile.Text {
		return value.Text()
	}
	return ""
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
		raw := columnText(row.Get("entry_json"))
		entry, decodeErr := jsonvalue.DecodeString(raw)
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
// archive marker.
func (a *agentDatabase) attachNodes(nodes []node, relayed map[string]bool) {
	for _, each := range nodes {
		entry, line, timestamp := each.entry, each.rowid, each.updatedAt
		sessionID := columnStr(each.currentSessionID)
		if grant := entry.Get("publicShare"); grant.IsObject() && !grant.Get("id").IsEmpty() {
			generation := parser.Str(grant.Get("sessionId"))
			a.store.attach(orText(generation, sessionID), line,
				timeutil.FromValue(grant.Get("createdAt")), fmt.Sprintf("session synced: public"+
					" share %s generation %s (from openclaw-agent.sqlite session_nodes.entry_json,"+
					" publicShare.createdAt)", orDash(parser.Str(grant.Get("id"))), orDash(generation)), "")
		}
		if relay := deliveryRelay(entry.Get("delivery")); relay != "" && !relayed[sessionID] {
			a.store.attach(sessionID, line, timestamp, relay, "")
		}
		if override := entry.Get("modelOverride"); override.IsString() && override.StringValue() != "" {
			a.store.attach(sessionID, line, timestamp, overrideText(entry), override.StringValue())
		}
		for _, key := range sessionPermissionKeys() {
			if value := entry.Get(key); !isEmptySetting(value) {
				a.store.attach(sessionID, line, timestamp, fmt.Sprintf("permission setting: %s=%s"+
					" (from openclaw-agent.sqlite session_nodes.entry_json, updated_at)", key,
					textOrJSON(value)), "")
			}
		}
		if archived, ok := entry.Lookup("archivedAt"); ok && !archived.IsNull() {
			a.store.attach(sessionID, line, timeutil.FromValue(archived), archiveText(entry), "")
		}
	}
}

// isEmptySetting reports whether a permission setting is absent: null, an
// empty string, an empty object or an empty list.
func isEmptySetting(value jsonvalue.Value) bool {
	return value.IsNull() || isText(value, "") ||
		((value.IsObject() || value.IsArray()) && value.Len() == 0)
}

// textOrJSON returns a string as it is and anything else as CompactJSON.
func textOrJSON(value jsonvalue.Value) string {
	if value.IsString() {
		return value.StringValue()
	}
	return parser.CompactJSON(value)
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
	return fmt.Sprintf("session archived: %s by %s (from openclaw-agent.sqlite"+
		" session_nodes.entry_json, archivedAt)", orDash(parser.Str(entry.Get("archiveReason"))),
		orDash(strings.Join(actor, ":")))
}

// deliveryRelay returns the `session relayed:` text of an external
// entry_json.delivery, else "": its kind is external and its route's
// channel a platform (src/config/sessions/types.ts:60-83; route is a
// ChannelRouteRef, src/plugin-sdk/channel-route.ts:18-25).
func deliveryRelay(delivery jsonvalue.Value) string {
	if !isText(delivery.Get("kind"), "external") {
		return ""
	}
	route, origin := delivery.Get("route"), delivery.Get("origin")
	channel := route.Get("channel")
	if !channel.IsString() || !isPlatform(channel.StringValue()) {
		return ""
	}
	field := func(value jsonvalue.Value) string { return orDash(parser.Str(value)) }
	return fmt.Sprintf("session relayed: %s account %s %s from %s to %s (from openclaw-agent.sqlite"+
		" session_nodes.entry_json delivery, updated_at)", channel.StringValue(),
		orDash(firstStr(route.Get("accountId"), origin.Get("accountId"))), field(origin.Get("chatType")),
		field(origin.Get("from")), field(origin.Get("to")))
}

// firstStr returns the str() of the first value Python counts as true, or
// "".
func firstStr(values ...jsonvalue.Value) string {
	for _, value := range values {
		if !value.IsEmpty() {
			return value.Text()
		}
	}
	return ""
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
	return fmt.Sprintf("model: %s provider=%s source=%s%s (from openclaw-agent.sqlite"+
		" session_nodes.entry_json modelOverride, updated_at)", entry.Get("modelOverride").Text(),
		orDash(parser.Str(entry.Get("providerOverride"))),
		orDash(parser.Str(entry.Get("modelOverrideSource"))), fallback)
}
