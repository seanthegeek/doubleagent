// Package archive writes the run's archive: the staging directory as a gzipped
// tar, its SHA-256 and the sidecar file that records it.
package archive

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// WriteTarGz writes stageDir as a gzipped tar at archivePath. Its members are
// "./", then every directory, file and symlink under stageDir with a "./"
// prefix, each directory before its children and each directory's entries
// sorted bytewise. The tar format is PAX. Symlinks are stored, never followed.
// A failed write removes the partial archive.
func WriteTarGz(stageDir, archivePath string) error {
	file, err := os.Create(archivePath)
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	err = writeMembers(file, stageDir)
	if closeErr := file.Close(); err == nil && closeErr != nil {
		err = fmt.Errorf("close archive %s: %w", archivePath, closeErr)
	}
	if err != nil {
		_ = os.Remove(archivePath)
	}
	return err
}

// writeMembers writes the gzip and tar layers around the stage tree.
func writeMembers(out io.Writer, stageDir string) error {
	compressed := gzip.NewWriter(out)
	archive := tar.NewWriter(compressed)
	if err := addTree(archive, stageDir, "."); err != nil {
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
func addTree(archive *tar.Writer, stageDir, member string) error {
	full := filepath.Join(stageDir, member)
	info, err := os.Lstat(full)
	if err != nil {
		return err
	}
	if err := addEntry(archive, full, member, info); err != nil {
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
		if err := addTree(archive, stageDir, member+"/"+name); err != nil {
			return err
		}
	}
	return nil
}

// listNames returns the names in a directory, sorted bytewise.
func listNames(dir string) ([]string, error) {
	handle, err := os.Open(dir)
	if err != nil {
		return nil, err
	}
	defer handle.Close()
	entries, err := handle.ReadDir(-1)
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
func addEntry(archive *tar.Writer, full, member string, info os.FileInfo) error {
	link := ""
	if info.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(full)
		if err != nil {
			return err
		}
		link = target
	}
	header, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return fmt.Errorf("archive %s: %w", full, err)
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
		return fmt.Errorf("archive %s: %w", full, err)
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	return copyContent(archive, full)
}

// copyContent writes a regular file's bytes into the current member.
func copyContent(archive *tar.Writer, full string) error {
	source, err := os.Open(full)
	if err != nil {
		return err
	}
	defer source.Close()
	if _, err := io.Copy(archive, source); err != nil {
		return fmt.Errorf("archive %s: %w", full, err)
	}
	return nil
}

// SHA256File returns the SHA-256 of the file at path, in lower-case hex, and
// its size in bytes.
func SHA256File(path string) (string, int64, error) {
	source, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer source.Close()
	digest := sha256.New()
	size, err := io.Copy(digest, source)
	if err != nil {
		return "", 0, fmt.Errorf("hash %s: %w", path, err)
	}
	return hex.EncodeToString(digest.Sum(nil)), size, nil
}

// WriteSidecar writes archivePath + ".sha256", one line, "<hex>  <name>", with
// the archive's base name, in the layout sha256sum -c reads.
func WriteSidecar(archivePath, sha256Hex string) error {
	line := sha256Hex + "  " + filepath.Base(archivePath) + "\n"
	if err := os.WriteFile(archivePath+".sha256", []byte(line), 0o666); err != nil {
		return fmt.Errorf("write checksum file: %w", err)
	}
	return nil
}
