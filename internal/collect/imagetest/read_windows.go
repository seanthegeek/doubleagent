package imagetest

import (
	"fmt"
	"io/fs"
	"strings"
	"syscall"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// kindOf returns the kind of the entry at path, whose Lstat is info. A
// reparse point is told apart by its tag: a symbolic link with the directory
// attribute is a directory link, a mount point a junction, and an AF_UNIX one
// a socket, which the os package already reports.
func kindOf(path string, info fs.FileInfo) (Kind, error) {
	attributes, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok || attributes.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT == 0 ||
		info.Mode()&fs.ModeSocket != 0 {
		return kindOfMode(path, info.Mode())
	}
	reparse, err := fsmeta.ReadReparse(path)
	if err != nil {
		return "", fmt.Errorf("%s: %w", path, err)
	}
	switch {
	case reparse.Tag == fsmeta.TagMountPoint:
		return KindJunction, nil
	case reparse.Tag == fsmeta.TagSymlink && attributes.FileAttributes&syscall.FILE_ATTRIBUTE_DIRECTORY != 0:
		return KindDirLink, nil
	case reparse.Tag == fsmeta.TagSymlink:
		return KindLink, nil
	}
	return "", fmt.Errorf("%s: a reparse point with tag %#x has no manifest kind", path, reparse.Tag)
}

// modeOf returns the default mode of kind: Windows has no permission bits
// for the builder to set, so a manifest records none.
func modeOf(kind Kind, _ fs.FileInfo) fs.FileMode {
	return defaultMode(kind)
}

// linkTarget returns the target of the link or junction at path, as the os
// package reads it, without the separator a junction's target ends in.
func linkTarget(path string, kind Kind) (string, error) {
	target, err := osfile.Readlink(path)
	if kind == KindJunction {
		target = strings.TrimSuffix(target, `\`)
	}
	return target, err
}
