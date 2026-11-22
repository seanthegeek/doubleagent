package input

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// dumpField returns text's bytes with every byte outside printable ASCII,
// and the backslash, as \xHH, so a dump shows exactly the bytes a field
// holds.
func dumpField(text string) string {
	var out strings.Builder
	for i := 0; i < len(text); i++ {
		if character := text[i]; character >= 0x20 && character <= 0x7e && character != '\\' {
			out.WriteByte(character)
		} else {
			fmt.Fprintf(&out, `\x%02x`, character)
		}
	}
	return out.String()
}

// dumper writes a collection as text, a path under the input's directory
// starting <input> and one under the extraction directory starting
// <extract>, both with slashes, so a dump is the same on every run and
// platform.
type dumper struct {
	roots     []string
	extracted string
}

// newDumper returns a dumper for a collection opened from an input under
// root.
func newDumper(root string, collection *Collection) dumper {
	roots := []string{root}
	if resolved, err := filepath.EvalSymlinks(root); err == nil && resolved != root {
		roots = append(roots, resolved)
	}
	return dumper{roots: roots, extracted: collection.ExtractedDir()}
}

// path returns a disk path as the dump prints it.
func (d dumper) path(text string) string {
	under := func(base, label string) (string, bool) {
		if base == "" {
			return "", false
		}
		if text == base {
			return label, true
		}
		if rest, ok := strings.CutPrefix(text, base+string(filepath.Separator)); ok {
			rest = filepath.ToSlash(rest)
			// An archive path naming an absolute path is joined below
			// the collection, so the test directory can appear again:
			// without its leading slash, or on Windows as C:Users/...
			for _, root := range d.roots {
				slashed := strings.TrimPrefix(filepath.ToSlash(root), "/")
				rest = strings.ReplaceAll(rest, slashed, "<input>")
				rest = strings.ReplaceAll(rest, strings.Replace(slashed, ":/", ":", 1), "<input>")
			}
			return label + "/" + rest, true
		}
		return "", false
	}
	if found, ok := under(d.extracted, "<extract>"); ok {
		return dumpField(found)
	}
	for _, root := range d.roots {
		if found, ok := under(root, "<input>"); ok {
			return dumpField(found)
		}
	}
	return dumpField(text)
}

// text returns a note or problem as the dump prints it, a path below the
// input's directory starting <input>/ on every platform.
func (d dumper) text(text string) string {
	for _, root := range d.roots {
		text = strings.ReplaceAll(text, root+string(filepath.Separator), "<input>/")
		text = strings.ReplaceAll(text, root, "<input>")
	}
	return dumpField(text)
}

// dump returns the collection's kind, host, notes, problems, homes with
// their staged paths, and artifacts, one line each, fields separated by
// tabs.
func (d dumper) dump(collection *Collection) string {
	lines := []string{"kind\t" + collection.Kind, "host\t" + dumpField(collection.Host)}
	for _, note := range collection.Notes {
		lines = append(lines, "note\t"+d.text(note))
	}
	for _, problem := range collection.Problems {
		lines = append(lines, "problem\t"+d.text(problem))
	}
	index := map[*Home]int{}
	for i, home := range collection.Homes {
		index[home] = i
		lines = append(lines, strings.Join([]string{"home", strconv.Itoa(i), d.path(home.DiskPath),
			dumpField(home.Original), dumpField(home.User), dumpField(home.Host),
			strconv.FormatBool(home.Inferred), strconv.FormatBool(home.Project)}, "\t"))
		for _, rel := range slices.Sorted(func(yield func(string) bool) {
			for rel := range home.StagedPaths {
				if !yield(rel) {
					return
				}
			}
		}) {
			lines = append(lines, strings.Join([]string{"staged", strconv.Itoa(i), dumpField(rel),
				d.path(home.StagedPaths[rel])}, "\t"))
		}
	}
	for _, artifact := range collection.Artifacts {
		manifest := "-"
		if artifact.Manifest.IsObject() {
			manifest = dumpField(artifact.Manifest.JSON())
		}
		lines = append(lines, strings.Join([]string{"artifact", strconv.Itoa(index[artifact.Home]),
			dumpField(artifact.Agent), dumpField(artifact.Rel), dumpField(artifact.Original),
			d.path(artifact.DiskPath), manifest}, "\t"))
	}
	return strings.Join(lines, "\n") + "\n"
}

