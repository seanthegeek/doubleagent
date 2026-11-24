package stage

import (
	"io/fs"
	"os"
	"syscall"
	"unsafe"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
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

// handleIdentity returns the volume serial number and file index of the
// open source, read from its handle, the identity fsmeta.Lstat gives.
func handleIdentity(file *os.File, _ fs.FileInfo) [2]uint64 {
	var data windows.ByHandleFileInformation
	err := onHandle(file, func(handle windows.Handle) error {
		return windows.GetFileInformationByHandle(handle, &data)
	})
	if err != nil {
		return [2]uint64{}
	}
	return [2]uint64{uint64(data.VolumeSerialNumber),
		uint64(data.FileIndexHigh)<<32 | uint64(data.FileIndexLow)}
}

// MarkCaseSensitive marks dir, a directory the run just made and nothing is
// in yet, case-sensitive (FileCaseSensitiveInfo), so two entries whose names
// differ only in case, as a case-sensitive image directory holds them, are
// staged as two; directories made in it later inherit the mark. It fails on
// a file system or Windows build without per-directory case sensitivity.
func MarkCaseSensitive(dir string) error {
	name, err := windows.UTF16PtrFromString(syspath.Extended(dir))
	if err != nil {
		return &fs.PathError{Op: "mark case-sensitive", Path: syspath.ToNative(dir), Err: err}
	}
	handle, err := windows.CreateFile(name, windows.FILE_WRITE_ATTRIBUTES, shareAll, nil,
		windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS|
			windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return &fs.PathError{Op: "mark case-sensitive", Path: syspath.ToNative(dir), Err: err}
	}
	defer windows.CloseHandle(handle)
	flags := uint32(windows.FILE_CS_FLAG_CASE_SENSITIVE_DIR)
	err = windows.SetFileInformationByHandle(handle, windows.FileCaseSensitiveInfo,
		(*byte)(unsafe.Pointer(&flags)), uint32(unsafe.Sizeof(flags)))
	if err != nil {
		return &fs.PathError{Op: "mark case-sensitive", Path: syspath.ToNative(dir), Err: err}
	}
	return nil
}

// createDest creates the staged copy at name in the tree exclusively
// (CREATE_NEW), never following a reparse point: when the volume takes the
// name for an existing entry's, by any rule it has, the create fails and
// nothing is opened for write. It returns the file, its identity and the
// path Windows gives for it, and holds the copy's access and write times so
// the times preserve sets are the ones the copy keeps.
func createDest(tree *Tree, name string) (createdFile, error) {
	file, err := tree.openFile(name,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL|syscall.FILE_FLAG_OPEN_REPARSE_POINT, 0o600)
	if err != nil {
		return createdFile{}, err
	}
	created := createdFile{file: file, staged: handleIdentity(file, nil)}
	_ = onHandle(file, func(handle windows.Handle) error {
		final, err := finalPath(handle)
		if err == nil {
			created.place = syspath.FromNative(final)
		}
		// Best effort: preserve still sets the times where the hold is refused.
		return fsmeta.HoldTimes(handle, true)
	})
	return created, nil
}

// preserve gives the copy at name in the tree the source's creation, access
// and write times. They are set on the open handle, because os.Chtimes
// opens the path again and would follow a link put there.
func preserve(target *os.File, tree *Tree, name string, info fs.FileInfo) error {
	times, ok := info.Sys().(*syscall.Win32FileAttributeData)
	if !ok {
		return nil
	}
	err := onHandle(target, func(handle windows.Handle) error {
		return syscall.SetFileTime(syscall.Handle(handle), &times.CreationTime,
			&times.LastAccessTime, &times.LastWriteTime)
	})
	if err != nil {
		return &fs.PathError{Op: "utimes", Path: syspath.ToNative(tree.full(name)), Err: err}
	}
	return nil
}

// PhysicalDir returns dir with every symlink and junction resolved, in the
// canonical form, or an error when dir is not a directory or cannot be
// opened. The handle is opened with no access right and follows every link
// on the way; Windows resolves .. before the links, as for every path.
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
