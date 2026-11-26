package input

import (
	"errors"
	"io/fs"
	"slices"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/osfile"
)

// longPartReason is the reason a member is skipped whose name holds a part
// longer than NTFS stores: the system refuses the part as an invalid name.
const longPartReason = reasonNotAName

// NTFS stores a device name and a name ending in a dot or a space under the
// \\?\ form osfile opens, and refuses a question mark itself.
func TestWindowsExtractsEveryMemberNameTheSystemStores(t *testing.T) {
	stored := []string{"plain.jsonl", "CON.jsonl", "dot..jsonl.", "space.jsonl "}
	var members []tarMember
	for _, name := range append(stored, "q?.jsonl") {
		members = append(members, tarMember{name: "Users/u/.claude/projects/p/" + name, typeflag: '0', body: "{}\n"})
	}
	collection, err := openArchiveOf(t, "x.tar", tarBytes(t, false, members...))
	opened(t, err)
	wantProblem(t, collection, "member Users/u/.claude/projects/p/q?.jsonl not extracted: "+reasonNotAName)
	if len(collection.Problems) != 1 {
		t.Fatalf("problems %q", collection.Problems)
	}
	var want []string
	for _, name := range stored {
		want = append(want, ".claude/projects/p/"+name)
	}
	slices.Sort(want)
	wantSlice(t, "artifacts", rels(collection), want)
	for _, artifact := range collection.Artifacts {
		if _, err := osfile.Lstat(artifact.DiskPath); err != nil {
			t.Errorf("%s: %v", artifact.Rel, err)
		}
	}
	directory := collection.ExtractedDir()
	opened(t, collection.Cleanup())
	if _, err := osfile.Lstat(directory); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("the extraction directory is left: %v", err)
	}
}
