package sqlitefile

import (
	"bytes"
	"encoding/binary"
	"io"
)

// The rollback journal, after "The Rollback Journal" in the file format
// document. The conditions under which SQLite plays one back are in the
// package comment; testdata/generate.py records what SQLite reads from a
// copy of a torn database beside each kind of journal.
var journalMagic = [8]byte{0xd9, 0xd5, 0x05, 0xf9, 0x20, 0xa1, 0x63, 0xd7}

const (
	// firstSectorSize is the sector size SQLite assumes before it reads the
	// journal's own: 512 on every file system that overwrites a sector
	// safely, its default. A journal shorter than it is never played back.
	firstSectorSize = 512
	maxSectorSize   = 65536
	// pendingByte is the offset of the lock-byte page ("The Lock-Byte Page").
	pendingByte = 1073741824
	// maxSuperJournalName is the longest super-journal name SQLite reads on
	// Unix: a name of its full path buffer or more counts as no name.
	maxSuperJournalName = 512
	defaultPageSize     = 4096
)

// journalHeader is the part of a journal header playback uses.
type journalHeader struct {
	records    uint32
	nonce      uint32
	pages      uint32
	sectorSize uint32
	pageSize   uint32
}

// journalPlayback holds the state of one rollback: the journal, its size,
// the page size and sector size its first header set, and the database
// size in pages the rollback restores.
type journalPlayback struct {
	journal    io.ReaderAt
	size       int64
	pageSize   int64
	sectorSize int64
	pages      int64
}

// readHeader reads the journal header at offset and reports whether
// playback goes on. The first header also sets the page and sector sizes;
// one that is out of range ends playback before anything is restored.
func (p *journalPlayback) readHeader(offset int64) (journalHeader, bool, error) {
	if offset+p.sectorSize > p.size {
		return journalHeader{}, false, nil
	}
	data := make([]byte, 28)
	if err := readFull(p.journal, data, offset); err != nil {
		return journalHeader{}, false, err
	}
	if !bytes.Equal(data[:8], journalMagic[:]) {
		return journalHeader{}, false, nil
	}
	header := journalHeader{records: binary.BigEndian.Uint32(data[8:]),
		nonce: binary.BigEndian.Uint32(data[12:]), pages: binary.BigEndian.Uint32(data[16:]),
		sectorSize: binary.BigEndian.Uint32(data[20:]), pageSize: binary.BigEndian.Uint32(data[24:])}
	if offset != 0 {
		return header, true, nil
	}
	pageSize := int64(header.pageSize)
	if pageSize == 0 {
		// Journals of SQLite before 3.5.8 leave the page size to the database.
		pageSize = p.pageSize
	}
	sectorSize := int64(header.sectorSize)
	if !powerOfTwo(pageSize) || pageSize < 512 || pageSize > maxPageSize ||
		!powerOfTwo(sectorSize) || sectorSize < 32 || sectorSize > maxSectorSize {
		return journalHeader{}, false, nil
	}
	p.pageSize, p.sectorSize = pageSize, sectorSize
	return header, true, nil
}

// truncate cuts the view to the database size the first header records, or
// grows it when the file is at least a page short of it, as SQLite does.
func (p *journalPlayback) truncate(view *mainView) {
	view.journal = p.journal
	view.chunk = p.pageSize
	view.restored = map[int64]int64{}
	newSize := p.pages * p.pageSize
	if view.size > newSize || view.size+p.pageSize <= newSize {
		view.size = newSize
	}
}

// journalChecksum is a page record's checksum: the nonce plus every 200th
// byte of the page counted back from 200 bytes before its end. The format
// document's loop also counts offset 0, SQLite's stops above it; no page
// size of a valid database reaches 0, so the two agree.
func journalChecksum(data []byte, nonce uint32) uint32 {
	sum := nonce
	for i := len(data) - 200; i > 0; i -= 200 {
		sum += uint32(data[i])
	}
	return sum
}

