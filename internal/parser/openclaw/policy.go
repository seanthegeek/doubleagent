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
// that does not decode is reported with the reader's error through
// parser.ReaderErrorText, since the reader can quote that token, and a raw_json,
// agents, policy, allowlist or mcpTools of the wrong kind adds a problem.
func (s *stateRows) execConfig(record sqlitefile.Row) {
	document, err := jsonvalue.DecodeString(parser.TextColumn(record.Get("raw_json")))
	if err != nil {
		s.problems.Add(int(record.Rowid), "exec_approvals_config raw_json unparseable ("+
			parser.ReaderErrorText(s.artifact, err)+")")
		return
	}
	s.execDocument(record.Rowid, document, parser.ColumnTime(record.Get("updated_at_ms")))
}

// execDocument emits the permission rows of a decoded approvals file read
// from the row with the given rowid, and adds the problems of its members
// of the wrong kind.
func (s *stateRows) execDocument(rowid int64, document jsonvalue.Value, timestamp string) {
	type scope struct {
		name   string
		policy jsonvalue.Value
	}
	line := int(rowid)
	parser.ObjectMember(document, "raw_json", line, &s.problems)
	scopes := []scope{{"defaults", document.Get("defaults")}}
	for _, member := range parser.ObjectMember(document.Get("agents"), "agents", line, &s.problems) {
		scopes = append(scopes, scope{"agents." + member.Key, member.Value})
	}
	for _, each := range scopes {
		parser.ObjectMember(each.policy, each.name, line, &s.problems)
		for _, setting := range policySettings(each.policy, each.name, line, &s.problems) {
			s.emit(s.stateRow(s.artifact, rowid, timestamp, "permission setting: "+each.name+"."+setting+
				" (from state/openclaw.sqlite exec_approvals_config, updated_at_ms)", ""))
		}
	}
}

// policySettings returns "<key>=<value>" per field, allowlist entry and MCP
// tool grant of one defaults or agents.<id> policy of an exec approvals
// file (packages/gateway-protocol/src/schema/exec-approvals.ts:15-75). An
// allowlist or mcpTools that is not an array adds a problem naming it under
// the policy's name, at line.
func policySettings(policy jsonvalue.Value, name string, line int, problems *parser.Problems) []string {
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
	for _, entry := range parser.ArrayMember(policy.Get("allowlist"), name+".allowlist", line, problems) {
		if pattern := entry.Get("pattern"); entry.IsObject() && !pattern.IsNull() {
			argument := ""
			if value := entry.Get("argPattern"); !value.IsEmpty() {
				argument = " argPattern=" + value.Text()
			}
			out = append(out, "allowlist="+pattern.Text()+argument+" source="+
				cmp.Or(parser.Text(entry.Get("source")), "-"))
		}
	}
	for _, grant := range parser.ArrayMember(policy.Get("mcpTools"), name+".mcpTools", line, problems) {
		if grant.IsObject() {
			out = append(out, "mcpTools="+cmp.Or(parser.Text(grant.Get("server")), "-")+"/"+
				cmp.Or(parser.Text(grant.Get("tool")), "-")+" source="+
				cmp.Or(parser.Text(grant.Get("source")), "-"))
		}
	}
	return out
}
