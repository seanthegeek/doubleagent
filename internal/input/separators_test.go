package input

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// zipMadeBy returns a zip archive holding each name of names with its body,
// each member's "version made by" naming the system system.
func zipMadeBy(t *testing.T, system uint16, names []string, bodies map[string]string) []byte {
	t.Helper()
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for _, name := range names {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate,
			CreatorVersion: system << 8})
		opened(t, err)
		_, err = entry.Write([]byte(bodies[name]))
		opened(t, err)
	}
	opened(t, writer.Close())
	return archive.Bytes()
}

// zipSystems returns the systems a zip test's members record: MS-DOS,
// UNIX, Windows NTFS, OS X and one APPNOTE.TXT does not list.
func zipSystems() []uint16 { return []uint16{0, 3, 10, 19, 99} }

func TestAManifestRowNamesTheFileItsArchiveExtracted(t *testing.T) {
	t.Parallel()
	posix := collectedRow("alice", "/home/alice", "claude-code", `/home/alice/.claude/a\b.jsonl`,
		`fs/home/alice/.claude/a\b.jsonl`) + "\n"
	windows := collectedRow("bob", `C:\Users\bob`, "claude-code", `C:\Users\bob\.claude\x.jsonl`,
		`fs\C\Users\bob\.claude\x.jsonl`) + "\n"
	type archiveCase struct {
		name, file string
		data       []byte
		wantRel    string
	}
	zipOf := func(system uint16, manifest, member string) []byte {
		return zipMadeBy(t, system, []string{"manifest.jsonl", member},
			map[string]string{"manifest.jsonl": manifest, member: "{}"})
	}
	tarOf := func(manifest, member string) []byte {
		return tarBytes(t, false, tarMember{name: "manifest.jsonl", typeflag: tar.TypeReg, body: manifest},
			tarMember{name: member, typeflag: tar.TypeReg, body: "{}"})
	}
	cases := []archiveCase{
		{name: "a POSIX row from a tar keeps the backslash in its file name", file: "c.tar",
			data: tarOf(posix, `fs/home/alice/.claude/a\b.jsonl`), wantRel: `.claude/a\b.jsonl`},
		{name: "a Windows row from a tar with slashes", file: "c.tar",
			data: tarOf(windows, "fs/C/Users/bob/.claude/x.jsonl"), wantRel: ".claude/x.jsonl"},
	}
	for _, system := range zipSystems() {
		cases = append(cases,
			archiveCase{name: fmt.Sprintf("a POSIX row with a backslash from a zip of system %d", system),
				file: "c.zip", data: zipOf(system, posix, `fs/home/alice/.claude/a\b.jsonl`),
				wantRel: `.claude/a\b.jsonl`},
			archiveCase{name: fmt.Sprintf("a Windows row from a zip of system %d with backslashes", system),
				file: "c.zip", data: zipOf(system, windows, `fs\C\Users\bob\.claude\x.jsonl`),
				wantRel: ".claude/x.jsonl"},
			archiveCase{name: fmt.Sprintf("a Windows row from a zip of system %d with slashes", system),
				file: "c.zip", data: zipOf(system, windows, "fs/C/Users/bob/.claude/x.jsonl"),
				wantRel: ".claude/x.jsonl"})
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			collection, err := openArchiveOf(t, test.file, test.data)
			opened(t, err)
			wantSlice(t, "problems", collection.Problems, nil)
			wantSlice(t, "artifacts", rels(collection), []string{test.wantRel})
		})
	}
}

