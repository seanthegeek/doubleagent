//go:build parity

package replay

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// brokenTree marks a work tree whose last switch did not finish, so its
// disk state is unknown and the next switch starts from an empty root.
const brokenTree = "{broken}"

// WorkTree is one root on disk that takes the state of one recorded tree
// after another. A switch changes only what differs between the tree on
// disk and the next one, comparing the content-addressed tree objects so an
// unchanged directory is skipped whole, and sets the times that differ: the
// recorded fixture homes are large and mostly alike, so writing each one
// whole would cost far more than the parses read. The root keeps one path
// for the whole run, as the recorder's root did.
type WorkTree struct {
	corpus *Corpus
	Root   string
	tree   string
	times  string
	mtimes map[string]int64
	// entries caches the tree objects read, by hash.
	entries map[string][]Entry
	// denied caches, by tree hash, whether a tree holds an entry whose mode
	// denies its owner reading it.
	denied map[string]bool
}

// NewWorkTree returns a work tree with an empty root of the corpus's root
// length under the test's temporary directory.
func (c *Corpus) NewWorkTree(t testing.TB) *WorkTree {
	t.Helper()
	return &WorkTree{corpus: c, Root: c.PaddedRoot(t), entries: map[string][]Entry{},
		denied: map[string]bool{}}
}

// Switch gives the root the state of the tree and times objects: the
// entries, bytes, modes and modification times Materialise would write.
func (w *WorkTree) Switch(t testing.TB, tree, times string) {
	t.Helper()
	if tree == w.tree && times == w.times {
		return
	}
	mtimes := map[string]int64{}
	if times != "" {
		var err error
		if mtimes, err = w.corpus.ReadTimes(times); err != nil {
			t.Fatal(err)
		}
	}
	old := w.tree
	if old == brokenTree {
		if err := w.clear(); err != nil {
			t.Fatal(err)
		}
		old, w.mtimes = "", map[string]int64{}
	}
	w.tree = brokenTree
	changed := changedTimes(w.mtimes, mtimes)
	sync := treeSync{work: w, mtimes: mtimes, changed: changed, holders: holders(changed)}
	if _, err := sync.directory(w.Root, "", old, tree, 0o700); err != nil {
		t.Fatalf("switch to tree %s: %v", tree, err)
	}
	w.tree, w.times, w.mtimes = tree, times, mtimes
}

