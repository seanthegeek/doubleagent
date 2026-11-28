package archive

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/stage"
	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// buildStage creates a small staging tree and returns its path, and whether
// the two symbolic links in it were created: a Windows process without the
// symbolic-link privilege gets a tree without them.
func buildStage(t *testing.T) (stage string, linked bool) {
	t.Helper()
	stage = t.TempDir()
	for relative, content := range map[string]string{
		"fs/home/a/z.txt": "zed", "fs/home/a/B.txt": "bee", "fs/home/a/a.txt": "ay",
		"fs/home/a-b/f": "dash", "fs/home/a.d/f": "dot", "manifest.jsonl": "{}\n",
		"fs/home/a/sub/deep/f": "deep",
	} {
		full := filepath.Join(stage, relative)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	err := os.Symlink("/etc/hosts", filepath.Join(stage, "fs/home/a/link"))
	if symlinkNeedsPrivilege(err) {
		return stage, false
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("sub", filepath.Join(stage, "fs/home/a/dirlink")); err != nil {
		t.Fatal(err)
	}
	return stage, true
}

// systemTar is the tar program the listing tests check the archive with.
const systemTar = "/usr/bin/tar"

// requireSystemTar skips the test when the system tar is not installed (on
// Windows it is not at that path), the one reason a tar test may skip: once
// tar is there, a failed listing is a failure.
func requireSystemTar(t *testing.T) {
	t.Helper()
	if _, err := os.Stat(systemTar); errors.Is(err, fs.ErrNotExist) {
		t.Skipf("%s is not installed", systemTar)
	}
}

// tarListing returns what `tar -tzf` prints for archivePath, or an error
// holding tar's standard error when tar cannot list it.
func tarListing(archivePath string) ([]byte, error) {
	output, err := exec.Command(systemTar, "-tzf", archivePath).Output()
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		return output, fmt.Errorf("tar -tzf %s: %w: %s", archivePath, err, exitError.Stderr)
	}
	if err != nil {
		return output, fmt.Errorf("tar -tzf %s: %w", archivePath, err)
	}
	return output, nil
}

// readMembers returns the archive's headers and the contents of its files.
func readMembers(t *testing.T, archivePath string) ([]*tar.Header, map[string]string) {
	t.Helper()
	file, err := os.Open(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	unzipped, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	reader := tar.NewReader(unzipped)
	var headers []*tar.Header
	contents := map[string]string{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return headers, contents
		}
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		headers = append(headers, header)
		contents[header.Name] = string(body)
	}
}

// openTree opens dir as a staging tree for the test, closed when it ends.
func openTree(t *testing.T, dir string) *stage.Tree {
	t.Helper()
	tree, err := stage.OpenTree(syspath.FromNative(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tree.Close() })
	return tree
}

func TestWriteTarGzMembers(t *testing.T) {
	stage, linked := buildStage(t)
	archivePath := filepath.Join(t.TempDir(), "out.tar.gz")
	if _, err := WriteTarGz(context.Background(), openTree(t, stage), archivePath); err != nil {
		t.Fatal(err)
	}
	headers, contents := readMembers(t, archivePath)
	var names []string
	for _, header := range headers {
		names = append(names, header.Name)
	}
	want := []string{
		"./", "./fs/", "./fs/home/", "./fs/home/a/", "./fs/home/a/B.txt", "./fs/home/a/a.txt",
		"./fs/home/a/dirlink", "./fs/home/a/link", "./fs/home/a/sub/", "./fs/home/a/sub/deep/",
		"./fs/home/a/sub/deep/f", "./fs/home/a/z.txt", "./fs/home/a-b/", "./fs/home/a-b/f",
		"./fs/home/a.d/", "./fs/home/a.d/f", "./manifest.jsonl",
	}
	if !linked {
		want = slices.DeleteFunc(want, func(name string) bool {
			return strings.HasSuffix(name, "/link") || strings.HasSuffix(name, "/dirlink")
		})
	}
	if !slices.Equal(names, want) {
		t.Errorf("members:\n%s\nwant:\n%s", strings.Join(names, "\n"), strings.Join(want, "\n"))
	}
	if contents["./fs/home/a/z.txt"] != "zed" {
		t.Errorf("content = %q", contents["./fs/home/a/z.txt"])
	}
	for _, header := range headers {
		switch header.Name {
		case "./fs/home/a/link":
			if header.Typeflag != tar.TypeSymlink || filepath.ToSlash(header.Linkname) != "/etc/hosts" {
				t.Errorf("link header = %+v", header)
			}
		case "./fs/home/a/dirlink":
			if header.Typeflag != tar.TypeSymlink || header.Linkname != "sub" {
				t.Errorf("dirlink header = %+v", header)
			}
		}
	}
}

func TestWriteTarGzIsReadableByTar(t *testing.T) {
	requireSystemTar(t)
	stage, _ := buildStage(t)
	archivePath := filepath.Join(t.TempDir(), "out.tar.gz")
	if _, err := WriteTarGz(context.Background(), openTree(t, stage), archivePath); err != nil {
		t.Fatal(err)
	}
	output, err := tarListing(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(output), "./\n./fs/\n") {
		t.Errorf("tar -tzf = %q", output)
	}
}

func TestTarListingFailsOnABrokenArchive(t *testing.T) {
	requireSystemTar(t)
	stage, _ := buildStage(t)
	archivePath := filepath.Join(t.TempDir(), "out.tar.gz")
	if _, err := WriteTarGz(context.Background(), openTree(t, stage), archivePath); err != nil {
		t.Fatal(err)
	}
	whole, err := os.ReadFile(archivePath)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		content []byte
	}{
		{"a gzip stream cut in half is an error", whole[:len(whole)/2]},
		{"a file that is not gzip at all is an error", []byte("not an archive\n")},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			broken := filepath.Join(t.TempDir(), "broken.tar.gz")
			if err := os.WriteFile(broken, testCase.content, 0o644); err != nil {
				t.Fatal(err)
			}
			if output, err := tarListing(broken); err == nil {
				t.Errorf("tar listed a broken archive without an error: %q", output)
			}
		})
	}
}

