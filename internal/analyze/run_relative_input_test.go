package analyze

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// historyLine is one Claude Code prompt history record, which gives a row.
const historyLine = `{"display":"hello","timestamp":1759300000000,"project":"/srv/p"}` + "\n"

// writeTar writes a tar at path holding each member name with its text, in
// order.
func writeTar(t *testing.T, path string, members [][2]string) {
	t.Helper()
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	for _, member := range members {
		if err := writer.WriteHeader(&tar.Header{Name: member[0], Mode: 0o644,
			Size: int64(len(member[1])), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := writer.Write([]byte(member[1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, archive.String())
}

// relativeInputCase is one command run with a relative input from the
// directory that holds it: build makes the fixture under that directory,
// and want is a text the output must hold, the input named as given.
type relativeInputCase struct {
	name  string
	build func(t *testing.T, work string)
	args  []string
	want  string
}

// relativeInputCases are the inputs whose paths reach what a command
// prints or writes: a problem line, a note, a home, an error, the
// extraction directory and the fleet CSV.
func relativeInputCases() []relativeInputCase {
	skippedMember := func(t *testing.T, work string) {
		writeTar(t, filepath.Join(work, "case", "case.tar"),
			[][2]string{{"alice/.claude/history.jsonl", historyLine}, {"../escape", "x"}})
	}
	rootHome := func(t *testing.T, work string) {
		writeFile(t, filepath.Join(work, "tree", ".claude", "history.jsonl"), historyLine)
	}
	agentDirectory := func(t *testing.T, work string) {
		writeFile(t, filepath.Join(work, "copy", ".claude", "history.jsonl"), historyLine)
	}
	notArchive := func(t *testing.T, work string) {
		writeFile(t, filepath.Join(work, "case", "junk.bin"), "neither tar nor zip")
	}
	archive := func(t *testing.T, work string) {
		writeTar(t, filepath.Join(work, "case", "case.tar"),
			[][2]string{{"alice/.claude/history.jsonl", historyLine}})
	}
	inventories := func(t *testing.T, work string) {
		writeFile(t, filepath.Join(work, "inv", "a.jsonl"), hostLine+"junk\n")
	}
	member := spelt("case", "case.tar") + ": member ../escape not extracted"
	return []relativeInputCase{
		{"detect names a skipped archive member by the archive as given", skippedMember,
			[]string{"detect", spelt("case", "case.tar")}, "problem: " + member},
		{"detect --json names a skipped archive member by the archive as given", skippedMember,
			[]string{"detect", "--json", spelt("case", "case.tar")}, member},
		{"timeline names a skipped archive member by the archive as given", skippedMember,
			[]string{"timeline", spelt("case", "case.tar"), "out"}, member},
		{"detect lists a loose tree that is a home by the input as given", rootHome,
			[]string{"detect", "tree"}, "tree      "},
		{"timeline records a loose tree that is a home with no path on the source host", rootHome,
			[]string{"timeline", "tree", "out"}, `"home": ""`},
		{"detect lists an agent directory's home by the input as given", agentDirectory,
			[]string{"detect", spelt("copy", ".claude")}, "copy      "},
		{"detect --json records an agent directory's home with no path on the source host",
			agentDirectory, []string{"detect", "--json", spelt("copy", ".claude")}, `"home": ""`},
		{"an input that is not an archive is named as given", notArchive,
			[]string{"detect", spelt("case", "junk.bin")}, spelt("case", "junk.bin") + " is neither"},
		{"an archive kept in a relative work directory is named under it", archive,
			[]string{"detect", "--work-dir", "work", "--keep-extracted", spelt("case", "case.tar")},
			"extracted to: work" + string(filepath.Separator)},
		{"fleet names its relative inputs and output as given", inventories,
			[]string{"fleet", "-o", spelt("csv", "fleet.csv"), "inv"}, "in " + spelt("inv", "a.jsonl")},
	}
}

// writtenFiles returns the text of every output file a run wrote under
// work, which is the working directory: the files of out, csv and work.
func writtenFiles(t *testing.T, work string) string {
	t.Helper()
	var text strings.Builder
	for _, directory := range []string{"out", "csv"} {
		entries, err := osfile.ReadDir(filepath.Join(work, directory))
		if err != nil {
			continue
		}
		for _, entry := range entries {
			data, err := osfile.ReadFile(filepath.Join(work, directory, entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			text.Write(data)
		}
	}
	return text.String()
}

// wantNoWorkingDirectory fails when printed holds the working directory's
// absolute path, as written or escaped in JSON.
func wantNoWorkingDirectory(t *testing.T, printed string) {
	t.Helper()
	working, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	escaped := jsonvalue.NewString(working).JSON()
	for _, form := range []string{working, escaped[1 : len(escaped)-1]} {
		if strings.Contains(printed, form) {
			t.Errorf("the output holds the working directory %s:\n%s", form, printed)
			return
		}
	}
}

func TestCommandsNameARelativeInputAsGiven(t *testing.T) {
	for _, test := range relativeInputCases() {
		t.Run(test.name, func(t *testing.T) {
			work := t.TempDir()
			test.build(t, work)
			t.Chdir(work)
			code, stdout, stderr := runCommand(System(), test.args...)
			printed := stdout + stderr + writtenFiles(t, work)
			if code == ExitError && !strings.Contains(test.name, "not an archive") {
				t.Fatalf("exit %d, stderr %q", code, stderr)
			}
			// JSON output writes a path separator of Windows as two
			// backslashes, so the wanted text is looked for in that form too.
			inJSON := strings.ReplaceAll(test.want, `\`, `\\`)
			if !strings.Contains(printed, test.want) && !strings.Contains(printed, inJSON) {
				t.Errorf("the output does not hold %q:\n%s", test.want, printed)
			}
			wantNoWorkingDirectory(t, printed)
		})
	}
}
