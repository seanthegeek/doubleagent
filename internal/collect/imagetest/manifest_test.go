package imagetest

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseLineReadsEachKindAndAttribute(t *testing.T) {
	t.Parallel()
	when := time.Date(2026, 9, 14, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		name, line string
		want       Entry
	}{
		{"a directory takes its default mode and time", `"a/b" dir`,
			Entry{Path: "a/b", Kind: KindDir, Mode: 0o755, ModTime: DefaultModTime()}},
		{"a file holds its quoted content", `"a/f" file "x\n"`,
			Entry{Path: "a/f", Kind: KindFile, Text: "x\n", Mode: 0o644, ModTime: DefaultModTime()}},
		{"a file joins its parts and repeats", `"f" file "a" 3*"\x00" "b"`,
			Entry{Path: "f", Kind: KindFile, Text: "a\x00\x00\x00b", Mode: 0o644, ModTime: DefaultModTime()}},
		{"an empty file has one empty part", `"f" file ""`,
			Entry{Path: "f", Kind: KindFile, Mode: 0o644, ModTime: DefaultModTime()}},
		{"a link keeps its target as written", `"l" link "../t\n"`,
			Entry{Path: "l", Kind: KindLink, Text: "../t\n", ModTime: DefaultModTime()}},
		{"a link from the work directory says so", `"l" dirlink "root/x" from=work`,
			Entry{Path: "l", Kind: KindDirLink, Text: "root/x", FromWork: true, ModTime: DefaultModTime()}},
		{"a junction has a target", `"j" junction "C:\\x"`,
			Entry{Path: "j", Kind: KindJunction, Text: `C:\x`, Mode: 0o755, ModTime: DefaultModTime()}},
		{"a fifo and its mode", `"p" fifo mode=0600`,
			Entry{Path: "p", Kind: KindFifo, Mode: 0o600, ModTime: DefaultModTime()}},
		{"a socket", `"s" socket`, Entry{Path: "s", Kind: KindSocket, Mode: 0o755, ModTime: DefaultModTime()}},
		{"a shut directory with its time", `"d" dir mode=0000 mtime=2026-09-14T00:00:00Z`,
			Entry{Path: "d", Kind: KindDir, ModTime: when}},
		{"a deny rule", `"d" dir deny=WD,AD`,
			Entry{Path: "d", Kind: KindDir, Mode: 0o755, ModTime: DefaultModTime(), Deny: "WD,AD"}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseLine(test.line)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, test.want) {
				t.Errorf("ParseLine(%s) = %+v, want %+v", test.line, got, test.want)
			}
		})
	}
}

func TestParseNamesTheNumberOfABadLine(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, line, want string }{
		{"an unquoted path", `a/b dir`, "line 3: path: a field must be"},
		{"an unknown kind", `"a" folder`, `line 3: unknown kind "folder"`},
		{"a path leaving the directory", `"a/../b" dir`, `line 3: path "a/../b" must be relative`},
		{"an absolute path", `"/a" dir`, `line 3: path "/a" must be relative`},
		{"a file without content", `"f" file`, "line 3: file: a file needs at least one content part"},
		{"a bad repeat", `"f" file 0*"x"`, "line 3: file: a repeated part"},
		{"an unterminated quote", `"f" file "x`, "line 3: file: "},
		{"a mode of the wrong length", `"d" dir mode=755`, `line 3: mode "755" is not four octal digits`},
		{"a mode on a link", `"l" link "t" mode=0755`, `line 3: attribute "mode=0755" does not apply to a link`},
		{"from=work on a file", `"f" file "" from=work`, `line 3: attribute "from=work" does not apply`},
		{"a bad time", `"d" dir mtime=yesterday`, `line 3: mtime "yesterday" is not an RFC 3339 time`},
		{"a word that is no attribute", `"d" dir open`, `line 3: attribute "open" is not name=value`},
		{"a second line for one path", `"a" dir`, `line 3: "a" was already given on line 2`},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(strings.NewReader("# header\n\"a\" dir\n" + test.line + "\n"))
			if err == nil || !strings.HasPrefix(err.Error(), test.want) {
				t.Errorf("Parse error = %v, want one starting %q", err, test.want)
			}
		})
	}
}

func TestFormatAndParseLineGiveEveryNameAndContentBack(t *testing.T) {
	t.Parallel()
	names := []string{"plain", "with space", `q"uote`, `back\slash`, "tab\there", "nl\nname",
		"uümlé 日本", "lone\xff\xfe", "line\u2028sep", "cr\rname", "trailing space "}
	contents := []string{"", "x\n", strings.Repeat("\x00", 600), "head\n" + strings.Repeat("#", 512) + "\ntail",
		strings.Repeat("ab", 40), "\xff\xfe" + strings.Repeat("\xff", 700)}
	for _, name := range names {
		for _, content := range contents {
			entry := Entry{Path: "root/" + name, Kind: KindFile, Text: content, Mode: 0o600,
				ModTime: time.Date(2026, 9, 14, 1, 2, 3, 400, time.UTC)}
			line := Format(entry)
			got, err := ParseLine(line)
			if err != nil || !reflect.DeepEqual(got, entry) {
				t.Errorf("ParseLine(Format(%q, %d bytes)) = %+v, %v", name, len(content), got, err)
			}
			if strings.ContainsAny(line, "\n\r\t") || len(line) > 400 {
				t.Errorf("Format(%q, %d bytes) = %q, want one short line", name, len(content), line)
			}
		}
	}
}

func TestEveryCommittedManifestIsSortedAndAsFormatWritesIt(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"testdata/image_unix.txt", "testdata/image_windows.txt"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Parse(bytes.NewReader(data)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		previous, number, entries := "", 0, 0
		scanner := bufio.NewScanner(bytes.NewReader(data))
		scanner.Buffer(nil, 1<<24)
		for scanner.Scan() {
			number++
			line := scanner.Text()
			if strings.HasPrefix(line, "#") && entries == 0 {
				continue
			}
			entries++
			if entry, err := ParseLine(line); err == nil && Format(entry) != line {
				t.Errorf("%s:%d: %s\nFormat writes %s", name, number, line, Format(entry))
			}
			if line <= previous {
				t.Errorf("%s:%d: %s is not after the line before it, %s", name, number, line, previous)
			}
			previous = line
		}
		if entries < 3000 {
			t.Errorf("%s holds %d entries, too few for the main image", name, entries)
		}
	}
}

func TestReplayMakesEachKindWithItsModeAndTime(t *testing.T) {
	t.Parallel()
	work := resolvedDirectory(t, t.TempDir())
	entries, err := Parse(strings.NewReader(platformKinds))
	if err != nil {
		t.Fatal(err)
	}
	var notes []string
	undo, err := Replay(work, entries, func(format string, args ...any) {
		notes = append(notes, fmt.Sprintf(format, args...))
	})
	t.Cleanup(func() { _ = undo() })
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 0 {
		t.Skipf("this account cannot make every kind: %s", notes)
	}
	requireShut(t, work+string(os.PathSeparator)+"shut")
	if err := undo(); err != nil {
		t.Fatal(err)
	}
	got, err := Read(work)
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range differences(openedAgain(entries), got, true) {
		t.Error(line)
	}
}
