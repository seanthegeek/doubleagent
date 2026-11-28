package fsmeta

import (
	"encoding/binary"
	"errors"
	"slices"
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

// exactName decodes units and reports whether the name re-encodes to the
// same UTF-16 units and holds no NUL unit: only then is the decoded name the
// name the file system follows.
func exactName(units []uint16) (string, bool) {
	if slices.Contains(units, 0) {
		return "", false
	}
	runes := utf16.Decode(units)
	return string(runes), slices.Equal(utf16.Encode(runes), units)
}

// ParseReparse reads a REPARSE_DATA_BUFFER: the tag and the data length,
// then for a symbolic link or junction the substitute name's offset and
// length and the name in UTF-16 (a symbolic link has a flags word before
// the names). Any other tag comes back with no name. A buffer shorter than
// it claims, a data length past the bytes returned, a name outside the
// data, or a substitute name that does not re-encode to the same UTF-16
// units (an unpaired surrogate would decode to U+FFFD, another file's name)
// or holds a NUL is an error, so the walk never checks one name while
// Windows follows another.
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
	size := 8 + int(binary.LittleEndian.Uint16(data[4:]))
	if size > len(data) || size < header {
		return Reparse{}, errReparseData
	}
	offset := int(binary.LittleEndian.Uint16(data[8:]))
	length := int(binary.LittleEndian.Uint16(data[10:]))
	start, end := header+offset, header+offset+length
	if length%2 != 0 || end > size {
		return Reparse{}, errReparseData
	}
	units := make([]uint16, length/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(data[start+2*i:])
	}
	name, ok := exactName(units)
	if !ok {
		return Reparse{}, errReparseData
	}
	parsed.Substitute = name
	if parsed.Tag == TagSymlink {
		parsed.Relative = binary.LittleEndian.Uint32(data[16:])&1 != 0
	}
	return parsed, nil
}
