package urlcredentials

import "testing"

func TestFirstSeparatorFindsTheEarlierOfEitherForm(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name          string
		text          string
		start, length int
	}{
		{"no separator gives none", "a:/b", -1, 0},
		{"the empty text gives none", "", -1, 0},
		{"a plain separator is found", "x https://h", 7, 3},
		{"an escaped separator is found", `x https:\/\/h`, 7, 5},
		{"an escaped one before a plain one wins", `a:\/\/b c://d`, 1, 5},
		{"a plain one before an escaped one wins", `a://b c:\/\/d`, 1, 3},
		{"an escaped one right before a plain one wins", `a:\/\/://`, 1, 5},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			start, length := FirstSeparator(test.text)
			if start != test.start || length != test.length {
				t.Errorf("FirstSeparator(%q) = %d, %d, want %d, %d", test.text, start, length,
					test.start, test.length)
			}
			if got := MayHold(test.text); got != (test.start >= 0) {
				t.Errorf("MayHold(%q) = %v, want %v", test.text, got, test.start >= 0)
			}
		})
	}
}
