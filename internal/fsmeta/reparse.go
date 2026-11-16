package fsmeta

import (
	"encoding/binary"
	"errors"
	"unicode/utf16"
)

// The reparse tags of the two kinds of Windows link.
const (
	// TagSymlink is IO_REPARSE_TAG_SYMLINK.
	TagSymlink = 0xA000000C
	// TagMountPoint is IO_REPARSE_TAG_MOUNT_POINT, a junction.
	TagMountPoint = 0xA0000003
)

// Reparse is what a Windows reparse point stores: its tag, the substitute
// name the system follows, exactly as stored, and for a symbolic link
// whether SYMLINK_FLAG_RELATIVE marks that name relative.
type Reparse struct {
	Tag        uint32
	Substitute string
	Relative   bool
}

// errReparseData is ParseReparse's error for a buffer it cannot read.
var errReparseData = errors.New("reparse data cannot be parsed")

// ParseReparse reads a REPARSE_DATA_BUFFER: the tag, then for a symbolic
// link or a junction the offsets and lengths of the substitute and print
// names (a symbolic link has a flags word before its names), each name in
// UTF-16 after them. Any other tag comes back with no name. A buffer too
// short for what it claims is an error.
func ParseReparse(data []byte) (Reparse, error) {
	if len(data) < 8 {
		return Reparse{}, errReparseData
	}
	parsed := Reparse{Tag: binary.LittleEndian.Uint32(data)}
	header := 8 + 8
	switch parsed.Tag {
	case TagSymlink:
		header += 4
	case TagMountPoint:
	default:
		return parsed, nil
	}
	if len(data) < header {
		return Reparse{}, errReparseData
	}
	offset := int(binary.LittleEndian.Uint16(data[8:]))
	length := int(binary.LittleEndian.Uint16(data[10:]))
	start, end := header+offset, header+offset+length
	if length%2 != 0 || end > len(data) {
		return Reparse{}, errReparseData
	}
	units := make([]uint16, length/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[start+2*i:])
	}
	parsed.Substitute = string(utf16.Decode(units))
	if parsed.Tag == TagSymlink {
		parsed.Relative = binary.LittleEndian.Uint32(data[16:])&1 != 0
	}
	return parsed, nil
}
