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
// refuses. testdata/default_values.jsonl.gz holds what SQLite reads for
// each form, and the tests compare with it.
func defaultValue(tokens []token, affinity Affinity) (Value, error) {
	if len(tokens) == 1 && (tokens[0].kind == word || tokens[0].kind == quoted) &&
		!isKeywordLiteral(tokens[0]) {
		// DEFAULT name, without parentheses, is the name as a string.
		return applyAffinity(TextValue(tokens[0].name()), affinity), nil
	}
	return evaluateDefault(tokens, affinity, 0)
}

// isKeywordLiteral reports whether a bare word is NULL, TRUE, FALSE or a
// CURRENT_ keyword, the words that are values rather than names.
func isKeywordLiteral(literal token) bool {
	switch upperASCII(literal.text) {
	case "NULL", "TRUE", "FALSE", "CURRENT_TIME", "CURRENT_DATE", "CURRENT_TIMESTAMP":
		return literal.kind == word
	}
	return false
}

// maxDefaultDepth is how deeply a DEFAULT value may nest: each pair of
// parentheses, sign and CAST around the value is one level. A default
// build of SQLite parses with a stack of 100 entries (YYSTACKDEPTH) and
// refuses deeper text as a parser stack overflow, so it writes no deeper
// DEFAULT; the bound keeps evaluating one to a hundred scans of its tokens.
const maxDefaultDepth = 100

// evaluateDefault evaluates the expression tokens hold, nested depth
// levels inside the DEFAULT clause, under affinity, as SQLite evaluates a
// DEFAULT for a short record (valueFromExpr).
func evaluateDefault(tokens []token, affinity Affinity, depth int) (Value, error) {
	for {
		if depth > maxDefaultDepth {
			return Value{}, errTooDeep
		}
		inner, ok := unwrapDefault(tokens)
		if !ok {
			break
		}
		tokens, depth = inner, depth+1
	}
	switch {
	case len(tokens) == 0:
		return Value{}, errDefault
	case len(tokens) > 1 && tokens[0].isPunctuation("-"):
		return negatedDefault(tokens[1:], affinity, depth+1)
	case len(tokens) > 1 && tokens[0].isKeyword("CAST"):
		return castDefault(tokens[1:], affinity, depth+1)
	case len(tokens) > 1:
		return Value{}, nil
	}
	return literalDefault(tokens[0], affinity)
}

// errTooDeep is the error of a DEFAULT value nested past maxDefaultDepth.
var errTooDeep = fmt.Errorf("%w: it nests more than %d deep", errDefault, maxDefaultDepth)

// unwrapDefault removes one level that adds nothing to a value: a pair of
// parentheses around all of tokens, or a leading plus sign.
func unwrapDefault(tokens []token) ([]token, bool) {
	if len(tokens) > 1 && tokens[0].isPunctuation("+") {
		return tokens[1:], true
	}
	return unwrapParentheses(tokens)
}

