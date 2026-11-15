package fsmeta

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
	"golang.org/x/sys/windows"
)

func TestOpenRegularErrorNamesTheNativePath(t *testing.T) {
	canonical := syspath.FromNative(t.TempDir())
	_, err := (&Opener{}).OpenRegular(canonical)
	if !errors.Is(err, ErrNotRegular) {
		t.Fatalf("OpenRegular(directory) error %v, want ErrNotRegular", err)
	}
	text := err.Error()
	if !strings.Contains(text, syspath.ToNative(canonical)) || strings.Contains(text, "/") ||
		strings.Contains(text, `\\?\`) {
		t.Errorf("error %q, want the backslashed path %q and no slash or \\\\?\\ prefix",
			text, syspath.ToNative(canonical))
	}
}

func TestOpenerAccessList(t *testing.T) {
	both := []uint32{windows.GENERIC_READ | windows.FILE_WRITE_ATTRIBUTES, windows.GENERIC_READ}
	readOnly := []uint32{windows.GENERIC_READ}
	cases := []struct {
		name   string
		opener *Opener
		want   []uint32
	}{
		{"a nil opener asks for the right, then reads only", nil, both},
		{"the default opener asks for the right, then reads only", NewOpener(false, false), both},
		{"--no-preserve-atime reads only", NewOpener(true, false), readOnly},
		{"updates off reads only", NewOpener(false, true), readOnly},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := accessList(test.opener.asksForRight())
			if fmt.Sprint(got) != fmt.Sprint(test.want) {
				t.Errorf("access list %#x, want %#x", got, test.want)
			}
		})
	}
}
