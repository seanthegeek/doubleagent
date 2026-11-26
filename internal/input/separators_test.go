package input

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"fmt"
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
