package analyze

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// hostLine is a saved inventory output of one host.
const hostLine = `{"type":"host","host":"h"}` + "\n"

// spelt returns the elements joined by the separator as written, a ".."
// kept, as an analyst may type a path; filepath.Join would clean it.
func spelt(elements ...string) string {
	return strings.Join(elements, string(filepath.Separator))
}

func TestFleetNeverWritesOverAnInputFile(t *testing.T) {
	work := t.TempDir()
	input := filepath.Join(work, "inv.txt")
	directory := filepath.Join(work, "inventories")
	inside := filepath.Join(directory, "h.jsonl")
	writeFile(t, inside, hostLine)
	linked := filepath.Join(work, "linked.txt")
	tests := []struct {
		name   string
		output string
		inputs []string
	}{
		{"the input file under its own name", input, []string{input}},
		{"the input file under another spelling", spelt(work, "inventories", "..", "inv.txt"), []string{input}},
		{"the input file among others", input, []string{inside, input}},
		{"a hard link to the input file", linked, []string{input}},
		{"a hard link to a file inside an input directory", filepath.Join(work, "inside-link"),
			[]string{directory}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			writeFile(t, input, hostLine)
			_ = os.Remove(linked)
			_ = os.Remove(filepath.Join(work, "inside-link"))
			if err := os.Link(input, linked); err != nil {
				t.Skip("no hard links here:", err)
			}
			if err := os.Link(inside, filepath.Join(work, "inside-link")); err != nil {
				t.Fatal(err)
			}
			args := append([]string{"fleet", "-o", test.output}, test.inputs...)
			code, _, stderr := runCommand(testEnvironment(), args...)
			if code != ExitError || !strings.Contains(stderr, "is the input file") {
				t.Errorf("exit %d, stderr %q", code, stderr)
			}
			for _, kept := range []string{input, inside} {
				if data, _ := os.ReadFile(kept); string(data) != hostLine {
					t.Errorf("%s now holds %q", kept, data)
				}
			}
		})
	}
}

func TestTimelineRefusesAnOutputInsideItsInputByAnySpelling(t *testing.T) {
	work := t.TempDir()
	input := filepath.Join(work, "in")
	writeFile(t, filepath.Join(input, "sub", "keep"), "x")
	tests := []struct {
		name   string
		input  string
		output string
	}{
		{"below the input through ..", input, spelt(input, "sub", "..", "out")},
		{"the input by another spelling", spelt(work, "x", "..", "in"), filepath.Join(input, "sub", "out")},
		{"deep below a missing directory", input, filepath.Join(input, "new", "deeper", "out")},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			code, _, stderr := runCommand(testEnvironment(), "timeline", test.input, test.output)
			if code != ExitError || !strings.Contains(stderr, "must be outside the input") {
				t.Errorf("exit %d, stderr %q", code, stderr)
			}
			for _, made := range []string{filepath.Join(input, "out"), filepath.Join(input, "new"),
				filepath.Join(input, "sub", "out")} {
				if lstatExists(made) {
					t.Errorf("%s was made inside the input", made)
				}
			}
		})
	}
}

func TestAFailedTimelineLeavesNoDirectoryNamedOnTheWayToItsOutput(t *testing.T) {
	work := t.TempDir()
	code, _, stderr := runCommand(testEnvironment(), "timeline", filepath.Join(work, "missing"),
		spelt(work, "nd2", "..", "out4"))
	if code != ExitError {
		t.Fatalf("exit %d, stderr %q", code, stderr)
	}
	for _, name := range []string{"nd2", "out4"} {
		if lstatExists(filepath.Join(work, name)) {
			t.Errorf("%s was left behind", name)
		}
	}
}

func TestAWorkDirectoryTheRunMadeIsRemovedWithItsExtraction(t *testing.T) {
	work := t.TempDir()
	notArchive := filepath.Join(work, "not-an-archive")
	writeFile(t, notArchive, "neither tar nor zip")
	archive := filepath.Join(work, "home.tar")
	writeHomeArchive(t, archive)
	for name, args := range map[string][]string{
		"a failed detect":       {"detect", "--work-dir", filepath.Join(work, "w", "deeper"), notArchive},
		"a detect that worked":  {"detect", "--work-dir", filepath.Join(work, "w", "deeper"), archive},
		"a timeline that works": {"timeline", "--work-dir", filepath.Join(work, "w"), archive, filepath.Join(work, "o")},
	} {
		t.Run(name, func(t *testing.T) {
			runCommand(testEnvironment(), args...)
			if lstatExists(filepath.Join(work, "w")) {
				t.Error("the work directory the run made was left behind")
			}
		})
	}
}
