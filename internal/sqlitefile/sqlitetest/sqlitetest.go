package sqlitetest

import (
	"encoding/binary"
	"fmt"
	"math"
)

// pageSize is the size of every page: the format's default.
const pageSize = 4096

// The b-tree page kinds the file format numbers.
const (
	interiorTable = 0x05
	leafTable     = 0x0d
)

// Table is one table of a database: its name, its CREATE TABLE statement
// as the schema table records it, and its rows, each a list of column
// values: nil, an int, an int64, a float64, a string or a []byte.
type Table struct {
	Name string
	SQL  string
	Rows [][]any
}

// file is a database being written: its pages, page 1 first.
type file struct {
	pages [][]byte
}

// Build returns the bytes of a database holding the tables, or an error
// when the schema does not fit on page 1 or a value is of a type the
// package does not write.
func Build(tables ...Table) ([]byte, error) {
	database := &file{pages: [][]byte{make([]byte, pageSize)}}
	var schema [][]byte
	for i, table := range tables {
		root, err := database.writeTree(table.Rows)
		if err != nil {
			return nil, fmt.Errorf("table %s: %w", table.Name, err)
		}
		record, err := encodeRecord([]any{"table", table.Name, table.Name, int64(root), table.SQL})
		if err != nil {
			return nil, err
		}
		schema = append(schema, leafCell(int64(i+1), record, nil))
	}
	if !fits(schema, pageSize-100-8) {
		return nil, fmt.Errorf("the schema of %d tables does not fit on page 1", len(tables))
	}
	database.setPage(1, leafTable, schema, 0)
	return database.bytes(), nil
}

// add appends a zeroed page and returns its number.
func (f *file) add() uint32 {
	f.pages = append(f.pages, make([]byte, pageSize))
	return uint32(len(f.pages))
}

// writeTree writes the rows as a table b-tree and returns its root page.
func (f *file) writeTree(rows [][]any) (uint32, error) {
	var cells [][]byte
	var keys []int64
	for i, row := range rows {
		record, err := encodeRecord(row)
		if err != nil {
			return 0, fmt.Errorf("row %d: %w", i+1, err)
		}
		cells = append(cells, leafCell(int64(i+1), record, f))
		keys = append(keys, int64(i+1))
	}
	children, lastKeys := f.packLeaves(cells, keys)
	for len(children) > 1 {
		children, lastKeys = f.packInterior(children, lastKeys)
	}
	if len(children) == 0 {
		page := f.add()
		f.setPage(page, leafTable, nil, 0)
		return page, nil
	}
	return children[0], nil
}

// packLeaves writes cells onto as few leaf pages as hold them, in order,
// and returns the pages and the last key on each.
func (f *file) packLeaves(cells [][]byte, keys []int64) ([]uint32, []int64) {
	var pages []uint32
	var lastKeys []int64
	for start := 0; start < len(cells); {
		end := start + 1
		for end < len(cells) && fits(cells[start:end+1], pageSize-8) {
			end++
		}
		page := f.add()
		f.setPage(page, leafTable, cells[start:end], 0)
		pages = append(pages, page)
		lastKeys = append(lastKeys, keys[end-1])
		start = end
	}
	return pages, lastKeys
}

// packInterior writes one level of interior pages over children: each
// page points to a run of children, the run's last through its right-most
// pointer and the others through a cell holding their last key. A run
// never leaves a single child for the next page, so no page is empty.
func (f *file) packInterior(children []uint32, lastKeys []int64) ([]uint32, []int64) {
	cells := make([][]byte, len(children))
	for i, child := range children {
		cells[i] = interiorCell(child, lastKeys[i])
	}
	var pages []uint32
	var keys []int64
	for start := 0; start < len(children); {
		end := start + 1
		for end+1 < len(children) && fits(cells[start:end+1], pageSize-12) {
			end++
		}
		if len(children)-end == 2 && end-start > 1 {
			end--
		}
		page := f.add()
		f.setPage(page, interiorTable, cells[start:end], children[end])
		pages = append(pages, page)
		keys = append(keys, lastKeys[end])
		start = end + 1
	}
	return pages, keys
}

// fits reports whether cells and their pointers fit in space bytes.
func fits(cells [][]byte, space int) bool {
	total := 0
	for _, cell := range cells {
		total += len(cell) + 2
	}
	return total <= space
}

// setPage writes a b-tree page of kind holding cells, laid out from the
// end of the page, with right as the right-most child of an interior page.
func (f *file) setPage(number uint32, kind byte, cells [][]byte, right uint32) {
	data := f.pages[number-1]
	start := 0
	if number == 1 {
		start = 100
	}
	data[start] = kind
	pointers := start + 8
	if kind == interiorTable {
		binary.BigEndian.PutUint32(data[start+8:], right)
		pointers += 4
	}
	end := pageSize
	for i, cell := range cells {
		end -= len(cell)
		copy(data[end:], cell)
		binary.BigEndian.PutUint16(data[pointers+2*i:], uint16(end))
	}
	binary.BigEndian.PutUint16(data[start+3:], uint16(len(cells)))
	binary.BigEndian.PutUint16(data[start+5:], uint16(end))
}

