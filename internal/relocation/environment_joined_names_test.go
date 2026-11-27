package relocation

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/syspath"
)

// joinedCase is one /etc/environment text and the names only its joins make
// whole, each on its join's first line, with the reason the value reader
// gives each.
type joinedCase struct {
	name, text string
	want       []assignment
}

// continued is the assignment a joined name gets on line.
func continued(line int, name string) assignment {
	return assignment{Line: line, Name: name, Reason: "continued"}
}

// joinedCases returns the rules of the pam_env join the name search follows.
func joinedCases() []joinedCase {
	return []joinedCase{
		{"a name split by a backslash", "CODEX_\\\nHOME=/opt/elsewhere\n",
			[]assignment{continued(1, "CODEX_HOME")}},
		{"blanks after the backslash", "CODEX_\\ \t\nHOME=/x\n", []assignment{continued(1, "CODEX_HOME")}},
		{"the next line's leading blanks are dropped", "CODEX_\\\n \tHOME=/x\n",
			[]assignment{continued(1, "CODEX_HOME")}},
		{"a blank before the backslash is kept", "CODEX_ \\\nHOME=/x\n", nil},
		{"a join of three lines", "A=1 \\\nCODEX_\\\nHOME=/x\n", []assignment{continued(1, "CODEX_HOME")}},
		{"a hash ends the line, so it is not joined", "CODEX_#\\\nHOME=/x\n", nil},
		{"text after a hash on the last line is cut", "CODEX_\\\nH#OME=/x\n", nil},
		{"a blank line in the join is passed over before pam_env 1.6",
			"CODEX_\\\n\nHOME=/x\n", []assignment{continued(1, "CODEX_HOME")}},
		{"a comment line in the join is passed over before pam_env 1.6",
			"CODEX_\\\n# note\nHOME=/x\n", []assignment{continued(1, "CODEX_HOME")}},
		{"a blank line ends the join from pam_env 1.6, which starts another",
			"A=\\\n\nCODEX_\\\nHOME=/x\n", []assignment{continued(1, "CODEX_HOME"), continued(3, "CODEX_HOME")}},
		{"a name a line holds whole is that line's hit", "CODEX_HOME=/a\\\nb\n",
			[]assignment{continued(1, "CODEX_HOME")}},
		{"a join the file ends in", "CODEX_\\\nHOME\\", []assignment{continued(1, "CODEX_HOME")}},
		{"a NUL reads as a space", "A=\x00CODEX_\\\nHOME=/x\n",
			[]assignment{{Line: 1, Name: "CODEX_HOME", Reason: "after a NUL byte"}}},
		{"a NUL splits a name", "CODEX\x00\\\n_HOME=/x\n", nil},
		{"two names in one join", "CODEX_\\\nHOME=/x AMP_\\\nHOME=/y\n",
			[]assignment{continued(1, "CODEX_HOME"), continued(1, "AMP_HOME")}},
	}
}

func TestResolveEnvironmentRefusesANameOnlyAJoinMakesWhole(t *testing.T) {
	for _, testCase := range joinedCases() {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := resolveEnvironment(strings.NewReader(testCase.text), testNames())
			if err != nil || !slices.Equal(got, testCase.want) {
				t.Errorf("got %#v, %v; want %#v", got, err, testCase.want)
			}
		})
	}
}

func TestJoinedNamesAreTheJoinsHits(t *testing.T) {
	for _, testCase := range joinedCases() {
		t.Run(testCase.name, func(t *testing.T) {
			var want []lineName
			for _, refused := range testCase.want {
				if refused.Value == "" && !strings.Contains(testCase.text, refused.Name) {
					want = append(want, lineName{Line: refused.Line, Name: refused.Name})
				}
			}
			got, err := joinedNames(strings.NewReader(testCase.text), testNames())
			if err != nil || !slices.Equal(got, want) {
				t.Errorf("got %v, %v; want %v", got, err, want)
			}
		})
	}
}

func TestSearchHintsANameSplitAcrossAnEnvironmentJoin(t *testing.T) {
	root := syspath.FromNative(t.TempDir())
	split := "LANG=C\nCODEX_\\\nHOME=/opt/elsewhere\n"
	writeTree(t, root, map[string]string{
		"etc/environment":                         split,
		"etc/environment.d/split.conf":            split,
		"home/alice/.config/environment.d/a.conf": split,
	})
	log, collected := &lineLog{}, []string{}
	if _, err := Search(context.Background(), searchDeps(t, root, log, &collected)); err != nil {
		t.Fatal(err)
	}
	// systemd's environment.d ends a name at the line end, so its files give
	// no hint for the same text.
	want := nativeLines([]string{"Relocation hints in system files",
		"relocation hint: " + root + "/etc/environment:2 names CODEX_HOME (codex-cli); " +
			"value not resolved: continued"})
	if !slices.Equal(log.lines, want) {
		t.Errorf("log:\n%s\nwant:\n%s", strings.Join(log.lines, "\n"), strings.Join(want, "\n"))
	}
	if wantCollected := []string{"||" + root + "/etc/environment"}; !slices.Equal(collected, wantCollected) {
		t.Errorf("collected = %q, want %q", collected, wantCollected)
	}
}