func TestWriteTarGzFailureLeavesNoArchive(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "out.tar.gz")
	closed := openTree(t, t.TempDir())
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteTarGz(context.Background(), closed, archivePath); err == nil {
		t.Fatal("a closed stage gave no error")
	}
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Errorf("partial archive left: %v", err)
	}
}

func TestWriteTarGzCreatesItsFileNewAndNeverWritesThroughAnEntryAtItsName(t *testing.T) {
	stage, _ := buildStage(t)
	cases := []struct {
		name string
		// plant puts an entry at path, beside the file outside, which holds
		// "outside".
		plant func(t *testing.T, path, outside string)
	}{
		{"a link to a file outside", func(t *testing.T, path, outside string) {
			if err := os.Symlink(outside, path); err != nil {
				t.Skipf("cannot create a symlink: %v", err)
			}
		}},
		{"a link to a name nothing holds", func(t *testing.T, path, outside string) {
			if err := os.Symlink(outside+".new", path); err != nil {
				t.Skipf("cannot create a symlink: %v", err)
			}
		}},
		{"a regular file", func(t *testing.T, path, _ string) {
			if err := os.WriteFile(path, []byte("kept"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"an empty directory", func(t *testing.T, path, _ string) {
			if err := os.Mkdir(path, 0o700); err != nil {
				t.Fatal(err)
			}
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name+" fails the write and is left as it was", func(t *testing.T) {
			dir := t.TempDir()
			path, outside := filepath.Join(dir, "out.tar.gz"), filepath.Join(dir, "outside")
			if err := os.WriteFile(outside, []byte("outside"), 0o600); err != nil {
				t.Fatal(err)
			}
			testCase.plant(t, path, outside)
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			// Windows refuses to create a file at a directory's name with its
			// own error, where the others say the name is taken.
			_, err = WriteTarGz(context.Background(), openTree(t, stage), path)
			if err == nil || (!before.IsDir() && !errors.Is(err, fs.ErrExist)) {
				t.Errorf("WriteTarGz = %v, want the name taken", err)
			}
			after, err := os.Lstat(path)
			if err != nil || after.Mode() != before.Mode() || after.Size() != before.Size() {
				t.Errorf("the entry at the name became %v (%v), was %v", after, err, before)
			}
			if text, err := os.ReadFile(outside); err != nil || string(text) != "outside" {
				t.Errorf("the file outside holds %d bytes (%v), want it untouched", len(text), err)
			}
			if _, err := os.Lstat(outside + ".new"); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("a file was created through the link: %v", err)
			}
		})
	}
}

// stopAfterChecks is a context that is cancelled once its Err has been asked
// allowed times, so a test stops a write at a point it chooses, and that
// counts the questions asked after that.
type stopAfterChecks struct {
	context.Context
	allowed, refused int
}

// Err reports no error for the first allowed calls and context.Canceled after.
func (stop *stopAfterChecks) Err() error {
	if stop.allowed > 0 {
		stop.allowed--
		return nil
	}
	stop.refused++
	return context.Canceled
}

func TestAStopRequestEndsTheArchiveAtItsNextCheckAndLeavesNoFile(t *testing.T) {
	stage, _ := buildStage(t)
	cases := []struct {
		name    string
		allowed int
	}{
		{"a stop before the first member writes nothing", 0},
		{"a stop after a few members ends the write", 3},
		{"a stop while a file's bytes are read ends the write", 6},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			archivePath := filepath.Join(t.TempDir(), "out.tar.gz")
			stop := &stopAfterChecks{Context: context.Background(), allowed: testCase.allowed}
			if _, err := WriteTarGz(stop, openTree(t, stage), archivePath); !errors.Is(err, context.Canceled) {
				t.Fatalf("WriteTarGz = %v, want the stop request", err)
			}
			if stop.refused != 1 {
				t.Errorf("the write asked %d times after the stop, want it to end at the first", stop.refused)
			}
			if _, err := os.Lstat(archivePath); !errors.Is(err, fs.ErrNotExist) {
				t.Errorf("a stopped write left its archive: %v", err)
			}
		})
	}
}

func TestWriteTarGzHashesTheBytesItWrote(t *testing.T) {
	stage, _ := buildStage(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "run.tar.gz")
	written, err := WriteTarGz(context.Background(), openTree(t, stage), path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if written.SHA256 != hex.EncodeToString(sum[:]) || written.Size != int64(len(data)) {
		t.Errorf("Written = %s, %d; the file's are %x, %d", written.SHA256, written.Size, sum, len(data))
	}
}

func TestStillNamesKnowsOnlyTheFileWritten(t *testing.T) {
	stage, _ := buildStage(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "run.tar.gz")
	written, err := WriteTarGz(context.Background(), openTree(t, stage), path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := StillNames(path, written.File); err != nil {
		t.Errorf("StillNames of the file just written = %v, want nil", err)
	}
	replacements := []struct {
		name  string
		plant func(t *testing.T)
	}{
		{"a copy put at the name is not the file written", func(t *testing.T) {
			if err := os.WriteFile(path, data, 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"a link put at the name is not the file written", func(t *testing.T) {
			if err := os.Symlink(filepath.Join(dir, "moved"), path); err != nil {
				t.Skipf("cannot create a symlink: %v", err)
			}
		}},
		{"a name holding nothing is not the file written", func(*testing.T) {}},
	}
	if err := os.Rename(path, filepath.Join(dir, "moved")); err != nil {
		t.Fatal(err)
	}
	for _, replacement := range replacements {
		t.Run(replacement.name, func(t *testing.T) {
			replacement.plant(t)
			defer os.Remove(path)
			if err := StillNames(path, written.File); err == nil {
				t.Error("StillNames = nil, want an error")
			}
		})
	}
}

func TestWriteSidecarCreatesItsFileNew(t *testing.T) {
	dir := t.TempDir()
	path, outside := filepath.Join(dir, "run.tar.gz"), filepath.Join(dir, "outside")
	created, err := WriteSidecar(path, "abc")
	if sidecar, readErr := os.ReadFile(path + ".sha256"); err != nil || !created || readErr != nil ||
		string(sidecar) != "abc  run.tar.gz\n" {
		t.Fatalf("WriteSidecar = %v, %v; the sidecar holds %q (%v)", created, err, sidecar, readErr)
	}
	if created, err := WriteSidecar(path, "def"); err == nil || created {
		t.Errorf("a second WriteSidecar = %v, %v, want the name taken", created, err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "linked.tar.gz.sha256")); err != nil {
		t.Skipf("cannot create a symlink: %v", err)
	}
	if created, err := WriteSidecar(filepath.Join(dir, "linked.tar.gz"), "abc"); err == nil || created {
		t.Errorf("WriteSidecar through a link = %v, %v, want the name taken", created, err)
	}
	if _, err := os.Lstat(outside); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a file was created through the link: %v", err)
	}
}

func TestWriteTarGzLongNameUsesPAX(t *testing.T) {
	stage := t.TempDir()
	long := filepath.Join(stage, strings.Repeat("d", 90), strings.Repeat("e", 90), strings.Repeat("g", 90))
	if err := os.MkdirAll(long, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(long, "f"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	archivePath := filepath.Join(t.TempDir(), "out.tar.gz")
	if _, err := WriteTarGz(context.Background(), openTree(t, stage), archivePath); err != nil {
		t.Fatal(err)
	}
	headers, _ := readMembers(t, archivePath)
	last := headers[len(headers)-1]
	if last.Format != tar.FormatPAX || last.PAXRecords["path"] != last.Name {
		t.Errorf("long member header = %+v", last)
	}
}
