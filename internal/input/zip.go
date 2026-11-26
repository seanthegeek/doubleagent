package input

import (
	"archive/zip"
	"context"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/seanthegeek/doubleagent/internal/textlines"
)

// Flags, fields and methods of zip members, from the format's specification
// (APPNOTE.TXT).
const (
	// zipUTF8Flag is general purpose bit 11: the name is UTF-8.
	zipUTF8Flag = 1 << 11
	// zipEncryptedFlag is general purpose bit 0.
	zipEncryptedFlag = 1
	// zipUnicodePathField is the Info-ZIP Unicode Path extra field.
	zipUnicodePathField = 0x7075
	// zipBzip2Method is compression method 12.
	zipBzip2Method = 12
)

// extractZip extracts the members of a zip archive: first those whose
// names hold no backslash, then those whose names hold one, each group in
// central-directory order. A backslash in a name separates folders on every
// host, whatever system the member records: a zip name separates folders
// with slashes (APPNOTE.TXT 4.4.17.1), some archivers write a backslash for
// one, and archiveParts reads a row's archive path the same way, so a row
// names the file extraction made. Where two members reach one place, or one
// the file system holds as one entry, the earlier holds it, so a member
// whose name is the path with slashes keeps it over one that reaches it
// only through a backslash, whatever order the archiver listed them in.
func extractZip(ctx context.Context, members []*zip.File, path string, out *extraction) error {
	passedOver, err := extractZipMembers(ctx, members, path, out, false)
	if err != nil || !passedOver {
		return err
	}
	_, err = extractZipMembers(ctx, members, path, out, true)
	return err
}

// extractZipMembers extracts, in order, the members whose names hold a
// backslash when backslashed is set, else the others, and reports whether
// it passed over a member of the other group. A name ending in a slash or
// a backslash is a directory. A member whose name cannot be used
// (memberParts) is skipped with a problem; an encrypted member, a name
// not valid UTF-8 where the member says it is, or a damaged member refuses
// the archive. Every member's name is read in each call, so the first call
// refuses an archive holding a name that cannot be read.
func extractZipMembers(ctx context.Context, members []*zip.File, path string, out *extraction,
	backslashed bool) (bool, error) {
	passedOver := false
	for _, member := range members {
		if err := ctx.Err(); err != nil {
			return false, err
		}
		name, err := zipMemberName(&member.FileHeader)
		if err != nil {
			return false, cannotExtract(path, err.Error())
		}
		if strings.Contains(name, `\`) != backslashed {
			passedOver = true
			continue
		}
		slashed := strings.ReplaceAll(name, `\`, "/")
		directory := strings.HasSuffix(slashed, "/")
		parts, reason := memberParts(slashed)
		if reason == "" && len(parts) == 0 && !directory {
			reason = reasonNoFile
		}
		if reason != "" {
			out.skip(name, reason)
			continue
		}
		if directory {
			err = out.makeDirectories(out.placeOf(parts), len(parts))
		} else {
			err = extractZipMember(member, parts, name, path, out)
		}
		if err != nil && !out.skipped(name, err) {
			return false, err
		}
	}
	return passedOver, nil
}

// extractZipMember writes one file member, called name, with the
// modification time its header holds.
func extractZipMember(member *zip.File, parts []string, name, path string, out *extraction) error {
	if member.Flags&zipEncryptedFlag != 0 {
		return cannotExtract(path, "encrypted members are not supported")
	}
	contents, err := member.Open()
	if err != nil {
		return cannotExtract(path, zipReason(err))
	}
	defer contents.Close()
	err = out.writeFile(parts, name, 0o666, contents, member.Modified)
	var collision *collisionError
	if err != nil && !isPathError(err) && !errors.As(err, &collision) {
		return cannotExtract(path, zipReason(err))
	}
	return err
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
