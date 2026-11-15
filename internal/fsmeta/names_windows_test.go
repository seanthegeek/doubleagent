package fsmeta

import (
	"os"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// mkdirExtended creates each directory below base, a canonical path, through
// the \\?\ form, so names NTFS keeps apart but Win32 might not are made as
// written.
func mkdirExtended(t *testing.T, base string, relatives ...string) {
	t.Helper()
	for _, relative := range relatives {
		if err := os.MkdirAll(syspath.Extended(base+"/"+relative), 0o755); err != nil {
			t.Fatal(err)
		}
	}
}

func TestOnDiskPathAsksTheFileSystem(t *testing.T) {
	base := OnDiskPath(syspath.FromNative(t.TempDir()))
	mkdirExtended(t, base, "\u00c9mile/sub", "img", "\u0131mg", ".secret", ".\u017fecret")
	cases := []struct{ name, path, want string }{
		{"an accented letter in another case takes the disk's spelling",
			base + "/\u00e9mile/SUB", base + "/\u00c9mile/sub"},
		{"a missing component and what follows stay as written",
			base + "/\u00e9mile/Gone/SUB", base + "/\u00c9mile/Gone/SUB"},
		{"dot-dot stops the respelling", base + "/\u00e9mile/../IMG", base + "/\u00c9mile/../IMG"},
		{"capital I names img, never dotless-i img beside it", base + "/IMG", base + "/img"},
		{"dotless-i img keeps its own spelling", base + "/\u0131MG", base + "/\u0131mg"},
		{"long s secret is its own entry", base + "/.\u017fECRET", base + "/.\u017fecret"},
		{"a wildcard component stays as written", base + "/i*", base + "/i*"},
		{"a relative path is unchanged", "\u00e9mile/sub", "\u00e9mile/sub"},
		{"a bare drive is unchanged", "C:", "C:"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := OnDiskPath(test.path); got != test.want {
				t.Errorf("OnDiskPath(%q) = %q, want %q", test.path, got, test.want)
			}
		})
	}
}
