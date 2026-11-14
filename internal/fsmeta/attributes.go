package fsmeta

import (
	"strconv"
	"strings"
)

// attributeName pairs a Win32 attribute bit with the name .NET gives it.
type attributeName struct {
	bit  uint32
	name string
}

// AttributesString writes Win32 attribute bits as the PowerShell collector's
// attributes field holds them, which is .NET's FileAttributes ToString: the
// names of the set bits in ascending value order joined by ", ", or the
// whole value in decimal when it is 0 or holds a bit .NET does not name (8,
// 65536 and the cloud-file bits from 262144 up).
func AttributesString(bits uint32) string {
	known := [...]attributeName{
		{0x1, "ReadOnly"}, {0x2, "Hidden"}, {0x4, "System"}, {0x10, "Directory"},
		{0x20, "Archive"}, {0x40, "Device"}, {0x80, "Normal"}, {0x100, "Temporary"},
		{0x200, "SparseFile"}, {0x400, "ReparsePoint"}, {0x800, "Compressed"},
		{0x1000, "Offline"}, {0x2000, "NotContentIndexed"}, {0x4000, "Encrypted"},
		{0x8000, "IntegrityStream"}, {0x20000, "NoScrubData"},
	}
	names := make([]string, 0, len(known))
	remaining := bits
	for _, attribute := range known {
		if bits&attribute.bit != 0 {
			names = append(names, attribute.name)
			remaining &^= attribute.bit
		}
	}
	if remaining != 0 || bits == 0 {
		return strconv.FormatUint(uint64(bits), 10)
	}
	return strings.Join(names, ", ")
}
