package codex

import (
	"slices"
	"strings"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
)

// policyFields are the turn_context and thread_settings_applied policy
// fields in the order of the "permission mode:" row, as label and payload
// key (research/codex-cli.md, Record schema). New fields go at the end,
// since the row text is an interface.
var policyFields = [...][2]string{
	{"approval", "approval_policy"},
	{"sandbox", "sandbox_policy"},
	{"profile", "permission_profile"},
	{"network", "network"},
	{"reviewer", "approvals_reviewer"},
	{"profile_id", "active_permission_profile"},
}

// settingsPolicyKeys are the fields a thread_settings_applied event is the
// authority for: the thread's approval policy, reviewer and access level.
// Its permission_profile is the thread's without the roots a turn adds, and
// it has no sandbox_policy; both stay with the last turn_context.
var settingsPolicyKeys = []string{"approval_policy", "approvals_reviewer",
	"active_permission_profile"}

// The positions in a policy of the fields a change of access level makes
// unknown, and of the access level itself.
const (
	sandboxField = 1
	profileField = 2
	accessField  = 5
)

// mode is a session's permission mode: each of policyFields' values last
// recorded for it, each as the composite writes it ("" when absent), and
// the parser.Composite of its pieces, which tells a change from the pieces
// an event sets without reading the fields it copies.
type mode struct {
	values    [len(policyFields)]jsonvalue.Value
	written   [len(policyFields)]string
	composite *parser.Composite
}

// newMode returns the mode of a session with no field recorded.
func newMode(work parser.Work) *mode {
	return &mode{composite: parser.NewComposite(len(policyFields), work)}
}

// set makes value the i-th field, the access level's object shown as its
// id, in the composite piece "<label>=<value>", "<label>=-" when absent.
func (m *mode) set(i int, value jsonvalue.Value) {
	m.values[i] = value
	shown := shownPolicyValue(i, value)
	m.written[i] = ""
	piece := policyFields[i][0] + "=-"
	if parser.Present(shown) {
		m.written[i] = parser.SettingValue(shown)
		piece = policyFields[i][0] + "=" + m.written[i]
	}
	if i > 0 {
		piece = " " + piece
	}
	m.composite.Set(i, piece)
}

// fromContext sets every field from a turn_context payload.
func (m *mode) fromContext(payload jsonvalue.Value) {
	for i, field := range policyFields {
		m.set(i, payload.Get(field[1]))
	}
}

// withSettings sets the fields a thread_settings_applied event's settings
// set (set, from settingsSet), and when the access level changes, makes
// the sandbox and profile unknown until the next turn_context.
func (m *mode) withSettings(settings jsonvalue.Value, set [len(policyFields)]bool) {
	old := shownPolicyValue(accessField, m.values[accessField])
	for i, isSet := range set {
		if isSet {
			m.set(i, settings.Get(policyFields[i][1]))
		}
	}
	if set[accessField] && !sameValue(old, shownPolicyValue(accessField, m.values[accessField])) {
		m.set(sandboxField, jsonvalue.NewNull())
		m.set(profileField, jsonvalue.NewNull())
	}
}

// shownPolicyValue returns a policy field as the composite shows it: the
// access level's object as its id, any other value as it is.
func shownPolicyValue(index int, value jsonvalue.Value) jsonvalue.Value {
	if index == accessField && value.IsObject() {
		return value.Get("id")
	}
	return value
}

// changed reports whether the composite differs from the one last
// recorded, recording it when it does; a mode with no field present is no
// change and is not recorded.
func (m *mode) changed() bool {
	if !slices.ContainsFunc(m.written[:], func(written string) bool { return written != "" }) ||
		!m.composite.Changed() {
		return false
	}
	m.composite.Record()
	return true
}

// text returns the composite permission mode, "approval=<p> sandbox=<s>
// profile=<pp> network=<n> reviewer=<r> profile_id=<id>" with "-" for a
// missing field, or "" when no field is present, each present field's
// value as shown returns it.
func (m *mode) text(shown func(int, string) string) string {
	parts := make([]string, len(m.written))
	empty := true
	for i, written := range m.written {
		if written != "" {
			written, empty = shown(i, written), false
		} else {
			written = "-"
		}
		parts[i] = policyFields[i][0] + "=" + written
	}
	if empty {
		return ""
	}
	return strings.Join(parts, " ")
}

// wholeText returns the composite with every value as it is.
func (m *mode) wholeText() string {
	return m.text(func(_ int, written string) string { return written })
}

// carriedText returns the composite as a thread_settings_applied row
// writes it: each field the event did not set (settingsSet) is the last
// turn_context's, carried onto every such row, so it is cut through cuts
// under the name "permission mode <label>".
func (m *mode) carriedText(set [len(policyFields)]bool, cuts *parser.Carried) string {
	return m.text(func(i int, shown string) string {
		if set[i] {
			return shown
		}
		return cuts.Cut("permission mode "+policyFields[i][0], shown)
	})
}

// settingsSet returns which policy fields a thread_settings_applied
// event's settings set, the ones withSettings takes from it.
func settingsSet(settings jsonvalue.Value) [len(policyFields)]bool {
	var set [len(policyFields)]bool
	for i, field := range policyFields {
		_, found := settings.Lookup(field[1])
		set[i] = found && slices.Contains(settingsPolicyKeys, field[1])
	}
	return set
}

// settingsModel returns the model a thread_settings object names: model,
// else collaboration_mode.settings.model, else "".
func settingsModel(settings jsonvalue.Value) string {
	if model := settings.Get("model"); !model.IsEmpty() {
		return model.Text()
	}
	return parser.Text(settings.Get("collaboration_mode").Get("settings").Get("model"))
}
