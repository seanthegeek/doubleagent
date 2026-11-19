package crush

import (
	"errors"
	"strings"
)

// The two ways a line fails to split into shell words, as a bad-line row
// words them.
var (
	errNoClosingQuote = errors.New("a quote is not closed")
	errNoEscaped      = errors.New("a backslash ends the line")
)

// shellSpace is what separates shell words.
const shellSpace = " \t\r\n"

// wordSplitter is the state of splitWords over one line.
type wordSplitter struct {
	words   []string
	word    strings.Builder
	started bool
}

// finish ends the current word, if one was started: a pair of quotes
// starts an empty word.
func (s *wordSplitter) finish() {
	if s.started {
		s.words = append(s.words, s.word.String())
	}
	s.word.Reset()
	s.started = false
}

// splitWords splits a line into words the way POSIX sh quoting does, with
// one difference: a # outside quotes starts a comment that ends the line
// wherever it is, even in the middle of a word (view#1 is the word view),
// where sh starts a comment only at the beginning of a word. Words separate
// at space, tab, CR and LF; single quotes keep everything to the next one;
// inside double quotes a backslash escapes only a double quote or a
// backslash; outside quotes a backslash escapes any character. No expansion
// of any kind is done. The standard library has no shell word splitter. For
// an unclosed quote or a backslash at the end it returns an error saying why
// the line does not split.
func splitWords(line string) ([]string, error) {
	splitter := &wordSplitter{}
	for i := 0; i < len(line); i++ {
		character := line[i]
		switch {
		case strings.IndexByte(shellSpace, character) >= 0:
			splitter.finish()
		case character == '#':
			splitter.finish()
			return splitter.words, nil
		case character == '\\':
			if i+1 == len(line) {
				return nil, errNoEscaped
			}
			i++
			splitter.word.WriteByte(line[i])
			splitter.started = true
		case character == '\'' || character == '"':
			end, err := splitter.quoted(line, i)
			if err != nil {
				return nil, err
			}
			i = end
		default:
			splitter.word.WriteByte(character)
			splitter.started = true
		}
	}
	splitter.finish()
	return splitter.words, nil
}

// quoted adds the quoted text that starts at the quote at offset start of
// line to the current word and returns the offset of its closing quote,
// or an error saying why the quote does not close.
func (s *wordSplitter) quoted(line string, start int) (int, error) {
	quote := line[start]
	s.started = true
	for i := start + 1; i < len(line); i++ {
		character := line[i]
		switch {
		case character == quote:
			return i, nil
		case character == '\\' && quote == '"':
			if i+1 == len(line) {
				return 0, errNoEscaped
			}
			if next := line[i+1]; next != '"' && next != '\\' {
				s.word.WriteByte('\\')
			}
			i++
			s.word.WriteByte(line[i])
		default:
			s.word.WriteByte(character)
		}
	}
	return 0, errNoClosingQuote
}
