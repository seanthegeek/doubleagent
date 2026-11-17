package sqlitefile

import (
	"fmt"
	"strings"
)

// stopsType reports whether the token ends a type name: one of the
// reserved words that open a column constraint. Other keywords may be words
// of a type name, as SQLite's grammar lets them stand for identifiers.
func stopsType(next token) bool {
	if next.kind != word {
		return false
	}
	switch strings.ToUpper(next.text) {
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
	for p.position < len(p.tokens) && !p.peek().isPunctuation(",") && !p.peek().isPunctuation(")") {
		if err := p.columnConstraint(shape, &column); err != nil {
			return fmt.Errorf("column %s: %w", name, err)
		}
	}
	shape.columns = append(shape.columns, column)
	return nil
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
		_, err := p.group()
		return err
	case next.isKeyword("DEFAULT"):
		expression, err := p.defaultClause()
		column.defaultExpression = expression
		return err
	case next.isKeyword("COLLATE"):
		name, err := p.identifier("a collation name")
		column.Collation = strings.ToUpper(name)
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
		p.position--
		return p.unexpected("a column constraint")
	}
}

// columnPrimaryKey reads the rest of a PRIMARY KEY column constraint. The
// column is the rowid alias when its type is exactly INTEGER and the key is
// not DESC ("ROWIDs and the INTEGER PRIMARY KEY").
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
	p.accept("AUTOINCREMENT")
	if len(shape.primaryKey) > 0 {
		return fmt.Errorf("%w: more than one PRIMARY KEY", errSchemaSyntax)
	}
	shape.primaryKey = []int{len(shape.columns)}
	column.RowidAlias = strings.EqualFold(column.DeclaredType, "INTEGER") && !descending
	return nil
}

// generated reads AS (expression) [STORED | VIRTUAL], VIRTUAL by default.
func (p *schemaParser) generated(column *Column) error {
	if _, err := p.group(); err != nil {
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

// defaultClause returns the tokens of a DEFAULT value: a parenthesised
// expression with its parentheses, a signed term, or one literal or name.
func (p *schemaParser) defaultClause() ([]token, error) {
	start := p.position
	next := p.peek()
	switch {
	case next.isPunctuation("("):
		if _, err := p.group(); err != nil {
			return nil, err
		}
	case next.isPunctuation("+") || next.isPunctuation("-"):
		p.position++
		if term := p.peek(); term.kind == punctuation || term.kind == 0 {
			return nil, p.unexpected("a default value")
		}
		p.position++
	case next.kind == punctuation || next.kind == 0:
		return nil, p.unexpected("a default value")
	default:
		p.position++
	}
	return p.tokens[start:p.position], nil
}

// foreignKey reads the rest of a REFERENCES clause: the parent table, its
// columns, the ON and MATCH actions and the deferral.
func (p *schemaParser) foreignKey() error {
	if _, err := p.identifier("a table name"); err != nil {
		return err
	}
	if p.peek().isPunctuation("(") {
		if _, err := p.group(); err != nil {
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
	if p.peek().isKeyword("NOT") && p.position+1 < len(p.tokens) && p.tokens[p.position+1].isKeyword("DEFERRABLE") {
		p.position++
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
		if _, err := p.group(); err != nil {
			return err
		}
		return p.conflictClause()
	case p.accept("CHECK"):
		_, err := p.group()
		return err
	case p.accept("FOREIGN"):
		if err := p.expect("KEY"); err != nil {
			return err
		}
		if _, err := p.group(); err != nil {
			return err
		}
		if err := p.expect("REFERENCES"); err != nil {
			return err
		}
		return p.foreignKey()
	}
	return p.unexpected("a table constraint")
}

// tablePrimaryKey reads the column list of a PRIMARY KEY table constraint.
func (p *schemaParser) tablePrimaryKey(shape *tableShape) error {
	inner, err := p.group()
	if err != nil {
		return err
	}
	if len(shape.primaryKey) > 0 {
		return fmt.Errorf("%w: more than one PRIMARY KEY", errSchemaSyntax)
	}
	for _, item := range splitTopLevel(inner) {
		name, ok := keyColumnName(item)
		if !ok {
			return fmt.Errorf("%w: PRIMARY KEY holds an expression", errSchemaSyntax)
		}
		index := shape.columnIndex(name)
		if index < 0 {
			return fmt.Errorf("%w: PRIMARY KEY names no column %s", errSchemaSyntax, name)
		}
		if !containsIndex(shape.primaryKey, index) {
			shape.primaryKey = append(shape.primaryKey, index)
		}
	}
	if len(shape.primaryKey) == 1 {
		column := &shape.columns[shape.primaryKey[0]]
		column.RowidAlias = strings.EqualFold(column.DeclaredType, "INTEGER")
	}
	return p.conflictClause()
}

// keyColumnName reads one item of a key's column list: a column name, in
// any number of parentheses, then an optional COLLATE name and ASC or DESC.
// Any other item is an expression, which SQLite refuses in a PRIMARY KEY.
func keyColumnName(item []token) (string, bool) {
	end := 0
	for depth := 0; end < len(item); end++ {
		if item[end].isPunctuation("(") {
			depth++
		} else if item[end].isPunctuation(")") {
			depth--
		}
		if depth == 0 {
			break
		}
	}
	if end >= len(item) {
		return "", false
	}
	name := withoutOuterParentheses(item[:end+1])
	rest := item[end+1:]
	if len(name) != 1 || name[0].kind != word && name[0].kind != quoted && name[0].kind != stringToken {
		return "", false
	}
	if len(rest) >= 2 && rest[0].isKeyword("COLLATE") {
		rest = rest[2:]
	}
	if len(rest) == 1 && (rest[0].isKeyword("ASC") || rest[0].isKeyword("DESC")) {
		rest = rest[1:]
	}
	return name[0].name(), len(rest) == 0
}

// containsIndex reports whether list holds index.
func containsIndex(list []int, index int) bool {
	for _, item := range list {
		if item == index {
			return true
		}
	}
	return false
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
	if !strings.EqualFold(module, "fts5") {
		return nil, fmt.Errorf("virtual table module %s is not supported", module)
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
		option := strings.ToLower(argument[0].name())
		if option == "content" || option == "contentless_delete" || option == "contentless_unindexed" {
			return fmt.Errorf("fts5 table %s keeps no content table of its own (%s=)", s.name, option)
		}
		return nil
	}
	if kind := argument[0].kind; kind != word && kind != quoted && kind != stringToken {
		return fmt.Errorf("%w: fts5 argument %q", errSchemaSyntax, argument[0].text)
	}
	s.columns = append(s.columns, Column{Name: argument[0].name()})
	return nil
}
