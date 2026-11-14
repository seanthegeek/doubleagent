package archive

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteTarGzKeepsNamesEndingInADotOrASpace(t *testing.T) {
	stage := t.TempDir()
	// Win32 trims a trailing dot or space unless the path has the \\?\
	// prefix, so the staged tree is written through that form.
	contents := map[string]string{"fs/x./a": "dot", "fs/x/b": "plain", "fs/y ": "space", "fs/y": "y"}
	for relative, content := range contents {
		full := `\\?\` + filepath.Join(stage, relative)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	archivePath := filepath.Join(t.TempDir(), "out.tar.gz")
	if err := WriteTarGz(stage, archivePath); err != nil {
		t.Fatal(err)
	}
	headers, members := readMembers(t, archivePath)
	var names []string
	for _, header := range headers {
		names = append(names, header.Name)
	}
	for relative, content := range contents {
		if got, ok := members["./"+relative]; !ok || got != content {
			t.Errorf("member ./%s = %q (present %v), want %q; members %q", relative, got, ok,
				content, strings.Join(names, " "))
		}
	}
}
