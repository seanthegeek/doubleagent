package archive

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// buildStage creates a small staging tree and returns its path.
func buildStage(t *testing.T) string {
	t.Helper()
	stage := t.TempDir()
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
	if err := os.Symlink("/etc/hosts", filepath.Join(stage, "fs/home/a/link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("sub", filepath.Join(stage, "fs/home/a/dirlink")); err != nil {
		t.Fatal(err)
	}
	return stage
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
	stage := buildStage(t)
	archivePath := filepath.Join(t.TempDir(), "out.tar.gz")
	if err := WriteTarGz(stage, archivePath); err != nil {
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
	if !slices.Equal(names, want) {
		t.Errorf("members:\n%s\nwant:\n%s", strings.Join(names, "\n"), strings.Join(want, "\n"))
	}
	if contents["./fs/home/a/z.txt"] != "zed" {
		t.Errorf("content = %q", contents["./fs/home/a/z.txt"])
	}
	for _, header := range headers {
		switch header.Name {
		case "./fs/home/a/link":
			if header.Typeflag != tar.TypeSymlink || header.Linkname != "/etc/hosts" {
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
	stage := buildStage(t)
	archivePath := filepath.Join(t.TempDir(), "out.tar.gz")
	if err := WriteTarGz(stage, archivePath); err != nil {
		t.Fatal(err)
	}
	output, err := exec.Command("/usr/bin/tar", "-tzf", archivePath).CombinedOutput()
	if err != nil {
		t.Skipf("system tar unavailable: %v", err)
	}
	if !strings.HasPrefix(string(output), "./\n./fs/\n") {
		t.Errorf("tar -tzf = %q", output)
	}
}

func TestWriteTarGzFailureLeavesNoArchive(t *testing.T) {
	archivePath := filepath.Join(t.TempDir(), "out.tar.gz")
	if err := WriteTarGz(filepath.Join(t.TempDir(), "missing"), archivePath); err == nil {
		t.Fatal("a missing stage gave no error")
	}
	if _, err := os.Stat(archivePath); !os.IsNotExist(err) {
		t.Errorf("partial archive left: %v", err)
	}
}

func TestSHA256FileAndSidecar(t *testing.T) {
	path := filepath.Join(t.TempDir(), "run.tar.gz")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte("content"))
	want := hex.EncodeToString(sum[:])
	got, size, err := SHA256File(path)
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
	if _, _, err := SHA256File(filepath.Join(t.TempDir(), "none")); err == nil {
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
	if err := WriteTarGz(stage, archivePath); err != nil {
		t.Fatal(err)
	}
	headers, _ := readMembers(t, archivePath)
	last := headers[len(headers)-1]
	if last.Format != tar.FormatPAX || last.PAXRecords["path"] != last.Name {
		t.Errorf("long member header = %+v", last)
	}
}
