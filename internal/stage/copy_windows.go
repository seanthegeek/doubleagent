package stage

import (
	"io/fs"
	"os"
	"syscall"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// shareAll lets other processes read, write, rename or delete a directory
// while PhysicalDir's handle on it is open.
const shareAll = windows.FILE_SHARE_READ | windows.FILE_SHARE_WRITE | windows.FILE_SHARE_DELETE

// volumeNameDOS is the GetFinalPathNameByHandle flag word asking for the
// drive-letter form of a normalised path (VOLUME_NAME_DOS and
// FILE_NAME_NORMALIZED are both 0; golang.org/x/sys defines neither).
const volumeNameDOS = 0

// onHandle runs call on the handle of file, giving the error either returns.
func onHandle(file *os.File, call func(windows.Handle) error) error {
	connection, err := file.SyscallConn()
	if err != nil {
		return err
	}
	var callError error
	err = connection.Control(func(handle uintptr) {
		callError = call(windows.Handle(handle))
	})
	if err != nil {
		return err
	}
	return callError
}

// createDest opens the staged copy for writing, creating it, without
// following a reparse point: one already at the path is refused, and the
// open takes a reparse point swapped in after that check as itself, which
// the regular-file test then refuses. The copy is emptied only after that
// test, since truncating at open would empty whatever a link led to. The
// copy's access and write times are then held, so the times preserve sets
// are the ones the copy keeps whatever the volume records.
func createDest(path string) (*os.File, error) {
	refused := &fs.PathError{Op: "open", Path: syspath.ToNative(path), Err: notRegular{}}
	if info, err := osfile.Lstat(path); err == nil && isReparsePoint(info) {
		return nil, refused
	}
	file, err := osfile.OpenFile(path,
		os.O_WRONLY|os.O_CREATE|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0o600)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err == nil && (!info.Mode().IsRegular() || isReparsePoint(info)) {
		err = refused
	}
	if err == nil {
		err = file.Truncate(0)
	}
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	// Best effort: preserve still sets the times where the hold is refused.
	_ = onHandle(file, func(handle windows.Handle) error { return fsmeta.HoldTimes(handle, true) })
	return file, nil
}

// isReparsePoint reports whether info describes a reparse point of any
// kind: a symlink, a junction, a socket or a placeholder.
func isReparsePoint(info fs.FileInfo) bool {
	attributes, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && attributes.FileAttributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0
}

// preserve gives the copy the source's creation, access and write times, as
// Copy-FileShared does. They are set on the open handle, because os.Chtimes
// opens the path again and would follow a link put there.
func preserve(target *os.File, dest string, info fs.FileInfo) error {
	times, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return nil
	}
	err := onHandle(target, func(handle windows.Handle) error {
		return syscall.SetFileTime(syscall.Handle(handle), &times.CreationTime,
			&times.LastAccessTime, &times.LastWriteTime)
	})
	if err != nil {
		return &fs.PathError{Op: "utimes", Path: syspath.ToNative(dest), Err: err}
	}
	return nil
}

// PhysicalDir returns dir with every symlink and junction resolved, in the
// canonical form, as Get-PhysicalPath gives it, or an error when dir is not
// a directory or cannot be opened. The handle is opened with no access
// right and follows every link on the way; Windows resolves .. before the
// links, as it does for every path.
func PhysicalDir(dir string) (string, error) {
	name, err := windows.UTF16PtrFromString(syspath.Extended(dir))
	if err != nil {
		return "", &fs.PathError{Op: "chdir", Path: syspath.ToNative(dir), Err: err}
	}
	handle, err := windows.CreateFile(name, 0, shareAll, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return "", &fs.PathError{Op: "chdir", Path: syspath.ToNative(dir), Err: err}
	}
	defer windows.CloseHandle(handle)
	var data windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &data); err != nil {
		return "", &fs.PathError{Op: "chdir", Path: syspath.ToNative(dir), Err: err}
	}
	if data.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY == 0 {
		return "", &fs.PathError{Op: "chdir", Path: syspath.ToNative(dir), Err: syscall.ENOTDIR}
	}
	final, err := finalPath(handle)
	if err != nil {
		return "", &fs.PathError{Op: "chdir", Path: syspath.ToNative(dir), Err: err}
	}
	return syspath.FromNative(final), nil
}

// finalPath returns the path Windows holds for an open handle, in the \\?\
// form, growing the buffer to the length the first call asks for.
func finalPath(handle windows.Handle) (string, error) {
	size := uint32(windows.MAX_PATH)
	for {
		buffer := make([]uint16, size)
		n, err := windows.GetFinalPathNameByHandle(handle, &buffer[0], size, volumeNameDOS)
		if err != nil {
			return "", err
		}
		if n < size {
			return windows.UTF16ToString(buffer[:n]), nil
		}
		size = n
	}
}
