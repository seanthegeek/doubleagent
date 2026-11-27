package sqlitefile

import (
	"bytes"
	"encoding/binary"
	"testing"
)

// Each fuzz target opens what it is given and walks the schema and every
// tree: reading damaged input must return, never panic or hang.

func FuzzMainFile(fuzz *testing.F) {
	fuzz.Add([]byte{})
	for _, name := range []string{"page_512.db", "page_1024.db", "shapes.db", "autovacuum.db",
		"log_fresh.db", "utf16le.db"} {
		fuzz.Add(testdataBytes(fuzz, name))
	}
	fuzz.Fuzz(func(_ *testing.T, main []byte) {
		walkEverything(OpenBytes(main, nil, nil))
	})
}

// withLogChecksums returns a copy of log whose header checksum, and each
// whole frame's salts and running checksum, are the ones SQLite expects,
// so a mutated log reaches the frames rather than failing its checksums.
func withLogChecksums(log []byte) []byte {
	log = bytes.Clone(log)
	if len(log) < logHeaderSize {
		return log
	}
	bigEndian := binary.BigEndian.Uint32(log) == logMagicBig
	first, second := logChecksum(log[:24], bigEndian, 0, 0)
	binary.BigEndian.PutUint32(log[24:], first)
	binary.BigEndian.PutUint32(log[28:], second)
	pageSize := int(binary.BigEndian.Uint32(log[8:]))
	if pageSize < 512 || pageSize > maxPageSize {
		return log
	}
	frame := logFrameHeaderSize + pageSize
	for offset := logHeaderSize; offset+frame <= len(log); offset += frame {
		copy(log[offset+8:offset+16], log[16:24])
		first, second = logChecksum(log[offset:offset+8], bigEndian, first, second)
		first, second = logChecksum(log[offset+logFrameHeaderSize:offset+frame], bigEndian, first, second)
		binary.BigEndian.PutUint32(log[offset+16:], first)
		binary.BigEndian.PutUint32(log[offset+20:], second)
	}
	return log
}

func FuzzLog(fuzz *testing.F) {
	for _, name := range []string{"log_rows.db", "log_big_endian.db", "log_reset.db", "log_fresh.db"} {
		fuzz.Add(testdataBytes(fuzz, name+"-wal"))
	}
	main := testdataBytes(fuzz, "log_rows.db")
	fuzz.Fuzz(func(_ *testing.T, log []byte) {
		walkEverything(OpenBytes(main, log, nil))
		walkEverything(OpenBytes(main, withLogChecksums(log), nil))
	})
}

func FuzzJournal(fuzz *testing.F) {
	for _, name := range []string{"journal_hot.db", "journal_super.db", "journal_count_all.db"} {
		fuzz.Add(testdataBytes(fuzz, name+"-journal"))
	}
	main := testdataBytes(fuzz, "journal_hot.db")
	fuzz.Fuzz(func(_ *testing.T, journal []byte) {
		walkEverything(OpenBytes(main, nil, journal))
	})
}