// bytes returns the file, with the database header for its page count.
func (f *file) bytes() []byte {
	header := f.pages[0][:100]
	copy(header, "SQLite format 3\x00")
	binary.BigEndian.PutUint16(header[16:], pageSize)
	header[18], header[19] = 1, 1
	header[21], header[22], header[23] = 64, 32, 32
	binary.BigEndian.PutUint32(header[28:], uint32(len(f.pages)))
	binary.BigEndian.PutUint32(header[44:], 4)
	binary.BigEndian.PutUint32(header[56:], 1)
	data := make([]byte, 0, len(f.pages)*pageSize)
	for _, page := range f.pages {
		data = append(data, page...)
	}
	return data
}

// leafCell is a table leaf cell: the payload's size, the rowid and as much
// of the payload as the page keeps, the rest written to overflow pages of
// database (which may be nil only for a payload that needs none).
func leafCell(rowid int64, payload []byte, database *file) []byte {
	cell := appendVarint(nil, uint64(len(payload)))
	cell = appendVarint(cell, uint64(rowid))
	local := localSize(len(payload))
	if local == len(payload) {
		return append(cell, payload...)
	}
	cell = append(cell, payload[:local]...)
	return binary.BigEndian.AppendUint32(cell, database.overflow(payload[local:]))
}

// localSize is how many bytes of a table leaf payload of size bytes stay
// on its page (the file format's "Cell Payload Overflow Pages"): all of
// it up to U-35, else M plus the remainder that fills whole overflow
// pages when that is no more than U-35, else M. It repeats the arithmetic
// of sqlitefile's splitPayload, since a fixture built with the reader's own
// code could not show the reader wrong.
func localSize(size int) int {
	usable := pageSize
	most := usable - 35
	if size <= most {
		return size
	}
	least := (usable-12)*32/255 - 23
	local := least + (size-least)%(usable-4)
	if local <= most {
		return local
	}
	return least
}

// overflow writes data on a chain of overflow pages and returns the first.
func (f *file) overflow(data []byte) uint32 {
	var first, last uint32
	for len(data) > 0 {
		page := f.add()
		if last != 0 {
			binary.BigEndian.PutUint32(f.pages[last-1], page)
		} else {
			first = page
		}
		n := copy(f.pages[page-1][4:], data)
		data = data[n:]
		last = page
	}
	return first
}

// interiorCell is a table interior cell: the left child and its largest
// rowid.
func interiorCell(child uint32, key int64) []byte {
	return appendVarint(binary.BigEndian.AppendUint32(nil, child), uint64(key))
}

// encodeRecord encodes values as a record: a header of serial types, then
// the values, integers in 8 bytes and floats as 8-byte IEEE numbers.
func encodeRecord(values []any) ([]byte, error) {
	var types, body []byte
	for _, value := range values {
		switch typed := value.(type) {
		case nil:
			types = appendVarint(types, 0)
		case int:
			types = appendVarint(types, 6)
			body = binary.BigEndian.AppendUint64(body, uint64(typed))
		case int64:
			types = appendVarint(types, 6)
			body = binary.BigEndian.AppendUint64(body, uint64(typed))
		case float64:
			types = appendVarint(types, 7)
			body = binary.BigEndian.AppendUint64(body, math.Float64bits(typed))
		case string:
			types = appendVarint(types, uint64(13+2*len(typed)))
			body = append(body, typed...)
		case []byte:
			types = appendVarint(types, uint64(12+2*len(typed)))
			body = append(body, typed...)
		default:
			return nil, fmt.Errorf("a value of type %T", value)
		}
	}
	// The header size counts the varint that holds it (Record Format), so
	// it grows until the varint's own length no longer changes it.
	size := len(types) + 1
	for size != len(types)+len(appendVarint(nil, uint64(size))) {
		size = len(types) + len(appendVarint(nil, uint64(size)))
	}
	record := appendVarint(nil, uint64(size))
	return append(append(record, types...), body...), nil
}

// maxVarint is the largest value appendVarint writes: eight groups of
// seven bits. A larger one takes the varint's nine-byte form, whose last
// byte holds eight bits, which no rowid or size of a table this package
// writes needs.
const maxVarint = 1<<56 - 1

// appendVarint appends value in the file format's varint (the B-tree Pages
// section of the file format document): groups of seven bits, most
// significant first, the high bit set on all but the last. It panics for a
// value past maxVarint, which it does not write.
func appendVarint(data []byte, value uint64) []byte {
	if value > maxVarint {
		panic(fmt.Sprintf("sqlitetest: varint %d needs the nine-byte form, which is not written", value))
	}
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
