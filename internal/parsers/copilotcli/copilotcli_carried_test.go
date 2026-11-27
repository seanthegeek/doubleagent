package copilotcli

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/osfile"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/parserkit/parsertest"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// A working directory and a model of 100,000 bytes each are carried onto
// every row after them, the project path and the model: each row holds them
// cut, so the output grows by 8,329 bytes per message, not by their
// length, and the kit adds one problem row per field.
func TestALongWorkingDirectoryAndModelAreCutInEveryRow(t *testing.T) {
	cwd := "/w/" + strings.Repeat("p", 100_000)
	model := strings.Repeat("m", 100_000)
	tests := []struct {
		messages, size int
	}{
		{500, 4_385_625},
		{1000, 8_550_127},
	}
	for _, test := range tests {
		t.Run(strconv.Itoa(test.messages)+" messages", func(t *testing.T) {
			lines := []string{`{"type":"session.start","timestamp":"2026-01-02T03:04:05Z","data":{"context":{"cwd":"` +
				cwd + `"}}}`, `{"type":"session.model_change","data":{"newModel":"` + model + `"}}`}
			for i := range test.messages {
				lines = append(lines, `{"type":"user.message","data":{"content":"m`+strconv.Itoa(i)+`"}}`)
			}
			rel := sessionDir + "events.jsonl"
			home := t.TempDir()
			path := filepath.Join(home, filepath.FromSlash(rel))
			if err := osfile.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := osfile.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			artifact := &input.Artifact{DiskPath: path, Original: "/home/u/" + rel, Rel: rel, Agent: "copilot-cli",
				Home: &input.Home{DiskPath: home, Original: "/home/u", User: "u", Host: "h"}, Manifest: jsonvalue.NewNull()}
			var rows []timeline.Row
			size := 0
			if err := parsertest.ParseWithoutSecrets(t, New(), artifact, parserkit.Options{}, func(row timeline.Row) {
				rows = append(rows, row)
				for _, field := range row.Strings() {
					size += len(field)
				}
			}); err != nil {
				t.Fatal(err)
			}
			// The session start and the model rows precede the messages.
			want := []string{parserkit.ProblemPrefix + parserkit.CarriedProblem("project_path", test.messages+2),
				parserkit.ProblemPrefix + parserkit.CarriedProblem("model", test.messages+1)}
			if size != test.size || rows[len(rows)-2].Text != want[0] || rows[len(rows)-1].Text != want[1] {
				t.Errorf("%d rows of %d bytes ending %q and %q, want %d bytes, %q", len(rows), size,
					rows[len(rows)-2].Text, rows[len(rows)-1].Text, test.size, want)
			}
		})
	}
}

// requestEvent returns a permission.requested event with the given data
// members before its request.
func requestEvent(members string) string {
	return `{"type":"permission.requested","timestamp":"2026-01-02T03:04:05Z","data":{` + members +
		`"permissionRequest":{"kind":"shell","toolCallId":"c"}}}`
}

// modeRows returns the "permission mode:" and problem rows of an events
// file of the given lines.
func modeRows(t *testing.T, lines []string) []string {
	t.Helper()
	var kept []string
	for _, text := range parseWith(t, sessionDir+"events.jsonl", strings.Join(lines, "\n")+"\n", parserkit.Options{}) {
		if strings.HasPrefix(text, "permission mode:") || strings.HasPrefix(text, parserkit.ProblemPrefix) {
			kept = append(kept, text)
		}
	}
	return kept
}

// cutOf returns a value of letters longer than parserkit.CarriedLimit as the
// kit writes it once cut.
func cutOf(value string) string {
	mark := " [cut from " + strconv.Itoa(len(value)) + " bytes]"
	return value[:parserkit.CarriedLimit-len(mark)] + mark
}

