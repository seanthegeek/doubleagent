package analyze

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// goldenFixtures is the directory of the committed archives the golden
// runs read, kept by internal/input for its own tests.
const goldenFixtures = "../input/testdata/fixtures"

// goldenInputFile returns the name, under the shared working directory, of
// the input a golden case reads.
func goldenInputFile(input string) string {
	if input == "example" {
		return "example.tar.gz"
	}
	return input
}

// writeTarMember writes the current member of a tar at target.
func writeTarMember(reader io.Reader, target string) error {
	if err := osfile.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	out, err := osfile.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, reader); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// unpackWithTimes writes the directories and regular files of a gzip tar
// under directory with the modification times the archive records: a
// parser that stamps a row with its file's time then gives the same row on
// every run.
func unpackWithTimes(archive, directory string) error {
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
			err = osfile.MkdirAll(target, 0o755)
		case tar.TypeReg:
			err = writeTarMember(reader, target)
		default:
			err = errors.New(header.Name + ": not a file or directory")
		}
		if err == nil {
			err = osfile.Chtimes(target, header.ModTime, header.ModTime)
		}
		if err != nil {
			return err
		}
	}
}

// buildGoldenInputs prepares the inputs of the golden runs once, in a
// directory the runs share and only read: the example collection copied as
// the archive it is, the fixture disk image and the fixture home unpacked
// as directories, and the saved inventory outputs fleet merges.
func buildGoldenInputs(t *testing.T) string {
	t.Helper()
	work := t.TempDir()
	data, err := osfile.ReadFile(filepath.Join(goldenFixtures, "example.tar.gz"))
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(work, "example.tar.gz"), string(data))
	for _, name := range []string{"image", "loose"} {
		if err := unpackWithTimes(filepath.Join(goldenFixtures, name+".tar.gz"),
			filepath.Join(work, name)); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := osfile.ReadDir("testdata/golden-inputs/fleet")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		text, err := osfile.ReadFile(filepath.Join("testdata/golden-inputs/fleet", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, filepath.Join(work, "fleet", entry.Name()), string(text))
	}
	return work
}
