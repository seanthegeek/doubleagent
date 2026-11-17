package sqlitefile

import (
	"encoding/hex"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// errDefault is wrapped by the error of a DEFAULT clause this package does
// not evaluate; it is reported only when a short record needs the value.
var errDefault = fmt.Errorf("%w: unsupported DEFAULT value", errSchemaSyntax)

// defaultValue evaluates a column's DEFAULT clause as SQLite evaluates it
// for a record too short to hold the column: literals, signs, parentheses
// and CAST with the column's affinity applied, the CURRENT_ keywords and
// any other expression as NULL, since SQLite computes those only when a
// row is written. A name in parentheses is no constant, which SQLite
// refuses. The rules were taken from what SQLite returns for each form.
func defaultValue(tokens []token, affinity Affinity) (Value, error) {
	if len(tokens) == 1 && (tokens[0].kind == word || tokens[0].kind == quoted) &&
		!isKeywordLiteral(tokens[0]) {
		// DEFAULT name, without parentheses, is the name as a string.
		return applyAffinity(TextValue(tokens[0].name()), affinity), nil
	}
	return evaluateDefault(tokens, affinity)
}

// isKeywordLiteral reports whether a bare word is NULL, TRUE, FALSE or a
// CURRENT_ keyword, the words that are values rather than names.
func isKeywordLiteral(literal token) bool {
	switch strings.ToUpper(literal.text) {
	case "NULL", "TRUE", "FALSE", "CURRENT_TIME", "CURRENT_DATE", "CURRENT_TIMESTAMP":
		return literal.kind == word
	}
	return false
}

// evaluateDefault evaluates the expression tokens hold.
func evaluateDefault(tokens []token, affinity Affinity) (Value, error) {
	tokens = withoutOuterParentheses(tokens)
	for len(tokens) > 1 && tokens[0].isPunctuation("+") {
		tokens = withoutOuterParentheses(tokens[1:])
	}
	switch {
	case len(tokens) == 0:
		return Value{}, errDefault
	case len(tokens) == 2 && tokens[0].isPunctuation("-") && tokens[1].kind == number:
		return numberLiteral(tokens[1].text, true, affinity)
	case len(tokens) > 1 && tokens[0].isPunctuation("-"):
		return negatedDefault(tokens[1:], affinity)
	case len(tokens) > 1 && tokens[0].isKeyword("CAST"):
		return castDefault(tokens[1:], affinity)
	case len(tokens) > 1:
		return Value{}, nil
	}
	return literalDefault(tokens[0], affinity)
}

// withoutOuterParentheses removes parentheses that enclose all of tokens.
func withoutOuterParentheses(tokens []token) []token {
	for len(tokens) >= 2 && tokens[0].isPunctuation("(") && tokens[len(tokens)-1].isPunctuation(")") {
		depth := 0
		for i, current := range tokens {
			if current.isPunctuation("(") {
				depth++
			} else if current.isPunctuation(")") {
				depth--
			}
			if depth == 0 && i < len(tokens)-1 {
				return tokens
			}
		}
		tokens = tokens[1 : len(tokens)-1]
	}
	return tokens
}

// literalDefault evaluates one token.
func literalDefault(literal token, affinity Affinity) (Value, error) {
	switch {
	case literal.kind == number:
		return numberLiteral(literal.text, false, affinity)
	case literal.kind == stringToken:
		return applyAffinity(TextValue(literal.value), affinity), nil
	case literal.kind == blobToken:
		data, err := hex.DecodeString(literal.value)
		return BlobValue(data), err
	case literal.isKeyword("TRUE"):
		return applyAffinity(IntValue(1), affinity), nil
	case literal.isKeyword("FALSE"):
		return applyAffinity(IntValue(0), affinity), nil
	case isKeywordLiteral(literal):
		return Value{}, nil
	}
	return Value{}, fmt.Errorf("%w: default value %s is not constant", errSchemaSyntax, literal.text)
}

// numberLiteral evaluates a numeric literal, negated when negative. An
// integer that fits 64 bits (hexadecimal ones as their 64-bit pattern) is
// an integer; any other literal keeps its text, sign included, for a TEXT
// column, and is read as a number for the others, an integer only when the
// column is NUMERIC, INTEGER or REAL.
func numberLiteral(text string, negative bool, affinity Affinity) (Value, error) {
	text = strings.ReplaceAll(text, "_", "")
	if integer, ok := integerLiteral(text); ok {
		if negative {
			integer = -integer
		}
		return applyAffinity(IntValue(integer), affinity), nil
	}
	if negative {
		text = "-" + text
	}
	if affinity == TextAffinity {
		return TextValue(text), nil
	}
	value, ok := parseNumeric(text, affinity != BlobAffinity)
	if !ok {
		return Value{}, fmt.Errorf("%w: malformed number %s", errSchemaSyntax, text)
	}
	return applyAffinity(value, affinity), nil
}

// integerLiteral reads a decimal integer that fits 64 bits or a
// hexadecimal one of at most 16 digits.
func integerLiteral(text string) (int64, bool) {
	if len(text) > 2 && text[0] == '0' && text[1]|0x20 == 'x' {
		pattern, err := strconv.ParseUint(text[2:], 16, 64)
		return int64(pattern), err == nil
	}
	if strings.ContainsAny(text, ".eE") {
		return 0, false
	}
	integer, err := strconv.ParseInt(text, 10, 64)
	return integer, err == nil
}

// negatedDefault evaluates "- expression": the operand as a number,
// negated, then the affinity applied.
func negatedDefault(operand []token, affinity Affinity) (Value, error) {
	value, err := evaluateDefault(operand, BlobAffinity)
	if err != nil || value.Type == Null {
		return value, err
	}
	if value.Type == Text {
		number, ok := parseNumeric(string(value.Bytes), true)
		if !ok {
			return Value{}, errDefault
		}
		value = number
	}
	switch value.Type {
	case Integer:
		if value.Int == -9223372036854775808 {
			value = FloatValue(9223372036854775808)
		} else {
			value.Int = -value.Int
		}
	case Float:
		value.Float = -value.Float
	default:
		return Value{}, errDefault
	}
	return applyAffinity(value, affinity), nil
}

// castDefault evaluates "CAST ( expression AS type )" for the conversions
// that need no rule beyond affinity: numbers to text, numeric text to a
// number, text to a blob.
func castDefault(rest []token, affinity Affinity) (Value, error) {
	if len(rest) < 4 || !rest[0].isPunctuation("(") || !rest[len(rest)-1].isPunctuation(")") {
		return Value{}, errDefault
	}
	inner := rest[1 : len(rest)-1]
	separator := -1
	for i, current := range inner {
		if current.isKeyword("AS") {
			separator = i
		}
	}
	if separator < 1 || separator == len(inner)-1 {
		return Value{}, errDefault
	}
	target := affinityOf(joinTokens(inner[separator+1:]), false)
	value, err := evaluateDefault(inner[:separator], target)
	if err != nil {
		return Value{}, err
	}
	switch {
	case value.Type == Null:
	case target == TextAffinity:
		value = Value{Type: Text, Bytes: value.Bytes}
	case target == BlobAffinity && value.Type == Text:
		value = BlobValue(value.Bytes)
	case target == IntegerAffinity && value.Type == Float:
		if value = integerIfExact(FloatValue(math.Trunc(value.Float))); value.Type != Integer {
			return Value{}, errDefault
		}
	case !target.numeric() || (value.Type != Integer && value.Type != Float):
		return Value{}, errDefault
	}
	return applyAffinity(value, affinity), nil
}
