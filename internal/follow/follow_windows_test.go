package follow

import (
	"net"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/fsmeta"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

func TestASocketIsAReparsePointOtherThanALink(t *testing.T) {
	path := syspath.FromNative(t.TempDir()) + "/s"
	listener, err := net.Listen("unix", syspath.ToNative(path))
	if err != nil {
		t.Skipf("creating an AF_UNIX socket needs Windows 10 1803 or later: %v", err)
	}
	defer listener.Close()
	if meta, err := fsmeta.Lstat(path); err != nil || !meta.IsOtherReparsePoint() {
		t.Errorf("an AF_UNIX socket at %s does not read as another reparse point: %v", path, err)
	}
}
