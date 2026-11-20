package cursor

import (
	"slices"
	"strings"
	"testing"
)

// SQL's = never finds NULL equal to NULL, so a conversation whose
// fts_rowid is NULL is joined to no full-text row, not even one whose id
// is NULL.
func TestANullFullTextKeyJoinsNoRow(t *testing.T) {
	texts := rowsOfTableCaseDatabase(t, "null_join.db")
	if !slices.Contains(texts, "conversation index: Fix the tests source=local archived=0") {
		t.Errorf("got %q, want the conversation's index row", texts)
	}
	for _, text := range texts {
		if strings.HasPrefix(text, "cached conversation text") {
			t.Errorf("got %q, want no cached text joined on NULL", text)
		}
	}
}

func TestAFullTextBodyIsWrittenInFullOnceAndPointedToByEachLaterConversation(t *testing.T) {
	got := rowsOfTableCaseDatabase(t, "shared_bodies.db")
	want := []string{
		"conversation index: Title c1 source=local archived=0",
		"cached conversation text (conversation-search.db, source=local): body one",
		"conversation index: Title c2 source=local archived=0",
		"cached conversation text (conversation-search.db, source=local): [shown in full above, in the row of source line 1]",
		"conversation index: Title c3 source=local archived=0",
		"cached conversation text (conversation-search.db, source=local): body two",
		"conversation index: Title c4 source=local archived=0",
		"cached conversation text (conversation-search.db, source=local): [shown in full above, in the row of source line 1]",
		"parser: 2 conversations name a full-text row an earlier conversation names",
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestTheTextOfConversationSearchGrowsWithItsBodiesNotWithTheConversationsSharingOne(t *testing.T) {
	size := func(name string) int {
		total := 0
		for _, text := range rowsOfTableCaseDatabase(t, name) {
			total += len(text)
		}
		return total
	}
	small, large := size("shared_body_20.db"), size("shared_body_40.db")
	// Twice the conversations sharing one body twice as long gives twice
	// the text; writing the body once per conversation would give four
	// times as much.
	if small == 0 || large > 5*small/2 {
		t.Errorf("text bytes: %d for 20 conversations, %d for 40", small, large)
	}
}
