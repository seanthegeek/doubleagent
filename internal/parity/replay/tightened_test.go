//go:build parity

package replay

import (
	"slices"
	"strconv"
	"strings"
	"testing"
)

// recordLine returns a canonical timeline line of codex-cli with the given
// time, user, turn type, text, source file and line.
func recordLine(timestamp, user, turnType, text, sourceFile string, sourceLine int) string {
	return `{"agent":"codex-cli","git_branch":"","host":"h","model":"","project_path":"","recovered":"",` +
		`"session_id":"s","source_file":"` + sourceFile + `","source_line":` + strconv.Itoa(sourceLine) +
		`,"text":"` + text + `","timestamp_utc":"` + timestamp + `","tool_name":"","tool_use_id":"",` +
		`"turn_type":"` + turnType + `","user":"` + user + `"}`
}

// timelinesAgree compares two timeline.jsonl texts as the command replay
// does, the loose tree's documented order included, for a run whose
// sources hold the times of cut.
func timelinesAgree(t *testing.T, loose bool, want, got []string, cut *cutTimes) bool {
	wantRun := commandOutput{outputs: map[string]string{"out/timeline.jsonl": strings.Join(want, "\n") + "\n"},
		loose: loose, cut: cut}
	gotRun := commandOutput{outputs: map[string]string{"out/timeline.jsonl": strings.Join(got, "\n") + "\n"},
		loose: loose}
	reporter := &recordingTB{TB: t}
	compareInformation(reporter, "n", "/root", wantRun, gotRun)
	if loose {
		checkDocumentedOrder(reporter, "n", gotRun.outputs)
	}
	return len(reporter.errors) == 0
}

func TestALooseTimelineComparesTheOrderInsideEachFile(t *testing.T) {
	const moment = "2026-01-01T00:00:00.000Z"
	assistant := recordLine(moment, "u", "assistant", "hi", "/r/a.jsonl", 2)
	toolUse := recordLine(moment, "u", "tool_use", "ls", "/r/a.jsonl", 2)
	other := recordLine(moment, "u", "user", "yo", "/r/b.jsonl", 1)
	tests := []struct {
		name      string
		want, got []string
		agree     bool
	}{
		{"the recorded order passes", []string{assistant, toolUse, other}, []string{assistant, toolUse, other}, true},
		{"two rows of one record in the other order fail", []string{assistant, toolUse}, []string{toolUse, assistant},
			false},
		{"the same swap beside another file's row fails", []string{assistant, toolUse, other},
			[]string{toolUse, assistant, other}, false},
		{"a changed row of another file still fails", []string{assistant, toolUse, other},
			[]string{assistant, toolUse, strings.Replace(other, `"yo"`, `"ya"`, 1)}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if agree := timelinesAgree(t, true, test.want, test.got, nil); agree != test.agree {
				t.Errorf("agree %v, want %v", agree, test.agree)
			}
		})
	}
}

func TestARowOneMillisecondEarlyPassesOnlyForATimeCutFromASource(t *testing.T) {
	recorded := recordLine("2026-01-01T00:00:00.123Z", "u", "user", "hi", "/r/a.jsonl", 1)
	early := recordLine("2026-01-01T00:00:00.122Z", "u", "user", "hi", "/r/a.jsonl", 1)
	wantRow := []Row{{TurnType: "user", Text: "hi", TimestampUTC: "2026-01-01T00:00:00.123Z"}}
	earlyRow := []Row{{TurnType: "user", Text: "hi", TimestampUTC: "2026-01-01T00:00:00.122Z"}}
	tests := []struct {
		name  string
		cut   *cutTimes
		agree bool
	}{
		{"a source with no time below the millisecond fails", nil, false},
		{"a source whose time cuts to another millisecond fails", fixedCutTimes("2026-01-01T00:00:00.121Z"), false},
		{"a source whose time cuts to the binary's passes", fixedCutTimes("2026-01-01T00:00:00.122Z"), true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if agree, _ := rowsAgreeCut(t, wantRow, earlyRow, test.cut); agree != test.agree {
				t.Errorf("rows: agree %v, want %v", agree, test.agree)
			}
			if agree := timelinesAgree(t, false, []string{recorded}, []string{early}, test.cut); agree != test.agree {
				t.Errorf("timeline.jsonl: agree %v, want %v", agree, test.agree)
			}
		})
	}
}

