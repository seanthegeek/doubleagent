//go:build parity

package parity

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/collect/imagetest"
)

// harnessImage returns the image the parity cases run over: the one
// imagetest builds from its manifest, or with PARITY_SMOKE_IMAGE=1 the one
// the smoke test builds.
func harnessImage(t *testing.T, repository string) image {
	t.Helper()
	if os.Getenv("PARITY_SMOKE_IMAGE") == "1" {
		return smokeImage(t, repository)
	}
	built := imagetest.Build(t, t.TempDir())
	return image{work: built.Work, root: built.Root, alice: built.Alice, bob: built.Bob, built: &built}
}

// fixtureStep returns what builds a case's fixtures (or checks after its
// run) over img: the harness's own step over the smoke test's image, or
// over the built image the imagetest builder of the same name, so the cases
// prove the builders too. A case with a step and no builder fails.
func fixtureStep(t *testing.T, name string, step func(*testing.T, image), img image,
	builders map[string]func(imagetest.Image, testing.TB)) func(*testing.T, image) {
	t.Helper()
	if step == nil || img.built == nil {
		return step
	}
	builder, found := builders[name]
	if !found {
		t.Fatalf("the case %s builds fixtures, and imagetest has no builder for it", name)
	}
	built := *img.built
	return func(t *testing.T, _ image) { builder(built, t) }
}

// isSmokeBookkeeping reports whether a top-level entry of the smoke test's
// work directory is a list its checks read, not part of the image: the run
// times and the expected rows of the startup and drift checks.
func isSmokeBookkeeping(path string) bool {
	return path == "run-times" || strings.HasPrefix(path, "startup-") || strings.HasPrefix(path, "drift-")
}

// imageEntry describes what the comparison of the two images looks at in
// an entry: its kind, link target, the hash of its content and its mode.
// Times are left out, since the smoke test's are those of the moment it ran.
func imageEntry(entry imagetest.Entry) string {
	target := ""
	if entry.Kind != imagetest.KindFile {
		target = entry.Text
	}
	return fmt.Sprintf("%s target %q from-work=%v content %x mode %04o", entry.Kind, target,
		entry.FromWork, sha256.Sum256([]byte(entry.Text)), uint32(entry.Mode))
}

// foldedKey returns the path an entry is matched by: with ASCII letters
// folded where the file system folds case, since there each pair of names
// differing only in case is one entry, spelled as its first-made name, and
// the smoke test and the builder make them in another order.
func foldedKey(path string, folds bool) string {
	if !folds {
		return path
	}
	return strings.Map(func(character rune) rune {
		if character >= 'A' && character <= 'Z' {
			return character + 'a' - 'A'
		}
		return character
	}, path)
}

// readImage returns the entries under work keyed by foldedKey, leaving out
// the smoke test's bookkeeping.
func readImage(t *testing.T, work string, folds bool) map[string]imagetest.Entry {
	t.Helper()
	entries, err := imagetest.Read(work)
	if err != nil {
		t.Fatal(err)
	}
	keyed := map[string]imagetest.Entry{}
	for _, entry := range entries {
		if !isSmokeBookkeeping(entry.Path) {
			keyed[foldedKey(entry.Path, folds)] = entry
		}
	}
	return keyed
}

func TestTheBuiltImageHoldsWhatTheSmokeTestBuilds(t *testing.T) {
	smoke := smokeImage(t, repositoryRoot(t))
	built := imagetest.Build(t, t.TempDir())
	folds := foldsCase(t, built.Work)
	want, got := readImage(t, smoke.work, folds), readImage(t, built.Work, folds)
	if len(want) < 3000 {
		t.Fatalf("the smoke image holds %d entries, too few for the main image", len(want))
	}
	for key, entry := range got {
		expected, found := want[key]
		switch {
		case !found:
			t.Errorf("only the built image holds %s", imagetest.Format(entry))
		case imageEntry(expected) != imageEntry(entry):
			t.Errorf("%q: the smoke image holds %s, the built image %s", entry.Path,
				imageEntry(expected), imageEntry(entry))
		}
	}
	for key, entry := range want {
		if _, found := got[key]; !found {
			t.Errorf("only the smoke image holds %s", imagetest.Format(entry))
		}
	}
}
