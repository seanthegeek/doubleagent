package input

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// extractMember writes one file member of an extraction into root.
func extractMember(t *testing.T, out *extraction, name, body string) error {
	t.Helper()
	return out.writeFile(strings.Split(name, "/"), 0o644, strings.NewReader(body), time.Time{})
}

func TestAMemberWhosePlaceHoldsAnotherMembersFileIsSkipped(t *testing.T) {
	root := t.TempDir()
	out := newExtraction(root, "c.tar")
	if err := extractMember(t, out, "d/first.jsonl", "first"); err != nil {
		t.Fatal(err)
	}
	// A second name for the same file is what a file system that folds
	// case or normalisation gives; a hard link gives it on every system.
	if err := os.Link(filepath.Join(root, "d", "first.jsonl"), filepath.Join(root, "d", "second.jsonl")); err != nil {
		t.Skip("no hard links here:", err)
	}
	err := extractMember(t, out, "d/second.jsonl", "second")
	if !out.skipped("d/second.jsonl", err) {
		t.Fatalf("error %v, want the member skipped", err)
	}
	want := "c.tar: member d/second.jsonl not extracted: this file system holds its name and that of member " +
		"d/first.jsonl as one entry; d/first.jsonl is kept"
	wantSlice(t, "problems", out.problems, []string{want})
	if data, _ := os.ReadFile(filepath.Join(root, "d", "first.jsonl")); string(data) != "first" {
		t.Errorf("the kept member holds %q, want first", data)
	}
}

func TestAnEntryTheExtractionNeverMadeIsNeverWrittenInto(t *testing.T) {
	for _, test := range []struct {
		name, planted, member string
		directory             bool
	}{
		{"a file", "d/f", "d/f", false},
		{"a directory on the way down", "x", "x/y", true},
		{"a directory at the member's place", "z", "z", true},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			planted := filepath.Join(root, filepath.FromSlash(test.planted))
			if test.directory {
				if err := os.MkdirAll(planted, 0o777); err != nil {
					t.Fatal(err)
				}
			} else {
				writeFile(t, planted, "planted")
			}
			out := newExtraction(root, "c.tar")
			out.made["d"], out.children[""] = true, []string{"d"}
			var collision *collisionError
			if err := extractMember(t, out, test.member, "member"); !errors.As(err, &collision) {
				t.Fatalf("error %v, want a collision", err)
			}
			if !test.directory {
				if data, _ := os.ReadFile(planted); string(data) != "planted" {
					t.Errorf("the planted file holds %q", data)
				}
			}
		})
	}
}

// foldsNames reports whether the file system under directory gives two
// names one entry: upper and lower case, or two Unicode normalisations.
func foldsNames(t *testing.T, directory, first, second string) bool {
	t.Helper()
	writeFile(t, filepath.Join(directory, first), "x")
	_, err := os.Lstat(filepath.Join(directory, second))
	return err == nil
}

func TestMembersOneFileSystemHoldsAsOneEntryAreNeverMerged(t *testing.T) {
	const projects = "alice/.claude/projects/p/"
	for _, test := range []struct {
		name, first, second string
	}{
		{"names differing in case", "A.jsonl", "a.jsonl"},
		{"names in two Unicode normalisations", "é.jsonl", "é.jsonl"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if !foldsNames(t, t.TempDir(), test.first, test.second) {
				t.Skip("this file system keeps the two names apart")
			}
			collection, err := openArchiveOf(t, "c.tar", tarBytes(t, false,
				tarMember{name: projects + test.first, typeflag: '0', body: "first"},
				tarMember{name: projects + test.second, typeflag: '0', body: "second"}))
			opened(t, err)
			wantProblem(t, collection, "member "+projects+test.second+" not extracted: this file system holds "+
				"its name and that of member "+projects+test.first+" as one entry")
			wantSlice(t, "artifacts", rels(collection), []string{".claude/projects/p/" + test.first})
		})
	}
}

func TestADirectoryMemberAfterAFileOneSystemHoldsAsOneIsSkipped(t *testing.T) {
	if !foldsNames(t, t.TempDir(), "X", "x") {
		t.Skip("this file system keeps the two names apart")
	}
	collection, err := openArchiveOf(t, "c.tar", tarBytes(t, false,
		tarMember{name: "alice/.claude/projects/X", typeflag: '0', body: "file"},
		tarMember{name: "alice/.claude/projects/x/y.jsonl", typeflag: '0', body: "row"},
		tarMember{name: "alice/.claude/settings.json", typeflag: '0', body: "{}"}))
	opened(t, err)
	wantProblem(t, collection, "member alice/.claude/projects/x/y.jsonl not extracted: this file system "+
		"holds its name and that of member alice/.claude/projects/X as one entry")
	wantSlice(t, "artifacts", rels(collection), []string{".claude/projects/X", ".claude/settings.json"})
}

func TestAManifestRowOfASkippedMemberIsAProblemNotTheKeptFile(t *testing.T) {
	if !foldsNames(t, t.TempDir(), "A", "a") {
		t.Skip("this file system keeps the two names apart")
	}
	dir := t.TempDir()
	writeManifest(t, dir,
		collectedRow("u", "/home/u", "claude-code", "/home/u/.claude/projects/p/A.jsonl",
			"fs/home/u/.claude/projects/p/A.jsonl"),
		collectedRow("u", "/home/u", "claude-code", "/home/u/.claude/projects/p/a.jsonl",
			"fs/home/u/.claude/projects/p/a.jsonl"))
	members := []tarMember{{name: "manifest.jsonl", typeflag: '0'}}
	data, err := os.ReadFile(filepath.Join(dir, "manifest.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	members[0].body = string(data)
	members = append(members,
		tarMember{name: "fs/home/u/.claude/projects/p/A.jsonl", typeflag: '0', body: "upper"},
		tarMember{name: "fs/home/u/.claude/projects/p/a.jsonl", typeflag: '0', body: "lower"})
	collection, err := openArchiveOf(t, "c.tar", tarBytes(t, false, members...))
	opened(t, err)
	wantEqual(t, "kind", collection.Kind, KindArchive)
	wantProblem(t, collection, "/home/u/.claude/projects/p/a.jsonl: missing from the collection")
	wantSlice(t, "artifacts", rels(collection), []string{".claude/projects/p/A.jsonl"})
}

func TestAMemberNameTheSystemCannotHoldIsSkipped(t *testing.T) {
	long := "alice/.claude/q/" + strings.Repeat("n", 300) + ".jsonl"
	collection, err := openArchiveOf(t, "c.tar", tarBytes(t, false,
		tarMember{name: long, typeflag: '0', body: "x"},
		tarMember{name: "alice/.claude/x", typeflag: '0', body: "x"}))
	opened(t, err)
	wantProblem(t, collection, "member "+long+" not extracted: "+longPartReason)
	wantSlice(t, "artifacts", rels(collection), []string{".claude/x"})
}

func TestAMemberNameThatIsNotUTF8IsExtractedOrSkipped(t *testing.T) {
	name := "alice/.claude/q/\xff.jsonl"
	collection, err := openArchiveOf(t, "c.tar", tarBytes(t, false,
		tarMember{name: name, typeflag: '0', body: "x"},
		tarMember{name: "alice/.claude/x", typeflag: '0', body: "x"}))
	opened(t, err)
	if len(collection.Problems) > 0 {
		wantProblem(t, collection, "member "+name+" not extracted: "+reasonNotAName)
	}
	if !slices.Contains(rels(collection), ".claude/x") {
		t.Errorf("artifacts %q", rels(collection))
	}
}
