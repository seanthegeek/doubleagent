package relocation

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"io"
	"unicode/utf16"
	"unicode/utf8"
)

// byteOrderMark is one byte-order mark (the bytes a Unicode text may start
// with to name its encoding) that decodeUnicodeText turns into UTF-8: its
// bytes, the width of one code unit and the byte order of a unit.
type byteOrderMark struct {
	mark  []byte
	width int
	order binary.ByteOrder
}

// byteOrderMarks lists the UTF-32 and UTF-16 marks in the order they are
// tested: a UTF-32 little-endian mark starts with the UTF-16 one, so it comes
// first.
func byteOrderMarks() []byteOrderMark {
	return []byteOrderMark{
		{[]byte{0xFF, 0xFE, 0x00, 0x00}, 4, binary.LittleEndian},
		{[]byte{0x00, 0x00, 0xFE, 0xFF}, 4, binary.BigEndian},
		{[]byte{0xFF, 0xFE}, 2, binary.LittleEndian},
		{[]byte{0xFE, 0xFF}, 2, binary.BigEndian},
	}
}

// markOf returns the UTF-16 or UTF-32 mark text starts with, if any.
func markOf(text []byte) (byteOrderMark, bool) {
	for _, candidate := range byteOrderMarks() {
		if bytes.HasPrefix(text, candidate.mark) {
			return candidate, true
		}
	}
	return byteOrderMark{}, false
}

// decodeUnicodeText returns the UTF-8 form of text when it starts with a
// UTF-16 or UTF-32 byte-order mark, the mark dropped; any other text, one
// with a UTF-8 mark included, comes back unchanged, to be searched as bytes.
// A unit that is no valid character, or a short unit at the end, becomes
// U+FFFD.
func decodeUnicodeText(text []byte) []byte {
	mark, ok := markOf(text)
	if !ok {
		return text
	}
	body := text[len(mark.mark):]
	decoded := make([]byte, 0, len(body))
	if mark.width == 2 {
		units := make([]uint16, len(body)/2)
		for i := range units {
			units[i] = mark.order.Uint16(body[2*i:])
		}
		for _, char := range utf16.Decode(units) {
			decoded = utf8.AppendRune(decoded, char)
		}
	} else {
		for i := 0; i+4 <= len(body); i += 4 {
			decoded = utf8.AppendRune(decoded, rune(mark.order.Uint32(body[i:])))
		}
	}
	if len(body)%mark.width != 0 {
		decoded = utf8.AppendRune(decoded, utf8.RuneError)
	}
	return decoded
}

// decodedBoundedNames returns boundedNames of r, its text decoded first when
// it starts with a UTF-16 or UTF-32 byte-order mark. Such a text is read whole
// (startup files are small and the sweep caps their size); any other is
// streamed.
func decodedBoundedNames(r io.Reader, names []string, powerShell bool) ([]lineName, error) {
	reader := bufio.NewReader(r)
	head, _ := reader.Peek(4)
	if _, ok := markOf(head); !ok {
		return boundedNames(reader, names, powerShell)
	}
	text, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return boundedNames(bytes.NewReader(decodeUnicodeText(text)), names, powerShell)
}
