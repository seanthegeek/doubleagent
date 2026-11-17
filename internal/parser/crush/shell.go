package crush

import (
	"strings"
)

// The two ways a line fails to split into shell words, worded as the
// analyzer words them in a bad-line row.
const (
	noClosingQuote = "No closing quotation"
	noEscaped      = "No escaped character"
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

// splitWords splits a line into words as POSIX sh quoting reads them, with
// comments: words separate at space, tab, CR and LF; single quotes keep
// everything to the next one; inside double quotes a backslash escapes only
// a double quote or a backslash; outside quotes a backslash escapes any
// character; a # outside quotes, even inside a word, ends the line. No
// expansion of any kind is done. For an unclosed quote or a backslash at
// the end it returns why the line does not split.
func splitWords(line string) ([]string, string) {
	splitter := &wordSplitter{}
	for i := 0; i < len(line); i++ {
		character := line[i]
		switch {
		case strings.IndexByte(shellSpace, character) >= 0:
			splitter.finish()
		case character == '#':
			splitter.finish()
			return splitter.words, ""
		case character == '\\':
			if i+1 == len(line) {
				return nil, noEscaped
			}
			i++
			splitter.word.WriteByte(line[i])
			splitter.started = true
		case character == '\'' || character == '"':
			end, problem := splitter.quoted(line, i)
			if problem != "" {
				return nil, problem
			}
			i = end
		default:
			splitter.word.WriteByte(character)
			splitter.started = true
		}
	}
	splitter.finish()
	return splitter.words, ""
}

// quoted adds the quoted text that starts at the quote at offset start of
// line to the current word and returns the offset of its closing quote,
// or why the quote does not close.
func (s *wordSplitter) quoted(line string, start int) (int, string) {
	quote := line[start]
	s.started = true
	for i := start + 1; i < len(line); i++ {
		character := line[i]
		switch {
		case character == quote:
			return i, ""
		case character == '\\' && quote == '"':
			if i+1 == len(line) {
				return 0, noEscaped
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
	return 0, noClosingQuote
}
