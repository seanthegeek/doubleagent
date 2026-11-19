package parser

import (
	"path/filepath"
	"testing"
)

func TestARowObjectKeysEachColumnByTheNameSQLFindsItBy(t *testing.T) {
	database, err := OpenDatabase(filepath.Join("testdata", "column_case.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()
	table, err := database.Table("things")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := table.Select().All()
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows %v, error %v", rows, err)
	}
	if got, want := RowObject(rows[0]).JSON(), `{"name":"a","value":7,"kind":"k"}`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	tests := []struct {
		name, column, want string
		found              bool
	}{
		{"a column in its own spelling", "VALUE", "VALUE", true},
		{"a column in lower case", "name", "Name", true},
		{"a column in upper case", "KIND", "kind", true},
		{"a letter outside ASCII is not folded", "Kınd", "", false},
		{"a column the table lacks", "absent", "", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			column, found := ColumnOf(table, test.column)
			if found != test.found || column.Name != test.want {
				t.Errorf("ColumnOf(%q) = %q, %v; want %q, %v", test.column, column.Name, found, test.want, test.found)
			}
		})
	}
}
