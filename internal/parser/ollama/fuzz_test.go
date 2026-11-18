package ollama

import (
	"testing"
	"unicode/utf8"
)

func FuzzUTF8Error(fuzz *testing.F) {
	for _, seed := range []string{"", "ok", "ab\xe2\x82", "\xed\xa0\x80", "\xf4\x90\x80\x80", "\xc3"} {
		fuzz.Add([]byte(seed))
	}
	fuzz.Fuzz(func(t *testing.T, data []byte) {
		if err := utf8Error(data); (err == nil) != utf8.Valid(data) {
			t.Errorf("utf8Error(%q) = %v, valid %v", data, err, utf8.Valid(data))
		}
	})
}
