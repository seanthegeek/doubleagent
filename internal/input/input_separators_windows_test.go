package input

import (
	"archive/tar"
	"fmt"
	"path/filepath"
	"strings"
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
// directory when its name ends in a slash and a regular file otherwise.
func tarOfMembers(t *testing.T, members []namedMember) []byte {
	t.Helper()
	var tarMembers []tarMember
	for _, member := range members {
		typeflag := byte(tar.TypeReg)
		if strings.HasSuffix(member.name, "/") {
			typeflag = tar.TypeDir
		}
		tarMembers = append(tarMembers, tarMember{name: member.name, typeflag: typeflag, body: member.body})
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
	for _, planted := range plantedToolOutputNames() {
		for _, plantedFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s, planted first %t", planted, plantedFirst), func(t *testing.T) {
				t.Parallel()
				collection, err := openArchiveOf(t, "c.tar", tarOfMembers(t, toolOutputMembers(plantedFirst, planted)))
				opened(t, err)
				wantGenuineToolOutput(t, collection)
			})
		}
	}
}

func TestASlashedTarMemberKeepsItsPlaceHoweverABackslashedOneMeetsItOnWindows(t *testing.T) {
	t.Parallel()
	folds := foldsNames(t, t.TempDir(), "B", "b")
	for _, test := range meetingCases() {
		for _, backslashedFirst := range []bool{false, true} {
			for _, withManifest := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s, backslashed first %t, manifest %t", test.label, backslashedFirst,
					withManifest), func(t *testing.T) {
					t.Parallel()
					if test.folds && !folds {
						t.Skip("this file system keeps the two names apart")
					}
					collection, err := openArchiveOf(t, "c.tar",
						tarOfMembers(t, meetingMembers(test, backslashedFirst, withManifest)))
					opened(t, err)
					wantMeetingOutcome(t, collection, test, withManifest)
				})
			}
		}
	}
}

func TestATarMemberWithDotPartsBetweenBackslashesIsItsRowsArtifactOnWindows(t *testing.T) {
	t.Parallel()
	collection, err := openArchiveOf(t, "c.tar", tarOfMembers(t, loneDottedMembers()))
	opened(t, err)
	wantLoneDottedArtifact(t, collection)
}
