package input

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/seanthegeek/doubleagent/internal/catalog"
	"github.com/seanthegeek/doubleagent/internal/pyjson"
)

// writeFile writes text at path, making its directories.
func writeFile(t testing.TB, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeTree writes each rel of files under root with its text.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, text := range files {
		writeFile(t, filepath.Join(root, filepath.FromSlash(rel)), text)
	}
}

// jsonRow returns a compact JSON object of key and value pairs; a value
// is a string, an int or a bool.
func jsonRow(pairs ...any) string {
	var members []pyjson.Member
	for i := 0; i+1 < len(pairs); i += 2 {
		var value pyjson.Value
		switch typed := pairs[i+1].(type) {
		case string:
			value = pyjson.NewString(typed)
		case int:
			value = pyjson.NewInt(int64(typed))
		case bool:
			value = pyjson.NewBool(typed)
		case pyjson.Value:
			value = typed
		}
		members = append(members, pyjson.Member{Key: pairs[i].(string), Value: value})
	}
	return pyjson.Compact(pyjson.NewObject(members...))
}

// collectedRow returns the manifest row of a collected file.
func collectedRow(user, home, agent, path, archivePath string) string {
	return jsonRow("user", user, "home", home, "agent", agent, "path", path,
		"archive_path", archivePath, "type", "file", "status", "collected", "size", 2)
}

// writeManifest writes rows, one per line, as root's manifest.jsonl, and
// makes fs/.
func writeManifest(t *testing.T, root string, rows ...string) {
	t.Helper()
	writeFile(t, filepath.Join(root, "manifest.jsonl"), strings.Join(rows, "\n")+"\n")
	if err := os.MkdirAll(filepath.Join(root, "fs"), 0o777); err != nil {
		t.Fatal(err)
	}
}

// tarMember is one member of a test archive.
type tarMember struct {
	name     string
	typeflag byte
	body     string
	linkname string
}

// tarBytes returns a tar archive of members, gzip compressed when
// compressed is set.
func tarBytes(t testing.TB, compressed bool, members ...tarMember) []byte {
	t.Helper()
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	for _, member := range members {
		header := &tar.Header{Name: member.name, Typeflag: member.typeflag, Linkname: member.linkname,
			Mode: 0o644, Size: int64(len(member.body)), ModTime: time.Unix(1700000000, 0),
			Format: tar.FormatPAX}
		if member.typeflag != tar.TypeReg {
			header.Size = 0
		}
		if member.typeflag == tar.TypeDir {
			header.Mode = 0o755
		}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(member.body)[:header.Size]); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if !compressed {
		return archive.Bytes()
	}
	var zipped bytes.Buffer
	gzipWriter := gzip.NewWriter(&zipped)
	if _, err := gzipWriter.Write(archive.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	return zipped.Bytes()
}

// zipBytes returns a zip archive holding each name of names with its body;
// a name ending in a slash is a directory entry.
func zipBytes(t testing.TB, names []string, bodies map[string]string) []byte {
	t.Helper()
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for _, name := range names {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(bodies[name])); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return archive.Bytes()
}

// treeMembers returns tar members for every directory and file under root,
// in sorted walk order, named relative to root.
func treeMembers(t *testing.T, root string) []tarMember {
	t.Helper()
	var members []tarMember
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || path == root {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		if entry.IsDir() {
			members = append(members, tarMember{name: rel + "/", typeflag: tar.TypeDir})
			return nil
		}
		data, err := os.ReadFile(path)
		members = append(members, tarMember{name: rel, typeflag: tar.TypeReg, body: string(data)})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return members
}

// matcherOf compiles catalog text, or the bundled catalog when text is "".
func matcherOf(t *testing.T, text string) *catalog.Matcher {
	t.Helper()
	tables, err := catalog.Load()
	if text != "" {
		tables, err = catalog.Parse([]byte(text))
	}
	if err != nil {
		t.Fatal(err)
	}
	matcher, err := catalog.NewMatcher(tables)
	if err != nil {
		t.Fatal(err)
	}
	return matcher
}
