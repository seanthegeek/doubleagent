package sqlitefile

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

// The write-ahead log, after "The Write-Ahead Log" in the file format
// document. Which logs and frames SQLite honours is in the package comment.
const (
	logHeaderSize      = 32
	logFrameHeaderSize = 24
	logMagicLittle     = 0x377f0682
	logMagicBig        = 0x377f0683
	logVersion         = 3007000
)

// logIndex is what a reader takes from a log: the page size of its frames,
// the offset of the page data of each page's last committed frame, and the
// database size in pages that the last commit frame records.
type logIndex struct {
	file      io.ReaderAt
	pageSize  int64
	frames    map[uint32]int64
	pageCount uint32
}

// logChecksum continues the log's checksum ("Checksum Algorithm") over
// data, read as 32-bit words in the byte order the log's magic names.
func logChecksum(data []byte, bigEndian bool, first, second uint32) (uint32, uint32) {
	order := binary.ByteOrder(binary.LittleEndian)
	if bigEndian {
		order = binary.BigEndian
	}
	for i := 0; i+8 <= len(data); i += 8 {
		first += order.Uint32(data[i:]) + second
		second += order.Uint32(data[i+4:]) + first
	}
	return first, second
}

// scan reads frames in order while each is valid: its salts equal the
// header's, its page number is not 0 and the running checksum over the
// header and every frame so far matches the one it stores. The pages of a
// transaction count only once its commit frame, the one recording a
// database size, has been read.
func (l *logIndex) scan(file *io.SectionReader, header []byte, bigEndian bool,
	checksum [2]uint32) error {
	frame := make([]byte, logFrameHeaderSize+l.pageSize)
	pending := map[uint32]int64{}
	for offset := int64(logHeaderSize); offset+int64(len(frame)) <= file.Size(); offset += int64(len(frame)) {
		if err := readFull(file, frame, offset); err != nil {
			return err
		}
		page := binary.BigEndian.Uint32(frame)
		if page == 0 || !bytes.Equal(frame[8:16], header[16:24]) {
			return nil
		}
		checksum[0], checksum[1] = logChecksum(frame[:8], bigEndian, checksum[0], checksum[1])
		checksum[0], checksum[1] = logChecksum(frame[logFrameHeaderSize:], bigEndian,
			checksum[0], checksum[1])
		if checksum[0] != binary.BigEndian.Uint32(frame[16:]) ||
			checksum[1] != binary.BigEndian.Uint32(frame[20:]) {
			return nil
		}
		pending[page] = offset + logFrameHeaderSize
		if commitSize := binary.BigEndian.Uint32(frame[4:]); commitSize != 0 {
			for number, dataOffset := range pending {
				l.frames[number] = dataOffset
			}
			clear(pending)
			l.pageCount = commitSize
		}
	}
	return nil
}

// readLog scans a log the way SQLite's recovery does and returns its
// committed frames, or nil when the log is absent, not valid or holds no
// commit. Only a format version SQLite does not know is an error.
func readLog(file *io.SectionReader) (*logIndex, error) {
	if file == nil || file.Size() < logHeaderSize {
		return nil, nil
	}
	header := make([]byte, logHeaderSize)
	if err := readFull(file, header, 0); err != nil {
		return nil, err
	}
	magic := binary.BigEndian.Uint32(header)
	pageSize := int64(binary.BigEndian.Uint32(header[8:]))
	if magic != logMagicLittle && magic != logMagicBig || !powerOfTwo(pageSize) ||
		pageSize < 512 || pageSize > maxPageSize {
		return nil, nil
	}
	bigEndian := magic == logMagicBig
	first, second := logChecksum(header[:24], bigEndian, 0, 0)
	if first != binary.BigEndian.Uint32(header[24:]) || second != binary.BigEndian.Uint32(header[28:]) {
		return nil, nil
	}
	if version := binary.BigEndian.Uint32(header[4:]); version != logVersion {
		return nil, fmt.Errorf("write-ahead log format %d is not supported", version)
	}
	index := &logIndex{file: file, pageSize: pageSize, frames: map[uint32]int64{}}
	if err := index.scan(file, header, bigEndian, [2]uint32{first, second}); err != nil {
		return nil, err
	}
	if index.pageCount == 0 {
		return nil, nil
	}
	return index, nil
}

// readPage fills target with the page's data from its last committed frame,
// or reports false when the log has none. Frames of another page size give
// as many bytes as both sizes have, the rest zero, as SQLite reads them.
func (l *logIndex) readPage(number uint32, target []byte) (bool, error) {
	dataOffset, ok := l.frames[number]
	if !ok {
		return false, nil
	}
	clear(target)
	count := min(int64(len(target)), l.pageSize)
	return true, readFull(l.file, target[:count], dataOffset)
}
