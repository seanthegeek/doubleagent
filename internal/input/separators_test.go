package input

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
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

// onePlaceCase is the members of an archive, by name below a home's
// .claude/projects/p and in archive order, whose files a backslash
// separating folders puts at place: kept is the member whose file stays,
// and notExtracted the members its problem lines name, in order.
type onePlaceCase struct {
	label        string
	names        []string
	kept, place  string
	notExtracted []string
}

// onePlaceCases returns each pair of onePlacePairs in both orders, two
// names that both need a backslash in both orders, and two cases of three
// members where the place's own name is kept and each other member, a name
// given twice included, has its line in archive order.
func onePlaceCases() []onePlaceCase {
	var cases []onePlaceCase
	for _, pair := range onePlacePairs() {
		cases = append(cases,
			onePlaceCase{"names differing " + pair.label + ", backslashed first",
				[]string{pair.backslashed, pair.slashed}, pair.slashed, pair.slashed, []string{pair.backslashed}},
			onePlaceCase{"names differing " + pair.label + ", slashed first",
				[]string{pair.slashed, pair.backslashed}, pair.slashed, pair.slashed, []string{pair.backslashed}})
	}
	return append(cases,
		onePlaceCase{"two backslashed names, the shorter first", []string{`a\b.jsonl`, `a\.\b.jsonl`},
			`a\b.jsonl`, "a/b.jsonl", []string{`a\.\b.jsonl`}},
		onePlaceCase{"two backslashed names, the longer first", []string{`a\.\b.jsonl`, `a\b.jsonl`},
			`a\.\b.jsonl`, "a/b.jsonl", []string{`a\b.jsonl`}},
		onePlaceCase{"a backslashed name, the place's own, the backslashed again",
			[]string{`a\b.jsonl`, "a/b.jsonl", `a\b.jsonl`}, "a/b.jsonl", "a/b.jsonl",
			[]string{`a\b.jsonl`, `a\b.jsonl`}},
		onePlaceCase{"two backslashed names, then the place's own",
			[]string{`a\b.jsonl`, `a\.\b.jsonl`, "a/b.jsonl"}, "a/b.jsonl", "a/b.jsonl",
			[]string{`a\b.jsonl`, `a\.\b.jsonl`}})
}

// onePlacePrefix returns the archive's path to .claude/projects/p.
func onePlacePrefix(withManifest bool) string {
	if withManifest {
		return "fs/home/u/.claude/projects/p/"
	}
	return "u/.claude/projects/p/"
}

// onePlaceMembers returns the archive members of a case, each holding its
// own name as its body, after a manifest with a row for each name when
// withManifest.
func onePlaceMembers(test onePlaceCase, withManifest bool) []namedMember {
	prefix := onePlacePrefix(withManifest)
	var members []namedMember
	var rows []string
	for _, name := range test.names {
		members = append(members, namedMember{prefix + name, name})
		row := collectedRow("u", "/home/u", "claude-code", "/home/u/.claude/projects/p/"+name, prefix+name)
		if !slices.Contains(rows, row) {
			rows = append(rows, row)
		}
	}
	if withManifest {
		members = append([]namedMember{{"manifest.jsonl", strings.Join(rows, "\n") + "\n"}}, members...)
	}
	return members
}

// wantOnePlaceProblems checks a collection read from onePlaceMembers holds,
// in order, a line naming each member not extracted and the member kept,
// then, with a manifest, a missing line for the row of each, in row order.
func wantOnePlaceProblems(t *testing.T, collection *Collection, test onePlaceCase, withManifest bool) {
	t.Helper()
	prefix := onePlacePrefix(withManifest)
	var want []string
	for _, name := range test.notExtracted {
		want = append(want, ": member "+prefix+name+" not extracted: its name and that of member "+
			prefix+test.kept+" name one place when a backslash separates folders; "+prefix+test.kept+" is kept")
	}
	var rows []string
	for _, name := range test.names {
		if withManifest && name != test.kept && !slices.Contains(rows, name) {
			rows = append(rows, name)
			want = append(want, "/home/u/.claude/projects/p/"+name+": missing from the collection")
		}
	}
	wantEqual(t, "problem count", len(collection.Problems), len(want))
	for i, line := range want {
		if !strings.HasSuffix(collection.Problems[i], line) {
			t.Errorf("problem %d = %q, want one ending %q", i, collection.Problems[i], line)
		}
	}
}

