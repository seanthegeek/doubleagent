package sqlitefile

import (
	"errors"
	"fmt"
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
	// defaultText holds the DEFAULT clause's text after the keyword, its
	// tokens scanned and evaluated once the whole statement is read and
	// the affinity known, so a statement refused later has kept none of
	// them; defaultErr is set when it is a form SQLite rejects, and
	// reading a short record that needs it fails with it.
	defaultText string
	defaultErr  error
}

// tableShape is a parsed CREATE TABLE statement: the columns in declared
// order, the table options and, for a WITHOUT ROWID table, the primary key
// columns, which its records store first.
type tableShape struct {
	name    string
	columns []Column
	// byName maps each folded column name to its column.
	byName       map[string]int
	withoutRowid bool
	strict       bool
	primaryKey   []int
	// autoincrement is set when the rowid alias is AUTOINCREMENT.
	autoincrement bool
	// fts5 is set for a CREATE VIRTUAL TABLE ... USING fts5 statement.
	fts5 bool
}

// maxColumns is the most columns a table, an fts5 table or a PRIMARY KEY
// may list: SQLITE_MAX_COLUMN as SQLite builds by default. A build may raise
// it as far as 32767, but a default build refuses to read a schema holding
// a wider table, so a database that needs a raised build is not read here
// either. The bound keeps the cost of a schema, and of each row, in
// proportion to the file.
const maxColumns = 2000

// errSchemaSyntax is wrapped by every error from CREATE TABLE text this
// package cannot read.
var errSchemaSyntax = errors.New("cannot read the CREATE TABLE statement")

// schemaParser reads one CREATE statement a token at a time, scanning
// each from the text only when the grammar asks for it, so a refusal (an
// unclosed parenthesis, a table past maxColumns, a malformed token) stops
// the reading where it is made and the tokens held are those of clauses
// already read whole. err is the first token that does not scan; the
// parser then sees the end of the text, and the statement fails with err.
type schemaParser struct {
	text   string
	offset int
	// ahead is the token scanned from aheadFrom, ending at aheadEnd.
	ahead               token
	aheadFrom, aheadEnd int
	err                 error
}

// newSchemaParser returns a parser at the start of sql.
func newSchemaParser(sql string) *schemaParser {
	return &schemaParser{text: sql, aheadFrom: -1}
}

// scan returns the token at or after offset and the offset after it,
// recording the first error and giving the end of the text in its place.
func (p *schemaParser) scan(offset int) (token, int) {
	next, end, err := scanFrom(p.text, offset)
	if err != nil && p.err == nil {
		p.err = fmt.Errorf("%w: %w", errSchemaSyntax, err)
	}
	return next, end
}

// peek returns the next token, or a zero token at the end.
func (p *schemaParser) peek() token {
	if p.aheadFrom != p.offset {
		p.ahead, p.aheadEnd = p.scan(p.offset)
		p.aheadFrom = p.offset
	}
	return p.ahead
}

// peekSecond returns the token after the next one.
func (p *schemaParser) peekSecond() token {
	p.peek()
	second, _ := p.scan(p.aheadEnd)
	return second
}

// atEnd reports whether the text holds no more tokens.
func (p *schemaParser) atEnd() bool {
	return p.peek().kind == 0
}

// next returns the next token and moves past it.
func (p *schemaParser) next() token {
	current := p.peek()
	p.offset = p.aheadEnd
	return current
}

// accept moves past the next token when it is the keyword.
func (p *schemaParser) accept(keyword string) bool {
	if p.peek().isKeyword(keyword) {
		p.next()
		return true
	}
	return false
}

