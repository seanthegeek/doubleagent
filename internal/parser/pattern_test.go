package parser

import (
	"regexp"
	"testing"
	"unicode"
)

func TestSpaceHoldsWhatUnicodeIsSpaceReports(t *testing.T) {
	class := regexp.MustCompile(`^[` + Space + `]\z`)
	for char := rune(0); char <= unicode.MaxRune; char++ {
		if char >= 0xD800 && char <= 0xDFFF {
			continue
		}
		if got, want := class.MatchString(string(char)), unicode.IsSpace(char); got != want {
			t.Errorf("U+%04X: in class %v, unicode.IsSpace %v", char, got, want)
		}
	}
}