// wantBody checks that home gives a path for rel that holds body.
func wantBody(t *testing.T, home *Home, rel, body string) {
	t.Helper()
	path, ok := home.PathOf(rel)
	wantEqual(t, "the home gives "+rel, ok, true)
	data, err := os.ReadFile(path)
	opened(t, err)
	wantEqual(t, "the body at "+rel, string(data), body)
}

// wantOnePlaceKept checks a collection read from onePlaceMembers: the kept
// member's file is its one artifact, holding its own body, which the home
// gives for the place and for the kept member's path; the home gives no
// member not extracted that file, and the problem lines are
// wantOnePlaceProblems'.
func wantOnePlaceKept(t *testing.T, collection *Collection, test onePlaceCase, withManifest bool) {
	t.Helper()
	keptRel, placeRel := ".claude/projects/p/"+test.kept, ".claude/projects/p/"+test.place
	if !withManifest {
		keptRel = placeRel
	}
	wantSlice(t, "artifacts", rels(collection), []string{keptRel})
	artifact := collection.Artifacts[0]
	data, err := os.ReadFile(artifact.DiskPath)
	opened(t, err)
	wantEqual(t, "the artifact's body", string(data), test.kept)
	wantBody(t, artifact.Home, placeRel, test.kept)
	if plainParts(splitSlash(keptRel)) {
		wantBody(t, artifact.Home, keptRel, test.kept)
	}
	for _, name := range test.notExtracted {
		if path, ok := artifact.Home.PathOf(".claude/projects/p/" + name); ok {
			if data, err := os.ReadFile(path); err == nil && string(data) == test.kept {
				t.Errorf("the home gives %s the kept member's file %s", name, path)
			}
		}
	}
	wantOnePlaceProblems(t, collection, test, withManifest)
}

func TestTheZipMemberWhoseNameIsThePlaceKeepsIt(t *testing.T) {
	t.Parallel()
	for _, system := range zipSystems() {
		for _, test := range onePlaceCases() {
			for _, withManifest := range []bool{false, true} {
				t.Run(fmt.Sprintf("system %d, %s, manifest %t", system, test.label, withManifest), func(t *testing.T) {
					t.Parallel()
					collection, err := openArchiveOf(t, "c.zip",
						zipOfMembers(t, system, onePlaceMembers(test, withManifest)...))
					opened(t, err)
					wantOnePlaceKept(t, collection, test, withManifest)
				})
			}
		}
	}
}

// toolOutputMembers returns a manifest, a Claude Code transcript p/s1.jsonl,
// its saved tool output below its session folder and a planted file, named
// plantedName below p, that a backslash puts at the same place or at one
// the file system holds as the same entry, the planted one first when
// plantedFirst.
func toolOutputMembers(plantedFirst bool, plantedName string) []namedMember {
	const prefix = "fs/home/u/.claude/projects/p/"
	genuine := namedMember{prefix + "s1/tool-results/out.txt", "genuine"}
	planted := namedMember{prefix + plantedName, "planted"}
	files := []namedMember{genuine, planted}
	if plantedFirst {
		files = []namedMember{planted, genuine}
	}
	var rows []string
	for _, name := range []string{"s1.jsonl", "s1/tool-results/out.txt", plantedName} {
		rows = append(rows, collectedRow("u", "/home/u", "claude-code", "/home/u/.claude/projects/p/"+name,
			prefix+name))
	}
	members := []namedMember{{"manifest.jsonl", strings.Join(rows, "\n") + "\n"}, {prefix + "s1.jsonl", "{}\n"}}
	return append(members, files...)
}

