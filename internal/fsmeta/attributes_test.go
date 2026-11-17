package fsmeta

import "testing"

func TestAttributesString(t *testing.T) {
	cases := []struct {
		name string
		bits uint32
		want string
	}{
		{"a hidden file is named alone", 0x2, "Hidden"},
		{"two bits are named in ascending order", 0x420, "Archive, ReparsePoint"},
		{"a directory is named", 0x10, "Directory"},
		{"no bit set prints zero", 0, "0"},
		{"every named bit is named", 0x2fff7, "ReadOnly, Hidden, System, Directory, Archive, " +
			"Device, Normal, Temporary, SparseFile, ReparsePoint, Compressed, Offline, " +
			"NotContentIndexed, Encrypted, IntegrityStream, NoScrubData"},
		{"an unnamed bit prints the whole value in decimal", 0x80020, "524320"},
		{"bit 8 is unnamed", 0x8, "8"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := AttributesString(testCase.bits); got != testCase.want {
				t.Errorf("AttributesString(%#x) = %q, want %q", testCase.bits, got, testCase.want)
			}
		})
	}
}
