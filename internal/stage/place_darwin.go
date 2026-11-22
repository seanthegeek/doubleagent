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

// reportedPath returns the path macOS gives for the open file (F_GETPATH),
// spelled as the volume holds it, or "" when it cannot be read.
func reportedPath(file *os.File) string {
	connection, err := file.SyscallConn()
	if err != nil {
		return ""
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
		return ""
	}
	length := 0
	for length < len(buffer) && buffer[length] != 0 {
		length++
	}
	return string(buffer[:length])
}
