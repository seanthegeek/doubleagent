// Package regtext reads the text of a REG_SZ or REG_EXPAND_SZ registry value
// as .NET's RegistryKey.GetValue returns it: every UTF-16 code unit the value
// holds, embedded NULs (zero code units) included, less one terminating NUL.
// The Win32 string helpers stop at the first NUL instead, which would turn a
// value such as C:\a<NUL>junk into the different, acceptable value C:\a.
package regtext

import "unicode/utf16"

// Decode returns the text of raw REG_SZ data: the little-endian UTF-16 code
// units of data, an odd last byte read as a code unit whose high byte is
// zero (.NET rounds the size up the same way), with exactly one trailing NUL
// removed when the data ends in one. Embedded NULs stay. A lone surrogate
// becomes U+FFFD.
func Decode(data []byte) string {
	units := make([]uint16, (len(data)+1)/2)
	for i := range units {
		units[i] = uint16(data[2*i])
		if 2*i+1 < len(data) {
			units[i] |= uint16(data[2*i+1]) << 8
		}
	}
	if n := len(units); n > 0 && units[n-1] == 0 {
		units = units[:n-1]
	}
	return Name(units)
}

// Name returns the text of a counted UTF-16 name, as .NET builds a string
// from the characters and count RegEnumValue reports: every code unit is
// kept, a NUL anywhere included. A lone surrogate becomes U+FFFD, as in
// Decode.
func Name(units []uint16) string {
	return string(utf16.Decode(units))
}
