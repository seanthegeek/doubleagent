package sqlitefile

// expression is the tokens of a DEFAULT clause, or of a part of one, with
// for each opening parenthesis how many tokens after it its closing one
// stands (0 for every other token and for one never closed), so a pair of
// parentheses around a part is found without reading the tokens inside.
type expression struct {
	tokens  []token
	partner []int
}

// pairParentheses returns tokens as an expression with each opening
// parenthesis paired with its closing one.
func pairParentheses(tokens []token) expression {
	clause := expression{tokens: tokens, partner: make([]int, len(tokens))}
	var opened []int
	for i, current := range tokens {
		switch {
		case current.isPunctuation("("):
			opened = append(opened, i)
		case current.isPunctuation(")") && len(opened) > 0:
			last := opened[len(opened)-1]
			clause.partner[last] = i - last
			opened = opened[:len(opened)-1]
		}
	}
	return clause
}

// size returns the number of tokens of the expression.
func (e expression) size() int { return len(e.tokens) }

// slice returns the tokens from position start up to position end.
func (e expression) slice(start, end int) expression {
	return expression{tokens: e.tokens[start:end], partner: e.partner[start:end]}
}

// unwrapped returns the expression inside a pair of parentheses around all
// of it, or the expression and false when there is none.
func (e expression) unwrapped() (expression, bool) {
	if e.size() < 2 || !e.tokens[0].isPunctuation("(") || e.partner[0] != e.size()-1 {
		return e, false
	}
	return e.slice(1, e.size()-1), true
}

// nullTest returns the operand of a test for NULL that ends part, and
// whether it tests for NOT NULL: operand ISNULL, IS NULL, NOTNULL, NOT
// NULL or IS NOT NULL. It is false when part ends in none of them after
// an operand.
func nullTest(part expression) (expression, bool, bool) {
	size := part.size()
	ends := func(back int, keyword string) bool { return part.tokens[size-back].isKeyword(keyword) }
	switch {
	case size > 1 && ends(1, "ISNULL"):
		return part.slice(0, size-1), false, true
	case size > 1 && ends(1, "NOTNULL"):
		return part.slice(0, size-1), true, true
	case size < 3 || !ends(1, "NULL"):
		return part, false, false
	case ends(2, "IS"):
		return part.slice(0, size-2), false, true
	case size > 3 && ends(2, "NOT") && ends(3, "IS"):
		return part.slice(0, size-3), true, true
	case ends(2, "NOT"):
		return part.slice(0, size-2), true, true
	}
	return part, false, false
}

// nullTestDefault evaluates a test for NULL of operand as SQLite 3.53.4
// reads one in a DEFAULT. Its parser folds the test to the integer 0 (IS
// NULL) or 1 (NOT NULL) when the operand is a number, string or blob
// literal under any signs and parentheses, or such a folded test itself;
// the column's affinity then applies as to any integer. Any other test is
// not evaluated, so it is NULL, as SQLite 3.51 reads every one.
func nullTestDefault(operand expression, notNull bool, affinity Affinity) Value {
	if !foldsToLiteral(operand) {
		return Value{}
	}
	result := int64(0)
	if notNull {
		result = 1
	}
	return applyAffinity(IntValue(result), affinity)
}

// foldsToLiteral reports whether operand is a number, string or blob
// literal under any signs, parentheses and tests for NULL. Each step takes
// at least one token off, so it costs the operand's length.
func foldsToLiteral(operand expression) bool {
	for operand.size() > 1 {
		if inner, ok := operand.unwrapped(); ok {
			operand = inner
		} else if first := operand.tokens[0]; first.isPunctuation("+") || first.isPunctuation("-") {
			operand = operand.slice(1, operand.size())
		} else if inner, _, ok := nullTest(operand); ok {
			operand = inner
		} else {
			return false
		}
	}
	return operand.size() == 1 && (operand.tokens[0].kind == number || operand.tokens[0].kind == stringToken ||
		operand.tokens[0].kind == blobToken)
}
