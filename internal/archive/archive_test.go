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

func TestWriteTarGzMembers(t *testing.T) {
	stage, linked := buildStage(t)
	archivePath := filepath.Join(t.TempDir(), "out.tar.gz")
	if err := WriteTarGz(context.Background(), stage, archivePath); err != nil {
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
	if err := WriteTarGz(context.Background(), stage, archivePath); err != nil {
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
	if err := WriteTarGz(context.Background(), stage, archivePath); err != nil {
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
	if err := WriteTarGz(context.Background(), filepath.Join(t.TempDir(), "missing"), archivePath); err == nil {
		t.Fatal("a missing stage gave no error")
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
			if err := WriteTarGz(context.Background(), stage, path); !errors.Is(err, fs.ErrExist) {
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
			if err := WriteTarGz(stop, stage, archivePath); !errors.Is(err, context.Canceled) {
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

func TestAStopRequestEndsTheHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.tar.gz")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	stop := &stopAfterChecks{Context: context.Background()}
	if _, _, err := SHA256File(stop, path); !errors.Is(err, context.Canceled) {
		t.Errorf("SHA256File = %v, want the stop request", err)
	}
}

func TestSHA256FileAndSidecar(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.tar.gz")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("content"))
	want := hex.EncodeToString(sum[:])
	got, size, err := SHA256File(context.Background(), path)
	if err != nil || got != want || size != 7 {
		t.Fatalf("SHA256File = %q, %d, %v", got, size, err)
	}
	if err := WriteSidecar(path, got); err != nil {
		t.Fatal(err)
	}
	sidecar, err := os.ReadFile(path + ".sha256")
	if err != nil || string(sidecar) != want+"  run.tar.gz\n" {
		t.Errorf("sidecar = %q, %v", sidecar, err)
	}
}

func TestSHA256FileMissing(t *testing.T) {
	if _, _, err := SHA256File(context.Background(), filepath.Join(t.TempDir(), "none")); err == nil {
		t.Error("a missing file gave no error")
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
	if err := WriteTarGz(context.Background(), stage, archivePath); err != nil {
		t.Fatal(err)
	}
	headers, _ := readMembers(t, archivePath)
	last := headers[len(headers)-1]
	if last.Format != tar.FormatPAX || last.PAXRecords["path"] != last.Name {
		t.Errorf("long member header = %+v", last)
	}
}