// wantGenuineToolOutput checks that the lookup a parser makes for a
// transcript's saved tool output, its session folder and then the names
// below it, reads the genuine file.
func wantGenuineToolOutput(t *testing.T, collection *Collection) {
	t.Helper()
	index := slices.IndexFunc(collection.Artifacts, func(artifact *Artifact) bool {
		return artifact.Rel == ".claude/projects/p/s1.jsonl"
	})
	wantEqual(t, "the transcript is an artifact", index >= 0, true)
	folder, ok := collection.Artifacts[index].Home.PathOf(".claude/projects/p/s1")
	wantEqual(t, "the home gives the session folder", ok, true)
	output, ok := Below(folder, "tool-results", "out.txt")
	wantEqual(t, "the tool output is below the session folder", ok, true)
	data, err := os.ReadFile(output)
	opened(t, err)
	wantEqual(t, "the tool output", string(data), "genuine")
}

// plantedToolOutputNames returns names below p that a backslash puts at
// the saved tool output's place: with "." parts, and differing in case,
// which a file system that folds case holds as the same entry.
func plantedToolOutputNames() []string {
	return []string{`s1\.\tool-results\out.txt`, `s1\tool-results\OUT.txt`}
}

func TestATranscriptsSavedToolOutputIsTheMemberNamedWithSlashes(t *testing.T) {
	t.Parallel()
	for _, system := range zipSystems() {
		for _, planted := range plantedToolOutputNames() {
			for _, plantedFirst := range []bool{false, true} {
				t.Run(fmt.Sprintf("system %d, %s, planted first %t", system, planted, plantedFirst), func(t *testing.T) {
					t.Parallel()
					collection, err := openArchiveOf(t, "c.zip",
						zipOfMembers(t, system, toolOutputMembers(plantedFirst, planted)...))
					opened(t, err)
					wantGenuineToolOutput(t, collection)
				})
			}
		}
	}
}