func TestABackslashSeparatesFoldersInEveryZipMemberName(t *testing.T) {
	t.Parallel()
	manifest := collectedRow("alice", "/home/alice", "claude-code", "/home/alice/.claude/x.jsonl",
		"fs/home/alice/.claude/x.jsonl") + "\n"
	withManifest := []string{"manifest.jsonl", `fs\home\alice\.claude\`, `fs\home\alice\.claude\x.jsonl`}
	loose := []string{`alice/.claude/a\b.jsonl`, `alice/.claude/d\`}
	for _, system := range zipSystems() {
		t.Run(fmt.Sprintf("system %d with a manifest", system), func(t *testing.T) {
			t.Parallel()
			collection, err := openArchiveOf(t, "c.zip", zipMadeBy(t, system, withManifest,
				map[string]string{"manifest.jsonl": manifest, withManifest[2]: "{}"}))
			opened(t, err)
			wantSlice(t, "problems", collection.Problems, nil)
			wantSlice(t, "artifacts", rels(collection), []string{".claude/x.jsonl"})
		})
		t.Run(fmt.Sprintf("system %d without a manifest", system), func(t *testing.T) {
			t.Parallel()
			collection, err := openArchiveOf(t, "c.zip", zipMadeBy(t, system, loose,
				map[string]string{loose[0]: "{}"}))
			opened(t, err)
			wantSlice(t, "problems", collection.Problems, nil)
			wantSlice(t, "artifacts", rels(collection), []string{".claude/a/b.jsonl"})
		})
	}
}

// namedMember is one file member of a test archive: its name and body.
type namedMember struct{ name, body string }

// zipOfMembers returns a zip archive holding members in order, a name
// given twice written twice, each member's "version made by" naming system.
func zipOfMembers(t *testing.T, system uint16, members ...namedMember) []byte {
	t.Helper()
	var archive bytes.Buffer
	writer := zip.NewWriter(&archive)
	for _, member := range members {
		entry, err := writer.CreateHeader(&zip.FileHeader{Name: member.name, Method: zip.Deflate,
			CreatorVersion: system << 8})
		opened(t, err)
		_, err = entry.Write([]byte(member.body))
		opened(t, err)
	}
	opened(t, writer.Close())
	return archive.Bytes()
}

// onePlacePair is two names of files below a home's .claude/projects/p
// that a backslash separating folders puts in one place.
type onePlacePair struct{ label, backslashed, slashed string }

// onePlacePairs returns a pair differing in the file's own part, one
// differing in a directory part, and one whose backslashed name holds "."
// parts, which extraction drops.
func onePlacePairs() []onePlacePair {
	return []onePlacePair{
		{"in the file's part", `a\b.jsonl`, "a/b.jsonl"},
		{"in a directory part", `a\b/c.jsonl`, "a/b/c.jsonl"},
		{"by dot parts", `a\.\.\b.jsonl`, "a/b.jsonl"},
	}
}

// onePlaceMembers returns the archive members of a pair, the backslashed
// name first unless slashFirst, each holding its own name as its body,
// after a manifest with a row for each when withManifest.
func onePlaceMembers(pair onePlacePair, slashFirst, withManifest bool) []namedMember {
	prefix := "u/.claude/projects/p/"
	if withManifest {
		prefix = "fs/home/u/.claude/projects/p/"
	}
	members := []namedMember{{prefix + pair.backslashed, pair.backslashed}, {prefix + pair.slashed, pair.slashed}}
	if slashFirst {
		members[0], members[1] = members[1], members[0]
	}
	if withManifest {
		row := func(name string) string {
			return collectedRow("u", "/home/u", "claude-code", "/home/u/.claude/projects/p/"+name,
				"fs/home/u/.claude/projects/p/"+name)
		}
		manifest := row(pair.backslashed) + "\n" + row(pair.slashed) + "\n"
		members = append([]namedMember{{"manifest.jsonl", manifest}}, members...)
	}
	return members
}

// wantFirstMemberKept checks a collection read from onePlaceMembers: the
// first member's file is its one artifact, holding its own body, and is
// what its home gives for the shared place; the second is skipped with a
// problem line naming both, and its row, if any, is missing.
func wantFirstMemberKept(t *testing.T, collection *Collection, members []namedMember, pair onePlacePair) {
	t.Helper()
	withManifest := members[0].name == "manifest.jsonl"
	if withManifest {
		members = members[1:]
	}
	kept, skipped := members[0], members[1]
	wantRel, wantProblems := ".claude/projects/p/"+pair.slashed, 1
	if withManifest {
		wantRel, wantProblems = ".claude/projects/p/"+kept.body, 2
	}
	wantSlice(t, "artifacts", rels(collection), []string{wantRel})
	artifact := collection.Artifacts[0]
	data, err := os.ReadFile(artifact.DiskPath)
	opened(t, err)
	wantEqual(t, "the artifact's body", string(data), kept.body)
	shared, ok := artifact.Home.PathOf(".claude/projects/p/" + pair.slashed)
	wantEqual(t, "the home gives the shared place", ok, true)
	data, err = os.ReadFile(shared)
	opened(t, err)
	wantEqual(t, "the shared place's body", string(data), kept.body)
	if withManifest {
		wantProblem(t, collection, "/home/u/.claude/projects/p/"+skipped.body+": missing from the collection")
	}
	wantProblem(t, collection, ": member "+skipped.name+" not extracted: its name and that of member "+
		kept.name+" name one place when a backslash separates folders; "+kept.name+" is kept")
	wantEqual(t, "problem count", len(collection.Problems), wantProblems)
}

func TestTwoZipMembersAtOnePlaceKeepTheFirstAndSkipTheSecond(t *testing.T) {
	t.Parallel()
	for _, system := range zipSystems() {
		for _, pair := range onePlacePairs() {
			for _, slashFirst := range []bool{false, true} {
				for _, withManifest := range []bool{false, true} {
					name := fmt.Sprintf("system %d, names differing %s, slash first %t, manifest %t",
						system, pair.label, slashFirst, withManifest)
					t.Run(name, func(t *testing.T) {
						t.Parallel()
						members := onePlaceMembers(pair, slashFirst, withManifest)
						collection, err := openArchiveOf(t, "c.zip", zipOfMembers(t, system, members...))
						opened(t, err)
						wantFirstMemberKept(t, collection, members, pair)
					})
				}
			}
		}
	}
}

func TestAZipMemberNamedTwiceIsReplacedWithNoProblem(t *testing.T) {
	t.Parallel()
	const name = "fs/home/u/.claude/projects/p/a/b.jsonl"
	manifest := collectedRow("u", "/home/u", "claude-code", "/home/u/.claude/projects/p/a/b.jsonl", name) + "\n"
	for _, system := range zipSystems() {
		t.Run(fmt.Sprintf("system %d", system), func(t *testing.T) {
			t.Parallel()
			collection, err := openArchiveOf(t, "c.zip", zipOfMembers(t, system,
				namedMember{"manifest.jsonl", manifest}, namedMember{name, "first"}, namedMember{name, "second"}))
			opened(t, err)
			wantSlice(t, "problems", collection.Problems, nil)
			wantSlice(t, "artifacts", rels(collection), []string{".claude/projects/p/a/b.jsonl"})
			data, err := os.ReadFile(collection.Artifacts[0].DiskPath)
			opened(t, err)
			wantEqual(t, "the file's body", string(data), "second")
		})
	}
}

func TestAZipDirectoryAtAFileMembersPlaceIsAnotherKindOfEntry(t *testing.T) {
	t.Parallel()
	for _, system := range zipSystems() {
		t.Run(fmt.Sprintf("system %d", system), func(t *testing.T) {
			t.Parallel()
			collection, err := openArchiveOf(t, "c.zip", zipOfMembers(t, system,
				namedMember{`u/.claude/projects/p/a\b`, "file"}, namedMember{"u/.claude/projects/p/a/b/", ""}))
			opened(t, err)
			wantProblem(t, collection, ": member u/.claude/projects/p/a/b/ not extracted: member "+
				"u/.claude/projects/p/a/b holds its place as another kind of entry")
			wantEqual(t, "problem count", len(collection.Problems), 1)
		})
	}
}

// loneDottedMembers returns a manifest and one member whose name holds "."
// parts between backslashes, with the row naming it.
func loneDottedMembers() []namedMember {
	const name = `fs/home/u/.claude/projects/p/a\.\b.jsonl`
	row := collectedRow("u", "/home/u", "claude-code", `/home/u/.claude/projects/p/a\.\b.jsonl`, name)
	return []namedMember{{"manifest.jsonl", row + "\n"}, {name, "dotted"}}
}

// wantLoneDottedArtifact checks a collection read from loneDottedMembers:
// the row is the member's artifact, with no problem.
func wantLoneDottedArtifact(t *testing.T, collection *Collection) {
	t.Helper()
	wantSlice(t, "problems", collection.Problems, nil)
	wantSlice(t, "artifacts", rels(collection), []string{`.claude/projects/p/a\.\b.jsonl`})
	data, err := os.ReadFile(collection.Artifacts[0].DiskPath)
	opened(t, err)
	wantEqual(t, "the artifact's body", string(data), "dotted")
}

func TestAZipMemberWithDotPartsBetweenBackslashesIsItsRowsArtifact(t *testing.T) {
	t.Parallel()
	for _, system := range zipSystems() {
		t.Run(fmt.Sprintf("system %d", system), func(t *testing.T) {
			t.Parallel()
			collection, err := openArchiveOf(t, "c.zip", zipOfMembers(t, system, loneDottedMembers()...))
			opened(t, err)
			wantLoneDottedArtifact(t, collection)
		})
	}
}

func TestARowWhoseArchivePathHoldsADotDotPartStaysRefused(t *testing.T) {
	t.Parallel()
	const archivePath = "fs/home/u/.claude/./../.claude/x.jsonl"
	row := collectedRow("u", "/home/u", "claude-code", "/home/u/.claude/x.jsonl", archivePath) + "\n"
	t.Run("in a collection directory", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		writeManifest(t, root, row)
		writeFile(t, filepath.Join(root, "fs", "home", "u", ".claude", "x.jsonl"), "{}")
		collection, err := Open(t.Context(), root, Options{Matcher: matcherOf(t, "")})
		opened(t, err)
		wantSlice(t, "artifacts", rels(collection), nil)
		wantProblem(t, collection, "/home/u/.claude/x.jsonl: outside the collection")
	})
	t.Run("in a zip", func(t *testing.T) {
		t.Parallel()
		collection, err := openArchiveOf(t, "c.zip", zipOfMembers(t, 3, namedMember{"manifest.jsonl", row},
			namedMember{"fs/home/u/.claude/x.jsonl", "{}"}))
		opened(t, err)
		wantSlice(t, "artifacts", rels(collection), nil)
		wantProblem(t, collection, "/home/u/.claude/x.jsonl: missing from the collection")
	})
}
