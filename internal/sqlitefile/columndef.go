package sqlitefile

import (
	"errors"
	"fmt"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// stopsType reports whether the token ends a type name: one of the
// reserved words that open a column constraint. Other keywords may be words
// of a type name, as SQLite's grammar lets them stand for identifiers.
func stopsType(next token) bool {
	if next.kind != word {
		return false
	}
	switch textlines.UpperASCII(next.text) {
	case "CONSTRAINT", "DEFAULT", "NOT", "NULL", "PRIMARY", "UNIQUE", "CHECK", "REFERENCES",
		"COLLATE", "AS":
		return true
	}
	return false
}

// columnDefinition reads one column: its name, its type and its constraints.
func (p *schemaParser) columnDefinition(shape *tableShape) error {
	name, err := p.identifier("a column name")
	if err != nil {
		return err
	}
	column := Column{Name: name}
	if column.DeclaredType, err = p.columnType(); err != nil {
		return err
	}
	for !p.atEnd() && !p.peek().isPunctuation(",") && !p.peek().isPunctuation(")") {
		if err := p.columnConstraint(shape, &column); err != nil {
			return fmt.Errorf("column %s: %w", name, err)
		}
	}
	return shape.addColumn(column)
}

// columnType reads a type name: identifiers and strings, then an optional
// parenthesised size. Like SQLite, it drops a closing "GENERATED ALWAYS",
// the optional start of a generated-column constraint, and it removes the
// quotes of a type written as one quoted name.
func (p *schemaParser) columnType() (string, error) {
	var words []token
	for {
		next := p.peek()
		if next.kind == quoted || next.kind == stringToken || next.kind == word && !stopsType(next) {
			words = append(words, p.next())
			continue
		}
		break
	}
	if n := len(words); n >= 2 && words[n-2].isKeyword("GENERATED") && words[n-1].isKeyword("ALWAYS") {
		words = words[:n-2]
	}
	if len(words) == 0 {
		return "", nil
	}
	if !p.peek().isPunctuation("(") {
		if len(words) == 1 {
			return words[0].name(), nil
		}
		return joinTokens(words), nil
	}
	size, err := p.group()
	if err != nil {
		return "", err
	}
	return joinTokens(words) + "(" + joinTokens(size) + ")", nil
}

// joinTokens writes tokens as text, separated by single spaces.
func joinTokens(tokens []token) string {
	texts := make([]string, len(tokens))
	for i, part := range tokens {
		texts[i] = part.text
	}
	return strings.Join(texts, " ")
}

// columnConstraint reads one column constraint into column.
func (p *schemaParser) columnConstraint(shape *tableShape, column *Column) error {
	before := p.offset
	switch next := p.next(); {
	case next.isKeyword("CONSTRAINT"):
		_, err := p.identifier("a constraint name")
		return err
	case next.isKeyword("PRIMARY"):
		return p.columnPrimaryKey(shape, column)
	case next.isKeyword("NOT"):
		if err := p.expect("NULL"); err != nil {
			return err
		}
		return p.conflictClause()
	case next.isKeyword("NULL") || next.isKeyword("UNIQUE"):
		return p.conflictClause()
	case next.isKeyword("CHECK"):
		return p.skipGroup()
	case next.isKeyword("DEFAULT"):
		text, err := p.defaultClause()
		column.defaultText = text
		return err
	case next.isKeyword("COLLATE"):
		name, err := p.identifier("a collation name")
		column.Collation = textlines.UpperASCII(name)
		return err
	case next.isKeyword("REFERENCES"):
		return p.foreignKey()
	case next.isKeyword("GENERATED"):
		if err := p.expect("ALWAYS"); err != nil {
			return err
		}
		if !p.peek().isKeyword("AS") {
			return p.unexpected("AS")
		}
		return nil
	case next.isKeyword("AS"):
		return p.generated(column)
	default:
		p.offset = before
		return p.unexpected("a column constraint")
	}
}

// columnPrimaryKey reads the rest of a PRIMARY KEY column constraint. The
// column is the rowid alias when its type is exactly INTEGER and the key is
// not DESC ("ROWIDs and the INTEGER PRIMARY KEY"), and only the rowid alias
// may be AUTOINCREMENT.
func (p *schemaParser) columnPrimaryKey(shape *tableShape, column *Column) error {
	if err := p.expect("KEY"); err != nil {
		return err
	}
	descending := p.accept("DESC")
	if !descending {
		p.accept("ASC")
	}
	if err := p.conflictClause(); err != nil {
		return err
	}
	autoincrement := p.accept("AUTOINCREMENT")
	if len(shape.primaryKey) > 0 {
		return fmt.Errorf("%w: more than one PRIMARY KEY", errSchemaSyntax)
	}
	shape.primaryKey = []int{len(shape.columns)}
	column.RowidAlias = textlines.EqualFoldASCII(column.DeclaredType, "INTEGER") && !descending
	return shape.setAutoincrement(autoincrement, column.RowidAlias)
}

// setAutoincrement records an AUTOINCREMENT key, which SQLite allows only
// on the rowid alias.
func (s *tableShape) setAutoincrement(autoincrement, rowidAlias bool) error {
	if autoincrement && !rowidAlias {
		return fmt.Errorf("%w: AUTOINCREMENT is only allowed on an INTEGER PRIMARY KEY", errSchemaSyntax)
	}
	s.autoincrement = autoincrement
	return nil
}

// generated reads AS (expression) [STORED | VIRTUAL], VIRTUAL by default.
func (p *schemaParser) generated(column *Column) error {
	if err := p.skipGroup(); err != nil {
		return err
	}
	column.Generated = VirtualGenerated
	if p.accept("STORED") {
		column.Generated = StoredGenerated
	} else {
		p.accept("VIRTUAL")
	}
	return nil
}

// conflictClause reads an optional ON CONFLICT clause.
func (p *schemaParser) conflictClause() error {
	if !p.accept("ON") {
		return nil
	}
	if err := p.expect("CONFLICT"); err != nil {
		return err
	}
	for _, action := range []string{"ROLLBACK", "ABORT", "FAIL", "IGNORE", "REPLACE"} {
		if p.accept(action) {
			return nil
		}
	}
	return p.unexpected("a conflict resolution")
}

// defaultClause moves past a DEFAULT value, a parenthesised expression, a
// signed term, or one literal or name, and returns its text, keeping none
// of its tokens.
func (p *schemaParser) defaultClause() (string, error) {
	start := p.offset
	next := p.peek()
	switch {
	case next.isPunctuation("("):
		if err := p.skipGroup(); err != nil {
			return "", err
		}
	case next.isPunctuation("+") || next.isPunctuation("-"):
		p.next()
		if term := p.peek(); term.kind == punctuation || term.kind == 0 {
			return "", p.unexpected("a default value")
		}
		p.next()
	case next.kind == punctuation || next.kind == 0:
		return "", p.unexpected("a default value")
	default:
		p.next()
	}
	return p.text[start:p.offset], nil
}

// foreignKey reads the rest of a REFERENCES clause: the parent table, its
// columns, the ON and MATCH actions and the deferral.
func (p *schemaParser) foreignKey() error {
	if _, err := p.identifier("a table name"); err != nil {
		return err
	}
	if p.peek().isPunctuation("(") {
		if err := p.skipGroup(); err != nil {
			return err
		}
	}
	for {
		switch {
		case p.accept("ON"):
			if !p.accept("DELETE") && !p.accept("UPDATE") {
				return p.unexpected("DELETE or UPDATE")
			}
			if err := p.foreignKeyAction(); err != nil {
				return err
			}
		case p.accept("MATCH"):
			if _, err := p.identifier("a match name"); err != nil {
				return err
			}
		default:
			return p.deferral()
		}
	}
}

// foreignKeyAction reads SET NULL, SET DEFAULT, CASCADE, RESTRICT or NO ACTION.
func (p *schemaParser) foreignKeyAction() error {
	switch {
	case p.accept("SET"):
		if !p.accept("NULL") && !p.accept("DEFAULT") {
			return p.unexpected("NULL or DEFAULT")
		}
	case p.accept("NO"):
		return p.expect("ACTION")
	case !p.accept("CASCADE") && !p.accept("RESTRICT"):
		return p.unexpected("a foreign key action")
	}
	return nil
}

// deferral reads an optional [NOT] DEFERRABLE [INITIALLY DEFERRED|IMMEDIATE];
// a NOT that is not followed by DEFERRABLE starts a NOT NULL constraint.
func (p *schemaParser) deferral() error {
	if p.peek().isKeyword("NOT") && p.peekSecond().isKeyword("DEFERRABLE") {
		p.next()
	}
	if !p.accept("DEFERRABLE") {
		return nil
	}
	if p.accept("INITIALLY") && !p.accept("DEFERRED") && !p.accept("IMMEDIATE") {
		return p.unexpected("DEFERRED or IMMEDIATE")
	}
	return nil
}

// tableConstraint reads one table constraint. A PRIMARY KEY sets the key
// columns; a single INTEGER column becomes the rowid alias whatever its
// sort order, unlike the column form.
func (p *schemaParser) tableConstraint(shape *tableShape) error {
	if p.accept("CONSTRAINT") {
		if _, err := p.identifier("a constraint name"); err != nil {
			return err
		}
	}
	switch {
	case p.accept("PRIMARY"):
		if err := p.expect("KEY"); err != nil {
			return err
		}
		return p.tablePrimaryKey(shape)
	case p.accept("UNIQUE"):
		if err := p.skipGroup(); err != nil {
			return err
		}
		return p.conflictClause()
	case p.accept("CHECK"):
		return p.skipGroup()
	case p.accept("FOREIGN"):
		if err := p.expect("KEY"); err != nil {
			return err
		}
		if err := p.skipGroup(); err != nil {
			return err
		}
		if err := p.expect("REFERENCES"); err != nil {
			return err
		}
		return p.foreignKey()
	}
	return p.unexpected("a table constraint")
}

// tablePrimaryKey reads the column list of a PRIMARY KEY table constraint,
// counting its items before keeping their tokens, so a list past
// maxColumns is refused having held none. A list of one INTEGER column
// makes it the rowid alias, whatever its sort order, and only then may
// AUTOINCREMENT follow it.
func (p *schemaParser) tablePrimaryKey(shape *tableShape) error {
	start := p.offset
	count, err := p.skipGroupItems()
	switch {
	case err != nil:
		return err
	case len(shape.primaryKey) > 0:
		return fmt.Errorf("%w: more than one PRIMARY KEY", errSchemaSyntax)
	case count > maxColumns:
		return fmt.Errorf("%w: PRIMARY KEY lists more than %d columns", errSchemaSyntax, maxColumns)
	}
	p.offset = start
	inner, err := p.group()
	if err != nil {
		return err
	}
	items := splitTopLevel(inner)
	listed := map[int]bool{}
	autoincrement := false
	for i, item := range items {
		name, last, ok := keyColumn(item)
		if !ok || last && i < len(items)-1 {
			return fmt.Errorf("%w: PRIMARY KEY holds an expression", errSchemaSyntax)
		}
		index := shape.columnIndex(name)
		if index < 0 {
			return fmt.Errorf("%w: PRIMARY KEY names no column %s", errSchemaSyntax, name)
		}
		if !listed[index] {
			listed[index] = true
			shape.primaryKey = append(shape.primaryKey, index)
		}
		autoincrement = last
	}
	alias := false
	if len(items) == 1 {
		column := &shape.columns[shape.primaryKey[0]]
		column.RowidAlias = textlines.EqualFoldASCII(column.DeclaredType, "INTEGER")
		alias = column.RowidAlias
	}
	if err := shape.setAutoincrement(autoincrement, alias); err != nil {
		return err
	}
	return p.conflictClause()
}

// keyColumn reads one item of a PRIMARY KEY's column list as SQLite's
// grammar has it: a column name, inside any parentheses that hold only it
// and COLLATE clauses, then any COLLATE clauses, then ASC or DESC, then
// AUTOINCREMENT. It returns the name and whether AUTOINCREMENT ends the
// item; any other item is an expression, which SQLite refuses in a key.
func keyColumn(item []token) (string, bool, bool) {
	matches := closingParentheses(item)
	start, end := 0, len(item)
	var rest []token
	for outer := true; start < end; outer = false {
		termEnd := start + 1
		if item[start].isPunctuation("(") {
			termEnd = matches[start] + 1
		} else if !isName(item[start]) {
			return "", false, false
		}
		if termEnd <= start || termEnd > end {
			return "", false, false
		}
		after := withoutCollations(item[termEnd:end])
		if outer {
			rest = after
		} else if len(after) > 0 {
			return "", false, false
		}
		if termEnd == start+1 {
			autoincrement, ok := keyItemEnd(rest)
			return item[start].name(), autoincrement, ok
		}
		start, end = start+1, termEnd-1
	}
	return "", false, false
}

// keyItemEnd reads what may follow a key column and its collations: ASC or
// DESC, then AUTOINCREMENT. It reports whether AUTOINCREMENT is there and
// whether nothing else is.
func keyItemEnd(rest []token) (bool, bool) {
	if len(rest) > 0 && (rest[0].isKeyword("ASC") || rest[0].isKeyword("DESC")) {
		rest = rest[1:]
	}
	autoincrement := len(rest) > 0 && rest[0].isKeyword("AUTOINCREMENT")
	if autoincrement {
		rest = rest[1:]
	}
	return autoincrement, len(rest) == 0
}

// withoutCollations removes the COLLATE name clauses that start tokens.
func withoutCollations(tokens []token) []token {
	for len(tokens) >= 2 && tokens[0].isKeyword("COLLATE") && isName(tokens[1]) {
		tokens = tokens[2:]
	}
	return tokens
}

// isName reports whether the token can be a name: a bare word, a quoted
// identifier or a string.
func isName(current token) bool {
	return current.kind == word || current.kind == quoted || current.kind == stringToken
}

// closingParentheses returns, for each opening parenthesis of tokens, the
// index of the one that closes it, and -1 for one left open and for every
// other token, in one pass however deeply they nest.
func closingParentheses(tokens []token) []int {
	matches := make([]int, len(tokens))
	var open []int
	for i, current := range tokens {
		matches[i] = -1
		switch {
		case current.isPunctuation("("):
			open = append(open, i)
		case current.isPunctuation(")") && len(open) > 0:
			matches[open[len(open)-1]] = i
			open = open[:len(open)-1]
		}
	}
	return matches
}

// splitTopLevel splits tokens at the commas outside parentheses.
func splitTopLevel(tokens []token) [][]token {
	var parts [][]token
	depth, start := 0, 0
	for i, current := range tokens {
		switch {
		case current.isPunctuation("("):
			depth++
		case current.isPunctuation(")"):
			depth--
		case current.isPunctuation(",") && depth == 0:
			parts = append(parts, tokens[start:i])
			start = i + 1
		}
	}
	return append(parts, tokens[start:])
}

// virtualTable reads CREATE VIRTUAL TABLE name USING module(arguments). Only
// fts5 is read: its columns are the arguments that are not options, and a
// table whose content lives elsewhere (content=) is refused.
func (p *schemaParser) virtualTable() (*tableShape, error) {
	if err := p.expect("TABLE"); err != nil {
		return nil, err
	}
	shape := &tableShape{fts5: true}
	var err error
	if shape.name, err = p.tableName(); err != nil {
		return nil, err
	}
	if err := p.expect("USING"); err != nil {
		return nil, err
	}
	module, err := p.identifier("a module name")
	if err != nil {
		return nil, err
	}
	if !textlines.EqualFoldASCII(module, "fts5") {
		return nil, errors.New("virtual table module " + module + " is not supported")
	}
	var arguments []token
	if p.peek().isPunctuation("(") {
		if arguments, err = p.group(); err != nil {
			return nil, err
		}
	}
	for _, argument := range splitTopLevel(arguments) {
		if err := shape.fts5Argument(argument); err != nil {
			return nil, err
		}
	}
	return shape, shape.finish()
}

// fts5Argument adds one fts5 argument: a column declaration (a name, then
// options such as UNINDEXED) or a name=value option ("FTS5 Table Creation
// and Initialization").
func (s *tableShape) fts5Argument(argument []token) error {
	if len(argument) == 0 {
		return fmt.Errorf("%w: an empty fts5 argument", errSchemaSyntax)
	}
	if len(argument) >= 2 && argument[1].isPunctuation("=") {
		option := textlines.LowerASCII(argument[0].name())
		if option == "content" || option == "contentless_delete" || option == "contentless_unindexed" {
			return errors.New("fts5 table " + s.name + " keeps no content table of its own (" + option + "=)")
		}
		return nil
	}
	if kind := argument[0].kind; kind != word && kind != quoted && kind != stringToken {
		return fmt.Errorf("%w: fts5 argument %q", errSchemaSyntax, argument[0].text)
	}
	return s.addColumn(Column{Name: argument[0].name()})
}