// clear removes everything under the root.
func (w *WorkTree) clear() error {
	entries, err := osfile.ReadDir(w.Root)
	if err != nil {
		return fmt.Errorf("clear work tree: %w", err)
	}
	for _, entry := range entries {
		if err := removeEntry(filepath.Join(w.Root, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

// read returns the entries of a tree object, none for "".
func (w *WorkTree) read(tree string) ([]Entry, error) {
	if tree == "" {
		return nil, nil
	}
	if entries, ok := w.entries[tree]; ok {
		return entries, nil
	}
	entries, err := w.corpus.ReadTree(tree)
	if err != nil {
		return nil, err
	}
	w.entries[tree] = entries
	return entries, nil
}

// DeniesReading reports whether the tree holds a file its owner may not
// read or a directory its owner may not list.
func (w *WorkTree) DeniesReading(tree string) (bool, error) {
	if denied, ok := w.denied[tree]; ok {
		return denied, nil
	}
	entries, err := w.read(tree)
	if err != nil {
		return false, err
	}
	denied := false
	for _, entry := range entries {
		if entry.Kind == "file" && entry.Mode&0o400 == 0 ||
			entry.Kind == "dir" && entry.Mode&0o500 != 0o500 {
			denied = true
		} else if entry.Kind == "dir" {
			if denied, err = w.DeniesReading(entry.Tree); err != nil {
				return false, err
			}
		}
		if denied {
			break
		}
	}
	w.denied[tree] = denied
	return denied, nil
}

// changedTimes returns the root-relative paths whose recorded time differs
// between two times objects, or that only one of them has.
func changedTimes(old, mtimes map[string]int64) map[string]bool {
	changed := map[string]bool{}
	for path, mtime := range mtimes {
		if previous, ok := old[path]; !ok || previous != mtime {
			changed[path] = true
		}
	}
	for path := range old {
		if _, ok := mtimes[path]; !ok {
			changed[path] = true
		}
	}
	return changed
}

// holders returns every directory that holds, at any depth, one of the
// paths: the directories a switch must enter to reach them.
func holders(paths map[string]bool) map[string]bool {
	found := map[string]bool{}
	for path := range paths {
		for {
			slash := strings.LastIndexByte(path, '/')
			if slash < 0 || found[path[:slash]] {
				break
			}
			path = path[:slash]
			found[path] = true
		}
	}
	return found
}

// treeSync is one switch of a work tree.
type treeSync struct {
	work    *WorkTree
	mtimes  map[string]int64
	changed map[string]bool
	holders map[string]bool
}

// unknownMode stands for the mode of an entry just created, which finish
// always sets.
const unknownMode = ^uint32(0)

// directory brings the directory at path, holding the entries of tree old,
// to the entries of tree next, and returns its mode after. relative is its
// root-relative path ("" for the root) and mode its mode before. A mode that
// keeps its owner out is opened up first; whoever holds the directory sets
// its time and mode after.
func (s treeSync) directory(path, relative, old, next string, mode uint32) (uint32, error) {
	oldEntries, err := s.work.read(old)
	if err != nil {
		return mode, err
	}
	nextEntries, err := s.work.read(next)
	if err != nil {
		return mode, err
	}
	if mode&0o700 != 0o700 || mode == unknownMode {
		if err := os.Chmod(path, 0o700); err != nil {
			return mode, fmt.Errorf("open up %s: %w", path, err)
		}
		mode = 0o700
	}
	previous := map[Text]Entry{}
	for _, entry := range oldEntries {
		previous[entry.Name] = entry
	}
	kept := map[Text]bool{}
	for _, entry := range nextEntries {
		kept[entry.Name] = true
	}
	for _, entry := range oldEntries {
		if !kept[entry.Name] {
			if err := removeEntry(filepath.Join(path, string(entry.Name))); err != nil {
				return mode, err
			}
		}
	}
	for _, entry := range nextEntries {
		child := string(entry.Name)
		if relative != "" {
			child = relative + "/" + child
		}
		before, had := previous[entry.Name]
		if err := s.entry(filepath.Join(path, string(entry.Name)), child, before, had, entry); err != nil {
			return mode, err
		}
	}
	return mode, nil
}

// entry brings one entry from its state before (when it had one) to entry.
func (s treeSync) entry(path, relative string, before Entry, had bool, entry Entry) error {
	if had && before.Kind != entry.Kind {
		if err := removeEntry(path); err != nil {
			return err
		}
		had = false
	}
	switch entry.Kind {
	case "dir":
		return s.dir(path, relative, before, had, entry)
	case "file":
		if had && before.Blob == entry.Blob {
			return s.finish(path, relative, false, before.Mode, entry.Mode)
		}
		if had {
			if err := removeEntry(path); err != nil {
				return err
			}
		}
		if err := s.writeFile(path, entry); err != nil {
			return err
		}
		return s.finish(path, relative, true, unknownMode, entry.Mode)
	case "link":
		if had && before.Target == entry.Target {
			return nil
		}
		if had {
			if err := removeEntry(path); err != nil {
				return err
			}
		}
		if err := os.Symlink(s.work.corpus.Expand(entry.Target, s.work.Root), path); err != nil {
			return fmt.Errorf("work tree link: %w", err)
		}
	}
	return nil
}

// dir brings one directory entry to its next state, entering it only when
// its tree or a time below it changed.
func (s treeSync) dir(path, relative string, before Entry, had bool, entry Entry) error {
	old, mode := before.Tree, before.Mode
	if !had {
		if err := osfile.MkdirAll(path, 0o700); err != nil {
			return err
		}
		old, mode = "", unknownMode
	}
	if !had || before.Tree != entry.Tree || s.holders[relative] {
		mode, err := s.directory(path, relative, old, entry.Tree, mode)
		if err != nil {
			return err
		}
		return s.finish(path, relative, true, mode, entry.Mode)
	}
	return s.finish(path, relative, false, mode, entry.Mode)
}

// writeFile writes a file's blob, the root put back where the placeholder
// stands.
func (s treeSync) writeFile(path string, entry Entry) error {
	var data []byte
	if entry.Blob != "" {
		blob, err := s.work.corpus.ReadBlob(entry.Blob, s.work.Root)
		if err != nil {
			return err
		}
		data = blob
	}
	return osfile.WriteFile(path, data, 0o600)
}

// finish sets an entry's recorded time when it changed or the entry was
// written or entered (touched), then, last, its recorded mode when the mode
// it has (current) differs, so a mode that denies writing applies after.
func (s treeSync) finish(path, relative string, touched bool, current, mode uint32) error {
	if mtime, ok := s.mtimes[relative]; ok && (touched || s.changed[relative]) {
		modified := time.Unix(0, mtime)
		if err := osfile.Chtimes(path, modified, modified); err != nil {
			return err
		}
	}
	if current == mode {
		return nil
	}
	if err := os.Chmod(path, fs.FileMode(mode)&fs.ModePerm); err != nil {
		return fmt.Errorf("work tree mode: %w", err)
	}
	return nil
}

// removeEntry removes a file, link or directory tree, opening up the modes
// a recorded tree may have closed first.
func removeEntry(path string) error {
	_ = filepath.WalkDir(path, func(entry string, info fs.DirEntry, _ error) error {
		if info != nil && info.Type()&fs.ModeSymlink == 0 {
			_ = os.Chmod(entry, 0o700)
		}
		return nil
	})
	if err := osfile.RemoveAll(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("work tree remove: %w", err)
	}
	return nil
}

// keepsModes reports whether the platform keeps the permission bits a test
// sets; Windows keeps only whether a file is read-only.
func keepsModes(t *testing.T) bool {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mode")
	if err := osfile.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	info, err := osfile.Lstat(path)
	return err == nil && info.Mode().Perm() == 0o640
}

// treeChecker compares a directory on disk with a recorded tree.
type treeChecker struct {
	t      *testing.T
	corpus *Corpus
	root   string
	mtimes map[string]int64
	modes  bool
}

// checkState fails the test where the work tree's root differs from the
// tree and times objects: an entry missing or extra, its kind, bytes, link
// target, permission bits (where the platform keeps them) or time.
func checkState(t *testing.T, work *WorkTree, tree, times string, modes bool) {
	t.Helper()
	mtimes := map[string]int64{}
	if times != "" {
		var err error
		if mtimes, err = work.corpus.ReadTimes(times); err != nil {
			t.Fatal(err)
		}
	}
	checker := treeChecker{t: t, corpus: work.corpus, root: work.Root, mtimes: mtimes, modes: modes}
	checker.directory(work.Root, "", tree)
}

// directory compares one directory's entries with a tree object.
func (c treeChecker) directory(path, relative, tree string) {
	c.t.Helper()
	entries, err := c.corpus.ReadTree(tree)
	if tree == "" {
		entries, err = nil, nil
	}
	if err != nil {
		c.t.Fatal(err)
	}
	info, err := osfile.Lstat(path)
	if err != nil {
		c.t.Fatal(err)
	}
	// Opened up while its entries are checked; a mode change keeps the time.
	if err := os.Chmod(path, 0o700); err != nil {
		c.t.Fatal(err)
	}
	defer func() { _ = os.Chmod(path, info.Mode().Perm()) }()
	listed, err := osfile.ReadDir(path)
	if err != nil {
		c.t.Fatal(err)
	}
	onDisk := map[string]bool{}
	for _, entry := range listed {
		onDisk[entry.Name()] = true
	}
	for _, entry := range entries {
		name := string(entry.Name)
		child := name
		if relative != "" {
			child = relative + "/" + name
		}
		created := entry.Kind == "dir" || entry.Kind == "file" || entry.Kind == "link"
		if onDisk[name] != created {
			c.t.Errorf("%s: on disk %v, recorded %s", child, onDisk[name], entry.Kind)
			continue
		}
		delete(onDisk, name)
		if created {
			c.entry(filepath.Join(path, name), child, entry)
		}
	}
	for name := range onDisk {
		c.t.Errorf("%s/%s: on disk, not recorded", relative, name)
	}
}

// entry compares one entry with its record.
func (c treeChecker) entry(path, relative string, entry Entry) {
	c.t.Helper()
	info, err := osfile.Lstat(path)
	if err != nil {
		c.t.Fatal(err)
	}
	switch entry.Kind {
	case "link":
		target, err := osfile.Readlink(path)
		if want := c.corpus.Expand(entry.Target, c.root); err != nil || target != want {
			c.t.Errorf("%s: link to %q (%v), want %q", relative, target, err, want)
		}
		return
	case "dir":
		c.directory(path, relative, entry.Tree)
	case "file":
		c.content(path, relative, entry)
	}
	if c.modes && info.Mode().Perm() != fs.FileMode(entry.Mode)&fs.ModePerm {
		c.t.Errorf("%s: mode %v, want %v", relative, info.Mode().Perm(), fs.FileMode(entry.Mode))
	}
	const unit = 100 * time.Nanosecond
	if mtime, ok := c.mtimes[relative]; ok &&
		!info.ModTime().Truncate(unit).Equal(time.Unix(0, mtime).Truncate(unit)) {
		c.t.Errorf("%s: time %v, want %v", relative, info.ModTime(), time.Unix(0, mtime))
	}
}

// content compares a file's bytes with its blob.
func (c treeChecker) content(path, relative string, entry Entry) {
	c.t.Helper()
	data, err := withOpenMode(path, 0o600, func() ([]byte, error) { return osfile.ReadFile(path) })
	if err != nil {
		c.t.Fatal(err)
	}
	var want []byte
	if entry.Blob != "" {
		if want, err = c.corpus.ReadBlob(entry.Blob, c.root); err != nil {
			c.t.Fatal(err)
		}
	}
	if !bytes.Equal(data, want) {
		c.t.Errorf("%s: %d bytes differ from blob %s", relative, len(data), entry.Blob)
	}
}

// withOpenMode runs read with path's mode set to open, then puts the mode
// back; changing a mode leaves the modification time as it is.
func withOpenMode[T any](path string, open fs.FileMode, read func() (T, error)) (T, error) {
	info, err := osfile.Lstat(path)
	if err != nil {
		var zero T
		return zero, err
	}
	if err := os.Chmod(path, open); err != nil {
		var zero T
		return zero, err
	}
	value, err := read()
	if chmodErr := os.Chmod(path, info.Mode().Perm()); err == nil {
		err = chmodErr
	}
	return value, err
}

func TestWorkTreeSwitchesBetweenTreesChangingOnlyWhatDiffers(t *testing.T) {
	built := newTestCorpus(t)
	first, second := built.blob("first\n"), built.blob("second at "+built.header.RootPlaceholder+"\n")
	secret := built.blob("secret\n")
	sub := built.tree(map[string]any{"name": "b.txt", "kind": "file", "mode": 0o644, "blob": first})
	locked := built.tree(map[string]any{"name": "secret", "kind": "file", "mode": 0, "blob": secret})
	treeA := built.tree(map[string]any{"name": "top", "kind": "dir", "mode": 0o755,
		"tree": built.tree(
			map[string]any{"name": "a.txt", "kind": "file", "mode": 0o644, "blob": first},
			map[string]any{"name": "link", "kind": "link", "mode": 0o777, "target": "{ROOT}/top/a.txt"},
			map[string]any{"name": "locked", "kind": "dir", "mode": 0, "tree": locked},
			map[string]any{"name": "sub", "kind": "dir", "mode": 0o755, "tree": sub})})
	treeB := built.tree(map[string]any{"name": "top", "kind": "dir", "mode": 0o750,
		"tree": built.tree(
			map[string]any{"name": "a.txt", "kind": "file", "mode": 0o600, "blob": second},
			map[string]any{"name": "locked", "kind": "file", "mode": 0o400, "blob": first},
			map[string]any{"name": "new.txt", "kind": "file", "mode": 0o644, "blob": ""},
			map[string]any{"name": "sub", "kind": "dir", "mode": 0o700, "tree": sub})})
	moment := time.Date(2026, 10, 1, 9, 0, 0, 123456700, time.UTC).UnixNano()
	timesA := built.times(map[string]int64{"top": moment, "top/a.txt": moment + 1e9,
		"top/locked": moment + 2e9, "top/locked/secret": moment + 3e9, "top/sub": moment + 4e9,
		"top/sub/b.txt": moment + 5e9})
	timesB := built.times(map[string]int64{"top": moment + 6e9, "top/a.txt": moment + 7e9,
		"top/locked": moment + 8e9, "top/sub": moment + 4e9, "top/sub/b.txt": moment + 9e9})
	corpus := built.open()
	work := corpus.NewWorkTree(t)
	modes := keepsModes(t)
	steps := []struct{ name, tree, times string }{
		{"the first tree is written whole", treeA, timesA},
		{"a changed file, kind, mode and time are each brought over", treeB, timesB},
		{"the same tree with other times only moves the times", treeB, timesA},
		{"switching back restores the removed entries", treeA, timesA},
		{"an empty tree leaves an empty root", "", ""},
	}
	for _, step := range steps {
		t.Run(step.name, func(t *testing.T) {
			work.Switch(t, step.tree, step.times)
			checkState(t, work, step.tree, step.times, modes)
		})
	}
}

func TestWorkTreeTakesEachRecordedTreeExactly(t *testing.T) {
	corpus := Open(t)
	work := corpus.NewWorkTree(t)
	modes := keepsModes(t)
	seen := map[[2]string]bool{}
	checked := 0
	for _, agent := range caseFiles(t, corpus, "parsers") {
		for _, recorded := range corpus.ParserCases(t, agent) {
			key := [2]string{recorded.Tree, recorded.Times}
			if recorded.Method != "parse" || recorded.Tree == "" || seen[key] {
				continue
			}
			seen[key] = true
			if len(seen)%25 != 1 {
				continue
			}
			work.Switch(t, recorded.Tree, recorded.Times)
			checkState(t, work, recorded.Tree, recorded.Times, modes)
			checked++
		}
	}
	t.Logf("%d of %d distinct trees checked after a switch", checked, len(seen))
}
