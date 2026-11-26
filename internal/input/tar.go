package input

import (
	"archive/tar"
	"bytes"
	"compress/bzip2"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"strings"
	"time"
)

// tarBlockSize is the size of one tar header block.
const tarBlockSize = 512

// countingReader counts the bytes read through it.
type countingReader struct {
	reader io.Reader
	count  int64
}

// Read reads from the underlying reader and counts what it returned.
func (r *countingReader) Read(data []byte) (int, error) {
	n, err := r.reader.Read(data)
	r.count += int64(n)
	return n, err
}

// tarDirectory is a directory member whose time is set once every member
// is written, as a later member would change it; setting a directory's
// time changes no other directory's, so the order does not matter.
type tarDirectory struct {
	parts    []string
	modified time.Time
}

// extractTar extracts a tar archive, plain or gzip or bzip2 compressed.
// Link, hard-link, device and FIFO members are skipped (the manifest holds
// a link's target), as is, with a problem, a member whose name cannot be
// used (memberParts); a sparse member or one of an unknown type refuses
// the archive.
func extractTar(ctx context.Context, file *os.File, path string, out *extraction) error {
	reader, header, err := openTarStream(file, path)
	if err != nil || header == nil {
		return err
	}
	var directories []tarDirectory
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		directory, err := extractTarMember(reader, header, path, out)
		if err != nil {
			return err
		}
		if directory != nil {
			directories = append(directories, *directory)
		}
		header, err = reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return cannotExtract(path, tarReason(err))
		}
	}
	for _, directory := range directories {
		_ = out.setTime(directory.parts, directory.modified)
	}
	return nil
}

// openTarStream returns a reader positioned after the archive's first
// header, trying gzip, then bzip2, then no compression; the header is nil
// for an archive with no member. A file none of them reads is not an
// archive, unless it is xz or zstd compressed, which the extractor refuses.
func openTarStream(file *os.File, path string) (*tar.Reader, *tar.Header, error) {
	magic := make([]byte, 6)
	n, _ := file.ReadAt(magic, 0)
	magic = magic[:n]
	for _, method := range []string{"gzip", "bzip2", "none"} {
		if _, err := file.Seek(0, io.SeekStart); err != nil {
			return nil, nil, &fs.PathError{Op: "seek", Path: path, Err: err}
		}
		stream := decompressed(file, magic, method)
		if stream == nil {
			continue
		}
		counter := &countingReader{reader: stream}
		reader := tar.NewReader(counter)
		header, err := reader.Next()
		if err == nil {
			return reader, header, nil
		}
		if err == io.EOF && counter.count >= tarBlockSize {
			return reader, nil, nil
		}
	}
	switch {
	case bytes.HasPrefix(magic, []byte{0xfd, '7', 'z', 'X', 'Z', 0}):
		return nil, nil, cannotExtract(path, "xz-compressed tar archives are not supported")
	case bytes.HasPrefix(magic, []byte{0x28, 0xb5, 0x2f, 0xfd}):
		return nil, nil, cannotExtract(path, "zstd-compressed tar archives are not supported")
	}
	return nil, nil, notAnArchive(path)
}

// decompressed returns file read through method, or nil when its magic
// bytes are not that method's.
func decompressed(file *os.File, magic []byte, method string) io.Reader {
	switch method {
	case "gzip":
		if !bytes.HasPrefix(magic, []byte{0x1f, 0x8b}) {
			return nil
		}
		stream, err := gzip.NewReader(file)
		if err != nil {
			return nil
		}
		return stream
	case "bzip2":
		if !bytes.HasPrefix(magic, []byte("BZh")) {
			return nil
		}
		return bzip2.NewReader(file)
	}
	return file
}

// extractTarMember writes one member, returning the directory whose time
// is set at the end when the member is one.
func extractTarMember(reader *tar.Reader, header *tar.Header, path string,
	out *extraction) (*tarDirectory, error) {
	switch header.Typeflag {
	case tar.TypeXGlobalHeader, tar.TypeSymlink, tar.TypeLink, tar.TypeChar, tar.TypeBlock,
		tar.TypeFifo:
		return nil, nil
	case tar.TypeDir, tar.TypeReg, tar.TypeCont, tar.TypeGNUSparse:
	default:
		return nil, cannotExtract(path, "a member is of a type that cannot be extracted")
	}
	if isSparse(header) {
		return nil, cannotExtract(path, "sparse members are not supported")
	}
	parts, reason := memberParts(header.Name)
	if reason == "" && len(parts) == 0 && header.Typeflag != tar.TypeDir {
		reason = reasonNoFile
	}
	if reason != "" {
		out.skip(header.Name, reason)
		return nil, nil
	}
	if header.Typeflag == tar.TypeDir {
		if err := out.makeDirectories(out.placeOf(parts), len(parts)); err != nil {
			if out.skipped(header.Name, err) {
				return nil, nil
			}
			return nil, err
		}
		return &tarDirectory{parts: parts, modified: header.ModTime}, nil
	}
	err := out.writeFile(parts, slashOnlyName(header.Name), tarFileMode(header.Mode), reader,
		header.ModTime)
	if err != nil && out.skipped(header.Name, err) {
		return nil, nil
	}
	if err != nil && !isPathError(err) {
		return nil, cannotExtract(path, tarReason(err))
	}
	return nil, err
}

// isSparse reports whether header describes a sparse file, in the old GNU
// form or the PAX one. archive/tar keeps the old form's type letter, 'S',
// in the header (handleSparseFile in archive/tar/reader.go), so the switch
// in extractTarMember lets it through to this check; the PAX form is a
// regular member whose PAXRecords hold GNU.sparse. keys.
func isSparse(header *tar.Header) bool {
	if header.Typeflag == tar.TypeGNUSparse {
		return true
	}
	for key := range header.PAXRecords {
		if strings.HasPrefix(key, "GNU.sparse.") {
			return true
		}
	}
	return false
}

// tarFileMode returns the permissions a regular member is created with:
// no bits above 0o755, execute bits only when the owner's is set, and the
// owner's read and write always.
func tarFileMode(mode int64) fs.FileMode {
	permissions := fs.FileMode(mode) & 0o755
	if permissions&0o100 == 0 {
		permissions &^= 0o111
	}
	return permissions | 0o600
}

// tarReason returns the reason an archive read failed, in plain words.
func tarReason(err error) string {
	switch {
	case errors.Is(err, io.ErrUnexpectedEOF):
		return "unexpected end of data"
	case errors.Is(err, tar.ErrHeader):
		return "invalid tar header"
	case errors.Is(err, gzip.ErrChecksum):
		return "gzip checksum mismatch"
	}
	return strings.ToLower(err.Error())
}

// isPathError reports whether err is an operating-system error naming a
// path, which is passed through as it is.
func isPathError(err error) bool {
	var pathError *fs.PathError
	return errors.As(err, &pathError)
}
