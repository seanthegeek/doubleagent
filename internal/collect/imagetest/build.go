package imagetest

import (
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// Image is a built image: its work directory, the image root inside it
// (work/root), and the two homes the cases name alice's and bob's. Every path
// is native and has its links and short names resolved, as a collector
// resolves -r, so the paths a run prints match them byte for byte.
type Image struct {
	Work  string
	Root  string
	Alice string
	Bob   string
}

// Build replays this platform's committed manifest under directory, which
// becomes the image's work directory, and returns the image. It fails the
// test on any entry it cannot make, except a link where the account may not
// create links (Windows without the privilege), which it leaves out with a
// note, as the smoke tests did.
func Build(test testing.TB, directory string) Image {
	test.Helper()
	entries, err := Parse(bytes.NewReader(manifestText))
	if err != nil {
		test.Fatalf("%s: %v", manifestName, err)
	}
	work := resolvedDirectory(test, directory)
	undo, err := Replay(work, entries, test.Logf)
	test.Cleanup(func() {
		if err := undo(); err != nil {
			test.Errorf("reopening the image for its removal: %v", err)
		}
	})
	if err != nil {
		test.Fatal(err)
	}
	img := imageAt(work)
	settle(test, img.Work)
	return img
}

// Entries returns the entries of this platform's committed manifest.
func Entries() ([]Entry, error) {
	return Parse(bytes.NewReader(manifestText))
}

// Replay makes the entries under the directory work, then sets their
// modification times and modes deepest first, so setting a directory's
// time is not undone by making or changing something below it, and a
// directory whose mode shuts it is shut last. A link the account may not
// create is reported through note and left out. The undo it returns, never
// nil, opens again what a mode or deny rule shut, so the tree can be
// removed; it is to be called even when Replay fails.
func Replay(work string, entries []Entry, note func(format string, args ...any)) (undo func() error, err error) {
	var undos []func() error
	undo = func() error {
		var failures []error
		for _, step := range slices.Backward(undos) {
			failures = append(failures, step())
		}
		return errors.Join(failures...)
	}
	sorted := slices.Clone(entries)
	slices.SortFunc(sorted, func(first, second Entry) int { return strings.Compare(first.Path, second.Path) })
	made := make([]Entry, 0, len(sorted))
	for _, entry := range sorted {
		skipped, err := create(work, entry)
		if err != nil {
			return undo, fmt.Errorf("%s: %w", entry.Path, err)
		}
		if skipped != "" {
			note("note: %s left out: %s", entry.Path, skipped)
			continue
		}
		made = append(made, entry)
	}
	for _, entry := range slices.Backward(made) {
		step, err := finish(nativePath(work, entry.Path), entry)
		if err != nil {
			return undo, fmt.Errorf("%s: %w", entry.Path, err)
		}
		if step != nil {
			undos = append(undos, step)
		}
	}
	return undo, nil
}

// nativePath returns the native path of the slash-separated path below work.
func nativePath(work, path string) string {
	return work + string(filepath.Separator) + filepath.FromSlash(path)
}

// create makes one entry, and its parent directories, under work. It returns
// why it left the entry out, or "" when it made it.
func create(work string, entry Entry) (skipped string, err error) {
	path := nativePath(work, entry.Path)
	if entry.Kind == KindDir {
		return "", osfile.MkdirAll(path, 0o755)
	}
	if err := osfile.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	switch entry.Kind {
	case KindFile:
		return "", osfile.WriteFile(path, []byte(entry.Text), 0o644)
	case KindLink, KindDirLink, KindJunction:
		target := entry.Text
		if entry.FromWork {
			target = nativePath(work, entry.Text)
		}
		return makeLink(path, target, entry.Kind)
	}
	return "", makeSpecial(path, entry.Kind)
}

// imageAt returns the image whose work directory is work.
func imageAt(work string) Image {
	root := nativePath(work, "root")
	return Image{Work: work, Root: root, Alice: nativePath(root, aliceHome),
		Bob: nativePath(root, bobHome)}
}
