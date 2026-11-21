package openclaw

import (
	"cmp"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
	"github.com/seanthegeek/doubleagent/internal/timeutil"
)

// relays attaches the `session relayed:` rows and returns the generations
// that got one: one per platform conversation linked to a generation
// (session_conversations joined to conversations), else one per window row
// that names a platform channel, and one per window row of a generation a
// Gmail hook started. Each stored row of a window id stored more than once
// gives its own, a row of its parent in the id's group.
func (a *agentDatabase) relays() (map[string]bool, error) {
	relayed := map[string]bool{}
	if a.names.Has("session_conversations") && a.names.Has("conversations") {
		if err := a.conversationRelays(relayed); err != nil {
			return nil, err
		}
	}
	for _, sessionID := range a.windows.order {
		linked := relayed[sessionID]
		for i, window := range a.windows.byID[sessionID] {
			if a.windowRelays(sessionID, window, a.windows.repeats[sessionID].parent(i), linked) {
				relayed[sessionID] = true
			}
		}
	}
	return relayed, nil
}

// windowRelays attaches the relay rows of one window row, of parent (nil
// unless the id is stored more than once), the channel's only when linked
// says no conversation recorded the relay, and reports whether it did.
func (a *agentDatabase) windowRelays(sessionID string, window sqlitefile.Row, parent *timeline.Parent,
	linked bool) bool {
	timestamp := timeutil.FirstNonEmpty(parser.ColumnTime(window.Get("started_at")),
		parser.ColumnTime(window.Get("created_at")))
	channel := parser.TextColumn(window.Get("channel"))
	attached := false
	if !linked && isPlatform(channel) {
		a.store.attachOwn(sessionID, parent, int(window.Rowid), timestamp, "session relayed: "+channel+
			" account "+cmp.Or(parser.NonEmptyColumn(window.Get("account_id")), "-")+" "+
			cmp.Or(parser.NonEmptyColumn(window.Get("chat_type")), "-")+
			" (from openclaw-agent.sqlite session_windows, started_at)")
		attached = true
	}
	if parser.TextColumn(window.Get("hook_external_content_source")) == "gmail" {
		a.store.attachOwn(sessionID, parent, int(window.Rowid), timestamp, "session relayed: gmail hook"+
			" (from openclaw-agent.sqlite session_windows hook_external_content_source,"+
			" started_at)")
		attached = true
	}
	return attached
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
	// The join copies a conversation's columns onto each link's row.
	carried := parser.NewCarried(a.artifact)
	defer carried.Report(0, &a.problems)
	for _, link := range links {
		conversation, found := firstConversation(byID, link)
		if !found {
			continue
		}
		channel := parser.TextColumn(conversation.Get("channel"))
		if !isPlatform(channel) {
			continue
		}
		sessionID := parser.NonEmptyColumn(link.Get("session_id"))
		field := func(name string) string {
			return carried.Cut("conversations."+name, cmp.Or(parser.NonEmptyColumn(conversation.Get(name)), "-"))
		}
		a.store.attach(sessionID, int(link.Rowid), parser.ColumnTime(link.Get("first_seen_at")),
			"session relayed: "+carried.Cut("conversations.channel", channel)+" account "+field("account_id")+" "+
				field("kind")+" peer "+field("peer_id")+" thread "+field("thread_id")+
				" role "+cmp.Or(parser.NonEmptyColumn(link.Get("role")), "-")+
				" (from openclaw-agent.sqlite session_conversations and conversations, first_seen_at)", "")
		relayed[sessionID] = true
	}
	return nil
}

// firstConversation returns the first conversations row whose
// conversation_id is the one row names, and false when none is. The id is
// the conversations table's key, so a second row of one id, which only a
// file the agent did not write holds, is not joined again to every row
// naming it, nor gathered again for each.
func firstConversation(byID *sqlitefile.Lookup, row sqlitefile.Row) (sqlitefile.Row, bool) {
	return byID.First(row, "conversation_id")
}
