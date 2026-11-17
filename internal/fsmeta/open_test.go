package fsmeta

import "testing"

func TestHoldsAccessTime(t *testing.T) {
	cases := []struct {
		name           string
		readOnly       bool
		ntfsUpdatesOff bool
		fileSystem     string
		want           bool
	}{
		{"updates on: an NTFS file is held", false, false, "NTFS", true},
		{"updates on: a ReFS file is held", false, false, "ReFS", true},
		{"updates off: an NTFS file is opened read-only", false, true, "NTFS", false},
		{"updates off: a ReFS file is still held", false, true, "ReFS", true},
		{"updates off: an exFAT file is still held", false, true, "exFAT", true},
		{"updates off: a FAT32 file is still held", false, true, "FAT32", true},
		{"updates off: a volume whose file system is unknown is held", false, true, "", true},
		{"--no-preserve-atime: an NTFS file is opened read-only", true, false, "NTFS", false},
		{"--no-preserve-atime: a ReFS file is opened read-only", true, true, "ReFS", false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := holdsAccessTime(test.readOnly, test.ntfsUpdatesOff, test.fileSystem); got != test.want {
				t.Errorf("holdsAccessTime = %v, want %v", got, test.want)
			}
		})
	}
}
