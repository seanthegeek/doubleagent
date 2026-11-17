package sqlitefile

import (
	"errors"
	"fmt"
	"io"
)

// mainView is the main file as SQLite would leave it after rolling back a
// hot journal: the file's bytes, with the journal's saved pages laid over
// them and the length the rollback cuts or grows it to. Bytes past the
// file's end and inside that length read as zeros, as SQLite's short reads do.
type mainView struct {
	file     io.ReaderAt
	fileSize int64
	size     int64
	journal  io.ReaderAt
	// chunk is the journal's page size, and restored maps a chunk index of
	// the main file to the journal offset of its saved content; both are
	// zero and nil when no journal is played back.
	chunk    int64
	restored map[int64]int64
}

// newMainView returns the view of a main file with no journal played.
func newMainView(file io.ReaderAt, size int64) *mainView {
	return &mainView{file: file, fileSize: size, size: size}
}

// readAt fills target with the view's bytes from offset, zeros past its end.
func (v *mainView) readAt(target []byte, offset int64) error {
	clear(target)
	end := min(offset+int64(len(target)), v.size)
	for position := offset; position < end; {
		source, sourceOffset, limit := v.locate(position)
		count := min(limit, end) - position
		if source != nil {
			part := target[position-offset : position-offset+count]
			if err := readFull(source, part, sourceOffset); err != nil {
				return err
			}
		}
		position += count
	}
	return nil
}

// locate returns where the view's byte at position comes from (the journal,
// the file, or nil for zeros), its offset there, and the view offset where
// that source stops serving.
func (v *mainView) locate(position int64) (io.ReaderAt, int64, int64) {
	limit := v.size
	if v.chunk > 0 {
		index := position / v.chunk
		limit = (index + 1) * v.chunk
		if saved, ok := v.restored[index]; ok {
			return v.journal, saved + position - index*v.chunk, limit
		}
	}
	if position < v.fileSize {
		return v.file, position, min(limit, v.fileSize)
	}
	return nil, 0, limit
}

// readFull reads exactly len(target) bytes at offset, which the caller has
// checked lie inside the source.
func readFull(source io.ReaderAt, target []byte, offset int64) error {
	n, err := source.ReadAt(target, offset)
	if n == len(target) {
		return nil
	}
	if err == nil || errors.Is(err, io.EOF) {
		err = io.ErrUnexpectedEOF
	}
	return fmt.Errorf("read %d bytes at offset %d: %w", len(target), offset, err)
}
