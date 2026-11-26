package sqlitefile

import (
	"errors"
	"fmt"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// tokenKind is the class of one token of SQL text.
type tokenKind uint8

const (
	// word is a bare identifier or keyword: SQLite tells them apart by
	// position, so the schema parser compares words case-insensitively.
	word tokenKind = iota + 1
	// quoted is an identifier in double quotes, backquotes or brackets.
	quoted
	// stringToken is a string literal in single quotes.
	stringToken
	number
	blobToken
	punctuation
)

// token is one token: its kind, its text as written and, for a quoted
// identifier or a string, the text inside the quotes with doubled quotes
// made single.
type token struct {
	kind  tokenKind
	text  string
	value string
}

// isKeyword reports whether the token is the bare word keyword, in any ASCII case.
func (t token) isKeyword(keyword string) bool {
	return t.kind == word && textlines.EqualFoldASCII(t.text, keyword)
}

// isPunctuation reports whether the token is the punctuation mark mark.
func (t token) isPunctuation(mark string) bool {
	return t.kind == punctuation && t.text == mark
}

// name returns the token as an identifier: a bare word as written, a
// quoted identifier or a string without its quotes.
func (t token) name() string {
	if t.kind == quoted || t.kind == stringToken {
		return t.value
	}
	return t.text
}

// scanFrom returns the token at or after byte position of text and the
// offset after it, read as SQLite's tokenizer ("tokenize.c") reads: past
// whitespace and both comment forms. At the end of the text it returns the
// zero token; an unterminated quote is an error, and an unterminated block
// comment ends the text, as it does for SQLite.
func scanFrom(text string, position int) (token, int, error) {
	for position < len(text) {
		switch {
		case isSpace(text[position]):
			position++
		case strings.HasPrefix(text[position:], "--"):
			end := strings.IndexByte(text[position:], '\n')
			if end < 0 {
				return token{}, len(text), nil
			}
			position += end + 1
		case strings.HasPrefix(text[position:], "/*"):
			end := strings.Index(text[position+2:], "*/")
			if end < 0 {
				return token{}, len(text), nil
			}
			position += end + 4
		default:
			next, length, err := scanToken(text[position:])
			if err != nil {
				return token{}, len(text), fmt.Errorf("at offset %d: %w", position, err)
			}
			return next, position + length, nil
		}
	}
	return token{}, len(text), nil
}

// isSpace reports whether octet is whitespace to SQLite: space, tab, line
// feed, vertical tab, form feed or carriage return.
func isSpace(octet byte) bool {
	return octet == ' ' || (octet >= '\t' && octet <= '\r')
}

// isIdentifierByte reports whether octet may continue a bare identifier:
// ASCII letters, digits, underscore, dollar sign and any byte of a
// multi-byte UTF-8 character.
func isIdentifierByte(octet byte) bool {
	return octet >= 0x80 || octet == '_' || octet == '$' || isDigit(octet) ||
		(octet|0x20 >= 'a' && octet|0x20 <= 'z')
}

// isDigit reports whether octet is an ASCII digit.
func isDigit(octet byte) bool {
	return octet >= '0' && octet <= '9'
}

// scanToken reads the token at the start of text and returns it with its
// length in bytes.
func scanToken(text string) (token, int, error) {
	octet := text[0]
	switch {
	case octet == '\'':
		return scanQuoted(text, '\'', stringToken)
	case octet == '"' || octet == '`':
		return scanQuoted(text, octet, quoted)
	case octet == '[':
		end := strings.IndexByte(text, ']')
		if end < 0 {
			return token{}, 0, errors.New("unterminated [ identifier")
		}
		return token{kind: quoted, text: text[:end+1], value: text[1:end]}, end + 1, nil
	case (octet == 'x' || octet == 'X') && len(text) > 1 && text[1] == '\'':
		return scanBlob(text)
	case isDigit(octet) || (octet == '.' && len(text) > 1 && isDigit(text[1])):
		length := scanNumber(text)
		if text[length-1]|0x20 == 'x' || length < len(text) && isIdentifierByte(text[length]) {
			return token{}, 0, fmt.Errorf("malformed number %q", text[:min(length+1, len(text))])
		}
		return token{kind: number, text: text[:length]}, length, nil
	case isIdentifierByte(octet):
		length := 1
		for length < len(text) && isIdentifierByte(text[length]) {
			length++
		}
		return token{kind: word, text: text[:length]}, length, nil
	}
	for _, mark := range []string{"||", "<=", ">=", "<>", "!=", "==", "<<", ">>", "->>", "->"} {
		if strings.HasPrefix(text, mark) {
			return token{kind: punctuation, text: mark}, len(mark), nil
		}
	}
	if strings.IndexByte("(),;.+-*/%<>=&|~", octet) >= 0 {
		return token{kind: punctuation, text: text[:1]}, 1, nil
	}
	return token{}, 0, fmt.Errorf("unrecognized character %q", octet)
}

// scanQuoted reads text quoted by quote, where a doubled quote stands for
// one, as a token of kind.
func scanQuoted(text string, quote byte, kind tokenKind) (token, int, error) {
	var value strings.Builder
	for i := 1; i < len(text); i++ {
		if text[i] != quote {
			value.WriteByte(text[i])
			continue
		}
		if i+1 < len(text) && text[i+1] == quote {
			value.WriteByte(quote)
			i++
			continue
		}
		return token{kind: kind, text: text[:i+1], value: value.String()}, i + 1, nil
	}
	return token{}, 0, fmt.Errorf("unterminated %c quote", quote)
}

// scanBlob reads a blob literal X'...', whose hex digits must come in pairs.
func scanBlob(text string) (token, int, error) {
	end := strings.IndexByte(text[2:], '\'')
	if end < 0 {
		return token{}, 0, errors.New("unterminated blob literal")
	}
	digits := text[2 : 2+end]
	if len(digits)%2 != 0 || strings.IndexFunc(digits, notHexDigit) >= 0 {
		return token{}, 0, errors.New("malformed blob literal " + text[:end+3])
	}
	return token{kind: blobToken, text: text[:end+3], value: digits}, end + 3, nil
}

// notHexDigit reports whether char is not a hexadecimal digit.
func notHexDigit(char rune) bool {
	return !(char >= '0' && char <= '9' || char|0x20 >= 'a' && char|0x20 <= 'f')
}

// scanNumber returns the length of the numeric literal at the start of
// text: a hexadecimal integer, or digits with an optional fraction and
// exponent, with the underscores SQLite allows between digits.
func scanNumber(text string) int {
	if len(text) > 1 && text[0] == '0' && text[1]|0x20 == 'x' {
		length := 2
		for length < len(text) && (strings.IndexByte("0123456789abcdefABCDEF_", text[length]) >= 0) {
			length++
		}
		return length
	}
	length := digitRun(text, 0)
	if length < len(text) && text[length] == '.' {
		length = digitRun(text, length+1)
	}
	if length < len(text) && text[length]|0x20 == 'e' {
		exponent := length + 1
		if exponent < len(text) && (text[exponent] == '+' || text[exponent] == '-') {
			exponent++
		}
		if exponent < len(text) && isDigit(text[exponent]) {
			length = digitRun(text, exponent)
		}
	}
	return length
}

// digitRun returns the offset after the run of digits and underscores that
// starts at offset start.
func digitRun(text string, start int) int {
	for start < len(text) && (isDigit(text[start]) || text[start] == '_') {
		start++
	}
	return start
}
