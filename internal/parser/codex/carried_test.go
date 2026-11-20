package codex

import (
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
