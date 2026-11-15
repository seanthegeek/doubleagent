package fsmeta

import (
	"errors"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

func TestOpenRegularErrorNamesTheNativePath(t *testing.T) {
	canonical := syspath.FromNative(t.TempDir())
	_, err := OpenRegular(canonical)
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
