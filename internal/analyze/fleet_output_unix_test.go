//go:build unix

package analyze

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/reason"
)

// evidenceText is the content of a file below an input directory, which
// fleet must leave as it was.
const evidenceText = "evidence that must not change\n"

func TestFleetReplacesAnOutputThatIsAHardLinkToAFileBelowTheInput(t *testing.T) {
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "inv", "a.json"), hostLine)
	evidence := filepath.Join(work, "inv", "sub", "evidence.txt")
	writeFile(t, evidence, evidenceText)
	output := filepath.Join(work, "h2.csv")
	if err := os.Link(evidence, output); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runCommand(testEnvironment(), "fleet", "-o", output, filepath.Join(work, "inv"))
	if code != ExitFound {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if data, err := osfile.ReadFile(evidence); err != nil || string(data) != evidenceText {
		t.Errorf("the evidence file now holds %q (%v)", data, err)
	}
	if data, err := osfile.ReadFile(output); err != nil ||
		!strings.HasPrefix(string(data), strings.Join(FleetColumns(), ",")+"\n") {
		t.Errorf("the output holds %q (%v)", data, err)
	}
	info, err := osfile.Lstat(evidence)
	if err != nil {
		t.Fatal(err)
	}
	if links := info.Sys().(*syscall.Stat_t).Nlink; links != 1 {
		t.Errorf("the evidence file has %d names, want 1", links)
	}
}

func TestFleetRefusesAnOutputThatIsASymbolicLink(t *testing.T) {
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "inv", "a.json"), hostLine)
	inside := filepath.Join(work, "inv", "sub", "evidence.txt")
	outside := filepath.Join(work, "elsewhere", "target.txt")
	writeFile(t, inside, evidenceText)
	writeFile(t, outside, evidenceText)
	tests := []struct{ name, link, target string }{
		{"a link to a file below the input", "inside.csv", inside},
		{"a link to a file outside the input", "outside.csv", outside},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			link := filepath.Join(work, test.link)
			if err := os.Symlink(test.target, link); err != nil {
				t.Fatal(err)
			}
			code, _, stderr := runCommand(testEnvironment(), "fleet", "-o", link, filepath.Join(work, "inv"))
			if code != ExitError || !strings.Contains(stderr, link+" is a symbolic link; give its target") {
				t.Errorf("exit %d, stderr %q", code, stderr)
			}
			if data, err := osfile.ReadFile(test.target); err != nil || string(data) != evidenceText {
				t.Errorf("the link's target now holds %q (%v)", data, err)
			}
			if info, err := osfile.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
				t.Errorf("the link is no longer a link (%v)", err)
			}
		})
	}
}

func TestReplacingALinkReplacesTheLinkAndNotItsTarget(t *testing.T) {
	work := t.TempDir()
	target := filepath.Join(work, "target.txt")
	writeFile(t, target, evidenceText)
	link := filepath.Join(work, "out.csv")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := replaceFile(link, outputParent(link), []byte("new\n"), fleetTemporaryName); err != nil {
		t.Fatal(err)
	}
	if data, err := osfile.ReadFile(target); err != nil || string(data) != evidenceText {
		t.Errorf("the target now holds %q (%v)", data, err)
	}
	info, err := osfile.Lstat(link)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("the output is not a regular file (%v)", err)
	}
	if data, err := osfile.ReadFile(link); err != nil || string(data) != "new\n" {
		t.Errorf("the output holds %q (%v)", data, err)
	}
}

func TestFleetGivesItsNewCSVTheModeAPlainWriteGives(t *testing.T) {
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "inv", "a.json"), hostLine)
	plain := filepath.Join(work, "plain")
	if err := osfile.WriteFile(plain, nil, 0o666); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(work, "out.csv")
	if code, _, stderr := runCommand(testEnvironment(), "fleet", "-o", output,
		filepath.Join(work, "inv")); code != ExitFound {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	want, err := osfile.Lstat(plain)
	if err != nil {
		t.Fatal(err)
	}
	got, err := osfile.Lstat(output)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode() != want.Mode() {
		t.Errorf("the CSV's mode is %v, a plain write gives %v", got.Mode(), want.Mode())
	}
}

func TestFleetRefusesToReplaceAnOutputInADirectoryItCannotWrite(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("the superuser writes any directory")
	}
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "inv", "a.json"), hostLine)
	output := filepath.Join(work, "locked", "out.csv")
	writeFile(t, output, "earlier\n")
	if err := os.Chmod(filepath.Dir(output), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Dir(output), 0o755) })
	code, _, stderr := runCommand(testEnvironment(), "fleet", "-o", output, filepath.Join(work, "inv"))
	if code != ExitError || stderr != "error: "+output+": "+reason.PermissionDenied+"\n" {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if data, err := osfile.ReadFile(output); err != nil || string(data) != "earlier\n" {
		t.Errorf("the output now holds %q (%v)", data, err)
	}
}

