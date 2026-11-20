package sqlitefile

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestAffinityFollowsTheFiveRules(t *testing.T) {
	// The examples of "Affinity Name Examples" in datatype3.html.
	cases := []struct {
		declared string
		strict   bool
		want     Affinity
	}{
		{"INT", false, IntegerAffinity}, {"UNSIGNED BIG INT", false, IntegerAffinity},
		{"INT8", false, IntegerAffinity}, {"VARCHAR(255)", false, TextAffinity},
		{"NATIVE CHARACTER(70)", false, TextAffinity}, {"CLOB", false, TextAffinity},
		{"BLOB", false, BlobAffinity}, {"", false, BlobAffinity},
		{"DOUBLE PRECISION", false, RealAffinity}, {"FLOAT", false, RealAffinity},
		{"DECIMAL(10,5)", false, NumericAffinity}, {"BOOLEAN", false, NumericAffinity},
		{"DATETIME", false, NumericAffinity}, {"FLOATING POINT", false, IntegerAffinity},
		{"STRING", false, NumericAffinity}, {"CHARINT", false, IntegerAffinity},
		{"ANY", false, NumericAffinity}, {"ANY", true, BlobAffinity}, {"text", false, TextAffinity},
	}
	for _, test := range cases {
		t.Run("type "+test.declared+" has its affinity", func(t *testing.T) {
			if got := affinityOf(test.declared, test.strict); got != test.want {
				t.Errorf("affinityOf(%q, %v) = %s, want %s", test.declared, test.strict, got, test.want)
			}
		})
	}
}

// columnSummary writes a parsed column in one line for comparison.
func columnSummary(column Column) string {
	parts := []string{column.Name, column.DeclaredType, column.Affinity.String()}
	if column.RowidAlias {
		parts = append(parts, "alias")
	}
	if column.Collation != "" {
		parts = append(parts, column.Collation)
	}
	switch column.Generated {
	case VirtualGenerated:
		parts = append(parts, "virtual")
	case StoredGenerated:
		parts = append(parts, "stored")
	}
	return strings.Join(parts, "|")
}

func TestParseCreateTableReadsEachPart(t *testing.T) {
	cases := []struct {
		name, sql string
		columns   []string
		key       []int
		options   string
	}{
		{"types stop at a constraint keyword",
			"CREATE TABLE t(a VARCHAR(10) NOT NULL, b UNSIGNED BIG INT DEFAULT 0, c)",
			[]string{"a|VARCHAR(10)|TEXT", "b|UNSIGNED BIG INT|INTEGER", "c||BLOB"}, nil, ""},
		{"all four quote styles name columns",
			"CREATE TABLE \"t\"\"x\"([a b], `c``d`, 'e''f', \"g\" TEXT)",
			[]string{"a b||BLOB", "c`d||BLOB", "e'f||BLOB", "g|TEXT|TEXT"}, nil, ""},
		{"comments are skipped",
			"CREATE TABLE t(a -- note, with a comma\n INTEGER /* x, y */ PRIMARY KEY, b)",
			[]string{"a|INTEGER|INTEGER|alias", "b||BLOB"}, []int{0}, ""},
		{"generated columns are virtual unless stored",
			"CREATE TABLE t(a INT, b INT GENERATED ALWAYS AS (a*2), c AS (a) STORED, d AS (1) VIRTUAL)",
			[]string{"a|INT|INTEGER", "b|INT|INTEGER|virtual", "c||BLOB|stored", "d||BLOB|virtual"},
			nil, ""},
		{"a generated column without a type",
			"CREATE TABLE t(a, b GENERATED ALWAYS AS (a))", []string{"a||BLOB", "b||BLOB|virtual"}, nil, ""},
		{"foreign keys, checks and collations are read past",
			"CREATE TABLE t(a TEXT COLLATE nocase REFERENCES p(x) ON DELETE SET NULL ON UPDATE NO" +
				" ACTION MATCH simple NOT DEFERRABLE INITIALLY DEFERRED NOT NULL, b CHECK (b > 0)" +
				" UNIQUE ON CONFLICT IGNORE, FOREIGN KEY (a) REFERENCES p(x) DEFERRABLE)",
			[]string{"a|TEXT|TEXT|NOCASE", "b||BLOB"}, nil, ""},
		{"table constraints may omit their commas",
			"CREATE TABLE t(a, b, PRIMARY KEY((b) DESC, a COLLATE nocase ASC, b) UNIQUE(a) CHECK(a)" +
				" CONSTRAINT c UNIQUE (b))",
			[]string{"a||BLOB", "b||BLOB"}, []int{1, 0}, ""},
		{"WITHOUT ROWID and STRICT are read in either order",
			"CREATE TABLE IF NOT EXISTS main.t(k INTEGER PRIMARY KEY, v ANY) STRICT, WITHOUT ROWID;",
			[]string{"k|INTEGER|INTEGER", "v|ANY|BLOB"}, []int{0}, "strict withoutrowid"},
		{"a type spelled with GENERATED ALWAYS words",
			"CREATE TABLE t(a generated always)", []string{"a||BLOB"}, nil, ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			shape, err := parseCreateTable(test.sql)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, column := range shape.columns {
				got = append(got, columnSummary(column))
			}
			if strings.Join(got, "; ") != strings.Join(test.columns, "; ") {
				t.Errorf("columns = %q, want %q", got, test.columns)
			}
			if !slices.Equal(shape.primaryKey, test.key) {
				t.Errorf("primary key = %v, want %v", shape.primaryKey, test.key)
			}
			options := strings.TrimSpace(map[bool]string{true: "strict"}[shape.strict] + " " +
				map[bool]string{true: "withoutrowid"}[shape.withoutRowid])
			if options != test.options {
				t.Errorf("options = %q, want %q", options, test.options)
			}
		})
	}
}

