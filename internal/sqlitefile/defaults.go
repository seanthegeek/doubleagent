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

// readDefault evaluates the DEFAULT clause text, which has been scanned
// before without error, as defaultValue does. Its tokens are kept only
// while they nest at most maxDefaultDepth deep, so a clause refused for
// its depth holds none of the tokens past that depth.
func readDefault(text string, affinity Affinity) (Value, error) {
	parser := newSchemaParser(text)
	var tokens []token
	var levels nesting
	for !parser.atEnd() {
		current := parser.next()
		if levels.add(current, tokens) > maxDefaultDepth {
			return Value{}, errTooDeep
		}
		tokens = append(tokens, current)
	}
	return defaultValue(pairParentheses(tokens), affinity)
}

// nesting counts the levels open at each token of a DEFAULT clause as it
// is scanned: the parentheses not yet closed, and the signs and CASTs
// waiting for their operand, outside each open parenthesis and within
// the innermost one.
type nesting struct {
	open, waiting int
	// saved holds, for each open parenthesis, the signs and CASTs that
	// were waiting when it opened, and savedSum their sum.
	saved    []int
	savedSum int
}

// add counts current, the token after before, and returns the levels open
// once it is read.
func (n *nesting) add(current token, before []token) int {
	switch {
	case current.isPunctuation("("):
		n.saved = append(n.saved, n.waiting)
		n.savedSum += n.waiting
		n.open, n.waiting = n.open+1, 0
	case current.isPunctuation(")") && n.open > 0:
		last := len(n.saved) - 1
		n.savedSum -= n.saved[last]
		n.saved = n.saved[:last]
		n.open, n.waiting = n.open-1, 0
	case current.isPunctuation("+") || current.isPunctuation("-"):
		if len(before) == 0 || !endsOperand(before[len(before)-1]) {
			n.waiting++
		}
	case current.isKeyword("CAST"):
		n.waiting++
	default:
		n.waiting = 0
	}
	return n.open + n.savedSum + n.waiting
}

// endsOperand reports whether a sign after the token is a binary operator
// rather than the sign of the next operand: after a literal, a name or a
// closing parenthesis.
func endsOperand(previous token) bool {
	return previous.kind != punctuation || previous.isPunctuation(")")
}

// defaultValue evaluates a column's DEFAULT clause as SQLite evaluates it
// for a record too short to hold the column: literals, signs, parentheses,
// CAST and tests for NULL of a literal with the column's affinity applied,
// the CURRENT_ keywords and any other expression as NULL, since SQLite computes those only when a
// row is written. A name in parentheses is no constant, which SQLite
// refuses. testdata/default_values.jsonl.gz holds what SQLite reads for
// each form, and the tests compare with it.
func defaultValue(clause expression, affinity Affinity) (Value, error) {
	if clause.size() == 1 && (clause.tokens[0].kind == word || clause.tokens[0].kind == quoted) &&
		!isKeywordLiteral(clause.tokens[0]) {
		// DEFAULT name, without parentheses, is the name as a string.
		return applyAffinity(TextValue(clause.tokens[0].name()), affinity), nil
	}
	return evaluateDefault(clause, affinity, 0)
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

// maxDefaultDepth is how deeply a DEFAULT value may nest: each open
// parenthesis, and each sign and CAST waiting for its operand, is one
// level, counted as the clause is scanned. SQLite 3.53.4, whose readings
// testdata/default_values.jsonl.gz holds, parses with a stack of at most
// 2,500 entries (SQLITE_LIMIT_PARSER_DEPTH), each level holding at least
// one, so it writes no deeper DEFAULT; 3.51 had a stack of 100 and refused
// a DEFAULT 150 deep that 3.53.4 writes, and the newer bound is followed.
// Each level costs a fixed step, so a value costs its length.
const maxDefaultDepth = 2500

// evaluateDefault evaluates the expression part, nested depth levels
// inside the DEFAULT clause, under affinity, as SQLite evaluates a DEFAULT
// for a short record (valueFromExpr), after its parser has folded a test
// for NULL of a literal (nullTestDefault).
func evaluateDefault(part expression, affinity Affinity, depth int) (Value, error) {
	for {
		if depth > maxDefaultDepth {
			return Value{}, errTooDeep
		}
		inner, ok := unwrapDefault(part)
		if !ok {
			break
		}
		part, depth = inner, depth+1
	}
	if operand, notNull, ok := nullTest(part); ok {
		return nullTestDefault(operand, notNull, affinity), nil
	}
	size := part.size()
	switch {
	case size == 0:
		return Value{}, errDefault
	case size > 1 && part.tokens[0].isPunctuation("-"):
		return negatedDefault(part.slice(1, size), affinity, depth+1)
	case size > 1 && part.tokens[0].isKeyword("CAST"):
		return castDefault(part.slice(1, size), affinity, depth+1)
	case size > 1:
		return Value{}, nil
	}
	return literalDefault(part.tokens[0], affinity)
}

// errTooDeep is the error of a DEFAULT value nested past maxDefaultDepth.
var errTooDeep = fmt.Errorf("%w: it nests more than %d deep", errDefault, maxDefaultDepth)

// unwrapDefault removes one level that adds nothing to a value: a pair of
// parentheses around all of part, or a leading plus sign.
func unwrapDefault(part expression) (expression, bool) {
	if part.size() > 1 && part.tokens[0].isPunctuation("+") {
		return part.slice(1, part.size()), true
	}
	return part.unwrapped()
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
func negatedDefault(operand expression, affinity Affinity, depth int) (Value, error) {
	literal := operand
	for range maxDefaultDepth - depth {
		inner, ok := literal.unwrapped()
		if !ok {
			break
		}
		literal = inner
	}
	if literal.size() == 1 && literal.tokens[0].kind == number && negatedInOneStep(literal.tokens[0].text) {
		return numberLiteral(literal.tokens[0].text, true, affinity)
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

// castDefault evaluates "CAST ( expression AS type )", rest following the
// word CAST, as SQLite does for a DEFAULT: the operand's value under the
// affinity of the type, converted to the type as castValue converts it,
// then the column's affinity applied. A CAST that is only the start of a
// larger expression, such as one with COLLATE or + 1 after it, is NULL,
// as every expression SQLite does not evaluate is. The type follows the
// last AS, found from the end, so each CAST reads only its own type.
func castDefault(rest expression, affinity Affinity, depth int) (Value, error) {
	inner, whole := rest.unwrapped()
	if !whole {
		return Value{}, nil
	}
	separator := -1
	for i := inner.size() - 1; i >= 0 && separator < 0; i-- {
		if inner.tokens[i].isKeyword("AS") {
			separator = i
		}
	}
	if separator < 1 || separator == inner.size()-1 {
		return Value{}, errDefault
	}
	target := affinityOf(joinTokens(inner.tokens[separator+1:]), false)
	value, err := evaluateDefault(inner.slice(0, separator), target, depth)
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
