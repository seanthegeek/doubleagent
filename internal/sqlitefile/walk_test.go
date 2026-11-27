package sqlitefile

import (
	"encoding/binary"
	"errors"
	"testing"
)

// The walks below use 512-byte pages, where a table leaf keeps a payload of
// up to 477 bytes whole, and keeps 39 bytes of one of 547 bytes, the other
// 508 filling one overflow page exactly ("B-tree Pages").
const (
	smallPage     = 512
	spilledLength = 547
	spilledLocal  = 39
)

// spilledRecord is a record of one blob that is spilledLength bytes long.
func spilledRecord(t *testing.T) []byte {
	t.Helper()
	record := testRecord(BlobValue(make([]byte, spilledLength-3)))
	if len(record) != spilledLength {
		t.Fatalf("the spilled record is %d bytes, want %d", len(record), spilledLength)
	}
	return record
}

// spilledCell is a table leaf cell keeping the first bytes of record on the
// page and naming overflow as the first page of the rest.
func spilledCell(rowid int64, record []byte, overflow uint32) []byte {
	cell := appendSQLiteVarint(nil, uint64(len(record)))
	cell = appendSQLiteVarint(cell, uint64(rowid))
	cell = append(cell, record[:spilledLocal]...)
	return binary.BigEndian.AppendUint32(cell, overflow)
}

// sharedOverflow returns a database whose table t holds count rows, every
// one naming the same overflow page for the rest of its payload.
func sharedOverflow(t *testing.T, count int) []byte {
	t.Helper()
	pages := newTestPages(smallPage)
	root, overflow := pages.add(), pages.add()
	record := spilledRecord(t)
	copy(pages.page(overflow)[4:], record[spilledLocal:])
	cells := make([][]byte, count)
	for i := range cells {
		cells[i] = spilledCell(int64(i+1), record, overflow)
	}
	pages.setPage(root, leafTable, cells, 0)
	pages.setSchema(schemaTable{"t", root, "CREATE TABLE t(a)"})
	return pages.bytes()
}

// overlappingCells returns a database whose table t has one leaf page
// listing count cells that all start at the same offset.
func overlappingCells(t *testing.T, count int) []byte {
	t.Helper()
	pages := newTestPages(smallPage)
	root := pages.add()
	record := testRecord(BlobValue(make([]byte, 50)))
	pages.setPage(root, leafTable, [][]byte{tableLeafCell(1, record)}, 0)
	page := pages.page(root)
	offset := binary.BigEndian.Uint16(page[8:])
	for i := range count {
		binary.BigEndian.PutUint16(page[8+2*i:], offset)
	}
	binary.BigEndian.PutUint16(page[3:], uint16(count))
	pages.setSchema(schemaTable{"t", root, "CREATE TABLE t(a)"})
	return pages.bytes()
}

// walkPayload walks table t's tree, reading every record, and returns the
// payload bytes the walk returned, the bytes it read from the file, the
// rows read and the error that ended it.
func walkPayload(t *testing.T, file []byte) (int64, int64, int, error) {
	t.Helper()
	var read int64
	database, err := openFiles(countedFiles(&read, file, nil, nil))
	if err != nil {
		t.Fatal(err)
	}
	rows := database.tableRows(rootOf(t, database, "t"))
	read = 0
	payload, count := int64(0), 0
	for rows.Next() {
		values, err := rows.Record()
		if err != nil {
			return payload, read, count, err
		}
		for _, value := range values {
			payload += int64(len(value.Bytes))
		}
		count++
	}
	return payload, read, count, rows.Err()
}

func TestAWalkReturnsNoMorePayloadThanItsPagesHold(t *testing.T) {
	cases := []struct {
		name string
		file []byte
		// most is the most rows read before the damage shows.
		most int
	}{
		{"rows sharing one overflow page", sharedOverflow(t, 8), 1},
		{"cells overlapping on one page", overlappingCells(t, 200), smallPage / 50},
	}
	for _, test := range cases {
		t.Run(test.name+" end the walk as damage", func(t *testing.T) {
			payload, read, count, err := walkPayload(t, test.file)
			pages := int64(len(test.file) / smallPage)
			if !errors.Is(err, ErrMalformed) || count > test.most {
				t.Errorf("walk = %d rows, %v; want at most %d rows, ErrMalformed", count, err, test.most)
			}
			if payload > pages*smallPage {
				t.Errorf("the walk returned %d payload bytes from %d pages of %d", payload, pages, smallPage)
			}
			if read > pages*smallPage {
				t.Errorf("the walk read %d bytes from %d pages of %d", read, pages, smallPage)
			}
		})
	}
}

func TestARowsOverflowPagesMayBeReadAgain(t *testing.T) {
	database := openOrFail(t, sharedOverflow(t, 1), nil, nil)
	rows := database.tableRows(rootOf(t, database, "t"))
	if !rows.Next() {
		t.Fatalf("no row: %v", rows.Err())
	}
	for range 2 {
		values, err := rows.Record()
		if err != nil || len(values) != 1 || len(values[0].Bytes) != spilledLength-3 {
			t.Fatalf("Record() = %v, %v; want the one blob", values, err)
		}
	}
	if rows.Next() || rows.Err() != nil {
		t.Errorf("a second row or an error: %v", rows.Err())
	}
}

// interiorChain returns a database whose table t is a path of depth pages:
// interior pages, each with only a right-most child, down to one leaf.
func interiorChain(depth int) []byte {
	pages := newTestPages(smallPage)
	numbers := make([]uint32, depth)
	for i := range numbers {
		numbers[i] = pages.add()
	}
	for i, number := range numbers[:depth-1] {
		pages.setPage(number, interiorTable, nil, numbers[i+1])
	}
	pages.setPage(numbers[depth-1], leafTable, [][]byte{tableLeafCell(1, testRecord(IntValue(1)))}, 0)
	pages.setSchema(schemaTable{"t", numbers[0], "CREATE TABLE t(a)"})
	return pages.bytes()
}

func TestTreesDeeperThanSQLiteReadsAreRefused(t *testing.T) {
	t.Run("a path of the most pages SQLite follows is read", func(t *testing.T) {
		if _, _, count, err := walkPayload(t, interiorChain(maxTreeDepth)); count != 1 || err != nil {
			t.Errorf("walk = %d rows, %v; want the one row", count, err)
		}
	})
	t.Run("a path one page deeper is damage", func(t *testing.T) {
		if _, _, count, err := walkPayload(t, interiorChain(maxTreeDepth+1)); count != 0 ||
			!errors.Is(err, ErrMalformed) {
			t.Errorf("walk = %d rows, %v; want ErrMalformed", count, err)
		}
	})
}
