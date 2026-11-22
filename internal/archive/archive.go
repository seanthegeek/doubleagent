// Package archive writes the run's archive: the staging directory as a gzipped
// tar, its SHA-256 and the sidecar file that records it.
package archive

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// WriteTarGz writes stageDir as a gzipped tar at archivePath. Its members are
// "./", then every directory, file and symlink under stageDir with a "./"
// prefix, each directory before its children and each directory's entries
// sorted bytewise. The tar format is PAX. Symlinks are stored, never followed.
// ctx is looked at before each member and each read of a file's bytes, so a
// stop request ends a long write promptly with ctx's error. A failed or
// stopped write removes the partial archive.
func WriteTarGz(ctx context.Context, stageDir, archivePath string) error {
	file, err := osfile.Create(archivePath)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	// A write or close error of the archive itself names it in the \\?\
	// form osfile opened it with.
	err = syspath.NativeErrors(writeMembers(ctx, file, stageDir))
	if closeError := file.Close(); err == nil && closeError != nil {
		err = fmt.Errorf("close archive %s: %w", syspath.ToNative(archivePath), syspath.NativeErrors(closeError))
	}
	if err != nil {
		_ = osfile.Remove(archivePath)
	}
	return err
}

// writeMembers writes the gzip and tar layers around the stage tree.
func writeMembers(ctx context.Context, out io.Writer, stageDir string) error {
	compressed := gzip.NewWriter(out)
	archive := tar.NewWriter(compressed)
	if err := addTree(ctx, archive, stageDir, "."); err != nil {
		return err
	}
	if err := archive.Close(); err != nil {
		return fmt.Errorf("finish tar: %w", err)
	}
	if err := compressed.Close(); err != nil {
		return fmt.Errorf("finish gzip: %w", err)
	}
	return nil
}

// addTree adds the entry at stageDir/member and, for a directory, everything
// under it. member is the name inside the archive, "." for the stage itself.
// It returns ctx's error before the first member written after a stop.
func addTree(ctx context.Context, archive *tar.Writer, stageDir, member string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	full := filepath.Join(stageDir, member)
	info, err := osfile.Lstat(full)
	if err != nil {
		return err
	}
	if err := addEntry(ctx, archive, full, member, info); err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	names, err := listNames(full)
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := addTree(ctx, archive, stageDir, member+"/"+name); err != nil {
			return err
		}
	}
	return nil
}

// listNames returns the names in a directory, sorted bytewise.
func listNames(dir string) ([]string, error) {
	entries, err := osfile.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	slices.Sort(names)
	return names, nil
}

// addEntry writes one member's header and, for a regular file, its bytes.
// Owner and group are kept as numbers only and times to the second, so the
// header holds what the staged file holds and no lookup of this host's names.
func addEntry(ctx context.Context, archive *tar.Writer, full, member string, info os.FileInfo) error {
	link := ""
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := osfile.Readlink(full)
		if err != nil {
			return err
		}
		link = target
	}
	header, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return fmt.Errorf("archive %s: %w", syspath.ToNative(full), err)
	}
	header.Name = "./" + strings.TrimPrefix(member, "./")
	if member == "." {
		header.Name = "./"
	} else if info.IsDir() {
		header.Name += "/"
	}
	header.Format = tar.FormatPAX
	header.Uname, header.Gname = "", ""
	header.ModTime = header.ModTime.Truncate(time.Second)
	header.AccessTime, header.ChangeTime = time.Time{}, time.Time{}
	if err := archive.WriteHeader(header); err != nil {
		return fmt.Errorf("archive %s: %w", syspath.ToNative(full), err)
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	return copyContent(ctx, archive, full)
}

// copyContent writes a regular file's bytes into the current member.
func copyContent(ctx context.Context, archive *tar.Writer, full string) error {
	source, err := osfile.Open(full)
	if err != nil {
		return err
	}
	defer source.Close()
	if _, err := io.Copy(archive, contextReader{ctx: ctx, reader: source}); err != nil {
		return fmt.Errorf("archive %s: %w", syspath.ToNative(full), syspath.NativeErrors(err))
	}
	return nil
}

// contextReader reads from reader until ctx is cancelled, then returns ctx's
// error, so a copy of a large file stops at its next read after a stop.
type contextReader struct {
	ctx    context.Context
	reader io.Reader
}

// Read reads from the reader unless ctx is cancelled.
func (r contextReader) Read(data []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.reader.Read(data)
}

// SHA256File returns the SHA-256 of the file at path, in lower-case hex, and
// its size in bytes. A stop request (ctx) ends the read with ctx's error.
func SHA256File(ctx context.Context, path string) (string, int64, error) {
	source, err := osfile.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer source.Close()
	digest := sha256.New()
	size, err := io.Copy(digest, contextReader{ctx: ctx, reader: source})
	if err != nil {
		return "", 0, fmt.Errorf("hash %s: %w", syspath.ToNative(path), syspath.NativeErrors(err))
	}
	return hex.EncodeToString(digest.Sum(nil)), size, nil
}

// WriteSidecar writes archivePath + ".sha256", one line, "<hex>  <name>", with
// the archive's base name, in the layout sha256sum -c reads.
func WriteSidecar(archivePath, sha256Hex string) error {
	line := sha256Hex + "  " + filepath.Base(archivePath) + "\n"
	if err := osfile.WriteFile(archivePath+".sha256", []byte(line), 0o666); err != nil {
		return fmt.Errorf("write checksum file: %w", err)
	}
	return nil
}
