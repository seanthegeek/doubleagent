package sqlitefile

import (
	"encoding/binary"
	"runtime"
)

// testPages builds a database file page by page, for the shapes no
// checked-in fixture holds: page 1 carries the header and the schema
// table's single leaf page, and every other page is written by the test.
type testPages struct {
	pageSize int
	pages    [][]byte
}

// newTestPages starts a database of pageSize-byte pages holding page 1.
func newTestPages(pageSize int) *testPages {
	return &testPages{pageSize: pageSize, pages: [][]byte{make([]byte, pageSize)}}
}

// add appends a zeroed page and returns its number.
func (b *testPages) add() uint32 {
	b.pages = append(b.pages, make([]byte, b.pageSize))
	return uint32(len(b.pages))
}

// page returns the bytes of page number, to change in place.
func (b *testPages) page(number uint32) []byte {
	return b.pages[number-1]
}

// setPage writes a b-tree page of kind holding cells, laid out from the
// end of the page, with right as the right-most child of an interior page.
func (b *testPages) setPage(number uint32, kind byte, cells [][]byte, right uint32) {
	data := b.page(number)
	start := 0
	if number == 1 {
		start = 100
	}
	data[start] = kind
	pointers := start + 8
	if kind == interiorIndex || kind == interiorTable {
		binary.BigEndian.PutUint32(data[start+8:], right)
		pointers += 4
	}
	end := b.pageSize
	for i, cell := range cells {
		end -= len(cell)
		copy(data[end:], cell)
		binary.BigEndian.PutUint16(data[pointers+2*i:], uint16(end))
	}
	binary.BigEndian.PutUint16(data[start+3:], uint16(len(cells)))
	binary.BigEndian.PutUint16(data[start+5:], uint16(end))
}

// schemaTable is one table of a test database's schema.
type schemaTable struct {
	name string
	root uint32
	sql  string
}

// setSchema writes the schema table on page 1, one row per table.
func (b *testPages) setSchema(tables ...schemaTable) {
	var cells [][]byte
	for i, table := range tables {
		record := testRecord(TextValue("table"), TextValue(table.name), TextValue(table.name),
			IntValue(int64(table.root)), TextValue(table.sql))
		cells = append(cells, tableLeafCell(int64(i+1), record))
	}
	b.setPage(1, leafTable, cells, 0)
}

// bytes returns the file, with the database header for its page count.
func (b *testPages) bytes() []byte {
	header := b.pages[0][:100]
	copy(header, headerString)
	binary.BigEndian.PutUint16(header[16:], uint16(b.pageSize))
	if b.pageSize == maxPageSize {
		binary.BigEndian.PutUint16(header[16:], 1)
	}
	header[18], header[19] = 1, 1
	header[21], header[22], header[23] = 64, 32, 32
	binary.BigEndian.PutUint32(header[28:], uint32(len(b.pages)))
	binary.BigEndian.PutUint32(header[44:], 4)
	binary.BigEndian.PutUint32(header[56:], 1)
	var file []byte
	for _, page := range b.pages {
		file = append(file, page...)
	}
	return file
}

// appendSQLiteVarint appends value in the file format's varint: groups of
// seven bits, most significant first, the high bit set on all but the last.
func appendSQLiteVarint(data []byte, value uint64) []byte {
	var groups []byte
	for {
		groups = append(groups, byte(value&0x7f))
		value >>= 7
		if value == 0 {
			break
		}
	}
	for i := len(groups) - 1; i >= 0; i-- {
		group := groups[i]
		if i > 0 {
			group |= 0x80
		}
		data = append(data, group)
	}
	return data
}

// testRecord encodes values as a record: NULL, 8-byte integers, text and
// blobs.
func testRecord(values ...Value) []byte {
	var types, body []byte
	for _, value := range values {
		switch value.Type {
		case Null:
			types = appendSQLiteVarint(types, 0)
		case Integer:
			types = appendSQLiteVarint(types, 6)
			body = binary.BigEndian.AppendUint64(body, uint64(value.Int))
		case Text:
			types = appendSQLiteVarint(types, uint64(13+2*len(value.Bytes)))
			body = append(body, value.Bytes...)
		case Blob:
			types = appendSQLiteVarint(types, uint64(12+2*len(value.Bytes)))
			body = append(body, value.Bytes...)
		}
	}
	size := len(types) + 1
	if len(appendSQLiteVarint(nil, uint64(size))) > 1 {
		size++
	}
	record := appendSQLiteVarint(nil, uint64(size))
	return append(append(record, types...), body...)
}

// tableLeafCell is a table leaf cell holding its whole payload on the page.
func tableLeafCell(rowid int64, payload []byte) []byte {
	cell := appendSQLiteVarint(nil, uint64(len(payload)))
	cell = appendSQLiteVarint(cell, uint64(rowid))
	return append(cell, payload...)
}

// allocatedBytes returns how many bytes of memory work allocates.
func allocatedBytes(work func()) uint64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	work()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}
