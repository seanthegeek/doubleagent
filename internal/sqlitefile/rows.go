package sqlitefile

import (
	"encoding/binary"
)

// maxPayload is the largest payload a cell may hold ("B-tree Pages").
const maxPayload = 2147483647

// Rows walks one b-tree in key order: a table b-tree's rows by rowid, or an
// index b-tree's keys, the form a WITHOUT ROWID table's rows take. Pages
// are read as the walk reaches them, so the rows before a damaged page are
// delivered before Err reports it:
//
//	rows := database.TableRows(root)
//	for rows.Next() { values, err := rows.Record(); ... }
//	if err := rows.Err(); err != nil { ... }
type Rows struct {
	database *Database
	table    bool
	root     uint32
	started  bool
	stack    []*treeStep
	visited  map[uint32]bool
	rowid    int64
	payload  cellPayload
	err      error
}

// treeStep is one page of the path from the root to the current cell: the
// next child or cell to visit, and for an interior page whether a child was
// just finished, after which an index page yields that child's divider key.
type treeStep struct {
	page       *btreePage
	next       int
	afterChild bool
}

// cellPayload is a cell's payload as the page holds it: the bytes stored
// on the page, the full length and the first overflow page, 0 when none.
type cellPayload struct {
	local    []byte
	total    int64
	overflow uint32
}

// TableRows returns the rows of the table b-tree whose root is page root.
func (d *Database) TableRows(root uint32) *Rows {
	return &Rows{database: d, table: true, root: root, visited: map[uint32]bool{}}
}

// IndexRows returns the keys of the index b-tree whose root is page root:
// an index's keys, or the rows of a WITHOUT ROWID table.
func (d *Database) IndexRows(root uint32) *Rows {
	return &Rows{database: d, root: root, visited: map[uint32]bool{}}
}

// Next moves to the next row and reports whether there is one. It returns
// false at the end of the b-tree and at the first error, which Err returns.
func (r *Rows) Next() bool {
	if r.err != nil {
		return false
	}
	if !r.started {
		r.started = true
		if !r.push(r.root) {
			return false
		}
	}
	for len(r.stack) > 0 {
		step := r.stack[len(r.stack)-1]
		if step.page.leaf() {
			if step.next < step.page.cellCount {
				step.next++
				return r.readCell(step.page, step.next-1)
			}
			r.stack = r.stack[:len(r.stack)-1]
			continue
		}
		if step.afterChild {
			step.afterChild = false
			if !r.table && step.next-1 < step.page.cellCount {
				return r.readCell(step.page, step.next-1)
			}
		}
		if step.next > step.page.cellCount {
			r.stack = r.stack[:len(r.stack)-1]
			continue
		}
		if !r.descend(step) {
			return false
		}
	}
	return false
}

// descend moves from an interior page into its next child: the left child
// of the next cell, or after the last cell the right-most child.
func (r *Rows) descend(step *treeStep) bool {
	child := step.page.rightChild
	if step.next < step.page.cellCount {
		var err error
		if child, err = r.database.leftChild(step.page, step.next); err != nil {
			r.err = err
			return false
		}
	}
	step.next++
	step.afterChild = true
	return r.push(child)
}

// push loads page number onto the path, refusing a page the walk has
// already visited, so a damaged tree that loops ends with an error.
func (r *Rows) push(number uint32) bool {
	if r.visited[number] {
		r.err = malformed("page %d appears twice in the b-tree rooted at page %d", number, r.root)
		return false
	}
	r.visited[number] = true
	page, err := r.database.loadPage(number, r.table)
	if err != nil {
		r.err = err
		return false
	}
	r.stack = append(r.stack, &treeStep{page: page})
	return true
}

// readCell makes cell i of page the current row: a table leaf cell's rowid
// and payload, or an index cell's key.
func (r *Rows) readCell(page *btreePage, i int) bool {
	cell, err := r.database.cell(page, i)
	if err != nil {
		r.err = err
		return false
	}
	if page.kind == interiorIndex {
		cell = cell[min(4, len(cell)):]
	}
	total, used := readVarint(cell)
	if used == 0 {
		r.err = malformed("page %d cell %d is cut short", page.number, i)
		return false
	}
	cell = cell[used:]
	r.rowid = 0
	if page.kind == leafTable {
		rowid, used := readVarint(cell)
		if used == 0 {
			r.err = malformed("page %d cell %d is cut short", page.number, i)
			return false
		}
		r.rowid, cell = int64(rowid), cell[used:]
	}
	r.payload, r.err = r.database.splitPayload(page, i, cell, total)
	return r.err == nil
}

// Rowid returns the current row's rowid; it is 0 in an index b-tree.
func (r *Rows) Rowid() int64 {
	return r.rowid
}

// Record reads the current row's payload, following its overflow pages,
// and decodes it. A record may hold fewer values than its table has
// columns, after an ALTER TABLE ADD COLUMN; the schema supplies the rest.
func (r *Rows) Record() ([]Value, error) {
	payload, err := r.database.readPayload(r.payload)
	if err != nil {
		return nil, err
	}
	return decodeRecord(payload)
}

// Err returns the error that ended the walk, or nil at a clean end.
func (r *Rows) Err() error {
	return r.err
}

// splitPayload divides a cell's payload of total bytes, starting at cell,
// into the part stored on the page and the first overflow page, after the
// thresholds of "B-tree Pages": X the most a page keeps whole, M the least
// it keeps when the payload spills.
func (d *Database) splitPayload(page *btreePage, i int, cell []byte, total uint64) (cellPayload, error) {
	if total > maxPayload {
		return cellPayload{}, malformed("page %d cell %d claims a payload of %d bytes", page.number, i, total)
	}
	size := int64(total)
	most := ((d.usable-12)*64/255 - 23)
	if page.kind == leafTable {
		most = d.usable - 35
	}
	local := size
	if size > most {
		least := (d.usable-12)*32/255 - 23
		local = least + (size-least)%(d.usable-4)
		if local > most {
			local = least
		}
	}
	stored := local
	if local < size {
		stored += 4
	}
	if stored > int64(len(cell)) {
		return cellPayload{}, malformed("page %d cell %d runs past the end of the page", page.number, i)
	}
	payload := cellPayload{local: cell[:local], total: size}
	if local < size {
		payload.overflow = binary.BigEndian.Uint32(cell[local:])
	}
	return payload, nil
}

// readPayload returns a payload whole, reading its overflow chain ("Cell
// Payload Overflow Pages"): each page holds the next page's number and then
// its share of the payload. A chain that ends early or loops is damage.
func (d *Database) readPayload(payload cellPayload) ([]byte, error) {
	if payload.overflow == 0 {
		return payload.local, nil
	}
	capacity := max(int64(len(payload.local)), min(payload.total, 1<<20))
	whole := make([]byte, len(payload.local), capacity)
	copy(whole, payload.local)
	visited := map[uint32]bool{}
	for next := payload.overflow; int64(len(whole)) < payload.total; {
		if next == 0 {
			return nil, malformed("overflow chain ends %d bytes short", payload.total-int64(len(whole)))
		}
		if visited[next] {
			return nil, malformed("overflow chain visits page %d twice", next)
		}
		visited[next] = true
		data, err := d.readPage(next)
		if err != nil {
			return nil, err
		}
		share := min(payload.total-int64(len(whole)), d.usable-4)
		whole = append(whole, data[4:4+share]...)
		next = binary.BigEndian.Uint32(data)
	}
	return whole, nil
}
