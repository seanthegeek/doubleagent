package clinelegacy

import (
	"math/rand/v2"
	"strconv"
	"strings"
	"testing"
)

// referenceArrayAfterKey finds the array after a key by hand: each place
// the quoted key occurs, in order, is checked for JSON whitespace, a colon,
// JSON whitespace and a bracket. It is kept to check the pattern against.
func referenceArrayAfterKey(text, key string) (int, bool) {
	quoted := `"` + key + `"`
	skip := func(offset int) int {
		for offset < len(text) && strings.IndexByte(" \t\r\n", text[offset]) >= 0 {
			offset++
		}
		return offset
	}
	for start := 0; ; start++ {
		found := strings.Index(text[start:], quoted)
		if found < 0 {
			return 0, false
		}
		start += found
		cursor := skip(start + len(quoted))
		if cursor < len(text) && text[cursor] == ':' {
			cursor = skip(cursor + 1)
			if cursor < len(text) && text[cursor] == '[' {
				return cursor, true
			}
		}
	}
}

// keyedPieces are what generated texts are made of: the key quoted and
// bare, quotes and escaped quotes, the key inside a string value, JSON and
// other whitespace, the punctuation around an array, and a byte that is
// not UTF-8.
func keyedPieces(key string) []string {
	return []string{`"` + key + `"`, key, `"`, `\"`, `\\`, `"x": "` + key + `"`, `"\"` + key + `\""`,
		":", "[", "]", "{", "}", ",", " ", "\t", "\n", "\r", " ", " ", "1", "\xff", `"` + key}
}

func TestTheKeyedPatternFindsWhatTheHandSearchFinds(t *testing.T) {
	for _, key := range []string{"messages", "entries", "a", "aa", `a"a`, "a.b", "[x]"} {
		keyed := KeyedArray(key)
		pieces := keyedPieces(key)
		for seed := range uint64(2000) {
			source := rand.New(rand.NewPCG(seed, 527))
			var text strings.Builder
			for range source.IntN(16) {
				text.WriteString(pieces[source.IntN(len(pieces))])
			}
			wantOffset, wantFound := referenceArrayAfterKey(text.String(), key)
			if offset, found := ArrayAfterKey(text.String(), keyed); offset != wantOffset || found != wantFound {
				t.Fatalf("key %q, text %s: got %d, %v; want %d, %v", key, strconv.Quote(text.String()), offset,
					found, wantOffset, wantFound)
			}
		}
	}
}
