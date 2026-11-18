package input

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
)

// unicodePath returns an Info-ZIP Unicode Path extra field of version and
// checksum holding name.
func unicodePath(version byte, checksum uint32, name string) []byte {
	data := append([]byte{version}, binary.LittleEndian.AppendUint32(nil, checksum)...)
	data = append(data, name...)
	field := binary.LittleEndian.AppendUint16(nil, zipUnicodePathField)
	field = binary.LittleEndian.AppendUint16(field, uint16(len(data)))
	return append(field, data...)
}

func TestUnicodePathFieldReadsOnlyAFieldMatchingTheName(t *testing.T) {
	checksum := crc32.ChecksumIEEE([]byte("stored"))
	other := binary.LittleEndian.AppendUint16(binary.LittleEndian.AppendUint16(nil, 0x5455), 1)
	other = append(other, 0)
	tests := []struct {
		name, want, problem string
		extra               []byte
	}{
		{"a matching field gives its name", "real/name.txt", "", unicodePath(1, checksum, "real/name.txt")},
		{"a field for another name is ignored", "", "", unicodePath(1, checksum+1, "real/name.txt")},
		{"a field of another version is ignored", "", "", unicodePath(2, checksum, "real/name.txt")},
		{"another field before it is skipped", "x", "", append(other, unicodePath(1, checksum, "x")...)},
		{"a field running past the data is damaged", "", "a member's extra field is damaged",
			[]byte{0x75, 0x70, 0x10, 0x00, 1}},
		{"a Unicode path field too short is damaged", "", "a member's Unicode path field is damaged",
			[]byte{0x75, 0x70, 0x02, 0x00, 1, 0}},
		{"a name that is not UTF-8 is refused", "", "a member's Unicode path is not valid UTF-8",
			unicodePath(1, checksum, "\xff")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := unicodePathField(test.extra, checksum)
			if got != test.want || (err == nil) != (test.problem == "") ||
				(err != nil && err.Error() != test.problem) {
				t.Errorf("got %q, %v; want %q, %q", got, err, test.want, test.problem)
			}
		})
	}
}

func TestADamagedArchiveIsRefusedWithItsReason(t *testing.T) {
	noise := make([]byte, 64<<10)
	for i := range noise {
		noise[i] = byte(i*7919 ^ i>>3)
	}
	archive := tarBytes(t, true, tarMember{name: "alice/.claude/x", typeflag: '0', body: string(noise)})
	plain := tarBytes(t, false, tarMember{name: "alice/.claude/x", typeflag: '0', body: "x"})
	var unended bytes.Buffer
	writer := gzip.NewWriter(&unended)
	if _, err := writer.Write(plain[:len(plain)-1024]); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	badChecksum := unended.Bytes()
	badChecksum[len(badChecksum)-8] ^= 0xff
	badHeader := tarBytes(t, false, tarMember{name: "alice/.claude/x", typeflag: '0', body: "x"},
		tarMember{name: "alice/.claude/y", typeflag: '0', body: "y"})
	badHeader[1024+148] ^= 0x01
	zipped := zipBytes(t, []string{"alice/.claude/x"}, map[string]string{"alice/.claude/x": "hello"})
	badZip := append([]byte{}, zipped...)
	badZip[strings.Index(string(badZip), "PK\x01\x02")+16] ^= 0xff
	tests := []struct {
		name, file, reason string
		data               []byte
	}{
		{"a gzip tar cut short", "c.tar.gz", "unexpected end of data", archive[:len(archive)*3/4]},
		{"a tar header with a wrong checksum", "c.tar", "invalid tar header", badHeader},
		{"a gzip tar without end blocks whose checksum does not match", "c.tar.gz", "gzip checksum mismatch",
			badChecksum},
		{"a zip member whose checksum does not match", "c.zip", "a member's checksum does not match", badZip},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := openArchiveOf(t, test.file, test.data)
			var inputError *Error
			if !errors.As(err, &inputError) || !strings.HasSuffix(err.Error(), ": cannot extract: "+test.reason) {
				t.Errorf("error %v, want the reason %q", err, test.reason)
			}
		})
	}
}

func TestParseSizeReadsAnIntegerOrRefuses(t *testing.T) {
	tests := []struct {
		name, json string
		want       int64
		ok         bool
	}{
		{"an integer", "12", 12, true},
		{"null is zero", "null", 0, true},
		{"true is one", "true", 1, true},
		{"a float is cut toward zero", "-2.9", -2, true},
		{"a float past the range is refused", "1e19", 0, false},
		{"a string of digits with spaces", `" 7 "`, 7, true},
		{"a string with a sign", `"-3"`, -3, true},
		{"other text is refused", `"1.5"`, 0, false},
		{"an integer past the range is refused", "9223372036854775808", 0, false},
		{"an array is refused", "[1]", 0, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value, err := jsonvalue.DecodeString(test.json)
			if err != nil {
				t.Fatal(err)
			}
			if got, ok := parseSize(value); ok != test.ok || ok && got != test.want {
				t.Errorf("parseSize(%s) = %d, %v; want %d, %v", test.json, got, ok, test.want, test.ok)
			}
		})
	}
}
