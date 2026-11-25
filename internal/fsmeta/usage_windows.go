package fsmeta

import (
	"os"

	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

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
