package imagetest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// foldsCase reports whether directory takes two names differing only in
// letter case for one, by making a lower-case probe and looking it up in
// upper case.
func foldsCase(t *testing.T, directory string) bool {
	t.Helper()
	probe := filepath.Join(directory, "case-probe")
	if err := os.WriteFile(probe, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(probe)
	_, err := os.Lstat(filepath.Join(directory, "CASE-PROBE"))
	return err == nil
}

// entryKey returns what two entries are matched by: the path, with ASCII
// letters folded when the file system folds case, since there a pair of
// names differing only in case is one entry spelled as it was first made.
func entryKey(path string, folds bool) string {
	if !folds {
		return path
	}
	return strings.Map(func(character rune) rune {
		if character >= 'A' && character <= 'Z' {
			return character + 'a' - 'A'
		}
		return character
	}, path)
}

// describe returns what the comparison of two images looks at in an entry:
// its kind, target, content, mode and, when withTime is set, its time.
func describe(entry Entry, withTime bool) string {
	text := fmt.Sprintf("%s %q from-work=%v mode=%04o", entry.Kind, entry.Text, entry.FromWork, uint32(entry.Mode))
	if withTime {
		text += " mtime=" + entry.ModTime.UTC().Format(time.RFC3339Nano)
	}
	return text
}

// Differences returns a line for each entry that want and got do not agree
// on, entries matched by path, folded as entryKey folds it.
func differences(want, got []Entry, folds, withTime bool) []string {
	wanted := map[string]Entry{}
	for _, entry := range want {
		wanted[entryKey(entry.Path, folds)] = entry
	}
	var lines []string
	for _, entry := range got {
		key := entryKey(entry.Path, folds)
		expected, found := wanted[key]
		delete(wanted, key)
		switch {
		case !found:
			lines = append(lines, "unexpected "+Format(entry))
		case describe(expected, withTime) != describe(entry, withTime):
			lines = append(lines, fmt.Sprintf("%q: want %s, got %s", entry.Path,
				describe(expected, withTime), describe(entry, withTime)))
		}
	}
	for _, entry := range wanted {
		lines = append(lines, "missing "+Format(entry))
	}
	return lines
}

func TestBuildMakesWhatTheManifestSays(t *testing.T) {
	want, err := Entries()
	if err != nil {
		t.Fatal(err)
	}
	img := Build(t, t.TempDir())
	got, err := Read(img.Work)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range differences(want, got, foldsCase(t, img.Work), true) {
		t.Error(line)
	}
	if img.Root != filepath.Join(img.Work, "root") || !strings.HasPrefix(img.Alice, img.Root) ||
		!strings.HasPrefix(img.Bob, img.Root) {
		t.Errorf("image paths = %+v, want the root and both homes under the work directory", img)
	}
}
