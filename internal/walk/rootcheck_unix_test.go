//go:build unix

package walk

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/stage"
)

// nastyImage builds an image root and an outside directory: a/b real, l a
// relative link to a/b, toLink a link to l, out an absolute link outside,
// up a link climbing above the root, dotted a link whose target holds `..`,
// and outside/x a file. It returns both, resolved.
func nastyImage(t *testing.T) (root, outside string) {
	t.Helper()
	root = fixture{dirs: []string{"a/b/c", "srv"}, files: []string{"a/b/f", "a/secret"}}.build(t)
	outside = fixture{files: []string{"x", "secret"}}.build(t)
	for target, link := range map[string]string{
		"a/b": "l", "l": "toLink", outside: "out", "../../../../../..": "up",
		"a/../a/b": "dotted", "nope/../a/b": "pastMissing",
	} {
		mustDo(t, os.Symlink(target, root+"/"+link))
	}
	return root, outside
}

func TestTheRootCheckAndTheKernelAgree(t *testing.T) {
	root, _ := nastyImage(t)
	physicalRoot, err := stage.PhysicalDir(root)
	mustDo(t, err)
	check := NewRootCheck(root)
	for _, path := range []string{
		"/nope/../out/x", "/l/../out/x", "/l/../../../out", "/toLink/../out/x", "/out/x",
		"/up/x", "/dotted/f", "/pastMissing/f", "/a/./b/.", "//a//b//f", "/a/b/c/../../../..",
		"/l/f", "/toLink/c/..", "/srv/../out", "/a/b/../secret", "/l/../secret",
	} {
		t.Run(path, func(t *testing.T) {
			full := root + path
			found, reached := check.resolve(full, imageProbes())
			if found.name != "" && !found.inImage {
				return
			}
			physical, err := filepath.EvalSymlinks(full)
			if err != nil {
				return
			}
			if _, inside := stage.Under(physical, physicalRoot); !inside {
				t.Errorf("the check let %s through, and the kernel opens %s, outside the root",
					full, physical)
			}
			if reached != physical {
				t.Errorf("the check reached %s for %s, the kernel %s", reached, full, physical)
			}
		})
	}
}
