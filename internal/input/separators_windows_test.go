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

func TestTwoTarMembersAtOnePlaceKeepTheFirstOnWindows(t *testing.T) {
	t.Parallel()
	for _, pair := range onePlacePairs() {
		for _, slashFirst := range []bool{false, true} {
			for _, withManifest := range []bool{false, true} {
				name := fmt.Sprintf("names differing %s, slash first %t, manifest %t", pair.label, slashFirst,
					withManifest)
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					members := onePlaceMembers(pair, slashFirst, withManifest)
					var tarMembers []tarMember
					for _, member := range members {
						tarMembers = append(tarMembers, tarMember{name: member.name, typeflag: tar.TypeReg,
							body: member.body})
					}
					collection, err := openArchiveOf(t, "c.tar", tarBytes(t, false, tarMembers...))
					opened(t, err)
					wantFirstMemberKept(t, collection, members, pair)
				})
			}
		}
	}
}