func TestAZipMemberNamedTwiceIsReplacedWithNoProblem(t *testing.T) {
	t.Parallel()
	for _, system := range zipSystems() {
		for _, name := range []string{"fs/home/u/.claude/projects/p/a/b.jsonl",
			`fs\home\u\.claude\projects\p\a\b.jsonl`} {
			manifest := collectedRow("u", "/home/u", "claude-code", "/home/u/.claude/projects/p/a/b.jsonl", name) + "\n"
			t.Run(fmt.Sprintf("system %d, %s", system, name), func(t *testing.T) {
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
}

// meetingCase is the members of an archive, by name below a home's
// .claude/projects/p, of which the backslashed ones reach a place a
// slashed one makes: the same place as another kind of entry, or one the
// file system holds as the same entry. A name ending in a slash is a
// folder. kept are the files that stay, and reason, given the archive's
// path to p, why each backslashed member is not extracted.
type meetingCase struct {
	label                string
	slashed, backslashed []string
	kept                 []string
	folds                bool
	reason               func(prefix string) string
}

// meetingCases returns a backslashed folder at a slashed file's place, a
// backslashed file at a slashed folder's place, a backslashed file whose
// name differs from a slashed one in case, and a backslashed name making
// a folder that differs in case from a slashed one on the way down.
func meetingCases() []meetingCase {
	anotherKind := func(prefix string) string {
		return "member " + prefix + "a/b.jsonl holds its place as another kind of entry"
	}
	oneEntry := func(kept string) func(string) string {
		return func(prefix string) string {
			return "this file system holds its name and that of member " + prefix + kept + " as one entry; " +
				prefix + kept + " is kept"
		}
	}
	return []meetingCase{
		{"a backslashed folder at a slashed file's place", []string{"a/b.jsonl"},
			[]string{`a\b.jsonl/`, `a\b.jsonl/x.jsonl`}, []string{"a/b.jsonl"}, false, anotherKind},
		{"a backslashed file at a slashed folder's place", []string{"a/b.jsonl/x.jsonl"},
			[]string{`a\b.jsonl`}, []string{"a/b.jsonl/x.jsonl"}, false, anotherKind},
		{"names differing in case", []string{"a/b.jsonl"}, []string{`a\B.jsonl`}, []string{"a/b.jsonl"},
			true, oneEntry("a/b.jsonl")},
		{"a folder on the way down differing in case", []string{"A/x.jsonl"}, []string{`a\y.jsonl`},
			[]string{"A/x.jsonl"}, true, oneEntry("A")},
	}
}

// meetingMembers returns the members of a case, each file holding its own
// name and each folder nothing, the backslashed first when backslashedFirst, after a manifest with
// a row for each file, slashed then backslashed, when withManifest.
func meetingMembers(test meetingCase, backslashedFirst, withManifest bool) []namedMember {
	prefix := onePlacePrefix(withManifest)
	names := append(slices.Clone(test.slashed), test.backslashed...)
	if backslashedFirst {
		names = append(slices.Clone(test.backslashed), test.slashed...)
	}
	var members []namedMember
	for _, name := range names {
		body := name
		if strings.HasSuffix(name, "/") {
			body = ""
		}
		members = append(members, namedMember{prefix + name, body})
	}
	var rows []string
	for _, name := range append(slices.Clone(test.slashed), test.backslashed...) {
		if !strings.HasSuffix(name, "/") {
			rows = append(rows, collectedRow("u", "/home/u", "claude-code", "/home/u/.claude/projects/p/"+name,
				prefix+name))
		}
	}
	if withManifest {
		members = append([]namedMember{{"manifest.jsonl", strings.Join(rows, "\n") + "\n"}}, members...)
	}
	return members
}

// wantMeetingOutcome checks a collection read from meetingMembers: the kept
// files are its artifacts, each holding its own name, and its problems are
// a line for each backslashed member, in archive order, then, with a
// manifest, a missing line for each backslashed file's row.
func wantMeetingOutcome(t *testing.T, collection *Collection, test meetingCase, withManifest bool) {
	t.Helper()
	prefix := onePlacePrefix(withManifest)
	var wantRels []string
	for _, name := range test.kept {
		wantRels = append(wantRels, ".claude/projects/p/"+name)
	}
	wantSlice(t, "artifacts", slices.Sorted(slices.Values(rels(collection))), wantRels)
	for _, artifact := range collection.Artifacts {
		data, err := os.ReadFile(artifact.DiskPath)
		opened(t, err)
		wantEqual(t, "the body of "+artifact.Rel, ".claude/projects/p/"+string(data), artifact.Rel)
	}
	var want []string
	for _, name := range test.backslashed {
		want = append(want, ": member "+prefix+name+" not extracted: "+test.reason(prefix))
	}
	for _, name := range test.backslashed {
		if withManifest && !strings.HasSuffix(name, "/") {
			want = append(want, "/home/u/.claude/projects/p/"+name+": missing from the collection")
		}
	}
	wantEqual(t, "problem count", len(collection.Problems), len(want))
	for i, line := range want {
		if i < len(collection.Problems) && !strings.HasSuffix(collection.Problems[i], line) {
			t.Errorf("problem %d = %q, want one ending %q", i, collection.Problems[i], line)
		}
	}
}

func TestASlashedZipMemberKeepsItsPlaceHoweverABackslashedOneMeetsIt(t *testing.T) {
	t.Parallel()
	folds := foldsNames(t, t.TempDir(), "B", "b")
	for _, system := range zipSystems() {
		for _, test := range meetingCases() {
			for _, backslashedFirst := range []bool{false, true} {
				for _, withManifest := range []bool{false, true} {
					t.Run(fmt.Sprintf("system %d, %s, backslashed first %t, manifest %t", system, test.label,
						backslashedFirst, withManifest), func(t *testing.T) {
						t.Parallel()
						if test.folds && !folds {
							t.Skip("this file system keeps the two names apart")
						}
						collection, err := openArchiveOf(t, "c.zip",
							zipOfMembers(t, system, meetingMembers(test, backslashedFirst, withManifest)...))
						opened(t, err)
						wantMeetingOutcome(t, collection, test, withManifest)
					})
				}
			}
		}
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
