package regtext

import "unicode/utf16"

// decode returns the text of raw REG_SZ data: the little-endian UTF-16 code
// units of data, an odd last byte read as a code unit whose high byte is
// zero (.NET rounds the size up the same way), with exactly one trailing NUL
// removed when the data ends in one. Embedded NULs stay. A lone surrogate
// becomes U+FFFD.
func decode(data []byte) string {
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
	return string(utf16.Decode(units))
}
