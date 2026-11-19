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
