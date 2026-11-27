package parserkit

import (
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

func TestTablesMatchANameAsSQLMatchesATableName(t *testing.T) {
	database, err := OpenDatabase(&input.Artifact{}, filepath.Join("..", "sqlitefile", "testdata", "sites.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	tables, err := TablesOf(database)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name, table string
		want        bool
	}{
		{"the name as the schema spells it is found", "ItemTable", true},
		{"the name in lower case is found", "itemtable", true},
		{"the name in upper case is found", "ITEMTABLE", true},
		{"a letter outside ASCII is not folded", "İtemTable", false},
		{"a table the schema lacks is not found", "absent", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := tables.Has(test.table); got != test.want {
				t.Errorf("Has(%q) = %v, want %v", test.table, got, test.want)
			}
		})
	}
	if (Tables{}).Has("ItemTable") {
		t.Error("the zero Tables holds a table")
	}
}

func TestTablesOfADamagedSchemaIsTheError(t *testing.T) {
	data, err := osfile.ReadFile(filepath.Join("..", "sqlitefile", "testdata", "sites.db"))
	if err != nil {
		t.Fatal(err)
	}
	// Byte 100 is the kind of the schema's first b-tree page; 0 is none.
	data[100] = 0
	path := filepath.Join(t.TempDir(), "damaged.db")
	if err := osfile.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	database, err := OpenDatabase(&input.Artifact{}, path)
	if err != nil {
		t.Fatalf("OpenDatabase(&input.Artifact{}, ) = %v, want the damage met reading the schema", err)
	}
	defer database.Close()
	if tables, err := TablesOf(database); err == nil || tables.Has("ItemTable") {
		t.Errorf("TablesOf() = %v, %v; want an error and no table", tables, err)
	}
}

func TestATableFindsAColumnInAnyASCIICaseAsSQLDoes(t *testing.T) {
	database, err := OpenDatabase(&input.Artifact{}, filepath.Join("..", "sqlitefile", "testdata", "sites.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	table, err := database.Table("ItemTable")
	if err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"key", "KEY", "Value"} {
		if !table.Has(column) {
			t.Errorf("Has(%q) = false, want the column found", column)
		}
	}
	if table.Has("Key") {
		t.Error("a letter outside ASCII was folded")
	}
}
