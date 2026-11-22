package sqlitefile

import (
	"errors"
	"testing"
)

// The letters whose Unicode case folding meets an ASCII letter: the long s
// folds with s, the Kelvin sign with k, and the dotless i upper-cases to I.
const (
	longS      = "ſ"
	kelvinSign = "K"
	dotlessI   = "ı"
)

func TestNamesMatchInASCIICaseOnly(t *testing.T) {
	pages := newTestPages(4096)
	lookalike, columns := pages.add(), pages.add()
	pages.setPage(lookalike, leafTable, [][]byte{tableLeafCell(1, testRecord(TextValue("lookalike")))}, 0)
	pages.setPage(columns, leafTable, [][]byte{tableLeafCell(1, testRecord(TextValue("long"),
		TextValue("kelvin")))}, 0)
	pages.setSchema(schemaTable{"me" + longS + longS + "ages", lookalike, "CREATE TABLE \"me" + longS + longS + "ages\"(x)"},
		schemaTable{"t", columns, "CREATE TABLE t(\"" + longS + "\", \"" + kelvinSign + "\")"})
	database := openOrFail(t, pages.bytes(), nil, nil)
	t.Run("a table name differing outside ASCII is another table", func(t *testing.T) {
		if _, err := database.Table("MESSAGES"); !errors.Is(err, ErrNoTable) {
			t.Errorf("Table(MESSAGES) error = %v, want ErrNoTable", err)
		}
	})
	table, err := database.Table("T")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"s", "S", "k", "K"} {
		t.Run("column "+name+" is not a look-alike column", func(t *testing.T) {
			if table.Has(name) {
				t.Errorf("Has(%s) = true", name)
			}
			if _, err := table.Select(name).All(); err == nil {
				t.Errorf("Select(%s) succeeded", name)
			}
			rows, err := table.Select().All()
			if err != nil || len(rows) != 1 || rows[0].Has(name) {
				t.Errorf("select * row has %s (%v)", name, err)
			}
		})
	}
	t.Run("a column is found by its own spelling", func(t *testing.T) {
		rows, err := table.Select(longS).All()
		if err != nil || len(rows) != 1 || rows[0].Get(longS).Text() != "long" {
			t.Errorf("Select(long s) = %v, %v", rows, err)
		}
	})
}

func TestSQLWordsMatchInASCIICaseOnly(t *testing.T) {
	refused := []struct {
		name, sql string
	}{
		{"a keyword spelled with the Kelvin sign", "CREATE TABLE t(a INTEGER PRIMARY " + kelvinSign + "EY)"},
		{"a module name spelled with a long s", "CREATE VIRTUAL TABLE f USING ft" + longS + "5(a)"},
		{"a table option spelled with a long s", "CREATE TABLE t(a PRIMARY KEY) WITHOUT ROWID, " + longS + "TRICT"},
	}
	for _, test := range refused {
		t.Run(test.name+" is not the keyword", func(t *testing.T) {
			if _, err := parseCreateTable(test.sql); err == nil {
				t.Errorf("parseCreateTable(%q) succeeded", test.sql)
			}
		})
	}
	t.Run("a type with a dotless i is not INT", func(t *testing.T) {
		if got := affinityOf(dotlessI+"nt", false); got != NumericAffinity {
			t.Errorf("affinityOf(dotless int) = %s, want NUMERIC", got)
		}
	})
	t.Run("a collation with a long s is not NOCASE", func(t *testing.T) {
		shape, err := parseCreateTable("CREATE TABLE t(a COLLATE noca" + longS + "e)")
		if err != nil || shape.columns[0].Collation != "NOCA"+longS+"E" {
			t.Errorf("collation = %q (%v), want only the ASCII letters raised", shape.columns[0].Collation, err)
		}
	})
	t.Run("a DEFAULT word with a long s is a name, not a keyword", func(t *testing.T) {
		shape, err := parseCreateTable("CREATE TABLE t(a DEFAULT FAL" + longS + "E)")
		if err != nil || shape.columns[0].Default.Text() != "FAL"+longS+"E" {
			t.Errorf("default = %v (%v), want the word as text", shape.columns[0].Default, err)
		}
	})
}
