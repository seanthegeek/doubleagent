//go:build parity

package parity

import (
	"fmt"
	"sort"
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
	names := make([]string, 0, len(members))
	for name := range members {
		names = append(names, name)
	}
	sort.Strings(names)
	var lines []string
	for _, name := range names {
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
