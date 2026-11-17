package crush

import "testing"

func FuzzSplitWords(fuzz *testing.F) {
	for _, seed := range []string{"permissions allow a 'b c' \"d\\\"e\" # x", "a\\", "'open", "\"x\\"} {
		fuzz.Add(seed)
	}
	fuzz.Fuzz(func(t *testing.T, line string) {
		_, _ = splitWords(line)
	})
}

func FuzzUnescapeHTML(fuzz *testing.F) {
	for _, seed := range []string{"&amp;&#65;&#x1F600;&ltx;", "&#99999999999999999999999", "&", "&#xD800"} {
		fuzz.Add(seed)
	}
	fuzz.Fuzz(func(t *testing.T, text string) {
		_ = unescapeHTML(text)
	})
}
