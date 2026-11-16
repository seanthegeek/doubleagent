//go:build unix

package walk

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/stage"
	"syscall"
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
	mustDo(t, syscall.Mkfifo(root+"/a/fifo", 0o644))
	mustDo(t, os.Mkdir(root+"/noexec", 0o755))
	mustDo(t, os.Chmod(root+"/noexec", 0o600))
	t.Cleanup(func() { _ = os.Chmod(root+"/noexec", 0o755) })
	physicalRoot, err := stage.PhysicalDir(root)
	mustDo(t, err)
	t.Chdir(filepath.Dir(root))
	paths := []string{
		"/nope/../out/x", "/l/../out/x", "/l/../../../out", "/toLink/../out/x", "/out/x",
		"/up/x", "/dotted/f", "/pastMissing/f", "/a/./b/.", "//a//b//f", "/a/b/c/../../../..",
		"/l/f", "/toLink/c/..", "/srv/../out", "/a/b/../secret", "/l/../secret",
		"/a/b/f/../c", "/a/secret/..", "/a/b/c/nope/..", "/l/f/x/../..",
		"/a/b/f/", "/a/b/f/.", "/a/fifo/.", "/l/f/", "/noexec/..", "/noexec/../a",
	}
	// The root as resolved, with a trailing slash, and relative to the
	// working directory.
	for _, given := range []string{root, root + "/", filepath.Base(root)} {
		check := NewRootCheck(given)
		for _, path := range paths {
			t.Run(given+path, func(t *testing.T) {
				if strings.HasPrefix(path, "/noexec") && os.Geteuid() == 0 {
					t.Skip("root searches any directory, so the kernel resolves noexec/..")
				}
				full := strings.TrimSuffix(given, "/") + path
				found, reached := check.resolve(full, imageProbes())
				// Where the kernel fails to resolve the path (a missing
				// component, a file followed by anything, a directory that
				// cannot be searched), the check reaches nothing; where it
				// succeeds, the two reach the same place.
				if _, err := os.Stat(full); err != nil {
					if reached != "" {
						t.Errorf("the kernel cannot resolve %s (%v), the check reached %s", full,
							err, reached)
					}
					return
				}
				if found.name != "" && !found.inImage {
					return
				}
				physical, err := filepath.EvalSymlinks(full)
				mustDo(t, err)
				if !filepath.IsAbs(physical) {
					physical = filepath.Join(filepath.Dir(root), physical)
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
}

func TestAClimbingProjectIsWordedAsClimbing(t *testing.T) {
	root := fixture{dirs: []string{"home/dot"}}.build(t)
	collector, rows, log := testCollector(t, root, nil)
	collector.Tables = &catalog.Tables{ProjectCatalog: []catalog.Entry{{Agent: "project",
		Glob: "AGENTS.md"}}}
	project := root + "/../../tmp/escape"
	mustDo(t, collector.CollectProject(context.Background(), "alice", "", project))
	native := project
	want := []string{"Project [alice] " + native + ": " + native +
		" climbs above the image root with ..; nothing behind it was looked at; not walked"}
	if !reflect.DeepEqual(log.lines, want) {
		t.Errorf("log %q, want %q", log.lines, want)
	}
	if rows.Len() != 0 {
		t.Errorf("rows %q, want none", rows.String())
	}
}

// reachedImage builds, under a new directory, the tree the Reached table
// walks: srv/dotlink a link to ../opt/escape, so srv/dotlink/../frank is
// opt/frank, a different directory from srv/frank; a file srv/file.txt; a
// home home/bob. It returns the directory, resolved, and a link to it.
func reachedImage(t *testing.T) (base, viaLink string) {
	t.Helper()
	base = fixture{dirs: []string{"opt/escape", "opt/frank", "srv/frank", "home/bob", "noexec"},
		files: []string{"srv/file.txt"}}.build(t)
	mustDo(t, os.Symlink("../opt/escape", base+"/srv/dotlink"))
	mustDo(t, syscall.Mkfifo(base+"/srv/fifo", 0o644))
	mustDo(t, os.Chmod(base+"/noexec", 0o600))
	t.Cleanup(func() { _ = os.Chmod(base+"/noexec", 0o755) })
	viaLink = t.TempDir() + "/via"
	mustDo(t, os.Symlink(base, viaLink))
	return base, viaLink
}

// TestReachedAgreesWithTheKernel is the table of Reached's inputs: the root
// none (a live run), plain, or given through a link; the path plain, dotted,
// dotted through a link, through a file, with a missing component, equal to
// the root, or above it. Where the kernel cannot resolve a dotted path, or
// the image root's check refuses it, the path comes back as written; where
// the kernel resolves it, Reached returns a path with no `.` or `..` naming
// the same directory, spelled under the root as given in image mode.
func TestReachedAgreesWithTheKernel(t *testing.T) {
	base, viaLink := reachedImage(t)
	paths := []string{"home/bob", "srv/dotlink/../frank", "home/./bob", "home/nope/../bob",
		"srv/file.txt/../../home/bob", "home/bob/../nope", "home/..", "home/bob/../..",
		"srv/dotlink/../../srv/frank", "..", "srv/dotlink/../../..", "srv/dotlink/..",
		"srv/file.txt/", "srv/file.txt/.", "srv/fifo/.", "srv/file.txt/./..", "noexec/..",
		"noexec/../home/bob"}
	t.Chdir(filepath.Dir(base))
	for _, root := range []struct{ name, given string }{
		{"live", ""}, {"plain", base}, {"through a link", viaLink},
		{"with a trailing slash", base + "/"}, {"relative", filepath.Base(base)},
	} {
		check := NewRootCheck(root.given)
		prefix := strings.TrimSuffix(root.given, "/")
		if prefix == "" {
			prefix = base
		}
		given := strings.TrimSuffix(root.given, "/")
		for _, rel := range paths {
			t.Run(root.name+"/"+rel, func(t *testing.T) {
				if strings.HasPrefix(rel, "noexec") && os.Geteuid() == 0 {
					t.Skip("root searches any directory, so the kernel resolves noexec/..")
				}
				path := prefix + "/" + rel
				got := check.Reached(path)
				kernel, kernelErr := os.Stat(path)
				if !hasDotComponent(rel) {
					if got != path {
						t.Errorf("Reached(%s) = %s, want it unchanged", path, got)
					}
					return
				}
				if kernelErr != nil {
					if got != path {
						t.Errorf("the kernel cannot resolve %s (%v), Reached gave %s", path,
							kernelErr, got)
					}
					return
				}
				real, err := filepath.EvalSymlinks(path)
				mustDo(t, err)
				real, err = filepath.Abs(real)
				mustDo(t, err)
				if _, inside := stage.Under(real, base); root.given != "" && !inside {
					if got != path {
						t.Errorf("Reached(%s) = %s; the kernel reaches %s, outside the root, "+
							"so it should come back as written", path, got, real)
					}
					return
				}
				if hasDotComponent(got) {
					t.Fatalf("Reached(%s) = %s, still dotted", path, got)
				}
				if info, err := os.Stat(got); err != nil || !os.SameFile(info, kernel) {
					t.Errorf("Reached(%s) = %s, which is not the kernel's %s", path, got, real)
				}
				if given != "" && got != given && !strings.HasPrefix(got, given+"/") {
					t.Errorf("Reached(%s) = %s, not spelled under the root as given %s", path, got,
						given)
				}
			})
		}
	}
}

// TestTheRootWithATrailingSlashIsInsideIt is relative's table for a path
// that is the root: as given or resolved, with and without a trailing slash.
func TestTheRootWithATrailingSlashIsInsideIt(t *testing.T) {
	base, viaLink := reachedImage(t)
	for _, given := range []string{base, viaLink} {
		walker := rootWalk{given: given, physical: base}
		for _, path := range []string{given, given + "/", base, base + "/"} {
			if rest, inside := walker.relative(path); !inside || rest != "" {
				t.Errorf("relative(%s) under %s = %q, %v; want the root itself", path, given,
					rest, inside)
			}
		}
	}
}
