package parser

import (
	"regexp"
	"testing"
	"unicode"

	"github.com/seanthegeek/doubleagent/internal/pytext"
)

// The class must hold exactly the characters Python's strip() removes,
// which pytext.Strip implements.
func TestPythonSpaceHoldsWhatPythonStrips(t *testing.T) {
	class := regexp.MustCompile(`^[` + PythonSpace + `]\z`)
	for char := rune(0); char <= unicode.MaxRune; char++ {
		if char >= 0xD800 && char <= 0xDFFF {
			continue
		}
		text := string(char)
		if got, want := class.MatchString(text), pytext.Strip("x"+text) == "x"; got != want {
			t.Errorf("U+%04X: in class %v, stripped %v", char, got, want)
		}
	}
}
