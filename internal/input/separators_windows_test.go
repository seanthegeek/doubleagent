package input

import (
	"archive/tar"
	"fmt"
	"path/filepath"
	"testing"
)

func TestAPOSIXRowWithABackslashIsFoundInACollectionDirectoryOnWindows(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeManifest(t, root, collectedRow("alice", "/home/alice", "claude-code", `/home/alice/.claude/a\b.jsonl`,
		`fs/home/alice/.claude/a\b.jsonl`))
	writeFile(t, filepath.Join(root, "fs", "home", "alice", ".claude", "a", "b.jsonl"), "{}")
	collection, err := Open(t.Context(), root, Options{Matcher: matcherOf(t, "")})
	opened(t, err)
	wantSlice(t, "problems", collection.Problems, nil)
	wantSlice(t, "artifacts", rels(collection), []string{`.claude/a\b.jsonl`})
}

// tarOfMembers returns a tar archive holding members in order, each a
// regular file.
func tarOfMembers(t *testing.T, members []namedMember) []byte {
	t.Helper()
	var tarMembers []tarMember
	for _, member := range members {
		tarMembers = append(tarMembers, tarMember{name: member.name, typeflag: tar.TypeReg, body: member.body})
	}
	return tarBytes(t, false, tarMembers...)
}

func TestTheTarMemberWhoseNameIsThePlaceKeepsItOnWindows(t *testing.T) {
	t.Parallel()
	for _, test := range onePlaceCases() {
		for _, withManifest := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s, manifest %t", test.label, withManifest), func(t *testing.T) {
				t.Parallel()
				collection, err := openArchiveOf(t, "c.tar", tarOfMembers(t, onePlaceMembers(test, withManifest)))
				opened(t, err)
				wantOnePlaceKept(t, collection, test, withManifest)
			})
		}
	}
}

func TestATranscriptsSavedToolOutputIsTheTarMemberNamedWithSlashesOnWindows(t *testing.T) {
	t.Parallel()
	for _, plantedFirst := range []bool{false, true} {
		t.Run(fmt.Sprintf("planted first %t", plantedFirst), func(t *testing.T) {
			t.Parallel()
			collection, err := openArchiveOf(t, "c.tar", tarOfMembers(t, toolOutputMembers(plantedFirst)))
			opened(t, err)
			wantGenuineToolOutput(t, collection)
		})
	}
}

func TestATarMemberWithDotPartsBetweenBackslashesIsItsRowsArtifactOnWindows(t *testing.T) {
	t.Parallel()
	collection, err := openArchiveOf(t, "c.tar", tarOfMembers(t, loneDottedMembers()))
	opened(t, err)
	wantLoneDottedArtifact(t, collection)
}
