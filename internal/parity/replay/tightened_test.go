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
// does, the documented order of the run's rows included, for a run whose
// sources hold the times of cut.
func timelinesAgree(t *testing.T, loose bool, want, got []string, cut *cutTimes) bool {
	wantRun := commandOutput{outputs: map[string]string{"out/timeline.jsonl": strings.Join(want, "\n") + "\n"},
		loose: loose, cut: cut}
	gotRun := commandOutput{outputs: map[string]string{"out/timeline.jsonl": strings.Join(got, "\n") + "\n"},
		loose: loose}
	reporter := &recordingTB{TB: t}
	compareInformation(reporter, "n", "/root", wantRun, gotRun)
	checkRunOrder(reporter, "n", wantRun, gotRun)
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

func TestDifference230ComparesTheRestOfTheRowAndThePosition(t *testing.T) {
	recorded := strings.Replace(recordLine("2026-01-01T00:00:00.000Z", "alice", "system",
		"parser: config.json did not parse: Expecting value: line 1 column 11 (char 10)", "/r/config.json", 0),
		`"agent":"codex-cli"`, `"agent":"nanobot"`, 1)
	binary := strings.Replace(recorded, "Expecting value: line 1 column 11 (char 10)",
		"invalid JSON at line 1, column 11: "+replayPlaceholder, 1)
	tests := []struct {
		name  string
		got   string
		agree bool
	}{
		{"the binary's position and placeholder pass", binary, true},
		{"the reader's detail in place of the placeholder fails", strings.Replace(binary, replayPlaceholder,
			"invalid character 's' looking for beginning of value", 1), false},
		{"another user fails", strings.Replace(binary, `"user":"alice"`, `"user":"mallory"`, 1), false},
		{"another time fails", strings.Replace(binary, "2026-01-01T00:00:00.000Z", "1999-09-09T09:09:09.000Z", 1),
			false},
		{"another column fails", strings.Replace(binary, "column 11", "column 12", 1), false},
		{"another agent's row fails", strings.Replace(binary, `"agent":"nanobot"`, `"agent":"codex-cli"`, 1), false},
		{"a read error in place of the position fails", strings.Replace(binary,
			"invalid JSON at line 1, column 11: "+replayPlaceholder, "permission denied", 1), false},
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

func TestAModificationTimeRoundedIntoTheNextMillisecondMayMoveItsRow(t *testing.T) {
	// A file modified at .3005 ms: the recording rounded its row to .301,
	// the binary cut it to .300, so the binary sorts the row among the rows
	// of .300 by file and line, before every row of .301.
	record := recordLine("2026-01-01T00:00:00.300Z", "u", "user", "hi", "/r/a.jsonl", 1)
	laterFile := recordLine("2026-01-01T00:00:00.300Z", "u", "user", "hi", "/r/c.jsonl", 1)
	recordedFile := recordLine("2026-01-01T00:00:00.301Z", "u", "system", "parser: broken.json is not JSON",
		"/r/broken.json", 0)
	binaryFile := strings.Replace(recordedFile, ".301Z", ".300Z", 1)
	after := recordLine("2026-01-01T00:00:00.302Z", "u", "user", "bye", "/r/a.jsonl", 2)
	modified := fixedModifiedTimes("2026-01-01T00:00:00.300Z")
	twoEarlier := strings.Replace(recordedFile, ".301Z", ".299Z", 1)
	later := strings.Replace(recordedFile, ".301Z", ".302Z", 1)
	early := recordLine("2026-01-01T00:00:00.299Z", "u", "user", "early", "/r/z.jsonl", 1)
	peer := recordLine("2026-01-01T00:00:00.301Z", "u", "user", "again", "/r/a.jsonl", 3)
	peerC := recordLine("2026-01-01T00:00:00.301Z", "u", "user", "again", "/r/c.jsonl", 3)
	peerD := recordLine("2026-01-01T00:00:00.301Z", "u", "user", "again", "/r/d.jsonl", 3)
	late := recordLine("2026-01-01T09:00:00.000Z", "u", "user", "later", "/r/a.jsonl", 4)
	recorded := []string{record, recordedFile, after}
	tests := []struct {
		name      string
		want, got []string
		cut       *cutTimes
		agree     bool
	}{
		{"the file's row cut and moved before a row of a later file passes", []string{laterFile, recordedFile, after},
			[]string{binaryFile, laterFile, after}, modified, true},
		{"the file's row cut in its recorded place passes", recorded, []string{record, binaryFile, after}, modified, true},
		{"the file's row moved past a row of the millisecond it was rounded into passes",
			[]string{record, peer, recordedFile, after}, []string{record, binaryFile, peer, after}, modified, true},
		{"the file's row moved past rows of both milliseconds passes", []string{laterFile, peer, recordedFile, after},
			[]string{binaryFile, laterFile, peer, after}, modified, true},
		{"the file's row moved before a row of an earlier file fails", recorded, []string{binaryFile, record, after},
			modified, false},
		{"the file's row moved before a row of an earlier millisecond fails", []string{early, recordedFile, after},
			[]string{binaryFile, early, after}, modified, false},
		{"the file's row moved later fails", []string{laterFile, recordedFile, after},
			[]string{laterFile, after, binaryFile}, modified, false},
		{"the file's row moved later past rows of the millisecond it was rounded into fails",
			[]string{record, recordedFile, peerC, peerD, after}, []string{record, peerC, peerD, binaryFile, after},
			modified, false},
		{"two milliseconds earlier fails", recorded, []string{twoEarlier, record, after},
			fixedModifiedTimes("2026-01-01T00:00:00.299Z"), false},
		{"one millisecond the other way fails", recorded, []string{record, after, later},
			fixedModifiedTimes("2026-01-01T00:00:00.302Z"), false},
		{"a time from a record rather than a file's modification time fails", []string{laterFile, recordedFile, after},
			[]string{binaryFile, laterFile, after}, fixedCutTimes("2026-01-01T00:00:00.300Z"), false},
		{"the other rows swapped still fail", []string{laterFile, recordedFile, after},
			[]string{binaryFile, after, laterFile}, modified, false},
		{"the moved row with another text fails", []string{laterFile, recordedFile, after},
			[]string{strings.Replace(binaryFile, "broken", "other", 1), laterFile, after}, modified, false},
		{"the file's row moved past a row of a later millisecond fails", recorded,
			[]string{record, after, binaryFile}, modified, false},
		{"the file's row moved past a row hours later fails", []string{record, recordedFile, after, late},
			[]string{record, after, late, binaryFile}, modified, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if agree := timelinesAgree(t, false, test.want, test.got, test.cut); agree != test.agree {
				t.Errorf("agree %v, want %v", agree, test.agree)
			}
		})
	}
}

func TestAMovedRowPassesOnlyRowsThatSortAfterItTowardsTheStart(t *testing.T) {
	cutRow := recordLine("2026-01-01T00:00:00.300Z", "u", "system", "x", "/r/b.json", 0)
	after := recordLine("2026-01-01T00:00:00.300Z", "u", "user", "y", "/r/c.jsonl", 1)
	before := recordLine("2026-01-01T00:00:00.300Z", "u", "user", "y", "/r/a.jsonl", 1)
	tests := []struct {
		name          string
		got           []string
		moved, target int
		want          bool
	}{
		{"towards the start past a row that sorts after it", []string{cutRow, after}, 0, 1, true},
		{"towards the start past a row that sorts before it", []string{cutRow, before}, 0, 1, false},
		{"towards the start past a row that sorts the same", []string{cutRow, cutRow}, 0, 1, false},
		{"towards the end", []string{before, cutRow}, 1, 0, false},
		{"in its own place", []string{cutRow}, 0, 0, false},
		{"to a place past the end", []string{cutRow, after}, 0, 2, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := movedEarlier(test.got, test.moved, test.target); got != test.want {
				t.Errorf("got %v, want %v", got, test.want)
			}
		})
	}
}
