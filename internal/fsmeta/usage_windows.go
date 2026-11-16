package fsmeta

import (
	"context"
	"os"

	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// size returns the size of dir as the PowerShell collector's Get-DirSize
// reports it: the sum of the sizes of the files under dir, each path counted
// once (a file with several hard links is counted under each). Only
// directories are descended; a symlink, junction or other reparse point is
// neither counted nor followed, dir included. An unreadable directory
// contributes what could be read; dir that cannot be examined gives 0.
func (walk usageWalk) size(ctx context.Context, dir string) (int64, error) {
	meta, err := Lstat(dir)
	if err != nil {
		return 0, nil
	}
	return walk.sumFileSizes(ctx, dir, meta)
}

// sumFileSizes returns the size of path when meta says it is a file, the
// sizes of the files below it when it is a directory, else 0, or the
// context's error when it is cancelled before a directory is read.
func (walk usageWalk) sumFileSizes(ctx context.Context, path string, meta Meta) (int64, error) {
	switch meta.Kind {
	case File:
		return meta.Size, nil
	case Dir:
	default:
		return 0, nil
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	walk.aboutToOpen(path)
	handle, err := openDirectoryItself(walk.opener, path)
	if err != nil {
		return 0, nil
	}
	defer handle.Close()
	entries, _ := handle.ReadDir(-1)
	var total int64
	for _, entry := range entries {
		child := path + "/" + entry.Name()
		childMeta, err := Lstat(child)
		if err != nil {
			continue
		}
		size, err := walk.sumFileSizes(ctx, child, childMeta)
		if err != nil {
			return 0, err
		}
		total += size
	}
	return total, nil
}

// openDirectoryItself opens path to list it, never following a reparse point
// at its last component, has opener's Admit judge the handle before anything
// else is done with it, and keeps the handle only when the handle itself is
// a directory and no reparse point. So a directory swapped for a junction or
// a symlink after its lstat is not listed, and on an image run a parent
// swapped for one that leads out of the root is refused by Admit.
func openDirectoryItself(opener *Opener, path string) (*os.File, error) {
	native := syspath.ToNative(path)
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	if err != nil {
		return nil, pathError("open", native, err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, shareAll, nil,
		windows.OPEN_EXISTING, noFollowFlags, 0)
	if err != nil {
		return nil, pathError("open", native, err)
	}
	if err := opener.admit(path, uintptr(handle)); err != nil {
		windows.CloseHandle(handle)
		return nil, err
	}
	var data windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &data); err != nil {
		windows.CloseHandle(handle)
		return nil, pathError("open", native, err)
	}
	if data.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 ||
		data.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		windows.CloseHandle(handle)
		return nil, pathError("open", native, errNotDirectory)
	}
	return os.NewFile(uintptr(handle), native), nil
}
