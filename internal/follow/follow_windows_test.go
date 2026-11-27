package follow

import (
	"errors"
	"net"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

// listenUnix makes an AF_UNIX socket at each of names, relative to dir, from
// inside dir so that the name Go hands the system stays under its 108-byte
// limit. It skips the test only where the system does not support AF_UNIX
// sockets, and fails it on any other error.
func listenUnix(t *testing.T, dir string, names ...string) {
	t.Helper()
	t.Chdir(syspath.ToNative(dir))
	for _, name := range names {
		listener, err := net.Listen("unix", syspath.ToNative(name))
		if errors.Is(err, windows.WSAEAFNOSUPPORT) || errors.Is(err, windows.WSAEPROTONOSUPPORT) {
			t.Skipf("this system does not support AF_UNIX sockets: %v", err)
		}
		if err != nil {
			t.Fatalf("creating an AF_UNIX socket at %s in %s: %v", name, dir, err)
		}
		t.Cleanup(func() { _ = listener.Close() })
	}
}

func TestASocketIsAReparsePointOtherThanALink(t *testing.T) {
	dir := syspath.FromNative(t.TempDir())
	path := dir + "/s"
	listenUnix(t, dir, "s")
	if meta, err := fsmeta.Lstat(path); err != nil || !meta.IsOtherReparsePoint() {
		t.Errorf("an AF_UNIX socket at %s does not read as another reparse point: %v", path, err)
	}
}
