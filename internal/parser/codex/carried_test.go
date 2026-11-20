package codex

import (
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/secrets"
)

// policyRollout returns a rollout of one turn_context with the given
// policy members, then a thread_settings_applied event per settings
// object, each written as JSON members.
func policyRollout(context string, settings ...string) string {
	lines := []string{`{"type":"session_meta","timestamp":"2026-01-02T00:00:00Z","payload":{` +
		`"id":"0198f0a1-0000-7000-8000-000000000001","cwd":"/p"}}`,
		`{"type":"turn_context","timestamp":"2026-01-02T00:00:01Z","payload":{"cwd":"/p",` + context + `}}`}
	for _, each := range settings {
		lines = append(lines, `{"type":"event_msg","timestamp":"2026-01-02T00:00:02Z","payload":{`+
			`"type":"thread_settings_applied","thread_settings":{`+each+`}}}`)
	}
	return strings.Join(lines, "\n") + "\n"
}

// modeRows returns the "permission mode:" and problem rows of a rollout.
func modeRows(t *testing.T, text string) []string {
	t.Helper()
	rel := ".codex/sessions/2026/01/02/rollout-2026-01-02T00-00-00-0198f0a1-0000-7000-8000-000000000001.jsonl"
	var kept []string
	for _, row := range parseWith(t, secrets.Setting{}, rel, text) {
		if strings.HasPrefix(row, "permission mode:") || strings.HasPrefix(row, parser.ProblemPrefix) {
			kept = append(kept, row)
		}
	}
	return kept
}

// cutOf returns a value of letters longer than parser.CarriedLimit as the
// kit writes it once cut.
func cutOf(value string) string {
	mark := " [cut from " + strconv.Itoa(len(value)) + " bytes]"
	return value[:parser.CarriedLimit-len(mark)] + mark
}

// alternatingApprovals returns n settings objects whose approval policy
// alternates between a and b.
func alternatingApprovals(n int) []string {
	settings := make([]string, n)
	for i := range settings {
		settings[i] = `"approval_policy":"` + "ab"[i%2:i%2+1] + `"`
	}
	return settings
}

func TestAPermissionModeRowCutsTheValuesItsEventDidNotSet(t *testing.T) {
	for _, n := range []int{500, 1000} {
		t.Run(strconv.Itoa(n)+" settings events after a long network value", func(t *testing.T) {
			network := strings.Repeat("N", 10*n)
			mode := func(approval, shown string) string {
				return "approval=" + approval + " sandbox=- profile=- network=" + shown + " reviewer=- profile_id=-"
			}
			want := []string{"permission mode: " + mode("x", network)}
			old := mode("x", network)
			for i := range n {
				shown := mode("ab"[i%2:i%2+1], cutOf(network))
				want = append(want, "permission mode: "+old+" -> "+shown)
				old = shown
			}
			want = append(want, parser.ProblemPrefix+parser.CarriedProblem("permission mode network", n))
			got := modeRows(t, policyRollout(`"approval_policy":"x","network":"`+network+`"`,
				alternatingApprovals(n)...))
			bytes := 0
			for _, row := range got {
				bytes += len(row)
			}
			t.Logf("%d rows, %d bytes of text", len(got), bytes)
			if strings.Join(got, "\n") != strings.Join(want, "\n") {
				t.Errorf("got %d rows, want %d; first difference near row %d", len(got), len(want),
					firstDifference(got, want))
			}
		})
	}
}

// firstDifference returns the position of the first row two lists differ
// in.
func firstDifference(got, want []string) int {
	for i := range min(len(got), len(want)) {
		if got[i] != want[i] {
			return i
		}
	}
	return min(len(got), len(want))
}

func TestEachCarriedPolicyValueIsCutUnderItsOwnName(t *testing.T) {
	long := strings.Repeat("L", parser.CarriedLimit+1)
	tests := []struct {
		name     string
		context  string
		settings []string
		want     []string
	}{
		{"an ordinary policy is written whole",
			`"approval_policy":"x","sandbox_policy":{"type":"read-only"},"network":"on"`,
			[]string{`"approval_policy":"y","approvals_reviewer":"user"`},
			[]string{
				`permission mode: approval=x sandbox={"type":"read-only"} profile=- network=on reviewer=- profile_id=-`,
				`permission mode: approval=x sandbox={"type":"read-only"} profile=- network=on reviewer=- ` +
					`profile_id=- -> approval=y sandbox={"type":"read-only"} profile=- network=on reviewer=user ` +
					`profile_id=-`,
			}},
		{"the sandbox, profile and network a turn set",
			`"approval_policy":"x","sandbox_policy":"` + long + `","permission_profile":"` + long +
				`","network":"` + long + `"`,
			[]string{`"approval_policy":"y"`},
			[]string{
				"permission mode: approval=x sandbox=" + long + " profile=" + long + " network=" + long +
					" reviewer=- profile_id=-",
				"permission mode: approval=x sandbox=" + long + " profile=" + long + " network=" + long +
					" reviewer=- profile_id=- -> approval=y sandbox=" + cutOf(long) + " profile=" + cutOf(long) +
					" network=" + cutOf(long) + " reviewer=- profile_id=-",
				parser.ProblemPrefix + parser.CarriedProblem("permission mode sandbox", 1),
				parser.ProblemPrefix + parser.CarriedProblem("permission mode profile", 1),
				parser.ProblemPrefix + parser.CarriedProblem("permission mode network", 1),
			}},
		{"an approval policy the event did not set, beside a long one it did",
			`"approval_policy":"` + long + `"`,
			[]string{`"approvals_reviewer":"` + long + `"`},
			[]string{
				"permission mode: approval=" + long + " sandbox=- profile=- network=- reviewer=- profile_id=-",
				"permission mode: approval=" + long + " sandbox=- profile=- network=- reviewer=- profile_id=- -> " +
					"approval=" + cutOf(long) + " sandbox=- profile=- network=- reviewer=" + long + " profile_id=-",
				parser.ProblemPrefix + parser.CarriedProblem("permission mode approval", 1),
			}},
		{"an event that changes nothing gives no row, however long a carried value",
			`"approval_policy":"x","network":"` + long + `"`,
			[]string{`"approval_policy":"x"`},
			[]string{"permission mode: approval=x sandbox=- profile=- network=" + long + " reviewer=- profile_id=-"}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := modeRows(t, policyRollout(test.context, test.settings...))
			if strings.Join(got, "\n") != strings.Join(test.want, "\n") {
				t.Errorf("got %d rows, want %d; first difference near row %d", len(got), len(test.want),
					firstDifference(got, test.want))
			}
		})
	}
}

