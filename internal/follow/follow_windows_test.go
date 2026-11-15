package follow

import (
	"net"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

func TestNotALinkTellsASocketFromALink(t *testing.T) {
	path := syspath.FromNative(t.TempDir()) + "/s"
	listener, err := net.Listen("unix", syspath.ToNative(path))
	if err != nil {
		t.Skipf("creating an AF_UNIX socket needs Windows 10 1803 or later: %v", err)
	}
	defer listener.Close()
	if !NotALink(path) {
		t.Errorf("NotALink(%s) = false for an AF_UNIX socket, a reparse point that is no link", path)
	}
}
