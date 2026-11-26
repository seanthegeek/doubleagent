package input

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenTarStreamReadsAPlainTarWhoseFirstNameLooksLikeBzip2(t *testing.T) {
	t.Parallel()
	data := tarBytes(t, false, tarMember{name: "BZhalice/notes.txt", typeflag: '0', body: "x"})
	if !bytes.HasPrefix(data, []byte("BZh")) {
		t.Fatalf("the archive starts %q, want BZh", data[:3])
	}
	path := filepath.Join(t.TempDir(), "lookalike.tar")
	writeFile(t, path, string(data))
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	_, header, err := openTarStream(file, path)
	if err != nil || header == nil || header.Name != "BZhalice/notes.txt" {
		t.Fatalf("openTarStream = %v, %v, want the header of BZhalice/notes.txt", header, err)
	}
}

func TestOpenTarStreamNamesTheCompressionItRefuses(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, data, want string
	}{
		{"xz data is refused as xz", "\xfd7zXZ\x00 and more", ": cannot extract: xz-compressed tar archives are not supported"},
		{"zstd data is refused as zstd", "\x28\xb5\x2f\xfd and more", ": cannot extract: zstd-compressed tar archives are not supported"},
		{"gzip magic with a broken header is no archive", "\x1f\x8b and more", " is neither a tar nor a zip archive"},
		{"bzip2 magic with broken data is no archive", "BZh and more", " is neither a tar nor a zip archive"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "archive")
			writeFile(t, path, test.data)
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			_, _, err = openTarStream(file, path)
			if err == nil || err.Error() != path+test.want {
				t.Fatalf("openTarStream error %v, want %q", err, path+test.want)
			}
		})
	}
}