func TestWrittenTimesBelowTheMillisecondAreFound(t *testing.T) {
	tests := []struct {
		name string
		text string
		want []string
	}{
		{"an ISO time with microseconds", `{"at":"2026-01-01T00:00:00.122999Z"}`, []string{"2026-01-01T00:00:00.122Z"}},
		{"an ISO time with an offset", `2026-01-01T01:00:00.1225+01:00`, []string{"2026-01-01T00:00:00.122Z"}},
		{"epoch seconds with a fraction", `"ts": 1767225600.1229`, []string{"2026-01-01T00:00:00.122Z"}},
		{"a time of whole milliseconds is not one", `2026-01-01T00:00:00.1220Z and 1767225600.122`, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := writtenCutTimes(test.text); strings.Join(got, ",") != strings.Join(test.want, ",") {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}

func TestDifference130ComparesTheRestOfTheRowAndThePosition(t *testing.T) {
	recorded := recordLine("2026-01-01T00:00:00.000Z", "alice", "system",
		"parser: config.json did not parse: Expecting value: line 1 column 11 (char 10)", "/r/config.json", 0)
	binary := strings.Replace(recorded, "Expecting value: line 1 column 11 (char 10)",
		"invalid JSON at line 1, column 11", 1)
	tests := []struct {
		name  string
		got   string
		agree bool
	}{
		{"the binary's position passes", binary, true},
		{"another user fails", strings.Replace(binary, `"user":"alice"`, `"user":"mallory"`, 1), false},
		{"another time fails", strings.Replace(binary, "2026-01-01T00:00:00.000Z", "1999-09-09T09:09:09.000Z", 1),
			false},
		{"another column fails", strings.Replace(binary, "column 11", "column 12", 1), false},
		{"a read error in place of the position fails", strings.Replace(binary, "invalid JSON at line 1, column 11",
			"permission denied", 1), false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if agree := timelinesAgree(t, false, []string{recorded}, []string{test.got}, nil); agree != test.agree {
				t.Errorf("agree %v, want %v", agree, test.agree)
			}
		})
	}
}

func TestADifferenceIsUsedOnlyWhereTheCaseNeedsIt(t *testing.T) {
	recorded := recordLine("2026-01-01T00:00:00.000Z", "u", "system",
		"parser: SQLite error: file is not a database", "/r/a.db", 0)
	tests := []struct {
		name   string
		got    string
		needed bool
	}{
		{"the same detail does not need 102", recorded, false},
		{"another detail needs 102", strings.Replace(recorded, "file is not a database", "malformed database", 1), true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			wantRun := commandOutput{outputs: map[string]string{"out/timeline.jsonl": recorded + "\n"}}
			gotRun := commandOutput{outputs: map[string]string{"out/timeline.jsonl": test.got + "\n"}}
			reporter := &recordingTB{TB: t}
			_, needed := compareInformation(reporter, "n", "/root", wantRun, gotRun)
			if len(reporter.errors) > 0 || slices.Contains(needed, 102) != test.needed {
				t.Errorf("needed %v, want 102 needed %v: %v", needed, test.needed, reporter.errors)
			}
		})
	}
}

func TestNeededAmongKeepsOnlyWhatTheComparisonFailsWithout(t *testing.T) {
	got := neededAmong([]int{102, 130, 160}, func(numbers []int) bool { return slices.Contains(numbers, 130) })
	if !slices.Equal(got, []int{130}) {
		t.Errorf("got %v, want [130]", got)
	}
}
