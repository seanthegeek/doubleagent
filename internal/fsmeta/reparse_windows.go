package fsmeta

import (
	"io/fs"

	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// ReadReparse returns the reparse data of path itself, a reparse point it
// opens without following (FILE_FLAG_OPEN_REPARSE_POINT), through
// FSCTL_GET_REPARSE_POINT: the stored substitute name, never the text
// os.Readlink rewrites it into.
func ReadReparse(path string) (Reparse, error) {
	name, err := windows.UTF16PtrFromString(syspath.Extended(path))
	if err != nil {
		return Reparse{}, &fs.PathError{Op: "readreparse", Path: syspath.ToNative(path), Err: err}
	}
	handle, err := windows.CreateFile(name, 0, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|
		windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, noFollowFlags, 0)
	if err != nil {
		return Reparse{}, &fs.PathError{Op: "readreparse", Path: syspath.ToNative(path), Err: err}
	}
	defer windows.CloseHandle(handle)
	buffer := make([]byte, windows.MAXIMUM_REPARSE_DATA_BUFFER_SIZE)
	var returned uint32
	err = windows.DeviceIoControl(handle, windows.FSCTL_GET_REPARSE_POINT, nil, 0, &buffer[0],
		uint32(len(buffer)), &returned, nil)
	if err != nil {
		return Reparse{}, &fs.PathError{Op: "readreparse", Path: syspath.ToNative(path), Err: err}
	}
	return ParseReparse(buffer[:returned])
}