// failingOutput is an output name whose last element ends in a slash: its
// directories can be made, but no file can be written there.
func failingOutput(parent string) string { return filepath.Join(parent, "x.csv") + "/" }

func TestFleetRemovesTheDirectoriesItMadeWhenTheWriteFails(t *testing.T) {
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "inv", "a.json"), hostLine)
	existing := filepath.Join(work, "O")
	if err := os.Mkdir(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	tests := []struct{ name, parent string }{
		{"two new directories", filepath.Join(existing, "newA", "b")},
		{"one new directory", filepath.Join(existing, "newB")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, _, stderr := runCommand(testEnvironment(), "fleet", "-o", failingOutput(test.parent),
				filepath.Join(work, "inv"))
			if code != ExitError {
				t.Errorf("exit %d, stderr %q", code, stderr)
			}
			if _, err := osfile.Lstat(test.parent); err == nil {
				t.Errorf("%s was made and left behind", test.parent)
			}
			if entries, err := os.ReadDir(existing); err != nil || len(entries) != 0 {
				t.Errorf("%s holds %v (%v), want it empty", existing, entries, err)
			}
		})
	}
}

func TestFleetLeavesAnExistingDirectoryWhenTheWriteFails(t *testing.T) {
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "inv", "a.json"), hostLine)
	existing := filepath.Join(work, "O", "kept")
	if err := os.MkdirAll(existing, 0o755); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runCommand(testEnvironment(), "fleet", "-o", failingOutput(existing),
		filepath.Join(work, "inv"))
	if code != ExitError {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if info, err := osfile.Lstat(existing); err != nil || !info.IsDir() {
		t.Errorf("the existing directory is gone (%v)", err)
	}
}

func TestFleetKeepsTheDirectoriesItMadeWhenTheWriteSucceeds(t *testing.T) {
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "inv", "a.json"), hostLine)
	output := filepath.Join(work, "O", "newA", "b", "x.csv")
	code, _, stderr := runCommand(testEnvironment(), "fleet", "-o", output, filepath.Join(work, "inv"))
	if code != ExitFound {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if info, err := osfile.Lstat(output); err != nil || !info.Mode().IsRegular() {
		t.Errorf("the CSV was not written (%v)", err)
	}
}

func TestFleetRemovesNothingWhenTheOutputParentHoldsADotElement(t *testing.T) {
	tests := []struct {
		name string
		// parent is written after the work directory; made lists what the
		// run makes there and must remain, kept what was there before.
		parent string
		made   []string
		kept   []string
	}{
		{"a .. after a missing directory", "/x/../keep", []string{"x"}, []string{"keep"}},
		{"a .. after a new directory", "/a/b/../c", []string{"a/b", "a/c"}, nil},
		{"a . element", "/./O/new", []string{"O/new"}, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			work := t.TempDir()
			writeFile(t, filepath.Join(work, "inv", "a.json"), hostLine)
			for _, kept := range test.kept {
				if err := os.MkdirAll(filepath.Join(work, kept), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			if strings.HasPrefix(test.parent, "/a/") {
				if err := os.Mkdir(filepath.Join(work, "a"), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			output := work + test.parent + "/x.csv/"
			code, _, stderr := runCommand(testEnvironment(), "fleet", "-o", output, filepath.Join(work, "inv"))
			if code != ExitError {
				t.Errorf("exit %d, stderr %q", code, stderr)
			}
			for _, kept := range test.kept {
				if info, err := osfile.Lstat(filepath.Join(work, kept)); err != nil || !info.IsDir() {
					t.Errorf("%s was there before and is gone (%v)", kept, err)
				}
			}
			for _, made := range test.made {
				if info, err := osfile.Lstat(filepath.Join(work, made)); err != nil || !info.IsDir() {
					t.Errorf("%s was made and must be left, not removed (%v)", made, err)
				}
			}
		})
	}
}

func TestFleetRemovesWhatItMadeBelowALinkedDirectoryWhenTheWriteFails(t *testing.T) {
	work := t.TempDir()
	writeFile(t, filepath.Join(work, "inv", "a.json"), hostLine)
	target := filepath.Join(work, "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(work, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runCommand(testEnvironment(), "fleet", "-o", link+"/new/x.csv/", filepath.Join(work, "inv"))
	if code != ExitError {
		t.Errorf("exit %d, stderr %q", code, stderr)
	}
	if entries, err := os.ReadDir(target); err != nil || len(entries) != 0 {
		t.Errorf("the link's target holds %v (%v), want it empty", entries, err)
	}
	if info, err := osfile.Lstat(link); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Errorf("the link is gone (%v)", err)
	}
}