// playRecords restores up to count page records from offset with the
// segment's checksum nonce. It returns the offset after the last record
// read and whether playback is over: at a short read, a page number of 0
// or of the lock-byte page, or a record whose checksum does not match.
func (p *journalPlayback) playRecords(view *mainView, offset, count int64,
	nonce uint32) (int64, bool, error) {
	data := make([]byte, p.pageSize)
	number := make([]byte, 4)
	lockPage := pendingByte/p.pageSize + 1
	for range count {
		if offset+4+p.pageSize > p.size {
			return offset, true, nil
		}
		if err := readFull(p.journal, number, offset); err != nil {
			return offset, true, err
		}
		page := int64(binary.BigEndian.Uint32(number))
		dataOffset := offset + 4
		offset += p.pageSize + 8
		if page == 0 || page == lockPage {
			return offset, true, nil
		}
		if page > p.pages {
			continue
		}
		if offset > p.size {
			return offset, true, nil
		}
		if err := readFull(p.journal, data, dataOffset); err != nil {
			return offset, true, err
		}
		stored := make([]byte, 4)
		if err := readFull(p.journal, stored, offset-4); err != nil {
			return offset, true, err
		}
		if binary.BigEndian.Uint32(stored) != journalChecksum(data, nonce) {
			return offset, true, nil
		}
		view.restored[page-1] = dataOffset
		view.size = max(view.size, page*p.pageSize)
	}
	return offset, false, nil
}

// run reads the journal's segments, each a header padded to a sector and
// then its page records, until a header or record ends playback.
func (p *journalPlayback) run(view *mainView) error {
	offset := int64(0)
	for {
		header, ok, err := p.readHeader(offset)
		if err != nil || !ok {
			return err
		}
		recordOffset := offset + p.sectorSize
		records := int64(header.records)
		if header.records == 0xffffffff {
			// A journal written without syncing holds records to its end.
			records = (p.size - p.sectorSize) / (p.pageSize + 8)
		}
		if offset == 0 {
			p.pages = int64(header.pages)
			p.truncate(view)
		}
		recordOffset, done, err := p.playRecords(view, recordOffset, records, header.nonce)
		if err != nil || done {
			return err
		}
		// The next header starts at the next sector boundary.
		offset = (recordOffset + p.sectorSize - 1) / p.sectorSize * p.sectorSize
	}
}

// namesSuperJournal reports whether the journal ends with the name of a
// super-journal: the name, its length, its checksum and the journal magic.
func namesSuperJournal(journal *io.SectionReader) (bool, error) {
	size := journal.Size()
	if size < 16 {
		return false, nil
	}
	tail := make([]byte, 16)
	if err := readFull(journal, tail, size-16); err != nil {
		return false, err
	}
	length := int64(binary.BigEndian.Uint32(tail))
	if !bytes.Equal(tail[8:], journalMagic[:]) || length == 0 ||
		length > maxSuperJournalName || length > size-16 {
		return false, nil
	}
	name := make([]byte, length)
	if err := readFull(journal, name, size-16-length); err != nil {
		return false, err
	}
	sum := binary.BigEndian.Uint32(tail[4:])
	for _, octet := range name {
		// SQLite sums the name as C chars, which are signed where it runs.
		sum -= uint32(int32(int8(octet)))
	}
	return sum == 0 && name[0] != 0, nil
}

// mainHeaderPageSize is the page size SQLite takes from the main file's
// own header before any rollback, or its default when that is not valid.
func mainHeaderPageSize(view *mainView) (int64, error) {
	header := make([]byte, 100)
	if err := view.readAt(header, 0); err != nil {
		return 0, err
	}
	if size := headerPageSize(header); size != 0 {
		return size, nil
	}
	return defaultPageSize, nil
}

// playJournal lays a hot journal's saved pages over the view, as SQLite's
// rollback writes them back into the main file. A journal that is not hot
// leaves the view as it is. Only a failed read is an error: damage in a
// journal ends playback, as it does in SQLite.
func playJournal(view *mainView, journal *io.SectionReader) error {
	if journal == nil || journal.Size() == 0 || view.fileSize == 0 {
		return nil
	}
	first := make([]byte, 1)
	if err := readFull(journal, first, 0); err != nil {
		return err
	}
	if first[0] == 0 {
		return nil
	}
	named, err := namesSuperJournal(journal)
	if err != nil || named {
		return err
	}
	pageSize, err := mainHeaderPageSize(view)
	if err != nil {
		return err
	}
	playback := &journalPlayback{journal: journal, size: journal.Size(),
		pageSize: pageSize, sectorSize: firstSectorSize}
	return playback.run(view)
}
