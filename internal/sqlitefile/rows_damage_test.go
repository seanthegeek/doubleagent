package sqlitefile

import (
	"encoding/binary"
	"errors"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// testdataBytes returns a test database's bytes.
func testdataBytes(t testing.TB, name string) []byte {
	t.Helper()
	data, err := osfile.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// openOrFail opens a database from memory and fails the test on an error.
func openOrFail(t *testing.T, main, log, journal []byte) *Database {
	t.Helper()
	database, err := OpenBytes(main, log, journal)
	if err != nil {
		t.Fatal(err)
	}
	return database
}

// rootOf returns the root page of the schema object called name.
func rootOf(t *testing.T, database *Database, name string) uint32 {
	t.Helper()
	schema, err := database.schemaRows()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range schema {
		if row.Name == name {
			return row.RootPage
		}
	}
	t.Fatalf("no schema object %s", name)
	return 0
}

// countRows walks a tree and returns how many rows decoded and its error.
func countRows(rows *treeRows) (int, error) {
	count := 0
	for rows.Next() {
		if _, err := rows.Record(); err != nil {
			return count, err
		}
		count++
	}
	return count, rows.Err()
}

// leafPages returns a table b-tree's leaf pages in key order.
func leafPages(t *testing.T, database *Database, number uint32) []uint32 {
	t.Helper()
	page, err := database.loadPage(number, true)
	if err != nil {
		t.Fatal(err)
	}
	if page.leaf() {
		return []uint32{number}
	}
	var leaves []uint32
	for i := range page.cellCount {
		child, err := database.leftChild(page, i)
		if err != nil {
			t.Fatal(err)
		}
		leaves = append(leaves, leafPages(t, database, child)...)
	}
	return append(leaves, leafPages(t, database, page.rightChild)...)
}

func TestRowsBeforeADamagedPageAreDelivered(t *testing.T) {
	main := testdataBytes(t, "deep.db")
	database := openOrFail(t, main, nil, nil)
	root := rootOf(t, database, "t")
	leaves := leafPages(t, database, root)
	if len(leaves) < 10 {
		t.Fatalf("deep.db has %d leaves, too few for the test", len(leaves))
	}
	before := 0
	for _, leaf := range leaves[:3] {
		page, err := database.loadPage(leaf, true)
		if err != nil {
			t.Fatal(err)
		}
		before += page.cellCount
	}
	damaged := append([]byte(nil), main...)
	damaged[int64(leaves[3]-1)*int64(database.PageSize())] = 0x07
	count, err := countRows(openOrFail(t, damaged, nil, nil).tableRows(root))
	if count != before || !errors.Is(err, ErrMalformed) {
		t.Errorf("walk of a tree with its fourth leaf damaged = %d rows, %v; want %d rows, ErrMalformed",
			count, err, before)
	}
}

func TestDamagedTreesEndWithAnError(t *testing.T) {
	main := testdataBytes(t, "deep.db")
	database := openOrFail(t, main, nil, nil)
	root := rootOf(t, database, "t")
	rootPage, err := database.loadPage(root, true)
	if err != nil {
		t.Fatal(err)
	}
	pageSize := int64(database.PageSize())
	rootOffset := int64(root-1) * pageSize
	cases := []struct {
		name   string
		damage func(data []byte)
	}{
		{"a child pointing back at the root", func(data []byte) {
			binary.BigEndian.PutUint32(data[rootOffset+8:], root)
		}},
		{"a child past the last page", func(data []byte) {
			binary.BigEndian.PutUint32(data[rootOffset+8:], 1<<30)
		}},
		{"a child numbered zero", func(data []byte) {
			binary.BigEndian.PutUint32(data[rootOffset+8:], 0)
		}},
		{"a cell count larger than the page", func(data []byte) {
			binary.BigEndian.PutUint16(data[rootOffset+3:], 0xffff)
		}},
		{"a cell offset inside the cell pointer array", func(data []byte) {
			binary.BigEndian.PutUint16(data[rootOffset+int64(rootPage.cellsAt):], 2)
		}},
		{"a cell offset past the page", func(data []byte) {
			binary.BigEndian.PutUint16(data[rootOffset+int64(rootPage.cellsAt):], 0xfff0)
		}},
		{"an index page inside a table tree", func(data []byte) {
			data[rootOffset] = interiorIndex
		}},
	}
	for _, test := range cases {
		t.Run(test.name+" is malformed", func(t *testing.T) {
			damaged := append([]byte(nil), main...)
			test.damage(damaged)
			_, err := countRows(openOrFail(t, damaged, nil, nil).tableRows(root))
			if !errors.Is(err, ErrMalformed) {
				t.Errorf("walk error = %v, want ErrMalformed", err)
			}
		})
	}
}

func TestDamagedOverflowChainsEndWithAnError(t *testing.T) {
	main := testdataBytes(t, "page_1024.db")
	database := openOrFail(t, main, nil, nil)
	root := rootOf(t, database, "t")
	rows := database.tableRows(root)
	var overflow uint32
	for overflow == 0 && rows.Next() {
		if rows.payload.total-int64(len(rows.payload.local)) > database.usable-4 {
			overflow = rows.payload.overflow
		}
	}
	if overflow == 0 {
		t.Fatal("page_1024.db has no overflow chain of two pages")
	}
	offset := int64(overflow-1) * int64(database.PageSize())
	for _, test := range []struct {
		name string
		next uint32
	}{{"a chain that loops back on itself", overflow}, {"a chain that ends early", 0},
		{"a chain that leaves the file", 70000}} {
		t.Run(test.name+" is malformed", func(t *testing.T) {
			damaged := append([]byte(nil), main...)
			binary.BigEndian.PutUint32(damaged[offset:], test.next)
			_, err := countRows(openOrFail(t, damaged, nil, nil).tableRows(root))
			if !errors.Is(err, ErrMalformed) {
				t.Errorf("walk error = %v, want ErrMalformed", err)
			}
		})
	}
}

// walkEverything reads the schema and every tree it names, both as a table
// and as an index, ignoring errors: fuzzing and damage tests only ask that
// it returns.
func walkEverything(database *Database, err error) {
	if err != nil {
		return
	}
	schema, _ := database.schemaRows()
	roots := []uint32{1}
	for _, row := range schema {
		roots = append(roots, row.RootPage)
	}
	for _, root := range roots {
		for _, rows := range []*treeRows{database.tableRows(root), database.indexRows(root)} {
			_, _ = countRows(rows)
		}
	}
}

func TestTruncatedFilesNeverPanic(t *testing.T) {
	for _, name := range []string{"page_1024.db", "shapes.db", "autovacuum.db"} {
		main := testdataBytes(t, name)
		t.Run(name+" cut anywhere opens or fails cleanly", func(t *testing.T) {
			pageSize := int(binary.BigEndian.Uint16(main[16:]))
			for size := 0; size < len(main); size += 97 {
				cut := main[:size]
				short := size > 100 && (size+pageSize-1)/pageSize < len(main)/pageSize
				if _, err := OpenBytes(cut, nil, nil); short && !errors.Is(err, ErrMalformed) {
					t.Fatalf("cut at %d: openFiles error = %v, want ErrMalformed for a short file", size, err)
				}
				unchecked := append([]byte(nil), cut...)
				if len(unchecked) >= 100 {
					unchecked[92]++
				}
				walkEverything(OpenBytes(unchecked, nil, nil))
			}
		})
	}
}

func TestHeaderChecks(t *testing.T) {
	main := testdataBytes(t, "page_1024.db")
	cases := []struct {
		name   string
		damage func(data []byte)
		want   error
	}{
		{"another header string is not a database", func(data []byte) { data[0] = 'X' }, ErrNotDatabase},
		{"a page size that is not a power of two", func(data []byte) {
			binary.BigEndian.PutUint16(data[16:], 1000)
		}, ErrMalformed},
		{"a page size of 256", func(data []byte) { binary.BigEndian.PutUint16(data[16:], 256) }, ErrMalformed},
		{"payload fractions other than 64, 32 and 32", func(data []byte) { data[21] = 65 }, ErrMalformed},
		{"reserved bytes leaving less than 480 usable", func(data []byte) {
			binary.BigEndian.PutUint16(data[16:], 512)
			data[20] = 33
		}, ErrMalformed},
		{"UTF-16le text", func(data []byte) { binary.BigEndian.PutUint32(data[56:], 2) }, errUTF16},
		{"UTF-16be text", func(data []byte) { binary.BigEndian.PutUint32(data[56:], 3) }, errUTF16},
	}
	for _, test := range cases {
		t.Run(test.name+" is refused", func(t *testing.T) {
			damaged := append([]byte(nil), main...)
			test.damage(damaged)
			if _, err := OpenBytes(damaged, nil, nil); !errors.Is(err, test.want) {
				t.Errorf("openFiles() error = %v, want %v", err, test.want)
			}
		})
	}
	for _, field := range []struct {
		name   string
		offset int
	}{{"read version 3", 19}, {"schema format 5", 47}} {
		t.Run("a "+field.name+" is not supported", func(t *testing.T) {
			damaged := append([]byte(nil), main...)
			damaged[field.offset] = map[int]byte{19: 3, 47: 5}[field.offset]
			if _, err := OpenBytes(damaged, nil, nil); err == nil {
				t.Error("openFiles() succeeded")
			}
		})
	}
}

func TestEncodingFieldReadsAsSQLiteReadsIt(t *testing.T) {
	main := testdataBytes(t, "page_1024.db")
	for _, encoding := range []uint32{0, 1, 4, 5} {
		damaged := append([]byte(nil), main...)
		binary.BigEndian.PutUint32(damaged[56:], encoding)
		if _, err := OpenBytes(damaged, nil, nil); err != nil {
			t.Errorf("encoding %d: openFiles() error = %v; SQLite reads its low bits as UTF-8", encoding, err)
		}
	}
}

func TestEmptyMainFileIsAnEmptyDatabase(t *testing.T) {
	for _, test := range []struct {
		name         string
		log, journal []byte
	}{{"alone", nil, nil}, {"beside a log", testdataBytes(t, "log_rows.db-wal"), nil},
		{"beside a journal", nil, testdataBytes(t, "journal_hot.db-journal")}} {
		t.Run("an empty main file "+test.name+" has no schema", func(t *testing.T) {
			database := openOrFail(t, []byte{}, test.log, test.journal)
			schema, err := database.schemaRows()
			if err != nil || len(schema) != 0 || database.PageCount() != 0 {
				t.Errorf("Schema() = %v, %v with %d pages, want nothing", schema, err, database.PageCount())
			}
			if _, err := countRows(database.tableRows(1)); !errors.Is(err, ErrMalformed) {
				t.Errorf("walk of page 1 error = %v, want ErrMalformed", err)
			}
		})
	}
}

func TestUnknownLogVersionIsAnError(t *testing.T) {
	_, err := OpenBytes(testdataBytes(t, "log_version.db"), testdataBytes(t, "log_version.db-wal"), nil)
	if err == nil {
		t.Error("openFiles() succeeded with a log of an unknown format version")
	}
}

func TestOpenWithoutMainFileIsAnError(t *testing.T) {
	if _, err := openFiles(databaseFiles{}); err == nil {
		t.Error("openFiles(databaseFiles{}) succeeded")
	}
}

func TestTestdataTreesAreDeep(t *testing.T) {
	database := openOrFail(t, testdataBytes(t, "deep.db"), nil, nil)
	for _, test := range []struct {
		name  string
		table bool
	}{{"t", true}, {"t_b", false}} {
		t.Run("the root of "+test.name+" has interior children", func(t *testing.T) {
			root, err := database.loadPage(rootOf(t, database, test.name), test.table)
			if err != nil {
				t.Fatal(err)
			}
			if root.leaf() {
				t.Fatal("the root is a leaf")
			}
			child, err := database.loadPage(root.rightChild, test.table)
			if err != nil || child.leaf() {
				t.Errorf("the right-most child is a leaf (%v); the tree is too shallow to test", err)
			}
		})
	}
}
