//go:build parity

package replay

import (
	"regexp"
	"slices"
	"strings"
	"testing"
)

// The recordings come from an analyzer that left some identity values out
// of a row: an id that names an account, an organisation or a machine and
// grants no access. The binary shows each in both modes, since it is not a
// secret, and difference 232 compares the row with the id with the row the
// recording gave without it.

// shownDifference returns difference 232.
func shownDifference() Difference {
	return Difference{Number: 232, Summary: "an identity the recording left out of a row is shown: at each " +
		"site of shownForms, the recording's row and the binary's row with the identity become one token " +
		"that keeps the rest of the row", Text: shownText(recordedSide), Binary: shownText(binarySide),
		RowText: func(agent, text string) (string, int) {
			return rowRewriteAll(shownForms(), recordedSide, agent, text)
		}, RowBinary: func(agent, text string) (string, int) {
			return rowRewriteAll(shownForms(), binarySide, agent, text)
		}, Within: 102}
}

// shownForms returns the entries of difference 232, each a parser's site
// where the recording gave a row without an identity the binary shows, in
// the form of withheldForms; a run of every case fails for an entry no case
// used.
func shownForms() []withheldForm {
	form := func(agent, name, recorded, binary, token string) withheldForm {
		return withheldForm{scope: parserScope, agent: agent, name: name, recorded: regexp.MustCompile(recorded),
			binary: regexp.MustCompile(binary), token: token}
	}
	return []withheldForm{
		// The recording said only that the machine id was set.
		form("claude-code", "remote control machine id",
			`remote control: machine id set( \(from [^"\n]*\.claude\.json, file mtime\))`,
			`remote control: machine id [0-9A-Fa-f-]+( \(from [^"\n]*\.claude\.json, file mtime\))`,
			"remote control: machine id {id}${1}"),
		// The recording gave the GitHub connection without its account
		// and organisation ids.
		{scope: parserScope, agent: "claude-code", name: "GitHub connection ids",
			recorded: regexp.MustCompile(`(source host: github status=[^ "\n]*)( \(from [^"\n]*\.claude\.json, ` +
				`file mtime\))`),
			added: regexp.MustCompile(` (?:account|org)=[0-9A-Za-z-]+`), token: "${1}{added ids}${2}"},
	}
}

// shownText returns the rewrite of difference 232 for one side: each
// entry's form of that side as its token, and the count.
func shownText(from side) func(text string) (string, int) {
	return func(text string) (string, int) {
		total := 0
		for _, entry := range shownForms() {
			var found int
			text, found = entry.rewrite(from, text)
			total += found
		}
		return text, total
	}
}

// shownEntriesIn returns the labels of the entries whose recorded form
// shows in a row of their agent among rows.
func shownEntriesIn(rows []Row) []string {
	return rowEntriesIn(shownForms(), rows)
}

// checkEveryShownFormUsed fails the test for each entry of shownForms that
// no case of a whole run of the parser replay needed.
func checkEveryShownFormUsed(t testing.TB, used map[string]int, whole bool) {
	t.Helper()
	if !whole {
		return
	}
	for _, entry := range shownForms() {
		if used[entry.label()] == 0 {
			t.Errorf("difference 232's entry %s is used by no recorded case: remove it", entry.label())
			continue
		}
		t.Logf("difference 232's entry %s used by %d cases", entry.label(), used[entry.label()])
	}
}

func TestAShownIdentityIsComparedWithTheRecordingsRowWithoutIt(t *testing.T) {
	machineRecorded := "remote control: machine id set (from .claude.json, file mtime)"
	machineBinary := "remote control: machine id 0f0e0d0c-0000-4000-8000-000000000001 (from .claude.json, file mtime)"
	githubRecorded := "source host: github status=connected (from .claude.json, file mtime)"
	githubBinary := "source host: github status=connected account=a1b2 org=c3d4 (from .claude.json, file mtime)"
	tests := []struct {
		name, want, got string
		agree           bool
	}{
		{"a machine id passes", machineRecorded, machineBinary, true},
		{"a machine id of another file fails", machineRecorded,
			strings.Replace(machineBinary, ".claude.json", ".claude-work/.claude.json", 1), false},
		{"a machine id that is not an id fails", machineRecorded,
			strings.Replace(machineBinary, "0f0e0d0c-0000-4000-8000-000000000001", "REDACTED x", 1), false},
		{"the GitHub connection's ids pass", githubRecorded, githubBinary, true},
		{"the GitHub connection without the ids the binary adds fails", githubRecorded, githubRecorded, false},
		{"the GitHub connection of another status fails", githubRecorded,
			strings.Replace(githubBinary, "connected", "error", 1), false},
		{"the GitHub connection with another field fails", githubRecorded,
			strings.Replace(githubBinary, " org=", " user=", 1), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reporter := &recordingTB{TB: t}
			want, got := NormaliseRows(reporter, "n", []int{102, 232}, []Row{{Agent: "claude-code", Text: Text(test.want)}},
				[]Row{{Agent: "claude-code", Text: Text(test.got)}})
			if agree := want[0].Text == got[0].Text; agree != test.agree {
				t.Errorf("agree %v, want %v: %q and %q", agree, test.agree, want[0].Text, got[0].Text)
			}
		})
	}
}

func TestEachShownFormIsFoundInItsRecordedText(t *testing.T) {
	texts := map[string]string{
		"claude-code remote control machine id": "remote control: machine id set (from .claude.json, file mtime)",
		"claude-code GitHub connection ids":     "source host: github status=- (from .claude.json, file mtime)",
	}
	for _, entry := range shownForms() {
		t.Run(entry.label(), func(t *testing.T) {
			text, ok := texts[entry.label()]
			if !ok {
				t.Fatal("no sample text for this entry")
			}
			if got := shownEntriesIn([]Row{{Agent: Text(entry.agent), Text: Text(text)}}); !slices.Equal(got, []string{entry.label()}) {
				t.Errorf("found %q", got)
			}
		})
	}
}
