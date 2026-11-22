// Package archive writes the run's archive: the staging directory as a gzipped
// tar, its SHA-256 and the sidecar file that records it.
package archive

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// Written is what WriteTarGz wrote: the SHA-256 of the bytes it wrote, in
// lower-case hex, their number, and the file as its open handle described
// it, which identifies the file whatever later takes its name.
type Written struct {
	SHA256 string
	Size   int64
	File   fs.FileInfo
}

// WriteTarGz writes the staging tree as a gzipped tar at archivePath: "./",
// then every directory, file and symlink read through the tree, "./"
// prefixed, each directory before its children and its entries in byte
// order, in PAX format, symlinks stored and never followed. ctx is looked
// at before each member and each read of a file's bytes, so a stop ends a
// long write promptly with ctx's error. archivePath is created new: anything
// at that name, a link included, fails the write and is neither followed
// nor changed (O_EXCL; on Windows CREATE_NEW with FILE_FLAG_OPEN_REPARSE_POINT,
// syscall/syscall_windows.go). The hash is of the bytes as written, never
// read back by name. A failed or stopped write removes the file it created.
func WriteTarGz(ctx context.Context, tree *stage.Tree, archivePath string) (Written, error) {
	file, err := osfile.OpenFile(archivePath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return Written{}, fmt.Errorf("create archive: %w", err)
	}
	digest := &countingHash{hash: sha256.New()}
	// A write or close error of the archive itself names it in the \\?\
	// form osfile opened it with.
	err = syspath.NativeErrors(writeMembers(ctx, io.MultiWriter(file, digest), tree))
	var info fs.FileInfo
	if err == nil {
		if info, err = file.Stat(); err != nil {
			err = fmt.Errorf("archive %s: %w", syspath.ToNative(archivePath), syspath.NativeErrors(err))
		}
	}
	if closeError := file.Close(); err == nil && closeError != nil {
		err = fmt.Errorf("close archive %s: %w", syspath.ToNative(archivePath), syspath.NativeErrors(closeError))
	}
	if err != nil {
		_ = osfile.Remove(archivePath)
		return Written{}, err
	}
	return Written{SHA256: hex.EncodeToString(digest.hash.Sum(nil)), Size: digest.size, File: info}, nil
}

// countingHash hashes the bytes written to it and counts them.
type countingHash struct {
	hash hash.Hash
	size int64
}

// Write adds data to the hash and the count.
func (digest *countingHash) Write(data []byte) (int, error) {
	digest.size += int64(len(data))
	return digest.hash.Write(data)
}

// StillNames returns nil when path names, without following a link, the
// regular file that file describes (os.SameFile of the handle's description
// against an lstat of path), and an error naming path otherwise: another
// entry took the name after the file was written.
func StillNames(path string, file fs.FileInfo) error {
	info, err := osfile.Lstat(path)
	switch {
	case err != nil:
		return err
	case !info.Mode().IsRegular() || !os.SameFile(info, file):
		return errors.New(syspath.ToNative(path) + ": no longer the file this run wrote")
	}
	return nil
}

// writeMembers writes the gzip and tar layers around the stage tree.
func writeMembers(ctx context.Context, out io.Writer, tree *stage.Tree) error {
	compressed := gzip.NewWriter(out)
	archive := tar.NewWriter(compressed)
	if err := addTree(ctx, archive, tree, ".", "."); err != nil {
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

// addTree adds the entry name of the directory dir, the archive's member,
// and for a directory everything under it, which it reads through the
// directory opened once (stage.Tree.OpenDir), so each name is one step from
// an open directory. name and member are "." for the stage itself. It
// returns ctx's error before the first member written after a stop.
func addTree(ctx context.Context, archive *tar.Writer, dir *stage.Tree, name, member string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	info, err := dir.Lstat(name)
	if err != nil {
		return err
	}
	if err := addEntry(ctx, archive, dir, name, member, info); err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	child := dir
	if name != "." {
		if child, err = dir.OpenDir(name); err != nil {
			return err
		}
		defer child.Close()
	}
	names, err := listNames(child)
	if err != nil {
		return err
	}
	for _, entry := range names {
		inner := entry
		if member != "." {
			inner = member + "/" + entry
		}
		if err := addTree(ctx, archive, child, entry, inner); err != nil {
			return err
		}
	}
	return nil
}

// listNames returns the names in the directory dir, sorted bytewise.
func listNames(dir *stage.Tree) ([]string, error) {
	entries, err := dir.ReadDir(".")
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

// addEntry writes the header of the entry name of dir, the archive's
// member, and for a regular file its bytes. Owner and group are kept as
// numbers only and times to the second, so the header holds what the
// staged file holds and no lookup of this host's names.
func addEntry(ctx context.Context, archive *tar.Writer, dir *stage.Tree, name, member string,
	info os.FileInfo,
) error {
	full := memberPath(dir, name)
	link := ""
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := dir.Readlink(name)
		if err != nil {
			return err
		}
		link = target
	}
	header, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return fmt.Errorf("archive %s: %w", syspath.ToNative(full), err)
	}
	header.Name = "./" + member
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
	return copyContent(ctx, archive, dir, name)
}

// copyContent writes the bytes of the regular file name of dir into the
// current archive member.
func copyContent(ctx context.Context, archive *tar.Writer, dir *stage.Tree, name string) error {
	full := memberPath(dir, name)
	source, err := dir.Open(name)
	if err != nil {
		return err
	}
	defer source.Close()
	if _, err := io.Copy(archive, contextReader{ctx: ctx, reader: source}); err != nil {
		return fmt.Errorf("archive %s: %w", syspath.ToNative(full), syspath.NativeErrors(err))
	}
	return nil
}

// memberPath returns the path of name in dir as the tree's name spells it,
// for messages.
func memberPath(dir *stage.Tree, name string) string {
	if name == "." {
		return dir.Path()
	}
	return dir.Path() + "/" + name
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

// WriteSidecar writes archivePath + ".sha256", one line, "<hex>  <name>", with
// the archive's base name, in the layout sha256sum -c reads. The file is
// created new, as the archive is: a name already taken, by a link or
// anything else, is an error, and what holds it is neither followed nor
// changed. created reports whether the file was made, so a caller removes
// only a file it made.
func WriteSidecar(archivePath, sha256Hex string) (created bool, err error) {
	path := archivePath + ".sha256"
	file, err := osfile.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o666)
	if err != nil {
		return false, fmt.Errorf("write checksum file: %w", err)
	}
	_, err = io.WriteString(file, sha256Hex+"  "+filepath.Base(archivePath)+"\n")
	if closeError := file.Close(); err == nil {
		err = closeError
	}
	if err != nil {
		return true, fmt.Errorf("write checksum file %s: %w", syspath.ToNative(path), syspath.NativeErrors(err))
	}
	return true, nil
}
