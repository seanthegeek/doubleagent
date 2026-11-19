package analyze

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"golang.org/x/sys/windows"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// makeJunction makes link a junction to the directory target without
// running a program: it creates an empty directory and sets on it the
// mount-point reparse data mklink /J would write. Both paths are absolute.
// Setting it needs no privilege on a directory the test made.
func makeJunction(t *testing.T, target, link string) {
	t.Helper()
	if err := osfile.MkdirAll(link, 0o755); err != nil {
		t.Fatal(err)
	}
	name, err := windows.UTF16PtrFromString(link)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_WRITE, 0, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		t.Fatalf("open %s: %v", link, err)
	}
	defer windows.CloseHandle(handle)
	data := mountPointData(`\??\`+target+`\`, target)
	var returned uint32
	err = windows.DeviceIoControl(handle, windows.FSCTL_SET_REPARSE_POINT, &data[0],
		uint32(len(data)), nil, 0, &returned, nil)
	if err != nil {
		t.Fatalf("set the junction data on %s: %v", link, err)
	}
}

// mountPointData lays out a REPARSE_DATA_BUFFER holding a mount-point
// (junction) reparse buffer: the tag, the data length, a reserved word, the
// offsets and lengths in bytes of the two names, then the substitute name
// the system follows and the print name shown to people, each ending in NUL.
func mountPointData(substitute, printName string) []byte {
	substituteText := utf16.Encode([]rune(substitute))
	printText := utf16.Encode([]rune(printName))
	names := make([]uint16, 0, len(substituteText)+len(printText)+2)
	names = append(append(names, substituteText...), 0)
	names = append(append(names, printText...), 0)
	substituteBytes := uint16(2 * len(substituteText))
	data := binary.LittleEndian.AppendUint32(nil, windows.IO_REPARSE_TAG_MOUNT_POINT)
	data = binary.LittleEndian.AppendUint16(data, uint16(8+2*len(names)))
	data = binary.LittleEndian.AppendUint16(data, 0)
	data = binary.LittleEndian.AppendUint16(data, 0)
	data = binary.LittleEndian.AppendUint16(data, substituteBytes)
	data = binary.LittleEndian.AppendUint16(data, substituteBytes+2)
	data = binary.LittleEndian.AppendUint16(data, uint16(2*len(printText)))
	for _, unit := range names {
		data = binary.LittleEndian.AppendUint16(data, unit)
	}
	return data
}

// windowsOutputCase is one output spelling given to timeline and fleet,
// whether it must be refused, and the text the refusal must hold.
type windowsOutputCase struct {
	name, output string
	refused      bool
	message      string
}

// runWindowsOutputCases runs timeline with output\out and fleet with
// output\f.csv against input for each case, from the directory working,
// and checks that each is refused or written as the case says, and that
// nothing is made inside input.
func runWindowsOutputCases(t *testing.T, working, input string, cases []windowsOutputCase) {
	t.Helper()
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(working)
			for _, args := range [][]string{{"timeline", input, test.output + `\out`},
				{"fleet", "-o", test.output + `\f.csv`, input}} {
				code, _, stderr := runCommand(testEnvironment(), args...)
				refused := code == ExitError && strings.Contains(stderr, test.message)
				if refused != test.refused || !refused && code == ExitError {
					t.Errorf("%s: exit %d, stderr %q", args[0], code, stderr)
				}
			}
			for _, name := range []string{"out", "f.csv"} {
				if lstatExists(filepath.Join(input, name)) {
					t.Errorf("%s was written inside the input", name)
					_ = osfile.RemoveAll(filepath.Join(input, name))
				}
			}
		})
	}
}

func TestAnOutputRootedWithoutADriveIsJudgedOnTheCurrentDrive(t *testing.T) {
	// \case\out names case on the working directory's drive, not a
	// directory below the working directory.
	work := t.TempDir()
	input := filepath.Join(work, "case")
	writeFile(t, filepath.Join(input, "a.jsonl"), hostLine)
	writeFile(t, filepath.Join(work, "elsewhere", "keep"), "")
	rooted := input[len(filepath.VolumeName(input)):]
	runWindowsOutputCases(t, filepath.Join(work, "elsewhere"), input, []windowsOutputCase{
		{"the input rooted without its drive", rooted, true, "must be outside the input"},
		{"a sibling rooted without its drive", rooted + "2", false, ""},
	})
}

func TestAnOutputInTheExtendedFormOfTheInputIsRefused(t *testing.T) {
	work := t.TempDir()
	input := filepath.Join(work, "case")
	writeFile(t, filepath.Join(input, "a.jsonl"), hostLine)
	runWindowsOutputCases(t, work, input, []windowsOutputCase{
		{`the input in the \\?\ form`, `\\?\` + input, true, "must be outside the input"},
		{`a sibling in the \\?\ form`, `\\?\` + input + "2", false, ""},
	})
}

func TestAnOutputThroughAJunctionIsRefusedAsUntold(t *testing.T) {
	// The check does not follow a junction, so it cannot tell where one
	// leads, into the input or not, and refuses both.
	work := t.TempDir()
	input := filepath.Join(work, "case")
	writeFile(t, filepath.Join(input, "a.jsonl"), hostLine)
	writeFile(t, filepath.Join(work, "elsewhere", "keep"), "")
	makeJunction(t, input, filepath.Join(work, "into"))
	makeJunction(t, filepath.Join(work, "elsewhere"), filepath.Join(work, "away"))
	runWindowsOutputCases(t, work, input, []windowsOutputCase{
		{"a junction to the input", filepath.Join(work, "into"), true, "cannot tell whether the output"},
		{"a junction elsewhere", filepath.Join(work, "away"), true, "cannot tell whether the output"},
	})
	// The input is known by the directory its read reaches through the
	// junction, so an output inside that directory is refused as inside.
	code, _, stderr := runCommand(testEnvironment(), "timeline", filepath.Join(work, "into"),
		filepath.Join(input, "out"))
	if code != ExitError || !strings.Contains(stderr, "must be outside the input") {
		t.Errorf("an input given through a junction: exit %d, stderr %q", code, stderr)
	}
	if lstatExists(filepath.Join(input, "out")) {
		t.Error("the output was made inside the input given through a junction")
	}
}

func TestAnOutputThroughADirectorySymlinkIsRefusedAsUntold(t *testing.T) {
	// On Windows the check follows no link, so it refuses one into the
	// input or not.
	work := t.TempDir()
	input := filepath.Join(work, "case")
	writeFile(t, filepath.Join(input, "a.jsonl"), hostLine)
	writeFile(t, filepath.Join(work, "elsewhere", "keep"), "")
	for link, target := range map[string]string{"into": input, "away": filepath.Join(work, "elsewhere")} {
		if err := os.Symlink(target, filepath.Join(work, link)); err != nil {
			t.Skip("a directory symlink cannot be made without the privilege or developer mode:", err)
		}
	}
	runWindowsOutputCases(t, work, input, []windowsOutputCase{
		{"a symlink to the input", filepath.Join(work, "into"), true, "cannot tell whether the output"},
		{"a symlink elsewhere", filepath.Join(work, "away"), true, "cannot tell whether the output"},
	})
}

func TestAnInputWithoutADriveOrARootIsReadWhereWindowsPlacesIt(t *testing.T) {
	// \tree names tree on the working directory's drive and C:tree names it
	// in drive C's current directory, not a directory below the working
	// directory; the read and the outside-the-input check both take it there.
	work := t.TempDir()
	tree := filepath.Join(work, "tree")
	writeFile(t, filepath.Join(tree, ".claude", "history.jsonl"), historyLine)
	elsewhere := filepath.Join(work, "elsewhere")
	writeFile(t, filepath.Join(elsewhere, "keep"), "")
	volume := filepath.VolumeName(work)
	tests := []struct{ name, working, input, output string }{
		{"rooted without a drive", elsewhere, tree[len(volume):], "rooted"},
		{"on the current drive without a root", work, volume + "tree", "drive"},
		{"relative", work, "tree", "relative"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if len(volume) != 2 || volume[1] != ':' {
				t.Skip("the temporary directory is not on a drive letter:", work)
			}
			t.Chdir(test.working)
			code, stdout, stderr := runCommand(System(), "detect", test.input)
			if code != ExitFound || !strings.Contains(stdout, test.input+" ") {
				t.Errorf("detect: exit %d, stdout %q, stderr %q", code, stdout, stderr)
			}
			output := filepath.Join(work, test.output)
			code, _, stderr = runCommand(System(), "timeline", test.input, output)
			if code == ExitError || !lstatExists(filepath.Join(output, "timeline.jsonl")) {
				t.Errorf("timeline: exit %d, stderr %q", code, stderr)
			}
			code, _, stderr = runCommand(System(), "timeline", test.input, filepath.Join(tree, "out"))
			if code != ExitError || !strings.Contains(stderr, "must be outside the input") {
				t.Errorf("timeline into the input: exit %d, stderr %q", code, stderr)
			}
			if lstatExists(filepath.Join(tree, "out")) {
				t.Error("timeline wrote inside the input")
			}
		})
	}
}
