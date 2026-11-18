package openclaw

import (
	"cmp"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// execConfig emits one `permission setting:` row per policy setting of the
// migrated exec approvals in exec_approvals_config: each default and
// per-agent policy field, allowlist entry and MCP tool grant. The socket
// member holds the approvals socket's token and is never read; a raw_json
// that does not decode is reported without its text.
func (s *stateRows) execConfig(record sqlitefile.Row) {
	document, err := jsonvalue.DecodeString(parser.TextColumn(record.Get("raw_json")))
	if err != nil {
		s.problems.Add(int(record.Rowid), "exec_approvals_config raw_json unparseable ("+parser.JSONPosition(err)+
			"; text withheld, it holds the approvals socket token)")
		return
	}
	type scope struct {
		name   string
		policy jsonvalue.Value
	}
	scopes := []scope{{"defaults", document.Get("defaults")}}
	if agents := document.Get("agents"); agents.IsObject() {
		for _, member := range agents.Members() {
			scopes = append(scopes, scope{"agents." + member.Key, member.Value})
		}
	}
	timestamp := parser.ColumnTime(record.Get("updated_at_ms"))
	for _, each := range scopes {
		for _, setting := range policySettings(each.policy) {
			s.row(record, timestamp, "permission setting: "+each.name+"."+setting+
				" (from state/openclaw.sqlite exec_approvals_config, updated_at_ms)", "")
		}
	}
}

// policySettings returns "<key>=<value>" per field, allowlist entry and MCP
// tool grant of one defaults or agents.<id> policy of an exec approvals
// file (packages/gateway-protocol/src/schema/exec-approvals.ts:15-75).
func policySettings(policy jsonvalue.Value) []string {
	if !policy.IsObject() {
		return nil
	}
	var out []string
	for _, key := range []string{"security", "ask", "askFallback", "autoAllowSkills"} {
		value := policy.Get(key)
		switch {
		case value.IsNull():
		case value.IsBool():
			out = append(out, key+"="+value.JSON())
		default:
			out = append(out, key+"="+value.Text())
		}
	}
	for _, entry := range parser.Elements(policy.Get("allowlist")) {
		if pattern := entry.Get("pattern"); entry.IsObject() && !pattern.IsNull() {
			argument := ""
			if value := entry.Get("argPattern"); !value.IsEmpty() {
				argument = " argPattern=" + value.Text()
			}
			out = append(out, "allowlist="+pattern.Text()+argument+" source="+
				cmp.Or(parser.Text(entry.Get("source")), "-"))
		}
	}
	for _, grant := range parser.Elements(policy.Get("mcpTools")) {
		if grant.IsObject() {
			out = append(out, "mcpTools="+cmp.Or(parser.Text(grant.Get("server")), "-")+"/"+
				cmp.Or(parser.Text(grant.Get("tool")), "-")+" source="+
				cmp.Or(parser.Text(grant.Get("source")), "-"))
		}
	}
	return out
}
