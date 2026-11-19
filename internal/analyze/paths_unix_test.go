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

func TestFleetComparesItsOutputByWhereTheFileSystemPutsIt(t *testing.T) {
	work := t.TempDir()
	input := filepath.Join(work, "in")
	writeFile(t, filepath.Join(input, "a.jsonl"), hostLine)
	writeFile(t, filepath.Join(input, "sub", "keep"), "")
	writeFile(t, filepath.Join(work, "elsewhere", "sub", "keep"), "")
	for link, target := range map[string]string{"lnk": filepath.Join(input, "sub"),
		"away": filepath.Join(work, "elsewhere", "sub")} {
		if err := os.Symlink(target, filepath.Join(work, link)); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name, output, written string
		refused               bool
	}{
		{"a link into the input and .. after it", spelt(work, "lnk", "..", "out.csv"),
			filepath.Join(input, "out.csv"), true},
		{"a new directory below a link into the input and ..", spelt(work, "lnk", "..", "new", "out.csv"),
			filepath.Join(input, "new"), true},
		{"a link out of the input and .. after it", spelt(work, "away", "..", "out.csv"),
			filepath.Join(work, "elsewhere", "out.csv"), false},
		{"a new directory below a link out of the input and ..", spelt(work, "away", "..", "new", "out.csv"),
			filepath.Join(work, "elsewhere", "new", "out.csv"), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, _, stderr := runCommand(testEnvironment(), "fleet", "-o", test.output, input)
			refused := code == ExitError && strings.Contains(stderr, "must be outside the input directory")
			if refused != test.refused || !refused && code != ExitFound {
				t.Errorf("exit %d, stderr %q", code, stderr)
			}
			if lstatExists(test.written) == test.refused {
				t.Errorf("%s exists: %v", test.written, lstatExists(test.written))
			}
		})
	}
}

func TestTimelineJudgesItsOutputAfterCleaningItsText(t *testing.T) {
	// timeline writes its output at the cleaned text, so a .. after a link
	// cancels the link's name, and the check judges that same place.
	work := t.TempDir()
	input := filepath.Join(work, "in")
	writeFile(t, filepath.Join(input, "a.jsonl"), hostLine)
	writeFile(t, filepath.Join(work, "elsewhere", "keep"), "")
	writeFile(t, filepath.Join(input, "sub", "keep"), "")
	for link, target := range map[string]string{filepath.Join(work, "lnk"): filepath.Join(input, "sub"),
		filepath.Join(input, "away"): filepath.Join(work, "elsewhere")} {
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
	}
	tests := []struct {
		name, output, written string
		refused               bool
	}{
		{"a link into the input and .. after it", spelt(work, "lnk", "..", "out1"),
			filepath.Join(work, "out1"), false},
		{"a link out of the input and .. after it", spelt(input, "away", "..", "out2"),
			filepath.Join(input, "out2"), true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, _, stderr := runCommand(testEnvironment(), "timeline", input, test.output)
			refused := code == ExitError && strings.Contains(stderr, "must be outside the input")
			if refused != test.refused || !refused && code == ExitError {
				t.Errorf("exit %d, stderr %q", code, stderr)
			}
			if lstatExists(test.written) == test.refused {
				t.Errorf("%s exists: %v", test.written, lstatExists(test.written))
			}
		})
	}
}

func TestAnOutputPastALinkToNothingIsRefusedAsUntold(t *testing.T) {
	work := t.TempDir()
	input := filepath.Join(work, "in")
	writeFile(t, filepath.Join(input, "a.jsonl"), hostLine)
	dangling := filepath.Join(work, "dangling")
	if err := os.Symlink(filepath.Join(work, "nothing"), dangling); err != nil {
		t.Fatal(err)
	}
	for name, test := range map[string]struct {
		args    []string
		message string
	}{
		"timeline": {[]string{"timeline", input, filepath.Join(dangling, "out")},
			"cannot tell whether the output directory " + filepath.Join(dangling, "out") +
				" is outside the input: "},
		"fleet": {[]string{"fleet", "-o", filepath.Join(dangling, "f.csv"), input},
			"cannot tell whether the output " + filepath.Join(dangling, "f.csv") +
				" is outside the input directory " + input + ": "},
	} {
		t.Run(name, func(t *testing.T) {
			code, _, stderr := runCommand(testEnvironment(), test.args...)
			if code != ExitError || !strings.Contains(stderr, test.message) ||
				strings.Contains(stderr, "must be outside") {
				t.Errorf("exit %d, stderr %q", code, stderr)
			}
		})
	}
	if lstatExists(filepath.Join(work, "nothing")) {
		t.Error("the link's target was made")
	}
	// An input through a link to nothing is the read's error to give.
	code, _, stderr := runCommand(testEnvironment(), "timeline", dangling, filepath.Join(work, "out"))
	if code != ExitError || strings.Contains(stderr, "cannot tell") {
		t.Errorf("an input through a link to nothing: exit %d, stderr %q", code, stderr)
	}
}

func TestTheParentOfAnOutputIsItsTextUpToTheLastSlash(t *testing.T) {
	for _, test := range []struct{ name, path, want string }{
		{"a .. after a link is kept", "/w/lnk/../out.csv", "/w/lnk/.."},
		{"a trailing slash is passed over", "/w/dir/", "/w"},
		{"doubled slashes are one separator", "/w//out.csv", "/w"},
		{"a file at the root", "/out.csv", "/"},
		{"the root", "/", "/"},
		{"a file at the root after doubled slashes", "//out.csv", "/"},
		{"a name of one element", "out.csv", ""},
		{"a relative path", "a/out.csv", "a"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := outputParent(test.path); got != test.want {
				t.Errorf("outputParent(%q) = %q, want %q", test.path, got, test.want)
			}
		})
	}
}

// A file name may hold a terminal escape character on Unix; Windows
// refuses to create one, so this runs on Unix only.
func TestAFleetRefusalShowsAListedNameVisibly(t *testing.T) {
	work := t.TempDir()
	directory := filepath.Join(work, "inv")
	listed := filepath.Join(directory, "a\x1b[2Jb.jsonl")
	writeFile(t, listed, hostLine)
	code, _, stderr := runCommand(testEnvironment(), "fleet", "-o", listed, directory)
	if code != ExitError || strings.ContainsRune(stderr, '\x1b') || !strings.Contains(stderr, `a\x1b[2Jb.jsonl`) {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
}
