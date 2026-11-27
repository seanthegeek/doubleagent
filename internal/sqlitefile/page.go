package sqlitefile

import (
	"encoding/binary"
	"fmt"
)

// B-tree page types, the flag byte of "B-tree Pages".
const (
	interiorIndex = 0x02
	interiorTable = 0x05
	leafIndex     = 0x0a
	leafTable     = 0x0d
)

// pointerMapPage reports whether page number holds pointer-map entries
// ("Pointer Map or Ptrmap Pages"): page 2 and every page after the run of
// usable/5 pages the previous one describes, moved one on when it would
// fall on the lock-byte page.
func (d *Database) pointerMapPage(number uint32) bool {
	if number < 2 {
		return false
	}
	perMap := d.usable/5 + 1
	mapPage := (int64(number)-2)/perMap*perMap + 2
	if mapPage == pendingByte/d.pageSize+1 {
		mapPage++
	}
	return mapPage == int64(number)
}

// readPage returns page number's bytes, from the log's last committed frame
// for it or from the main file. It refuses a number past the page count, the
// lock-byte page and, in an auto-vacuum database, a pointer-map page: none
// of them can belong to a b-tree or an overflow chain.
func (d *Database) readPage(number uint32) ([]byte, error) {
	if number == 0 || number > d.pageCount {
		return nil, malformed("page %d is outside the database's %d pages", number, d.pageCount)
	}
	if int64(number) == pendingByte/d.pageSize+1 {
		return nil, malformed("page %d is the lock-byte page", number)
	}
	if d.autoVacuum && d.pointerMapPage(number) {
		return nil, malformed("page %d is a pointer-map page", number)
	}
	data := make([]byte, d.pageSize)
	if d.log != nil {
		found, err := d.log.readPage(number, data)
		if err != nil {
			return nil, fmt.Errorf("read page %d from the write-ahead log: %w", number, err)
		}
		if found {
			return data, nil
		}
	}
	if err := d.main.readAt(data, (int64(number)-1)*d.pageSize); err != nil {
		return nil, fmt.Errorf("read page %d: %w", number, err)
	}
	return data, nil
}

// btreePage is one parsed b-tree page: its bytes, the offset of its b-tree
// header (100 on page 1, after the database header), its type and cells.
type btreePage struct {
	number     uint32
	data       []byte
	kind       byte
	cellCount  int
	cellsAt    int
	rightChild uint32
}

// leaf reports whether the page is a leaf page.
func (p *btreePage) leaf() bool {
	return p.kind == leafTable || p.kind == leafIndex
}

// loadPage reads and parses b-tree page number, checking that its type is
// one of the four, that it belongs to a table or index tree as wanted, and
// that its cell pointer array fits in the page.
func (d *Database) loadPage(number uint32, table bool) (*btreePage, error) {
	data, err := d.readPage(number)
	if err != nil {
		return nil, err
	}
	offset := 0
	if number == 1 {
		offset = 100
	}
	page := &btreePage{number: number, data: data, kind: data[offset]}
	switch page.kind {
	case leafTable, interiorTable:
		if !table {
			return nil, malformed("page %d is a table b-tree page inside an index b-tree", number)
		}
	case leafIndex, interiorIndex:
		if table {
			return nil, malformed("page %d is an index b-tree page inside a table b-tree", number)
		}
	default:
		return nil, malformed("page %d has b-tree page type %d", number, page.kind)
	}
	page.cellCount = int(binary.BigEndian.Uint16(data[offset+3:]))
	page.cellsAt = offset + 8
	if !page.leaf() {
		page.rightChild = binary.BigEndian.Uint32(data[offset+8:])
		page.cellsAt += 4
	}
	if int64(page.cellsAt+2*page.cellCount) > d.usable {
		return nil, malformed("page %d claims %d cells, more than fit", number, page.cellCount)
	}
	return page, nil
}

// cell returns the bytes of cell i, from its offset to the end of the
// page's usable space, after checking the offset lies past the cell
// pointer array and inside that space.
func (d *Database) cell(page *btreePage, i int) ([]byte, error) {
	offset := int(binary.BigEndian.Uint16(page.data[page.cellsAt+2*i:]))
	if offset < page.cellsAt+2*page.cellCount || int64(offset) >= d.usable {
		return nil, malformed("page %d cell %d starts at offset %d, outside its content area",
			page.number, i, offset)
	}
	return page.data[offset:d.usable], nil
}

// leftChild returns the child page number that starts an interior cell.
func (d *Database) leftChild(page *btreePage, i int) (uint32, error) {
	cell, err := d.cell(page, i)
	if err != nil {
		return 0, err
	}
	if len(cell) < 4 {
		return 0, malformed("page %d cell %d is cut short", page.number, i)
	}
	return binary.BigEndian.Uint32(cell), nil
}
