//go:build unix

package analyze

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTimelineRefusesAnOutputUnderALinkToItsInput(t *testing.T) {
	work := t.TempDir()
	realDir := filepath.Join(work, "realin")
	writeFile(t, filepath.Join(realDir, "keep"), "x")
	link := filepath.Join(work, "linkin")
	if err := os.Symlink(realDir, link); err != nil {
		t.Fatal(err)
	}
	for name, args := range map[string][]string{
		"the input given through the link":  {"timeline", link, filepath.Join(link, "out3")},
		"the output given through the link": {"timeline", realDir, filepath.Join(link, "out3")},
		"the input through the link, the output by its real name": {"timeline", link,
			filepath.Join(realDir, "out3")},
	} {
		t.Run(name, func(t *testing.T) {
			code, _, stderr := runCommand(testEnvironment(), args...)
			if code != ExitError || !strings.Contains(stderr, "must be outside the input") {
				t.Errorf("exit %d, stderr %q", code, stderr)
			}
			if lstatExists(filepath.Join(realDir, "out3")) {
				t.Error("the output was made inside the input")
			}
		})
	}
}

func TestAnOutputFileIsNeverWrittenThroughALink(t *testing.T) {
	work := t.TempDir()
	victim := filepath.Join(work, "victim.txt")
	writeFile(t, victim, "kept")
	planted := filepath.Join(work, "out", TimelineFile)
	writeFile(t, filepath.Join(work, "out", "x"), "")
	if err := os.Remove(filepath.Join(work, "out", "x")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(victim, planted); err != nil {
		t.Fatal(err)
	}
	write := func(w io.Writer) error {
		_, err := io.WriteString(w, "rows")
		return err
	}
	if err := writeOutput(planted, write); err == nil {
		t.Error("an output file was written through a link")
	}
	if data, _ := os.ReadFile(victim); string(data) != "kept" {
		t.Errorf("the link's target now holds %q", data)
	}
	input := filepath.Join(work, "inv.txt")
	writeFile(t, input, hostLine)
	csvLink := filepath.Join(work, "f.csv")
	if err := os.Symlink(victim, csvLink); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runCommand(testEnvironment(), "fleet", "-o", csvLink, input)
	if code != ExitError || !strings.Contains(stderr, "is a symbolic link") {
		t.Errorf("fleet: exit %d, stderr %q", code, stderr)
	}
	if data, _ := os.ReadFile(victim); string(data) != "kept" {
		t.Errorf("fleet wrote through the link: %q", data)
	}
}