// acceptPunctuation moves past the next token when it is the mark.
func (p *schemaParser) acceptPunctuation(mark string) bool {
	if p.peek().isPunctuation(mark) {
		p.next()
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
	if p.atEnd() {
		return fmt.Errorf("%w: the text ends where %s is expected", errSchemaSyntax, wanted)
	}
	return fmt.Errorf("%w: %q where %s is expected", errSchemaSyntax, p.peek().text, wanted)
}

// identifier reads a name: a bare word, a quoted identifier or a string.
func (p *schemaParser) identifier(what string) (string, error) {
	if !isName(p.peek()) {
		return "", p.unexpected(what)
	}
	return p.next().name(), nil
}

// group reads a parenthesised run of tokens, nested parentheses included,
// and returns the tokens inside the outer pair. The run is first read
// without keeping its tokens, so an unclosed one is refused having held
// none of them.
func (p *schemaParser) group() ([]token, error) {
	start := p.offset
	if err := p.skipGroup(); err != nil {
		return nil, err
	}
	end := p.offset
	p.offset = start
	p.next() // the opening parenthesis
	var inner []token
	for p.offset < end {
		inner = append(inner, p.next())
	}
	return inner[:len(inner)-1], nil
}

// skipGroup moves past a parenthesised run of tokens, nested parentheses
// included, keeping none of them.
func (p *schemaParser) skipGroup() error {
	_, err := p.skipGroupItems()
	return err
}

// skipGroupItems moves past a parenthesised run of tokens as skipGroup
// does and returns how many items its commas outside nested parentheses
// part it into, as splitTopLevel parts the tokens inside the pair.
func (p *schemaParser) skipGroupItems() (int, error) {
	if !p.acceptPunctuation("(") {
		return 0, p.unexpected("(")
	}
	items := 1
	for depth := 1; !p.atEnd(); {
		switch current := p.next(); {
		case current.isPunctuation("("):
			depth++
		case current.isPunctuation(")"):
			if depth--; depth == 0 {
				return items, nil
			}
		case current.isPunctuation(",") && depth == 1:
			items++
		}
	}
	return 0, fmt.Errorf("%w: a parenthesis is not closed", errSchemaSyntax)
}

// parseCreateTable parses the SQL text of a table's schema row. A token
// that does not scan fails the statement wherever it stands, the text
// after a whole fts5 declaration included.
func parseCreateTable(sql string) (*tableShape, error) {
	parser := newSchemaParser(sql)
	shape, err := parser.statement()
	for err == nil && parser.err == nil && !parser.atEnd() {
		parser.next()
	}
	if parser.err != nil {
		return nil, parser.err
	}
	return shape, err
}

// statement reads CREATE [TEMP] TABLE or CREATE VIRTUAL TABLE.
func (p *schemaParser) statement() (*tableShape, error) {
	if err := p.expect("CREATE"); err != nil {
		return nil, err
	}
	_ = p.accept("TEMP") || p.accept("TEMPORARY")
	if p.accept("VIRTUAL") {
		return p.virtualTable()
	}
	if err := p.expect("TABLE"); err != nil {
		return nil, err
	}
	shape := &tableShape{}
	var err error
	if shape.name, err = p.tableName(); err != nil {
		return nil, err
	}
	if err := p.definitions(shape); err != nil {
		return nil, err
	}
	if err := p.options(shape); err != nil {
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
	for !p.atEnd() {
		switch {
		case p.accept("WITHOUT"):
			if err := p.expect("ROWID"); err != nil {
				return err
			}
			shape.withoutRowid = true
		case p.accept("STRICT"):
			shape.strict = true
		case p.acceptPunctuation(";") && p.atEnd():
			return nil
		default:
			return p.unexpected("WITHOUT ROWID or STRICT")
		}
		if !p.acceptPunctuation(",") && !p.atEnd() && !p.peek().isPunctuation(";") {
			return p.unexpected(",")
		}
	}
	return nil
}

// columnIndex returns the index of the column called name, matched with
// ASCII case folding as SQL matches identifiers, or -1.
func (s *tableShape) columnIndex(name string) int {
	if index, ok := s.byName[FoldASCII(name)]; ok {
		return index
	}
	return -1
}

// addColumn appends a column, refusing one past maxColumns and a name
// already declared in any ASCII case, which SQLite refuses too.
func (s *tableShape) addColumn(column Column) error {
	if len(s.columns) == maxColumns {
		return fmt.Errorf("%w: table %s has more than %d columns", errSchemaSyntax, s.name, maxColumns)
	}
	if s.byName == nil {
		s.byName = map[string]int{}
	}
	key := FoldASCII(column.Name)
	if _, taken := s.byName[key]; taken {
		return fmt.Errorf("%w: duplicate column name: %s", errSchemaSyntax, column.Name)
	}
	s.byName[key] = len(s.columns)
	s.columns = append(s.columns, column)
	return nil
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
		if column.defaultText != "" {
			column.Default, column.defaultErr = readDefault(column.defaultText, column.Affinity)
			column.defaultText = ""
		}
		if s.withoutRowid {
			column.RowidAlias = false
		}
	}
	if s.withoutRowid && len(s.primaryKey) == 0 {
		return fmt.Errorf("%w: a WITHOUT ROWID table has no PRIMARY KEY", errSchemaSyntax)
	}
	if s.withoutRowid && s.autoincrement {
		return fmt.Errorf("%w: AUTOINCREMENT is not allowed on a WITHOUT ROWID table", errSchemaSyntax)
	}
	return nil
}
