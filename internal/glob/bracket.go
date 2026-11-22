package glob

import "strings"

// elementKind says what one item of a bracket expression is.
type elementKind int

const (
	elementByte        elementKind = iota // a byte, a `\` escape or a `[.x.]` symbol
	elementEquivalence                    // `[=x=]`, which under LC_ALL=C is the byte x
	elementClass                          // `[:name:]`
	elementInvalid                        // a construct the shell rejects
)

// bracketElement is one parsed item of a bracket expression.
type bracketElement struct {
	kind  elementKind
	value byte
	class string
	next  int
}

// matchBracket matches char against the bracket expression whose body starts
// at bodyAt, just after its `[`. Items are tried in order and the first that
// matches ends the test, the rest only scanned for the closing `]`, so a
// malformed item after a match goes unnoticed, as in the shell; a malformed
// item reached before one fails the whole test. Only `!` negates: the shell
// reads `[^...]` as a class holding `^`.
func matchBracket(pattern string, bodyAt int, char byte) (int, stepOutcome) {
	position, negate := bodyAt, false
	if position < len(pattern) && pattern[position] == '!' {
		position, negate = position+1, true
	}
	for first := true; ; first = false {
		if position == len(pattern) {
			return 0, stepInvalid
		}
		if pattern[position] == ']' && !first {
			return position + 1, outcomeOf(negate)
		}
		next, matched := matchBracketItem(pattern, position, char)
		if next < 0 {
			return 0, stepInvalid
		}
		if matched {
			end := skipBracket(pattern, next)
			if end < 0 {
				return 0, stepInvalid
			}
			return end, outcomeOf(!negate)
		}
		position = next
	}
}

// outcomeOf turns a boolean match into a step outcome.
func outcomeOf(matched bool) stepOutcome {
	if matched {
		return stepMatched
	}
	return stepMismatched
}

// matchBracketItem matches char against the item, or range of two items, at
// at. It returns the index after it, or -1 when the item is malformed or a
// class or equivalence ends a range.
func matchBracketItem(pattern string, position int, char byte) (int, bool) {
	low := parseElement(pattern, position)
	if low.kind == elementInvalid {
		return -1, false
	}
	isRange := low.next+1 < len(pattern) && pattern[low.next] == '-' &&
		pattern[low.next+1] != ']'
	if !isRange {
		if low.kind == elementClass {
			return low.next, inClass(low.class, char)
		}
		return low.next, char == low.value
	}
	high := parseElement(pattern, low.next+1)
	if low.kind != elementByte || high.kind != elementByte {
		return -1, false
	}
	return high.next, low.value <= char && char <= high.value
}

// parseElement reads one item of a bracket expression at position: `[:name:]`,
// `[.x.]`, `[=x=]`, a `\` escape or a plain byte. A `[` followed by anything
// else is a plain `[`.
func parseElement(pattern string, position int) bracketElement {
	if position+1 < len(pattern) && pattern[position] == '[' {
		switch delimiter := pattern[position+1]; delimiter {
		case ':', '.', '=':
			return parseDelimited(pattern, position+2, delimiter)
		}
	}
	if pattern[position] == '\\' && position+1 < len(pattern) {
		return bracketElement{kind: elementByte, value: pattern[position+1], next: position + 2}
	}
	return bracketElement{kind: elementByte, value: pattern[position], next: position + 1}
}

// parseDelimited reads the body of `[:name:]`, `[.x.]` or `[=x=]` from position up
// to its closing delimiter and `]`. A symbol or equivalence must be one
// ASCII byte, the C locale knowing no other, and a class one of the POSIX
// names; anything else is invalid.
func parseDelimited(pattern string, position int, delimiter byte) bracketElement {
	length := strings.Index(pattern[position:], string([]byte{delimiter, ']'}))
	if length < 0 {
		return bracketElement{kind: elementInvalid}
	}
	body, next := pattern[position:position+length], position+length+2
	switch {
	case delimiter == ':' && isClassName(body):
		return bracketElement{kind: elementClass, class: body, next: next}
	case len(body) != 1 || body[0] >= 0x80:
		return bracketElement{kind: elementInvalid}
	case delimiter == '.':
		return bracketElement{kind: elementByte, value: body[0], next: next}
	case delimiter == '=':
		return bracketElement{kind: elementEquivalence, value: body[0], next: next}
	}
	return bracketElement{kind: elementInvalid}
}

// skipBracket returns the index after the `]` that closes a bracket
// expression whose matching item ended at position, or -1 when none closes it. The
// shell's skip steps over a `[:`, `[.` or `[=` up to the first `]` after it
// when that `]` follows the same delimiter, valid or not, and otherwise
// closes the bracket at that `]`. It reads a backslash as a plain byte, so
// `[a\]` matches "a" although the matching side reads `\]` as an escaped `]`.
func skipBracket(pattern string, position int) int {
	for position < len(pattern) {
		if pattern[position] == ']' {
			return position + 1
		}
		if !opensDelimited(pattern, position) {
			position++
			continue
		}
		closing := strings.IndexByte(pattern[position+2:], ']')
		if closing < 0 {
			return -1
		}
		end := position + 2 + closing
		if closing == 0 || pattern[end-1] != pattern[position+1] {
			return end + 1
		}
		position = end + 1
	}
	return -1
}

// opensDelimited reports whether `[:`, `[.` or `[=` starts at position.
func opensDelimited(pattern string, position int) bool {
	return position+1 < len(pattern) && pattern[position] == '[' &&
		strings.IndexByte(":.=", pattern[position+1]) >= 0
}

// isClassName reports whether name is one of the twelve POSIX character
// classes.
func isClassName(name string) bool {
	switch name {
	case "alnum", "alpha", "blank", "cntrl", "digit", "graph",
		"lower", "print", "punct", "space", "upper", "xdigit":
		return true
	}
	return false
}

// inClass reports whether char belongs to the named class in the C locale,
// where no byte above 0x7f belongs to any class.
func inClass(class string, char byte) bool {
	isUpper := 'A' <= char && char <= 'Z'
	isLower := 'a' <= char && char <= 'z'
	isDigit := '0' <= char && char <= '9'
	isGraph := '!' <= char && char <= '~'
	switch class {
	case "alnum":
		return isUpper || isLower || isDigit
	case "alpha":
		return isUpper || isLower
	case "blank":
		return char == ' ' || char == '\t'
	case "cntrl":
		return char < ' ' || char == 0x7f
	case "digit":
		return isDigit
	case "graph":
		return isGraph
	case "lower":
		return isLower
	case "print":
		return isGraph || char == ' '
	case "punct":
		return isGraph && !isUpper && !isLower && !isDigit
	case "space":
		return char == ' ' || ('\t' <= char && char <= '\r')
	case "upper":
		return isUpper
	}
	return 'a' <= char && char <= 'f' || 'A' <= char && char <= 'F' || isDigit
}
