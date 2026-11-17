//go:build parity

package parity

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
)

// normaliser returns the run with what differs by design between the two
// collectors dropped. binary says the run is the Go binary's; logOnStderr
// says stderr carries the log, as it does without -q.
type normaliser func(run collectorRun, binary, logOnStderr bool) collectorRun

// normalMembers returns the archive's members as sorted "name hash" lines.
// The archived manifest is hashed after the manifest normalisation, since its
// atime and error texts differ by design, and the archived collection.json
// and collector.log are listed without a hash: the files beside the archive
// are the ones compared. manifest normalises the archived manifest's text.
func normalMembers(members map[string]string, manifest func(string) string) string {
	var lines []string
	for _, name := range slices.Sorted(maps.Keys(members)) {
		hash := members[name]
		switch name {
		case "./collection.json", "./collector.log":
			hash = "-"
		case "./manifest.jsonl":
			hash = hashText(manifest(hash))
		}
		lines = append(lines, fmt.Sprintf("%q %s", name, hash))
	}
	return strings.Join(lines, "\n")
}

// normalLinkTarget returns a link target with / separators and without the
// extended-length prefix: \\?\ as the binary and the PowerShell writer store
// it, //?/ as tar.exe stores it after turning every \ into /, and the UNC
// form of either (\\?\UNC\server\share) as //server/share.
func normalLinkTarget(target string) string {
	target = strings.ReplaceAll(target, `\`, "/")
	if rest, ok := strings.CutPrefix(target, "//?/UNC/"); ok {
		return "//" + rest
	}
	return strings.TrimPrefix(target, "//?/")
}

// emptyLinkTarget is the description of a link member stored without its
// target.
const emptyLinkTarget = `link ""`

// emptyLinkTargets returns the names of the link members stored without a
// target. The PowerShell collector's tar.exe has been seen to store a link
// so (one member, in the case whose output directory lies inside the image,
// on the lab VM, where tar.exe writes the archive); the cause was not
// established. That is a defect of its archiver, and the binary's target for
// the member is the real one.
func emptyLinkTargets(members map[string]string) map[string]bool {
	names := map[string]bool{}
	for name, description := range members {
		if description == emptyLinkTarget {
			names[name] = true
		}
	}
	return names
}

// blankLinkTargets returns members with the target of each link member
// named in names left out, so the member, its type and its name are still
// compared.
func blankLinkTargets(members map[string]string, names map[string]bool) map[string]string {
	blanked := make(map[string]string, len(members))
	for name, description := range members {
		if names[name] && strings.HasPrefix(description, "link ") {
			description = emptyLinkTarget
		}
		blanked[name] = description
	}
	return blanked
}

func TestAnEmptyReferenceLinkTargetToleratesOnlyThatMembersTarget(t *testing.T) {
	reference := map[string]string{
		"./fs/empty": emptyLinkTarget, "./fs/kept": `link "C:/a"`,
		"./fs/only-reference": emptyLinkTarget, "./fs/file": "hash1",
	}
	binary := blankLinkTargets(map[string]string{
		"./fs/empty": `link "C:/elsewhere/out"`, "./fs/kept": `link "C:/b"`,
		"./fs/only-binary": `link "C:/c"`, "./fs/file": "hash2",
	}, emptyLinkTargets(reference))
	cases := []struct {
		name, member string
		wantEqual    bool
	}{
		{"an empty reference target accepts any binary target", "./fs/empty", true},
		{"a non-empty reference target must still match", "./fs/kept", false},
		{"a member only the reference has still differs", "./fs/only-reference", false},
		{"a member only the binary has still differs", "./fs/only-binary", false},
		{"a file member is left as it was", "./fs/file", false},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			want, inReference := reference[test.member]
			got, inBinary := binary[test.member]
			if equal := inReference && inBinary && want == got; equal != test.wantEqual {
				t.Errorf("member %s: reference %q, binary %q", test.member, want, got)
			}
		})
	}
}

func TestNormalLinkTargetDropsTheExtendedPrefixInEitherSpelling(t *testing.T) {
	cases := []struct {
		name, target, want string
	}{
		{"a drive path with the backslash prefix", `\\?\C:\Users\a\x`, "C:/Users/a/x"},
		{"a drive path with the slash prefix tar.exe writes", "//?/C:/Users/a/x", "C:/Users/a/x"},
		{"a UNC path with the backslash prefix", `\\?\UNC\server\share\x`, "//server/share/x"},
		{"a UNC path with the slash prefix", "//?/UNC/server/share/x", "//server/share/x"},
		{"a relative target keeps its text", `..\dotfiles\camel`, "../dotfiles/camel"},
		{"a plain UNC path keeps its two slashes", `\\server\share\x`, "//server/share/x"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			if got := normalLinkTarget(test.target); got != test.want {
				t.Errorf("normalLinkTarget(%q) = %q, want %q", test.target, got, test.want)
			}
		})
	}
}

// compareRuns reports each part of the two normalised runs that differs, with
// the first differing line.
func compareRuns(t *testing.T, want, got collectorRun) {
	t.Helper()
	if want.code != got.code {
		t.Errorf("exit code %d, reference collector %d", got.code, want.code)
	}
	parts := map[string][2]string{"stdout": {want.stdout, got.stdout},
		"stderr": {want.stderr, got.stderr}}
	for suffix, text := range want.files {
		parts[suffix] = [2]string{text, got.files[suffix]}
	}
	for part, texts := range parts {
		for _, line := range differingLines(texts[0], texts[1], 3) {
			t.Errorf("%s differs at line %d:\nwant: %q\n got: %q", part, line.number, line.want, line.got)
		}
	}
}

// differingLine is one line where two texts differ.
type differingLine struct {
	number    int
	want, got string
}

// differingLines returns the first limit lines where the texts differ.
func differingLines(want, got string, limit int) []differingLine {
	if want == got {
		return nil
	}
	var found []differingLine
	wantLines, gotLines := strings.Split(want, "\n"), strings.Split(got, "\n")
	for i := 0; i < max(len(wantLines), len(gotLines)) && len(found) < limit; i++ {
		if lineAt(wantLines, i) != lineAt(gotLines, i) {
			found = append(found, differingLine{i + 1, lineAt(wantLines, i), lineAt(gotLines, i)})
		}
	}
	return found
}

// lineAt returns line i of lines, or a marker past the end.
func lineAt(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "<end>"
}
