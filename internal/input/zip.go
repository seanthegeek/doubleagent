package input

import (
	"archive/zip"
	"bytes"
	"compress/bzip2"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// Signatures and sizes of the zip records, from the format's specification
// (APPNOTE.TXT, sections 4.3.12 to 4.3.16).
const (
	zipEndSignature       = "PK\x05\x06"
	zipEnd64Signature     = "PK\x06\x06"
	zipLocator64Signature = "PK\x06\x07"
	zipCentralSignature   = "PK\x01\x02"
	zipEndSize            = 22
	zipEnd64Size          = 56
	zipLocator64Size      = 20
	zipCentralSize        = 46
	zipMaxComment         = 65535
	// zipUTF8Flag is general purpose bit 11: the name is UTF-8.
	zipUTF8Flag = 1 << 11
	// zipEncryptedFlag is general purpose bit 0.
	zipEncryptedFlag = 1
	// zipUnicodePathField is the Info-ZIP Unicode Path extra field.
	zipUnicodePathField = 0x7075
	// zipBzip2Method is compression method 12.
	zipBzip2Method = 12
)

// zipEnd is the end-of-central-directory record, completed from the zip64
// record when there is one.
type zipEnd struct {
	diskNumber, diskStart, entries uint64
	size, offset, location         uint64
}

// isZipFile reports whether file is a zip: an end of central directory
// record is found at the end, or within the last 64 KiB as a comment
// allows, and either the archive is empty or the central directory it
// points to starts with its signature.
func isZipFile(file io.ReaderAt, size int64) bool {
	end, ok := findZipEnd(file, size)
	if !ok {
		return false
	}
	if end.entries == 0 && end.size == 0 && end.offset == 0 {
		return true
	}
	if end.diskNumber != end.diskStart || end.size < zipCentralSize || end.location < end.size {
		return false
	}
	header := make([]byte, zipCentralSize)
	_, err := file.ReadAt(header, int64(end.location-end.size))
	return err == nil && string(header[:4]) == zipCentralSignature
}

// findZipEnd finds and decodes the end record: first as the file's last 22
// bytes with no comment, then as the last signature in the final 64 KiB.
func findZipEnd(file io.ReaderAt, size int64) (zipEnd, bool) {
	if size < zipEndSize {
		return zipEnd{}, false
	}
	record := make([]byte, zipEndSize)
	if _, err := file.ReadAt(record, size-zipEndSize); err != nil {
		return zipEnd{}, false
	}
	if string(record[:4]) == zipEndSignature && record[20] == 0 && record[21] == 0 {
		return withZip64(file, decodeZipEnd(record, uint64(size-zipEndSize)))
	}
	start := max(size-zipMaxComment-zipEndSize, 0)
	tail := make([]byte, size-start)
	if _, err := file.ReadAt(tail, start); err != nil {
		return zipEnd{}, false
	}
	found := bytes.LastIndex(tail, []byte(zipEndSignature))
	if found < 0 || len(tail)-found < zipEndSize {
		return zipEnd{}, false
	}
	return withZip64(file, decodeZipEnd(tail[found:found+zipEndSize], uint64(start)+uint64(found)))
}

// decodeZipEnd decodes the fields of an end record found at location.
func decodeZipEnd(record []byte, location uint64) zipEnd {
	return zipEnd{
		diskNumber: uint64(binary.LittleEndian.Uint16(record[4:])),
		diskStart:  uint64(binary.LittleEndian.Uint16(record[6:])),
		entries:    uint64(binary.LittleEndian.Uint16(record[10:])),
		size:       uint64(binary.LittleEndian.Uint32(record[12:])),
		offset:     uint64(binary.LittleEndian.Uint32(record[16:])),
		location:   location,
	}
}

// withZip64 completes end from the zip64 end record its locator points to,
// when a locator precedes it. A locator naming several disks, or a zip64
// record that is missing or inconsistent, means no zip.
func withZip64(file io.ReaderAt, end zipEnd) (zipEnd, bool) {
	if end.location < zipLocator64Size {
		return end, true
	}
	offset := end.location - zipLocator64Size
	locator := make([]byte, zipLocator64Size)
	if _, err := file.ReadAt(locator, int64(offset)); err != nil {
		return zipEnd{}, false
	}
	if string(locator[:4]) != zipLocator64Signature {
		return end, true
	}
	relative := binary.LittleEndian.Uint64(locator[8:])
	if binary.LittleEndian.Uint32(locator[4:]) != 0 || binary.LittleEndian.Uint32(locator[16:]) > 1 ||
		offset < zipEnd64Size || relative > offset-zipEnd64Size {
		return zipEnd{}, false
	}
	offset -= zipEnd64Size
	record, extra, ok := readZip64Record(file, relative, offset)
	if !ok {
		return zipEnd{}, false
	}
	directorySize := binary.LittleEndian.Uint64(record[40:])
	directoryOffset := binary.LittleEndian.Uint64(record[48:])
	if directoryOffset+directorySize != relative || binary.LittleEndian.Uint64(record[4:])+12 != zipEnd64Size+extra {
		return zipEnd{}, false
	}
	return zipEnd{
		diskNumber: uint64(binary.LittleEndian.Uint32(record[16:])),
		diskStart:  uint64(binary.LittleEndian.Uint32(record[20:])),
		entries:    binary.LittleEndian.Uint64(record[32:]),
		size:       directorySize, offset: directoryOffset, location: offset - extra,
	}, true
}

// readZip64Record reads the zip64 end record at relative, or, when data was
// prepended to the archive, at offset, and returns how far the two differ.
func readZip64Record(file io.ReaderAt, relative, offset uint64) ([]byte, uint64, bool) {
	record := make([]byte, zipEnd64Size)
	if _, err := file.ReadAt(record, int64(relative)); err != nil {
		return nil, 0, false
	}
	extra := offset - relative
	if string(record[:4]) != zipEnd64Signature && relative != offset {
		if _, err := file.ReadAt(record, int64(offset)); err != nil {
			return nil, 0, false
		}
		extra = 0
	}
	return record, extra, string(record[:4]) == zipEnd64Signature
}

// extractZip extracts a zip archive in central-directory order. Names use
// slashes for backslashes; a name that is absolute, holds a ".." component
// or starts with a drive letter (or that this platform refuses) is
// skipped, as is nothing else. A name ending in a slash is a directory.
func extractZip(file *os.File, size int64, path string, out *extraction) error {
	archive, err := zip.NewReader(file, size)
	if err != nil {
		return cannotExtract(path, zipReason(err))
	}
	archive.RegisterDecompressor(zipBzip2Method, func(r io.Reader) io.ReadCloser {
		return io.NopCloser(bzip2.NewReader(r))
	})
	for _, member := range archive.File {
		name, err := zipMemberName(&member.FileHeader)
		if err != nil {
			return cannotExtract(path, err.Error())
		}
		name = strings.ReplaceAll(name, `\`, "/")
		if !zipNameAllowed(name) {
			continue
		}
		parts := components(name, "/")
		if strings.HasSuffix(name, "/") {
			if err := out.makeDirectories(parts); err != nil {
				return err
			}
			continue
		}
		if err := extractZipMember(member, parts, path, out); err != nil {
			return err
		}
	}
	return nil
}

// extractZipMember writes one file member.
func extractZipMember(member *zip.File, parts []string, path string, out *extraction) error {
	if member.Flags&zipEncryptedFlag != 0 {
		return cannotExtract(path, "encrypted members are not supported")
	}
	contents, err := member.Open()
	if err != nil {
		return cannotExtract(path, zipReason(err))
	}
	defer contents.Close()
	err = out.writeFile(parts, 0o666, contents, time.Time{})
	if err != nil && !isPathError(err) {
		return cannotExtract(path, zipReason(err))
	}
	return err
}

// zipNameAllowed reports whether a member name, slashes only, stays inside
// the extraction directory: not absolute, no ".." component, no leading
// drive letter, and allowed on this platform.
func zipNameAllowed(name string) bool {
	return !strings.HasPrefix(name, "/") && !containsPart(splitSlash(name), "..") &&
		windowsDrive(name) == "" && memberNameAllowed(name)
}

// zipMemberName returns a member's name: UTF-8 when the flag says so (an
// invalid name is then an error), else its bytes with each one that is not
// valid UTF-8 replaced by U+FFFD; cut at a NUL; and replaced by an
// Info-ZIP Unicode Path field whose checksum matches the stored name.
func zipMemberName(header *zip.FileHeader) (string, error) {
	name := header.Name
	if header.Flags&zipUTF8Flag != 0 {
		if !utf8.ValidString(name) {
			return "", errors.New("a member name is not valid UTF-8")
		}
	} else {
		name = textlines.ValidUTF8([]byte(name))
	}
	name = cutAtNul(name)
	unicodeName, err := unicodePathField(header.Extra, crc32.ChecksumIEEE([]byte(header.Name)))
	if err != nil {
		return "", err
	}
	if unicodeName != "" {
		name = cutAtNul(unicodeName)
	}
	return name, nil
}

// cutAtNul returns text up to its first NUL.
func cutAtNul(text string) string {
	before, _, _ := strings.Cut(text, "\x00")
	return before
}

// unicodePathField returns the name an Info-ZIP Unicode Path field (version
// 1, checksum equal to nameChecksum) in extra gives, or "". An extra field
// whose length runs past the data, or a Unicode Path field too short or
// not UTF-8, is an error.
func unicodePathField(extra []byte, nameChecksum uint32) (string, error) {
	name := ""
	for len(extra) >= 4 {
		kind := binary.LittleEndian.Uint16(extra)
		length := int(binary.LittleEndian.Uint16(extra[2:]))
		if length+4 > len(extra) {
			return "", errors.New("a member's extra field is damaged")
		}
		data := extra[4 : 4+length]
		if kind == zipUnicodePathField {
			if len(data) < 5 {
				return "", errors.New("a member's Unicode path field is damaged")
			}
			if data[0] == 1 && binary.LittleEndian.Uint32(data[1:]) == nameChecksum {
				if !utf8.Valid(data[5:]) {
					return "", errors.New("a member's Unicode path is not valid UTF-8")
				}
				if len(data) > 5 {
					name = string(data[5:])
				}
			}
		}
		extra = extra[4+length:]
	}
	return name, nil
}

// zipReason returns the reason a zip read failed, in plain words.
func zipReason(err error) string {
	switch {
	case errors.Is(err, zip.ErrFormat):
		return "not a valid zip archive"
	case errors.Is(err, zip.ErrAlgorithm):
		return "a member's compression method is not supported"
	case errors.Is(err, zip.ErrChecksum):
		return "a member's checksum does not match"
	case errors.Is(err, io.ErrUnexpectedEOF):
		return "unexpected end of data"
	}
	return strings.ToLower(err.Error())
}
