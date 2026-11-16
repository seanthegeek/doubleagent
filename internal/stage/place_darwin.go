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

// occupantOf returns the staged path of the entry the copy open on file
// sits at, and true, when that is not dest: macOS gives the descriptor's
// path (F_GETPATH) spelled as the volume holds it (occupantAt). A path that
// cannot be read is no evidence of another entry's copy.
func (stager Stager) occupantOf(file *os.File, dest string) (string, bool) {
	connection, err := file.SyscallConn()
	if err != nil {
		return "", false
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
		return "", false
	}
	length := 0
	for length < len(buffer) && buffer[length] != 0 {
		length++
	}
	return stager.occupantAt(string(buffer[:length]), dest)
}
