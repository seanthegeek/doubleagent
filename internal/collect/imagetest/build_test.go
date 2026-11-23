package imagetest

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// describe returns what the comparison of two images looks at in an entry:
// its kind, target, content, mode and, when withTime is set, its time.
func describe(entry Entry, withTime bool) string {
	text := fmt.Sprintf("%s %q from-work=%v mode=%04o", entry.Kind, entry.Text, entry.FromWork, uint32(entry.Mode))
	if withTime {
		text += " mtime=" + entry.ModTime.UTC().Format(time.RFC3339Nano)
	}
	return text
}

// differences returns a line for each entry that want and got do not agree
// on, entries matched by path.
func differences(want, got []Entry, withTime bool) []string {
	wanted := map[string]Entry{}
	for _, entry := range want {
		wanted[entry.Path] = entry
	}
	var lines []string
	for _, entry := range got {
		expected, found := wanted[entry.Path]
		delete(wanted, entry.Path)
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
	for _, line := range differences(want, got, true) {
		t.Error(line)
	}
	if img.Root != filepath.Join(img.Work, "root") || !strings.HasPrefix(img.Alice, img.Root) ||
		!strings.HasPrefix(img.Bob, img.Root) {
		t.Errorf("image paths = %+v, want the root and both homes under the work directory", img)
	}
}
