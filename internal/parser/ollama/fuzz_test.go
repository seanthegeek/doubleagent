package ollama

import (
	"testing"
	"unicode/utf8"
)

func FuzzUTF8Problem(fuzz *testing.F) {
	for _, seed := range []string{"", "ok", "ab\xe2\x82", "\xed\xa0\x80", "\xf4\x90\x80\x80", "\xc3"} {
		fuzz.Add([]byte(seed))
	}
	fuzz.Fuzz(func(t *testing.T, data []byte) {
		if got := utf8Problem(data); (got == "") != utf8.Valid(data) {
			t.Errorf("utf8Problem(%q) = %q, valid %v", data, got, utf8.Valid(data))
		}
	})
}