func TestAModeRowCutsTheHalfItsRequestDidNotSet(t *testing.T) {
	for _, n := range []int{500, 1000} {
		t.Run(strconv.Itoa(n)+" agent mode changes after a long permission mode", func(t *testing.T) {
			mode := strings.Repeat("M", 10*n)
			lines := []string{requestEvent(`"permissionMode":"` + mode + `","agentMode":"a",`)}
			want := []string{"permission mode: " + mode + " agent=a"}
			old := "a"
			for i := range n {
				agent := "ab"[i%2 : i%2+1]
				lines = append(lines, requestEvent(`"agentMode":"`+agent+`",`))
				if agent != old {
					want = append(want, "permission mode: "+cutOf(mode)+" agent="+old+" -> "+cutOf(mode)+" agent="+agent)
				}
				old = agent
			}
			want = append(want, parserkit.ProblemPrefix+parserkit.CarriedProblem("permission mode", 2*(len(want)-1)))
			got := modeRows(t, lines)
			bytes := 0
			for _, text := range got {
				bytes += len(text)
			}
			t.Logf("%d rows, %d bytes of text", len(got), bytes)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("got %d rows, want %d", len(got), len(want))
			}
		})
	}
}

func TestEachCarriedHalfOfTheModeIsCutUnderItsOwnName(t *testing.T) {
	long := strings.Repeat("L", parserkit.CarriedLimit+1)
	tests := []struct {
		name  string
		lines []string
		want  []string
	}{
		{"ordinary modes are written whole",
			[]string{requestEvent(`"permissionMode":"manual","agentMode":"interactive",`),
				requestEvent(`"agentMode":"autopilot",`), requestEvent(`"permissionMode":"auto",`)},
			[]string{"permission mode: manual agent=interactive",
				"permission mode: manual agent=interactive -> manual agent=autopilot",
				"permission mode: manual agent=autopilot -> auto agent=autopilot"}},
		{"a long agent mode a later request carries",
			[]string{requestEvent(`"permissionMode":"manual","agentMode":"` + long + `",`),
				requestEvent(`"permissionMode":"auto",`)},
			[]string{"permission mode: manual agent=" + long,
				"permission mode: manual agent=" + cutOf(long) + " -> auto agent=" + cutOf(long),
				parserkit.ProblemPrefix + parserkit.CarriedProblem("agent mode", 2)}},
		{"a long mode a permissions change set, carried by a request",
			[]string{`{"type":"session.permissions_changed","data":{"previousMode":"manual","mode":"` + long + `"}}`,
				requestEvent(`"agentMode":"a",`)},
			[]string{"permission mode: manual -> " + long,
				"permission mode: " + cutOf(long) + " -> " + cutOf(long) + " agent=a",
				parserkit.ProblemPrefix + parserkit.CarriedProblem("permission mode", 2)}},
		{"a request that changes nothing gives no row, however long the mode",
			[]string{requestEvent(`"permissionMode":"` + long + `","agentMode":"a",`), requestEvent(`"agentMode":"a",`)},
			[]string{"permission mode: " + long + " agent=a"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := modeRows(t, test.lines)
			if strings.Join(got, "\n") != strings.Join(test.want, "\n") {
				t.Errorf("got %d rows %.300q, want %d", len(got), got, len(test.want))
			}
		})
	}
}

// Whether a permission request changes the mode is told from the halves
// it sets: a long half copied from an earlier request is neither joined
// into the mode's text nor compared again, so twice the requests allocate
// twice the bytes of their rows, not one more copy of the long half per
// request. The bytes are counted (runtime.MemStats.TotalAlloc), not timed.
func TestAPermissionModeChangeIsToldFromTheHalvesARequestSets(t *testing.T) {
	const size = 200_000
	allocated := map[int]int64{}
	for _, n := range []int{20, 40} {
		lines := []string{requestEvent(`"permissionMode":"` + strings.Repeat("M", size) + `","agentMode":"a",`)}
		for i := range n {
			lines = append(lines, requestEvent(`"agentMode":"`+"ab"[i%2:i%2+1]+`",`))
		}
		var before, after runtime.MemStats
		runtime.ReadMemStats(&before)
		rows := modeRows(t, lines)
		runtime.ReadMemStats(&after)
		allocated[n] = int64(after.TotalAlloc - before.TotalAlloc)
		// The first request after the long one sets the agent mode it had.
		if len(rows) != n+1 {
			t.Errorf("%d requests gave %d mode and problem rows, want %d", n, len(rows), n+1)
		}
	}
	if growth := allocated[40] - allocated[20]; growth > 5*size {
		t.Errorf("20 more requests allocated %d more bytes, more than five copies of the %d-byte half",
			growth, size)
	}
}