func TestParseCreateTableRefusesWhatItCannotRead(t *testing.T) {
	cases := []struct {
		name, sql, want string
	}{
		{"a statement that is not CREATE", "SELECT 1", "CREATE is expected"},
		{"an unclosed column list", "CREATE TABLE t(a, b", "expected"},
		{"an unterminated string", "CREATE TABLE t(a DEFAULT 'x)", "unterminated"},
		{"an unknown column constraint", "CREATE TABLE t(a TEXT LIMIT 3)", "a column constraint"},
		{"a WITHOUT ROWID table without a key", "CREATE TABLE t(a) WITHOUT ROWID", "no PRIMARY KEY"},
		{"two primary keys", "CREATE TABLE t(a PRIMARY KEY, b, PRIMARY KEY(b))", "more than one"},
		{"a key naming no column", "CREATE TABLE t(a, PRIMARY KEY(z))", "names no column"},
		{"a key holding an expression", "CREATE TABLE t(a, PRIMARY KEY(a+1))", "expression"},
		{"a key holding a sort order in parentheses", "CREATE TABLE t(a, PRIMARY KEY((a DESC)))",
			"expression"},
		{"an unknown table option", "CREATE TABLE t(a) WITHOUT OIDS", "ROWID is expected"},
		{"a malformed number", "CREATE TABLE t(a DEFAULT 12abc)", "malformed number"},
		{"another virtual table module", "CREATE VIRTUAL TABLE t USING fts4(a)", "module fts4"},
		{"an fts5 table with external content", "CREATE VIRTUAL TABLE t USING fts5(a, content='x')",
			"no content table"},
		{"an fts5 table without content", "CREATE VIRTUAL TABLE t USING fts5(a, content='')",
			"no content table"},
		{"an empty column list", "CREATE TABLE t()", ""},
		{"a column declared twice", "CREATE TABLE t(id integer primary key, a, a)",
			"duplicate column name: a"},
		{"a column declared twice in different case", "CREATE TABLE t(a, b, A)",
			"duplicate column name: A"},
		{"an fts5 column declared twice", "CREATE VIRTUAL TABLE t USING fts5(a, A)",
			"duplicate column name: A"},
	}
	for _, test := range cases {
		t.Run(test.name+" is an error", func(t *testing.T) {
			_, err := parseCreateTable(test.sql)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("parseCreateTable(%q) error = %v, want one containing %q", test.sql, err, test.want)
			}
		})
	}
}