// Whether a thread_settings_applied event changes the permission mode is
// told from the fields it sets: a long field it copies from the last
// turn_context is neither joined into a composite nor compared again, so
// twice the events allocate twice the bytes of their rows, not one more
// copy of the long value per event. The bytes are counted
// (runtime.MemStats.TotalAlloc), not timed.
func TestAPermissionModeChangeIsToldFromTheFieldsAnEventSets(t *testing.T) {
	const size = 200_000
	tests := []struct {
		name, context string
		long          int
	}{
		{"a long network", `"approval_policy":"x","network":"` + strings.Repeat("N", size) + `"`, 1},
		{"a long sandbox object", `"approval_policy":"x","sandbox_policy":{"mode":"` + strings.Repeat("S", size) +
			`"}`, 1},
		{"a long sandbox and network", `"approval_policy":"x","sandbox_policy":"` + strings.Repeat("S", size) +
			`","network":"` + strings.Repeat("N", size) + `"`, 2},
	}
	for _, test := range tests {
		allocated := map[int]int64{}
		for _, n := range []int{20, 40} {
			text := policyRollout(test.context, alternatingApprovals(n)...)
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			rows := modeRows(t, text)
			runtime.ReadMemStats(&after)
			allocated[n] = int64(after.TotalAlloc - before.TotalAlloc)
			if len(rows) != n+1+test.long {
				t.Errorf("%s: %d events gave %d mode and problem rows, want %d", test.name, n, len(rows),
					n+1+test.long)
			}
		}
		if growth := allocated[40] - allocated[20]; growth > 5*size {
			t.Errorf("%s: 20 more events allocated %d more bytes, more than five copies of the %d-byte field",
				test.name, growth, size)
		}
	}
}

// exitRollout returns a rollout whose CommandExecution item c1 failed with
// the exit code written as code, then outputs function_call_output rows of
// the call.
func exitRollout(code string, outputs int) string {
	lines := []string{`{"type":"session_meta","timestamp":"2026-01-02T00:00:00Z","payload":{` +
		`"id":"0198f0a1-0000-7000-8000-000000000001","cwd":"/p"}}`,
		`{"type":"event_msg","timestamp":"2026-01-02T00:00:01Z","payload":{"type":"item_completed",` +
			`"item":{"type":"CommandExecution","id":"c1","command":["ls"],"aggregated_output":"",` +
			`"exit_code":` + code + `}}}`}
	for range outputs {
		lines = append(lines, `{"type":"response_item","timestamp":"2026-01-02T00:00:02Z","payload":{`+
			`"type":"function_call_output","call_id":"c1","output":"ok"}}`)
	}
	return strings.Join(lines, "\n") + "\n"
}

func TestAnExitCodeCarriedIntoEachOutputOfACallIsCut(t *testing.T) {
	long := strings.Repeat("7", parser.CarriedLimit+1)
	tests := []struct {
		name, code string
		want       []string
	}{
		{"an ordinary exit code is written whole", `1`, []string{"[exit 1] ok", "[exit 1] ok"}},
		{"a zero exit code is not shown", `0`, []string{"ok", "ok"}},
		{"a long exit code is cut in each output and counted once per output", `"` + long + `"`,
			[]string{"[exit " + cutOf(long) + "] ok", "[exit " + cutOf(long) + "] ok",
				parser.ProblemPrefix + parser.CarriedProblem("exit code", 2)}},
	}
	rel := ".codex/sessions/2026/01/02/rollout-2026-01-02T00-00-00-0198f0a1-0000-7000-8000-000000000001.jsonl"
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var got []string
			for _, row := range parseWith(t, secrets.Setting{}, rel, exitRollout(test.code, 2)) {
				if strings.HasSuffix(row, "] ok") || row == "ok" || strings.HasPrefix(row, parser.ProblemPrefix) {
					got = append(got, row)
				}
			}
			if strings.Join(got, "\n") != strings.Join(test.want, "\n") {
				t.Errorf("got %d rows, want %d; first difference near row %d", len(got), len(test.want),
					firstDifference(got, test.want))
			}
		})
	}
}
