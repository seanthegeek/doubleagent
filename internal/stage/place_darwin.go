package stage

import (
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// pathMax is MAXPATHLEN from <sys/param.h>, the size of the buffer
// F_GETPATH fills; golang.org/x/sys/unix does not define it for macOS.
const pathMax = 1024

// placedFile reports whether the copy open on file sits at dest: the path
// macOS gives for the descriptor (F_GETPATH, spelled as the volume holds
// it) must match dest below the staging directory byte for byte (placedAt).
// A path that cannot be read is no evidence of another entry's copy.
func (stager Stager) placedFile(file *os.File, dest string) bool {
	connection, err := file.SyscallConn()
	if err != nil {
		return true
	}
	var buffer [pathMax]byte
	var callErr error
	err = connection.Control(func(descriptor uintptr) {
		// golang.org/x/sys/unix wraps F_GETPATH in no function, and its
		// SYS_FCNTL is deprecated; the standard library's Syscall goes
		// through libSystem on macOS.
		_, _, errno := syscall.Syscall(syscall.SYS_FCNTL, descriptor, unix.F_GETPATH,
			uintptr(unsafe.Pointer(&buffer[0])))
		if errno != 0 {
			callErr = errno
		}
	})
	if err != nil || callErr != nil {
		return true
	}
	length := 0
	for length < len(buffer) && buffer[length] != 0 {
		length++
	}
	return stager.placedAt(string(buffer[:length]), dest)
}
