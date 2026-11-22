package sqlitefile

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"
)

// ErrMalformed is wrapped by every error that reports a damaged database:
// a value out of range, a page of the wrong kind, a b-tree that loops.
var ErrMalformed = errors.New("malformed database")

// ErrNotDatabase reports a main file that does not start with the SQLite
// header string.
var ErrNotDatabase = errors.New("not a SQLite database")

// ErrUTF16 reports a database whose text is stored as UTF-16.
var ErrUTF16 = errors.New("UTF-16 databases are not supported")

// malformed returns an error wrapping ErrMalformed with the formatted detail.
func malformed(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrMalformed, fmt.Sprintf(format, args...))
}

const (
	headerString = "SQLite format 3\x00"
	maxPageSize  = 65536
	minUsable    = 480
)

// Files holds the files of one database, each with its size: the main file
// and, when present, its write-ahead log (name-wal) and its rollback journal
// (name-journal). A nil Log or Journal means the file is absent.
type Files struct {
	Main    *io.SectionReader
	Log     *io.SectionReader
	Journal *io.SectionReader
}

// Database is an open, read-only database. Its methods may be called from
// several goroutines; each Rows belongs to one.
type Database struct {
	main       *mainView
	log        *logIndex
	pageSize   int64
	usable     int64
	pageCount  uint32
	autoVacuum bool
	closers    []io.Closer
	// schemaOnce reads the schema table the first time it is asked for;
	// schema holds what it read, or schemaErr why it could not.
	schemaOnce sync.Once
	schema     *schemaIndex
	schemaErr  error
}

// Open reads the database in files, after playing back a hot journal and
// applying the committed frames of a valid log as the package comment says.
// A main file of 0 bytes is an empty database. Open fails on a main file
// that is not a database, a UTF-16 database, an unknown log format or a
// header that is out of range.
func Open(files Files) (*Database, error) {
	if files.Main == nil {
		return nil, errors.New("open database: no main file")
	}
	view := newMainView(files.Main, files.Main.Size())
	if err := playJournal(view, files.Journal); err != nil {
		return nil, fmt.Errorf("read rollback journal: %w", err)
	}
	database := &Database{main: view}
	if view.size == 0 {
		return database, nil
	}
	log, err := readLog(files.Log)
	if err != nil {
		return nil, fmt.Errorf("read write-ahead log: %w", err)
	}
	database.log = log
	if err := database.readHeader(); err != nil {
		return nil, err
	}
	return database, nil
}

// OpenBytes opens a database held in memory; a nil log or journal is absent.
func OpenBytes(main, log, journal []byte) (*Database, error) {
	files := Files{Main: sectionOf(main)}
	if log != nil {
		files.Log = sectionOf(log)
	}
	if journal != nil {
		files.Journal = sectionOf(journal)
	}
	return Open(files)
}

// sectionOf returns a reader over data that knows its size.
func sectionOf(data []byte) *io.SectionReader {
	return io.NewSectionReader(bytes.NewReader(data), 0, int64(len(data)))
}

// Close closes the files OpenPath opened; for other databases it does nothing.
func (d *Database) Close() error {
	err := closeAll(d.closers)
	d.closers = nil
	return err
}

// PageSize returns the page size in bytes, or 0 for an empty database.
func (d *Database) PageSize() int {
	return int(d.pageSize)
}

// PageCount returns the number of pages the database holds.
func (d *Database) PageCount() uint32 {
	return d.pageCount
}

// readHeader validates page 1's database header ("The Database Header"),
// read through the log when the log holds page 1, and sets the page size
// and the page count.
func (d *Database) readHeader() error {
	header := make([]byte, 100)
	inLog := false
	if d.log != nil {
		var err error
		if inLog, err = d.log.readPage(1, header); err != nil {
			return fmt.Errorf("read write-ahead log: %w", err)
		}
	}
	if !inLog {
		if err := d.main.readAt(header, 0); err != nil {
			return fmt.Errorf("read database header: %w", err)
		}
	}
	if err := checkHeader(header); err != nil {
		return err
	}
	d.pageSize = headerPageSize(header)
	d.usable = d.pageSize - int64(header[20])
	d.autoVacuum = binary.BigEndian.Uint32(header[52:]) != 0
	return d.countPages(header)
}

// checkHeader reports a header SQLite would refuse to read.
func checkHeader(header []byte) error {
	switch {
	case string(header[:16]) != headerString:
		return ErrNotDatabase
	case header[19] > 2:
		return fmt.Errorf("file format read version %d is not supported", header[19])
	case header[21] != 64 || header[22] != 32 || header[23] != 32:
		return malformed("header payload fractions are %d, %d and %d, not 64, 32 and 32",
			header[21], header[22], header[23])
	case headerPageSize(header) == 0:
		return malformed("header page size %d is not a power of two from 512 to 65536",
			binary.BigEndian.Uint16(header[16:]))
	case headerPageSize(header)-int64(header[20]) < minUsable:
		return malformed("header reserves %d bytes of each page, leaving fewer than 480", header[20])
	}
	// SQLite reads the encoding's low two bits, with 0 meaning UTF-8.
	if encoding := binary.BigEndian.Uint32(header[56:]) & 3; encoding > 1 {
		return ErrUTF16
	}
	if format := binary.BigEndian.Uint32(header[44:]); format > 4 {
		return fmt.Errorf("schema format %d is not supported", format)
	}
	return nil
}

// headerPageSize decodes the page size field, where 1 stands for 65536,
// and returns 0 when it is not a power of two from 512 to 65536.
func headerPageSize(header []byte) int64 {
	size := int64(header[16])<<8 | int64(header[17])<<16
	if !powerOfTwo(size) || size < 512 || size > maxPageSize {
		return 0
	}
	return size
}

// powerOfTwo reports whether n is a positive power of two.
func powerOfTwo(n int64) bool {
	return n > 0 && n&(n-1) == 0
}

// countPages sets the page count as SQLite's b-tree layer does: the
// header's count when it is valid ("In-header database size"), which must
// not exceed the pages the files hold, and otherwise the pages they hold:
// the size the last commit frame records, or the main file's length.
func (d *Database) countPages(header []byte) error {
	var available int64
	if d.log != nil {
		available = int64(d.log.pageCount)
	} else {
		available = (d.main.size + d.pageSize - 1) / d.pageSize
	}
	counted := int64(binary.BigEndian.Uint32(header[28:]))
	if counted == 0 || !bytes.Equal(header[24:28], header[92:96]) {
		counted = available
	}
	if counted > available {
		return malformed("header counts %d pages but the files hold %d", counted, available)
	}
	d.pageCount = uint32(min(counted, 0xffffffff))
	return nil
}
