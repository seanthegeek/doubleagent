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
