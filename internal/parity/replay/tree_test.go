//go:build parity

package replay

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// Entry is one entry of a tree object: a file with its blob, a directory with
// its tree, or a link with its target. Blob is empty for a file the recorder
// could never read. A fifo, socket or device has only its kind and mode.
type Entry struct {
	Name   Text   `json:"name"`
	Kind   string `json:"kind"`
	Mode   uint32 `json:"mode"`
	Blob   string `json:"blob"`
	Tree   string `json:"tree"`
	Target Text   `json:"target"`
}

// ReadTree returns the entries of one tree object.
func (c *Corpus) ReadTree(digest string) ([]Entry, error) {
	data, err := osfile.ReadFile(filepath.Join(c.Directory, "trees", digest+".json"))
	if err != nil {
		return nil, fmt.Errorf("read tree: %w", err)
	}
	var tree struct {
		Entries []Entry `json:"entries"`
	}
	if err := json.Unmarshal(data, &tree); err != nil {
		return nil, fmt.Errorf("tree %s: %w", digest, err)
	}
	return tree.Entries, nil
}

// ReadBlob returns a blob's bytes with the root placeholder replaced by root.
func (c *Corpus) ReadBlob(digest, root string) ([]byte, error) {
	data, err := osfile.ReadFile(filepath.Join(c.Directory, "blobs", digest))
	if err != nil {
		return nil, fmt.Errorf("read blob: %w", err)
	}
	return bytes.ReplaceAll(data, []byte(c.Header.RootPlaceholder), []byte(root)), nil
}

// PaddedRoot returns a new directory under the test's temporary directory
// whose path is exactly the corpus's root length, so it can stand where the
// placeholder stands. Its directories are opened up again before the test's
// directory is removed, since a tree may hold one at mode 000.
func (c *Corpus) PaddedRoot(t testing.TB) string {
	t.Helper()
	base := t.TempDir()
	padding := c.Header.RootLength - len(base) - 1
	if padding < 1 {
		t.Fatalf("the temporary directory %s is too long for a root of %d bytes", base,
			c.Header.RootLength)
	}
	root := filepath.Join(base, strings.Repeat("r", padding))
	if err := osfile.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { openUp(base) })
	return root
}

// openUp makes every directory under path listable and removable again.
// WalkDir hands over a directory before it reads it, so the change comes in
// time for the read.
func openUp(path string) {
	_ = filepath.WalkDir(path, func(entry string, info fs.DirEntry, _ error) error {
		if info != nil && info.IsDir() {
			_ = os.Chmod(entry, 0o700)
		}
		return nil
	})
}

// Materialise recreates a case's tree under a new padded root and returns
// the root. Files get their bytes and modes, directories their modes once
// their entries exist, links their targets, and each the modification time
// the times object gives it (a link's own time is not set). A fifo, socket or
// device is not created; its path is logged.
func (c *Corpus) Materialise(t testing.TB, tree, times string) string {
	t.Helper()
	root := c.PaddedRoot(t)
	if tree == "" {
		return root
	}
	writer := treeWriter{corpus: c, t: t, root: root}
	if times != "" {
		mtimes, err := c.ReadTimes(times)
		if err != nil {
			t.Fatal(err)
		}
		writer.mtimes = mtimes
	}
	if err := writer.writeTree(root, "", tree); err != nil {
		t.Fatal(err)
	}
	return root
}

// ReadTimes returns a times object: modification times in nanoseconds by
// root-relative path, a path that is not UTF-8 decoded from its hex form.
func (c *Corpus) ReadTimes(digest string) (map[string]int64, error) {
	data, err := osfile.ReadFile(filepath.Join(c.Directory, "times", digest+".json"))
	if err != nil {
		return nil, fmt.Errorf("read times: %w", err)
	}
	var times struct {
		Mtimes map[string]int64 `json:"mtime_ns"`
	}
	if err := json.Unmarshal(data, &times); err != nil {
		return nil, fmt.Errorf("times %s: %w", digest, err)
	}
	decoded := make(map[string]int64, len(times.Mtimes))
	for key, mtime := range times.Mtimes {
		if encoded, ok := strings.CutPrefix(key, "hex:"); ok {
			raw, err := hex.DecodeString(encoded)
			if err != nil {
				return nil, fmt.Errorf("times %s: %w", digest, err)
			}
			key = string(raw)
		}
		decoded[key] = mtime
	}
	return decoded, nil
}

// treeWriter writes the trees of one materialisation under root.
type treeWriter struct {
	corpus *Corpus
	t      testing.TB
	root   string
	mtimes map[string]int64
}

// writeTree writes the entries of one tree object into directory, whose path
// relative to the root is relative ("" for the root itself).
func (w treeWriter) writeTree(directory, relative, digest string) error {
	entries, err := w.corpus.ReadTree(digest)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		name := string(entry.Name)
		path := filepath.Join(directory, name)
		if relative != "" {
			name = relative + "/" + name
		}
		if err := w.writeEntry(path, name, entry); err != nil {
			return err
		}
	}
	return nil
}

// writeEntry creates one entry at path, then sets its time and mode.
func (w treeWriter) writeEntry(path, relative string, entry Entry) error {
	switch entry.Kind {
	case "dir":
		if err := osfile.MkdirAll(path, 0o700); err != nil {
			return err
		}
		if err := w.writeTree(path, relative, entry.Tree); err != nil {
			return err
		}
	case "file":
		var data []byte
		if entry.Blob != "" {
			blob, err := w.corpus.ReadBlob(entry.Blob, w.root)
			if err != nil {
				return err
			}
			data = blob
		}
		if err := osfile.WriteFile(path, data, 0o600); err != nil {
			return err
		}
	case "link":
		if err := os.Symlink(w.corpus.Expand(entry.Target, w.root), path); err != nil {
			return fmt.Errorf("materialise link: %w", err)
		}
		return nil
	default:
		w.t.Logf("not created: %s (%s)", path, entry.Kind)
		return nil
	}
	return w.setTimeAndMode(path, relative, entry.Mode)
}

// setTimeAndMode gives a created file or directory its recorded modification
// time, then its mode, last, so a mode that denies writing applies after.
func (w treeWriter) setTimeAndMode(path, relative string, mode uint32) error {
	if mtime, ok := w.mtimes[relative]; ok {
		modified := time.Unix(0, mtime)
		if err := osfile.Chtimes(path, modified, modified); err != nil {
			return err
		}
	}
	if err := os.Chmod(path, fs.FileMode(mode)&fs.ModePerm); err != nil {
		return fmt.Errorf("materialise mode: %w", err)
	}
	return nil
}