// openAndDump opens input, found under root, and returns its dump, or
// "error" when it does not open.
func openAndDump(t *testing.T, root, input, catalogText, host string) string {
	t.Helper()
	collection, err := Open(t.Context(), input, Options{Matcher: matcherOf(t, catalogText), Host: host})
	if err != nil {
		return "error\n"
	}
	defer func() {
		if err := collection.Cleanup(); err != nil {
			t.Error(err)
		}
	}()
	return newDumper(root, collection).dump(collection)
}

// loadDumps reads a file of testdata holding dumps, each after a line
// "=== <name>".
func loadDumps(t *testing.T, name string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	dumps := map[string]string{}
	for _, section := range strings.Split(string(data), "=== ")[1:] {
		title, body, _ := strings.Cut(section, "\n")
		dumps[title] = body
	}
	return dumps
}

func TestOpenGivesTheRecordedDumpOfEachCase(t *testing.T) {
	dumps := loadDumps(t, "case_dumps.txt")
	for _, test := range allInputCases() {
		t.Run(test.name, func(t *testing.T) {
			want, ok := dumps[test.name]
			if !ok {
				t.Fatal("no dump is recorded for this case")
			}
			root := t.TempDir()
			input := test.build(t, root)
			if got := openAndDump(t, root, input, test.catalog, test.host); got != want {
				t.Errorf("dump:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}

// fixtureInputs are the inputs under testdata/fixtures, by name: the
// example collection as a directory (unpacked from its gzip tar), a gzip
// tar, a bzip2 tar and a zip; the fixture home as a tree of homes, as one
// home, as its agent directories and as a gzip tar; and the fixture disk
// image. Each path is relative to the directory the fixtures are unpacked
// in, and image.tar.gz holds the disk image directory. The archives are
// fixed fixtures: no script in the tree builds them again.
func fixtureInputs() [][2]string {
	inputs := [][2]string{
		{"example collection", "example/wsdev14"},
		{"example gzip tar", "example.tar.gz"},
		{"example bzip2 tar", "example.tar.bz2"},
		{"example zip", "example.zip"},
		{"fixture home tree", "loose/home"},
		{"fixture home", "loose/home/alice"},
		{"fixture home gzip tar", "loose.tar.gz"},
		{"fixture image", "image"},
	}
	for _, name := range []string{".claude", ".gemini", ".codex", ".copilot", ".config"} {
		inputs = append(inputs, [2]string{"fixture agent directory " + name,
			"loose/home/alice/" + name})
	}
	return inputs
}

// unpackFixtures copies the archives of testdata/fixtures under root and
// unpacks the three gzip tars that are also opened as directories.
func unpackFixtures(t *testing.T, root string) {
	t.Helper()
	for _, name := range []string{"example.tar.gz", "example.tar.bz2", "example.zip",
		"loose.tar.gz"} {
		data, err := os.ReadFile(filepath.Join("testdata", "fixtures", name))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(root, name), string(data))
	}
	for archive, directory := range map[string]string{"example.tar.gz": "example/wsdev14",
		"loose.tar.gz": "loose", "image.tar.gz": "image"} {
		if err := unpackTar(filepath.Join("testdata", "fixtures", archive),
			filepath.Join(root, directory)); err != nil {
			t.Fatal(err)
		}
	}
}

// unpackTar writes the directories and regular files of a gzip tar under
// directory.
func unpackTar(archive, directory string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer file.Close()
	compressed, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	reader := tar.NewReader(compressed)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(directory, filepath.FromSlash(header.Name))
		switch header.Typeflag {
		case tar.TypeDir:
			err = os.MkdirAll(target, 0o755)
		case tar.TypeReg:
			err = writeMember(reader, target)
		default:
			err = errors.New(header.Name + ": not a file or directory")
		}
		if err != nil {
			return err
		}
	}
}

// writeMember writes the current member of a tar at target.
func writeMember(reader io.Reader, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, reader); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

func TestOpenGivesTheRecordedDumpOfTheFixtureInputs(t *testing.T) {
	dumps := loadDumps(t, "fixture_dumps.txt")
	root := t.TempDir()
	unpackFixtures(t, root)
	for _, input := range fixtureInputs() {
		t.Run(input[0], func(t *testing.T) {
			got := openAndDump(t, root, filepath.Join(root, filepath.FromSlash(input[1])), "", "")
			if want := dumps[input[0]]; got != want {
				t.Errorf("dump:\n%s\nwant:\n%s", got, want)
			}
		})
	}
}
