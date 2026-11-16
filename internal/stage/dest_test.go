package stage

import (
	"errors"
	"io/fs"
	"strings"
	"testing"
)

func TestDest(t *testing.T) {
	stager := Stager{Dir: "/out/.stage", Root: "/img"}
	cases := []struct {
		name        string
		stager      Stager
		path        string
		wantArchive string
	}{
		{"the root is removed from the front", stager, "/img/home/a/f", "fs/home/a/f"},
		{"a live run keeps the whole path", Stager{Dir: "/s"}, "/home/a/f", "fs/home/a/f"},
		{"a path outside the root keeps every byte", stager, "/other/f", "fs/other/f"},
		{"the root is removed as a string, not a component", stager, "/imgx/f", "fsx/f"},
		{"a relative path is joined to fs as is", stager, "zzz", "fszzz"},
		{"a path under an in-base link is staged at the physical path",
			stager.Through("/img/h/.a", "/img/h/dot/a"), "/img/h/.a/b/c", "fs/h/dot/a/b/c"},
		{"the in-base link itself is staged at the physical path",
			stager.Through("/img/h/.a", "/img/h/dot/a"), "/img/h/.a", "fs/h/dot/a"},
		{"a sibling sharing the link's prefix is not moved",
			stager.Through("/img/h/.a", "/img/h/dot/a"), "/img/h/.ab/c", "fs/h/.ab/c"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			archive, dest, _ := test.stager.Dest(test.path)
			if archive != test.wantArchive || dest != test.stager.Dir+"/"+test.wantArchive {
				t.Errorf("Dest(%q) = %q, %q", test.path, archive, dest)
			}
		})
	}
}

// mustDo fails the test on err.
func mustDo(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

func TestDestRefusesADotOrDotDotComponentOnWindows(t *testing.T) {
	stager := Stager{Dir: "/out/.stage", Root: "/img"}
	for _, path := range []string{"/img/../img/home/a/f", "/img/home/./a", "/img/a/..", "./f",
		"/img/a/b/../../../../etc/f"} {
		archive, dest, err := stager.Dest(path)
		var pathError *fs.PathError
		switch {
		case refusesDotted && (!errors.As(err, &pathError) || archive != "" || dest != ""):
			t.Errorf("Dest(%q) = %q, %q, %v; want an error", path, archive, dest, err)
		case !refusesDotted && (err != nil || archive != "fs"+strings.TrimPrefix(path, "/img")):
			// As set_dest_var does: the root removed as text, `..` kept.
			t.Errorf("Dest(%q) = %q, %v; want the sh collector's archive path", path, archive, err)
		}
	}
	for _, path := range []string{"/img/home/..a/f", "/img/home/a./f", "/img/.../f"} {
		if _, _, err := stager.Dest(path); err != nil {
			t.Errorf("Dest(%q): %v", path, err)
		}
	}
}
