package fsmeta

import (
	"encoding/binary"
	"testing"
	"unicode/utf16"
)

// reparseBuffer builds a REPARSE_DATA_BUFFER for tag with substitute name
// substitute and print name print; flags is written for a symbolic link.
func reparseBuffer(tag uint32, substitute, print string, flags uint32) []byte {
	sub := utf16.Encode([]rune(substitute))
	prt := utf16.Encode([]rune(print))
	header := 8
	if tag == TagSymlink {
		header = 12
	}
	data := make([]byte, 8+header+2*len(sub)+2*len(prt))
	binary.LittleEndian.PutUint32(data, tag)
	binary.LittleEndian.PutUint16(data[4:], uint16(len(data)-8))
	binary.LittleEndian.PutUint16(data[8:], uint16(2*len(prt)))
	binary.LittleEndian.PutUint16(data[10:], uint16(2*len(sub)))
	binary.LittleEndian.PutUint16(data[12:], 0)
	binary.LittleEndian.PutUint16(data[14:], uint16(2*len(prt)))
	if tag == TagSymlink {
		binary.LittleEndian.PutUint32(data[16:], flags)
	}
	offset := 8 + header
	for _, unit := range append(prt, sub...) {
		binary.LittleEndian.PutUint16(data[offset:], unit)
		offset += 2
	}
	return data
}

func TestParseReparseReadsTheStoredName(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want Reparse
	}{
		{"a relative symbolic link", reparseBuffer(TagSymlink, `..\x`, `..\x`, 1),
			Reparse{Tag: TagSymlink, Substitute: `..\x`, Relative: true}},
		{"an absolute symbolic link", reparseBuffer(TagSymlink, `\??\C:\x`, `C:\x`, 0),
			Reparse{Tag: TagSymlink, Substitute: `\??\C:\x`}},
		{"a volume GUID symbolic link",
			reparseBuffer(TagSymlink, `\??\Volume{1-2}\x`, `\\?\Volume{1-2}\x`, 0),
			Reparse{Tag: TagSymlink, Substitute: `\??\Volume{1-2}\x`}},
		{"a junction", reparseBuffer(TagMountPoint, `\??\D:\é`, `D:\é`, 0),
			Reparse{Tag: TagMountPoint, Substitute: `\??\D:\é`}},
		{"another tag", []byte{0x1a, 0, 0, 0x80, 0, 0, 0, 0},
			Reparse{Tag: 0x8000001a}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ParseReparse(testCase.data)
			if err != nil || got != testCase.want {
				t.Errorf("ParseReparse = %+v, %v; want %+v", got, err, testCase.want)
			}
		})
	}
}

func TestParseReparseRefusesAShortBuffer(t *testing.T) {
	whole := reparseBuffer(TagSymlink, `\??\C:\x`, `C:\x`, 0)
	odd := reparseBuffer(TagMountPoint, `\??\C:\x`, ``, 0)
	binary.LittleEndian.PutUint16(odd[10:], 3)
	cases := map[string][]byte{
		"empty":                  nil,
		"a tag only":             whole[:4],
		"a symlink header cut":   whole[:18],
		"a name past the end":    whole[:len(whole)-1],
		"an odd number of bytes": odd,
		"a junction header cut":  reparseBuffer(TagMountPoint, "", "", 0)[:12],
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			if got, err := ParseReparse(data); err == nil {
				t.Errorf("ParseReparse = %+v, want an error", got)
			}
		})
	}
}
