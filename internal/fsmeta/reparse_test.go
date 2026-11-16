package fsmeta

import (
	"encoding/binary"
	"slices"
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

func TestParseReparseRefusesANameThatDoesNotRoundTrip(t *testing.T) {
	for name, units := range map[string][]uint16{
		"an unpaired high surrogate": {'a', 0xD800, 'b'},
		"an unpaired low surrogate":  {0xDC00},
		"a reversed pair":            {0xDC00, 0xD800},
		"a NUL unit":                 {'a', 0, 'b'},
	} {
		t.Run(name, func(t *testing.T) {
			data := reparseUnits(TagSymlink, units, 1)
			if got, err := ParseReparse(data); err == nil {
				t.Errorf("ParseReparse = %+v, want an error", got)
			}
		})
	}
	// A stored U+FFFD, and a pair outside the Basic Multilingual Plane, are
	// names of their own and round-trip.
	for _, name := range []string{"\uFFFD", "𝄞"} {
		data := reparseBuffer(TagSymlink, name, name, 1)
		if got, err := ParseReparse(data); err != nil || got.Substitute != name {
			t.Errorf("ParseReparse(%q) = %+v, %v", name, got, err)
		}
	}
}

func TestParseReparseRefusesADataLengthPastTheBytes(t *testing.T) {
	data := reparseBuffer(TagSymlink, `\??\C:\x`, `C:\x`, 0)
	binary.LittleEndian.PutUint16(data[4:], uint16(len(data)-8+2))
	if got, err := ParseReparse(data); err == nil {
		t.Errorf("ParseReparse = %+v, want an error", got)
	}
	short := reparseBuffer(TagSymlink, `\??\C:\x`, `C:\x`, 0)
	binary.LittleEndian.PutUint16(short[4:], 12+2)
	if got, err := ParseReparse(short); err == nil {
		t.Errorf("ParseReparse with the name past the data length = %+v, want an error", got)
	}
}

// reparseUnits is reparseBuffer for a substitute name given as raw UTF-16
// units, with no print name.
func reparseUnits(tag uint32, units []uint16, flags uint32) []byte {
	data := reparseBuffer(tag, "", "", flags)
	binary.LittleEndian.PutUint16(data[4:], uint16(len(data)-8+2*len(units)))
	binary.LittleEndian.PutUint16(data[10:], uint16(2*len(units)))
	for _, unit := range units {
		data = binary.LittleEndian.AppendUint16(data, unit)
	}
	return data
}

// FuzzParseReparseNameReEncodes checks the rule ParseReparse keeps: any
// substitute name it accepts re-encodes to exactly the UTF-16 units stored.
func FuzzParseReparseNameReEncodes(fuzz *testing.F) {
	for _, seed := range [][]byte{{'a', 0}, {0x00, 0xD8}, {0x00, 0xDC, 0x00, 0xD8},
		{0x34, 0xD8, 0x1E, 0xDD}, {0xFD, 0xFF}, {}} {
		fuzz.Add(seed)
	}
	fuzz.Fuzz(func(t *testing.T, raw []byte) {
		units := make([]uint16, len(raw)/2)
		for i := range units {
			units[i] = binary.LittleEndian.Uint16(raw[2*i:])
		}
		parsed, err := ParseReparse(reparseUnits(TagSymlink, units, 1))
		if err != nil {
			return
		}
		if got := utf16.Encode([]rune(parsed.Substitute)); !slices.Equal(got, units) {
			t.Errorf("accepted %q, which re-encodes to %v, not the stored %v", parsed.Substitute,
				got, units)
		}
	})
}
