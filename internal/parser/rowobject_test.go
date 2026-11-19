package parser

import (
	"path/filepath"
	"strconv"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
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

// countedRow is a row of width columns that counts the values its Values
// builds: each call builds every column afresh, as a row read through a
// table's record layout does.
type countedRow struct {
	width int
	built *int
}

// Columns returns the names c0 to c<width-1>.
func (r countedRow) Columns() []string {
	names := make([]string, r.width)
	for i := range names {
		names[i] = "c" + strconv.Itoa(i)
	}
	return names
}

// Values returns width integers, counting each one built.
func (r countedRow) Values() []sqlitefile.Value {
	values := make([]sqlitefile.Value, r.width)
	for i := range values {
		values[i] = sqlitefile.IntValue(int64(i))
	}
	*r.built += r.width
	return values
}

func TestARowObjectBuildsEachColumnsValueOnce(t *testing.T) {
	for _, width := range []int{10, 2000} {
		t.Run("a row of "+strconv.Itoa(width)+" columns builds that many values", func(t *testing.T) {
			built := 0
			object := RowObject(countedRow{width: width, built: &built})
			if built != width {
				t.Errorf("RowObject built %d values, want %d", built, width)
			}
			if last := object.Get("c" + strconv.Itoa(width-1)).JSON(); last != strconv.Itoa(width-1) {
				t.Errorf("the last column is %s, want %d", last, width-1)
			}
		})
	}
}

func TestAColumnNameFoldsOnlyASCIILetters(t *testing.T) {
	tests := []struct {
		name, first, second string
		same                bool
	}{
		{"names differing in ASCII case are one name", "NaMe", "nAme", true},
		{"names differing in a byte that is not UTF-8 stay apart", "Na\xffme", "na\xfeme", false},
		{"a byte that is not UTF-8 is kept as it is", "Na\xffme", "na\xffme", true},
		{"a letter outside ASCII is not folded", "Kınd", "kind", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			first, second := ColumnName(test.first), ColumnName(test.second)
			if (first == second) != test.same {
				t.Errorf("ColumnName(%x) = %x, ColumnName(%x) = %x; want same = %v",
					test.first, first, test.second, second, test.same)
			}
		})
	}
	if got := ColumnName("Na\xffme"); got != "na\xffme" {
		t.Errorf("ColumnName(%x) = %x, want the bytes with N lowered", "Na\xffme", got)
	}
}
