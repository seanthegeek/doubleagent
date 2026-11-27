//go:build noasm && nounsafe

package zstdframe

import (
	"runtime/debug"
	"strings"
	"testing"
)

// The package does not compile without the two tags, so every test of it
// runs under them. This one records that, and decodes a frame as a check that
// the safe build works. What it cannot show is which files the linker took:
// that is read with go list, as internal/AGENTS.md describes.
func TestThePackageIsBuiltWithTheSafeTags(t *testing.T) {
	info, ok := debug.ReadBuildInfo()
	if ok {
		for _, setting := range info.Settings {
			if setting.Key == "-tags" && !(strings.Contains(setting.Value, "noasm") &&
				strings.Contains(setting.Value, "nounsafe")) {
				t.Fatalf("built with -tags=%s", setting.Value)
			}
		}
	}
	got, err := decodeAll(t, loadNamed(t, "multi-frame"), 1<<30)
	if err != nil || len(got) != 125300 {
		t.Fatalf("decoded %d bytes, err %v", len(got), err)
	}
}
