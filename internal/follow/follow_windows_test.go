package follow

import (
	"net"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

func TestOtherReparsePointTellsASocketFromALink(t *testing.T) {
	path := syspath.FromNative(t.TempDir()) + "/s"
	listener, err := net.Listen("unix", syspath.ToNative(path))
	if err != nil {
		t.Skipf("creating an AF_UNIX socket needs Windows 10 1803 or later: %v", err)
	}
	defer listener.Close()
	if !OtherReparsePoint(path) {
		t.Errorf("OtherReparsePoint(%s) = false for an AF_UNIX socket", path)
	}
}