// unwrapParentheses removes a pair of parentheses around all of tokens.
func unwrapParentheses(tokens []token) ([]token, bool) {
	if len(tokens) < 2 || !tokens[0].isPunctuation("(") || !tokens[len(tokens)-1].isPunctuation(")") {
		return tokens, false
	}
	depth := 0
	for i, current := range tokens {
		if current.isPunctuation("(") {
			depth++
		} else if current.isPunctuation(")") {
			depth--
		}
		if depth == 0 && i < len(tokens)-1 {
			return tokens, false
		}
	}
	return tokens[1 : len(tokens)-1], true
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

// numberLiteral evaluates a numeric literal, negated when negative, as
// SQLite does: an integer literal that fits 64 bits (a hexadecimal one as
// its 64-bit pattern) is that integer, with the affinity applied. Any
// other literal is its text, sign included, with the affinity applied;
// under BLOB affinity a real literal is read as a float and an integer one
// as NUMERIC affinity reads it.
func numberLiteral(text string, negative bool, affinity Affinity) (Value, error) {
	text = strings.ReplaceAll(text, "_", "")
	integerToken := isHexLiteral(text) || !strings.ContainsAny(text, ".eE")
	if integer, ok := integerLiteral(text); ok && integerToken {
		if negative {
			integer = -integer
		}
		return applyAffinity(IntValue(integer), affinity), nil
	}
	if negative {
		text = "-" + text
	}
	switch {
	case affinity == BlobAffinity && !integerToken:
		return FloatValue(readReal(text).value), nil
	case affinity == BlobAffinity:
		return applyAffinity(TextValue(text), NumericAffinity), nil
	}
	return applyAffinity(TextValue(text), affinity), nil
}

// isHexLiteral reports whether a number token is a hexadecimal literal.
func isHexLiteral(text string) bool {
	return len(text) > 2 && text[0] == '0' && text[1]|0x20 == 'x'
}

// negatedInOneStep reports whether SQLite reads a minus sign before the
// number literal text as part of it: for every literal but a hexadecimal
// one past 31 bits, whose 64-bit pattern is negated as a value instead.
func negatedInOneStep(text string) bool {
	if !isHexLiteral(text) {
		return true
	}
	digits := strings.TrimLeft(strings.ReplaceAll(text[2:], "_", ""), "0")
	pattern, err := strconv.ParseUint(digits, 16, 32)
	return digits == "" || (err == nil && pattern < 1<<31)
}

// integerLiteral reads a decimal integer that fits 64 bits or a
// hexadecimal one of at most 16 digits after its leading zeros.
func integerLiteral(text string) (int64, bool) {
	if isHexLiteral(text) {
		digits := strings.TrimLeft(text[2:], "0")
		if digits == "" {
			return 0, true
		}
		pattern, err := strconv.ParseUint(digits, 16, 64)
		return int64(pattern), err == nil
	}
	integer, err := strconv.ParseInt(text, 10, 64)
	return integer, err == nil
}

// negatedDefault evaluates "- expression" as SQLite does. When the operand
// is a number literal, in parentheses or not, other than a hexadecimal one
// past 31 bits, the sign is part of the literal. Any other operand is
// evaluated under the same affinity, made a number as numerify makes it,
// negated (-2^63 becoming the float 2^63), then the affinity applied.
func negatedDefault(operand []token, affinity Affinity, depth int) (Value, error) {
	literal := operand
	for range maxDefaultDepth - depth {
		inner, ok := unwrapParentheses(literal)
		if !ok {
			break
		}
		literal = inner
	}
	if len(literal) == 1 && literal[0].kind == number && negatedInOneStep(literal[0].text) {
		return numberLiteral(literal[0].text, true, affinity)
	}
	value, err := evaluateDefault(operand, affinity, depth)
	if err != nil || value.Type == Null {
		return value, err
	}
	value = numerify(value)
	switch {
	case value.Type == Float:
		value.Float = -value.Float
	case value.Int == math.MinInt64:
		value = FloatValue(0x1p63)
	default:
		value.Int = -value.Int
	}
	return applyAffinity(value, affinity), nil
}

// castDefault evaluates "CAST ( expression AS type )" as SQLite does for a
// DEFAULT: the operand's value under the affinity of the type, converted
// to the type as castValue converts it, then the column's affinity
// applied.
func castDefault(rest []token, affinity Affinity, depth int) (Value, error) {
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
	value, err := evaluateDefault(inner[:separator], target, depth)
	if err != nil {
		return Value{}, err
	}
	return applyAffinity(castValue(value, target), affinity), nil
}

// castValue converts a value as CAST to a type of affinity target does
// (sqlite3VdbeMemCast): NULL stays NULL; BLOB makes a number its text and
// text a blob of its bytes; TEXT makes a number or a blob text; INTEGER,
// REAL and NUMERIC convert as castToInteger, castToReal and numerify do.
func castValue(value Value, target Affinity) Value {
	switch {
	case value.Type == Null:
		return value
	case target == IntegerAffinity:
		return castToInteger(value)
	case target == RealAffinity:
		return castToReal(value)
	case target == NumericAffinity:
		return numerify(value)
	case value.Type == Integer || value.Type == Float:
		value = TextValue(numberText(value))
	}
	if target == TextAffinity {
		return Value{Type: Text, Bytes: value.Bytes}
	}
	return Value{Type: Blob, Bytes: value.Bytes}
}

// numerify makes a value a number as SQLite does when it must have one
// (sqlite3VdbeMemNumerify): a number stays as it is; text, and a blob read
// as text, becomes the integer its digits give when they have no decimal
// point or exponent and fit 64 bits, else the float readReal reads, an
// integer when that is one within 2^51 of zero.
func numerify(value Value) Value {
	if value.Type != Text && value.Type != Blob {
		return value
	}
	text := string(value.Bytes)
	reading := readReal(text)
	if integer, fits, _ := readInteger(text); !reading.pointOrPower && fits {
		return IntValue(integer)
	}
	if integer := realToInteger(reading.value); realSameAsInteger(reading.value, integer) {
		return IntValue(integer)
	}
	return FloatValue(reading.value)
}

// castToInteger converts a value that is not NULL as CAST to INTEGER does
// (sqlite3VdbeMemIntegerify): a float drops its fraction, and one past the
// 64-bit range becomes the nearer end of it; text, and a blob read as
// text, becomes the integer it starts with, as readInteger reads it.
func castToInteger(value Value) Value {
	switch value.Type {
	case Float:
		return IntValue(realToInteger(value.Float))
	case Text, Blob:
		integer, _, _ := readInteger(string(value.Bytes))
		return IntValue(integer)
	}
	return value
}

// castToReal converts a value that is not NULL as CAST to REAL does
// (sqlite3VdbeMemRealify): an integer becomes the nearest float; text, and
// a blob read as text, the number its longest numeric prefix holds.
func castToReal(value Value) Value {
	switch value.Type {
	case Integer:
		return FloatValue(float64(value.Int))
	case Text, Blob:
		return FloatValue(readReal(string(value.Bytes)).value)
	}
	return value
}
