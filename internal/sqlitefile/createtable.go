package sqlitefile

import (
	"errors"
	"fmt"
	"strings"
)

// Generated says whether a column's value is computed from an expression
// ("Generated Columns", https://www.sqlite.org/gencol.html).
type Generated uint8

// A stored generated column is kept in the record like any other; a
// virtual one is not stored at all.
const (
	NotGenerated Generated = iota
	VirtualGenerated
	StoredGenerated
)

// Column is one column of a table as its CREATE TABLE text declares it.
type Column struct {
	Name string
	// DeclaredType is the type as written, "" when none.
	DeclaredType string
	Affinity     Affinity
	// Collation is the COLLATE name in upper case, "" for the default BINARY.
	Collation string
	// RowidAlias is set for the INTEGER PRIMARY KEY column, which reads as
	// the row's rowid.
	RowidAlias bool
	Generated  Generated
	// Default is what a record too short to hold the column reads as, the
	// state of rows written before an ALTER TABLE ADD COLUMN.
	Default Value
	// defaultExpression holds the DEFAULT clause's tokens, evaluated once
	// the affinity is known; defaultErr is set when it is a form SQLite
	// rejects, and reading a short record that needs it fails with it.
	defaultExpression []token
	defaultErr        error
}

// tableShape is a parsed CREATE TABLE statement: the columns in declared
// order, the table options and, for a WITHOUT ROWID table, the primary key
// columns, which its records store first.
type tableShape struct {
	name         string
	columns      []Column
	withoutRowid bool
	strict       bool
	primaryKey   []int
	// fts5 is set for a CREATE VIRTUAL TABLE ... USING fts5 statement.
	fts5 bool
}

// errSchemaSyntax is wrapped by every error from CREATE TABLE text this
// package cannot read.
var errSchemaSyntax = errors.New("cannot read the CREATE TABLE statement")

// schemaParser walks the tokens of one CREATE statement.
type schemaParser struct {
	tokens   []token
	position int
}

// peek returns the next token, or a zero token at the end.
func (p *schemaParser) peek() token {
	if p.position < len(p.tokens) {
		return p.tokens[p.position]
	}
	return token{}
}

// next returns the next token and moves past it.
func (p *schemaParser) next() token {
	current := p.peek()
	if p.position < len(p.tokens) {
		p.position++
	}
	return current
}

// accept moves past the next token when it is the keyword.
func (p *schemaParser) accept(keyword string) bool {
	if p.peek().isKeyword(keyword) {
		p.position++
		return true
	}
	return false
}

// acceptPunctuation moves past the next token when it is the mark.
func (p *schemaParser) acceptPunctuation(mark string) bool {
	if p.peek().isPunctuation(mark) {
		p.position++
		return true
	}
	return false
}

// expect moves past the keyword or reports what stands in its place.
func (p *schemaParser) expect(keywords ...string) error {
	for _, keyword := range keywords {
		if !p.accept(keyword) {
			return p.unexpected(keyword)
		}
	}
	return nil
}

// unexpected reports the next token where wanted was expected.
func (p *schemaParser) unexpected(wanted string) error {
	if p.position >= len(p.tokens) {
		return fmt.Errorf("%w: the text ends where %s is expected", errSchemaSyntax, wanted)
	}
	return fmt.Errorf("%w: %q where %s is expected", errSchemaSyntax, p.peek().text, wanted)
}

// identifier reads a name: a bare word, a quoted identifier or a string.
func (p *schemaParser) identifier(what string) (string, error) {
	current := p.peek()
	if current.kind != word && current.kind != quoted && current.kind != stringToken {
		return "", p.unexpected(what)
	}
	p.position++
	return current.name(), nil
}

// group reads a parenthesised run of tokens, nested parentheses included,
// and returns the tokens inside the outer pair.
func (p *schemaParser) group() ([]token, error) {
	if !p.acceptPunctuation("(") {
		return nil, p.unexpected("(")
	}
	start, depth := p.position, 1
	for p.position < len(p.tokens) {
		current := p.next()
		switch {
		case current.isPunctuation("("):
			depth++
		case current.isPunctuation(")"):
			depth--
			if depth == 0 {
				return p.tokens[start : p.position-1], nil
			}
		}
	}
	return nil, fmt.Errorf("%w: a parenthesis is not closed", errSchemaSyntax)
}

