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

// policy is the value of each of policyFields last recorded for a session.
type policy [len(policyFields)]jsonvalue.Value

// policyOf returns the policy fields of a turn_context payload.
func policyOf(payload jsonvalue.Value) policy {
	var fields policy
	for i, field := range policyFields {
		fields[i] = payload.Get(field[1])
	}
	return fields
}

// withSettings returns the policy after a thread_settings_applied event's
// settings: its three thread-level fields replace the old ones, and when
// the access level changes, the sandbox and profile become unknown until
// the next turn_context.
func (p policy) withSettings(settings jsonvalue.Value) policy {
	state := p
	for i, set := range settingsSet(settings) {
		if set {
			state[i] = settings.Get(policyFields[i][1])
		}
	}
	if !sameValue(shownPolicyValue(accessField, p[accessField]),
		shownPolicyValue(accessField, state[accessField])) {
		state[sandboxField] = jsonvalue.NewNull()
		state[profileField] = jsonvalue.NewNull()
	}
	return state
}

// shownPolicyValue returns a policy field as the composite shows it: the
// access level's object as its id, any other value as it is.
func shownPolicyValue(index int, value jsonvalue.Value) jsonvalue.Value {
	if index == accessField && value.IsObject() {
		return value.Get("id")
	}
	return value
}

// text returns the composite permission mode, "approval=<p> sandbox=<s>
// profile=<pp> network=<n> reviewer=<r> profile_id=<id>" with "-" for a
// missing field, or "" when the policy has no field at all.
func (p policy) text() string {
	return p.shownText(func(_ int, shown string) string { return shown })
}

// carriedText returns the composite as a thread_settings_applied row
// writes it: each field the event did not set (settingsSet) is the last
// turn_context's, carried onto every such row, so it is cut through cuts
// under the name "permission mode <label>".
func (p policy) carriedText(set [len(policyFields)]bool, cuts *parser.Carried) string {
	return p.shownText(func(i int, shown string) string {
		if set[i] {
			return shown
		}
		return cuts.Cut("permission mode "+policyFields[i][0], shown)
	})
}

// shownText returns the composite with each present field's value as
// shown gives it.
func (p policy) shownText(shown func(int, string) string) string {
	parts := make([]string, len(p))
	empty := true
	for i, value := range p {
		value = shownPolicyValue(i, value)
		written := "-"
		if parser.Present(value) {
			written, empty = shown(i, parser.SettingValue(value)), false
		}
		parts[i] = policyFields[i][0] + "=" + written
	}
	if empty {
		return ""
	}
	return strings.Join(parts, " ")
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
