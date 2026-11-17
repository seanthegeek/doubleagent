package openclaw

import (
	"fmt"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// relays attaches the `session relayed:` rows and returns the generations
// that got one: one per platform conversation linked to a generation
// (session_conversations joined to conversations), else one per generation
// whose window names a platform channel, and one per generation a Gmail
// hook started.
func (a *agentDatabase) relays() (map[string]bool, error) {
	relayed := map[string]bool{}
	if a.names["session_conversations"] && a.names["conversations"] {
		if err := a.conversationRelays(relayed); err != nil {
			return nil, err
		}
	}
	for _, sessionID := range a.windows.order {
		window := a.windows.byID[sessionID]
		timestamp := timeutil.FirstNonEmpty(parser.ColumnTime(window.Get("started_at")),
			parser.ColumnTime(window.Get("created_at")))
		channel := columnText(window.Get("channel"))
		if !relayed[sessionID] && isPlatform(channel) {
			a.store.attach(sessionID, int(window.Rowid), timestamp, fmt.Sprintf(
				"session relayed: %s account %s %s (from openclaw-agent.sqlite session_windows,"+
					" started_at)", channel, orDash(columnStr(window.Get("account_id"))),
				orDash(columnStr(window.Get("chat_type")))), "")
			relayed[sessionID] = true
		}
		if columnText(window.Get("hook_external_content_source")) == "gmail" {
			a.store.attach(sessionID, int(window.Rowid), timestamp, "session relayed: gmail hook"+
				" (from openclaw-agent.sqlite session_windows hook_external_content_source,"+
				" started_at)", "")
			relayed[sessionID] = true
		}
	}
	return relayed, nil
}

// conversationRelays attaches one `session relayed:` row per platform
// conversation linked to a generation, by first_seen_at, and marks the
// generation relayed.
func (a *agentDatabase) conversationRelays(relayed map[string]bool) error {
	conversations, err := a.database.Table("conversations")
	if err != nil {
		return err
	}
	byID, err := conversations.Select("channel", "account_id", "kind", "peer_id", "thread_id").
		Lookup("conversation_id")
	if err != nil {
		return err
	}
	links, err := a.table("session_conversations", func(table *sqlitefile.Table) sqlitefile.Query {
		return table.Select("session_id", "role", "first_seen_at", "conversation_id").
			OrderBy("first_seen_at", "rowid")
	})
	if err != nil {
		return err
	}
	for _, link := range links {
		for _, conversation := range byID.Find(link, "conversation_id") {
			channel := columnText(conversation.Get("channel"))
			if !isPlatform(channel) {
				continue
			}
			sessionID := columnStr(link.Get("session_id"))
			field := func(name string) string { return orDash(columnStr(conversation.Get(name))) }
			a.store.attach(sessionID, int(link.Rowid), parser.ColumnTime(link.Get("first_seen_at")),
				fmt.Sprintf("session relayed: %s account %s %s peer %s thread %s role %s (from"+
					" openclaw-agent.sqlite session_conversations and conversations, first_seen_at)",
					channel, field("account_id"), field("kind"), field("peer_id"), field("thread_id"),
					orDash(columnStr(link.Get("role")))), "")
			relayed[sessionID] = true
		}
	}
	return nil
}