// parseCreateTable parses the SQL text of a table's schema row.
func parseCreateTable(sql string) (*tableShape, error) {
	tokens, err := tokenize(sql)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errSchemaSyntax, err)
	}
	parser := &schemaParser{tokens: tokens}
	if err := parser.expect("CREATE"); err != nil {
		return nil, err
	}
	_ = parser.accept("TEMP") || parser.accept("TEMPORARY")
	if parser.accept("VIRTUAL") {
		return parser.virtualTable()
	}
	if err := parser.expect("TABLE"); err != nil {
		return nil, err
	}
	shape := &tableShape{}
	if shape.name, err = parser.tableName(); err != nil {
		return nil, err
	}
	if err := parser.definitions(shape); err != nil {
		return nil, err
	}
	if err := parser.options(shape); err != nil {
		return nil, err
	}
	return shape, shape.finish()
}

// tableName reads [IF NOT EXISTS] [schema.]name and returns the name.
func (p *schemaParser) tableName() (string, error) {
	if p.accept("IF") {
		if err := p.expect("NOT", "EXISTS"); err != nil {
			return "", err
		}
	}
	name, err := p.identifier("a table name")
	if err != nil {
		return "", err
	}
	if p.acceptPunctuation(".") {
		return p.identifier("a table name")
	}
	return name, nil
}

// definitions reads the parenthesised list of column definitions and then
// table constraints; once a constraint appears only constraints follow,
// and SQLite lets their commas be left out.
func (p *schemaParser) definitions(shape *tableShape) error {
	if !p.acceptPunctuation("(") {
		return p.unexpected("(")
	}
	constraints := false
	for {
		if constraints || startsTableConstraint(p.peek()) {
			constraints = true
			if err := p.tableConstraint(shape); err != nil {
				return err
			}
		} else if err := p.columnDefinition(shape); err != nil {
			return err
		}
		if p.acceptPunctuation(")") {
			return nil
		}
		if !p.acceptPunctuation(",") && !constraints {
			return p.unexpected(", or )")
		}
	}
}

// startsTableConstraint reports whether the token opens a table constraint.
func startsTableConstraint(next token) bool {
	for _, keyword := range []string{"CONSTRAINT", "PRIMARY", "UNIQUE", "CHECK", "FOREIGN"} {
		if next.isKeyword(keyword) {
			return true
		}
	}
	return false
}

// options reads the table options after the closing parenthesis.
func (p *schemaParser) options(shape *tableShape) error {
	for p.position < len(p.tokens) {
		switch {
		case p.accept("WITHOUT"):
			if err := p.expect("ROWID"); err != nil {
				return err
			}
			shape.withoutRowid = true
		case p.accept("STRICT"):
			shape.strict = true
		case p.acceptPunctuation(";") && p.position == len(p.tokens):
			return nil
		default:
			return p.unexpected("WITHOUT ROWID or STRICT")
		}
		if !p.acceptPunctuation(",") && p.position < len(p.tokens) && !p.peek().isPunctuation(";") {
			return p.unexpected(",")
		}
	}
	return nil
}

// columnIndex returns the index of the column called name, matched with
// ASCII case folding as SQL matches identifiers, or -1.
func (s *tableShape) columnIndex(name string) int {
	for i, column := range s.columns {
		if strings.EqualFold(column.Name, name) {
			return i
		}
	}
	return -1
}

// finish settles what depends on the whole statement: a STRICT table's
// affinities, and the rowid alias, which a WITHOUT ROWID table never has.
func (s *tableShape) finish() error {
	if len(s.columns) == 0 {
		return fmt.Errorf("%w: the table has no columns", errSchemaSyntax)
	}
	for i := range s.columns {
		column := &s.columns[i]
		column.Affinity = affinityOf(column.DeclaredType, s.strict)
		if column.defaultExpression != nil {
			column.Default, column.defaultErr = defaultValue(column.defaultExpression,
				column.Affinity)
			column.defaultExpression = nil
		}
		if s.withoutRowid {
			column.RowidAlias = false
		}
	}
	if s.withoutRowid && len(s.primaryKey) == 0 {
		return fmt.Errorf("%w: a WITHOUT ROWID table has no PRIMARY KEY", errSchemaSyntax)
	}
	return nil
}