func TestDefaultsSQLiteRefusesFailOnlyTheRowsThatNeedThem(t *testing.T) {
	shape, err := parseCreateTable(`CREATE TABLE t(a, b DEFAULT ("x"), c DEFAULT (-'abc'))`)
	if err != nil {
		t.Fatal(err)
	}
	for _, column := range shape.columns[1:] {
		if !errors.Is(column.defaultErr, errSchemaSyntax) {
			t.Errorf("column %s default error = %v, want a schema error", column.Name, column.defaultErr)
		}
	}
	layout := newLayout(shape)
	if err := layout.checkDefaults(1, []Value{IntValue(1), IntValue(2), IntValue(3)}); err != nil {
		t.Errorf("a full record failed: %v", err)
	}
	if err := layout.checkDefaults(1, []Value{IntValue(1)}); err == nil || !strings.Contains(err.Error(), "column b") {
		t.Errorf("a short record error = %v, want one naming column b", err)
	}
}

// tokenize returns every token of text as the schema parser scans them,
// or the first error.
func tokenize(text string) ([]token, error) {
	var tokens []token
	for offset := 0; ; {
		next, end, err := scanFrom(text, offset)
		if err != nil || next.kind == 0 {
			return tokens, err
		}
		tokens, offset = append(tokens, next), end
	}
}

func TestTokenizeReadsLiteralsAndQuotes(t *testing.T) {
	tokens, err := tokenize(`x'0aFF' 1_000.5e-3 0x1F .5 "a""b" [c] ` + "`d``e`" + ` 'f''g' || <= -- end`)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, item := range tokens {
		got = append(got, item.text+"="+item.name())
	}
	want := []string{"x'0aFF'=x'0aFF'", "1_000.5e-3=1_000.5e-3", "0x1F=0x1F", ".5=.5", `"a""b"=a"b`,
		"[c]=c", "`d``e`=d`e", "'f''g'=f'g", "||=||", "<==<="}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("tokens = %q, want %q", got, want)
	}
	for _, bad := range []string{"x'0a1'", "x'zz'", "0x", "1e", "[open", "\"open", "#"} {
		if _, err := tokenize(bad); err == nil {
			t.Errorf("tokenize(%q) read without error", bad)
		}
	}
}

func FuzzParseCreateTable(fuzz *testing.F) {
	for _, seed := range []string{"CREATE TABLE t(a INTEGER PRIMARY KEY, b TEXT DEFAULT 'x')",
		"CREATE TABLE t(a, b, PRIMARY KEY(b, a)) WITHOUT ROWID, STRICT",
		"CREATE VIRTUAL TABLE s USING fts5(a UNINDEXED, b, tokenize='porter')",
		"CREATE TABLE t(a AS (1) STORED, b DEFAULT (CAST(-(1) AS TEXT)) REFERENCES p ON DELETE CASCADE)",
		widenedTable, "CREATE TABLE t(a DEFAULT -0x7fffffffffffffff, b DEFAULT (((-'1e5'))))"} {
		fuzz.Add(seed)
	}
	fuzz.Fuzz(func(t *testing.T, sql string) {
		shape, err := parseCreateTable(sql)
		if err != nil {
			return
		}
		if len(shape.columns) == 0 {
			t.Fatalf("parseCreateTable(%q) gave no columns and no error", sql)
		}
		positions := recordPositions(shape)
		for i, position := range positions {
			if position >= len(shape.columns) {
				t.Fatalf("column %d stored at %d of %d", i, position, len(shape.columns))
			}
		}
	})
}

// widenedTable is a CREATE TABLE text with every form of DEFAULT the
// generator's widened table holds.
const widenedTable = "CREATE TABLE widened(a, b DEFAULT CURRENT_TIMESTAMP, c DEFAULT (1+2), d DEFAULT" +
	" (-(5)), f DEFAULT abc, g REAL DEFAULT 3, i DEFAULT X'41', j TEXT DEFAULT (X'41'), k DEFAULT" +
	" (+5), l DEFAULT (CAST(1 AS TEXT)), m DEFAULT ('a'||'b'), n INTEGER DEFAULT ('7'))"

func TestPrimaryKeyItemsFollowSQLitesGrammar(t *testing.T) {
	// Each form was checked against the sqlite3 command-line tool: a key
	// item is a column name, inside any parentheses, with any number of
	// COLLATE clauses, then ASC or DESC, and AUTOINCREMENT after a single
	// INTEGER column of a table with rowids.
	accepted := []struct {
		name, key string
		want      []int
		alias     bool
	}{
		{"AUTOINCREMENT after the column", "PRIMARY KEY(id AUTOINCREMENT)", []int{0}, true},
		{"AUTOINCREMENT after a descending column", "PRIMARY KEY(id DESC AUTOINCREMENT)", []int{0}, true},
		{"AUTOINCREMENT after a collation and an order", "PRIMARY KEY(id COLLATE nocase ASC AUTOINCREMENT)",
			[]int{0}, true},
		{"a collation inside parentheses", "PRIMARY KEY((id COLLATE nocase))", []int{0}, true},
		{"a collation after parentheses", "PRIMARY KEY((id) COLLATE nocase DESC)", []int{0}, true},
		{"two collations", "PRIMARY KEY(id COLLATE nocase COLLATE binary)", []int{0}, true},
		{"a quoted name and a string", "PRIMARY KEY(\"ID\", 'x')", []int{0, 1}, false},
		{"one column listed twice", "PRIMARY KEY(id, id)", []int{0}, false},
	}
	for _, test := range accepted {
		t.Run(test.name+" is read", func(t *testing.T) {
			shape, err := parseCreateTable("CREATE TABLE t(id INTEGER, x, " + test.key + ")")
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(shape.primaryKey, test.want) || shape.columns[0].RowidAlias != test.alias {
				t.Errorf("key = %v, alias %v; want %v, alias %v", shape.primaryKey, shape.columns[0].RowidAlias,
					test.want, test.alias)
			}
		})
	}
	refused := []struct {
		name, sql string
	}{
		{"AUTOINCREMENT on a key of two columns", "CREATE TABLE t(id INTEGER, x, PRIMARY KEY(id, x AUTOINCREMENT))"},
		{"AUTOINCREMENT before the last column", "CREATE TABLE t(id INTEGER, x, PRIMARY KEY(id AUTOINCREMENT, x))"},
		{"AUTOINCREMENT twice", "CREATE TABLE t(id INTEGER, x, PRIMARY KEY(id AUTOINCREMENT AUTOINCREMENT))"},
		{"AUTOINCREMENT on a TEXT key", "CREATE TABLE t(id TEXT, x, PRIMARY KEY(id AUTOINCREMENT))"},
		{"AUTOINCREMENT on a TEXT column key", "CREATE TABLE t(id TEXT PRIMARY KEY AUTOINCREMENT)"},
		{"AUTOINCREMENT on a descending column key", "CREATE TABLE t(id INTEGER PRIMARY KEY DESC AUTOINCREMENT)"},
		{"AUTOINCREMENT in a WITHOUT ROWID table",
			"CREATE TABLE t(id INTEGER, x, PRIMARY KEY(id AUTOINCREMENT)) WITHOUT ROWID"},
		{"two sort orders", "CREATE TABLE t(id INTEGER, x, PRIMARY KEY(id ASC DESC))"},
		{"a NULLS clause", "CREATE TABLE t(id INTEGER, x, PRIMARY KEY(id NULLS FIRST))"},
		{"an expression", "CREATE TABLE t(id INTEGER, x, PRIMARY KEY(id+0))"},
	}
	for _, test := range refused {
		t.Run(test.name+" is refused", func(t *testing.T) {
			if _, err := parseCreateTable(test.sql); !errors.Is(err, errSchemaSyntax) {
				t.Errorf("parseCreateTable(%q) error = %v, want a schema error", test.sql, err)
			}
		})
	}
}
