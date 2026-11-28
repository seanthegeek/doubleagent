package openclaw

import (
	"cmp"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
)

// policySettings returns "<key>=<value>" per field, allowlist entry and MCP
// tool grant of one defaults or agents.<id> policy of an exec approvals
// file (packages/gateway-protocol/src/schema/exec-approvals.ts:15-75). An
// allowlist or mcpTools that is not an array adds a problem naming it under
// the policy's name, at line.
func policySettings(policy jsonvalue.Value, name string, line int, problems *parserkit.Problems) []string {
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
	for _, entry := range parserkit.EntriesOfKind(policy.Get("allowlist"), name+".allowlist", jsonvalue.Object, line,
		problems) {
		if pattern := entry.Get("pattern"); !pattern.IsNull() {
			argument := ""
			if value := entry.Get("argPattern"); !value.IsEmpty() {
				argument = " argPattern=" + value.Text()
			}
			out = append(out, "allowlist="+pattern.Text()+argument+" source="+
				cmp.Or(parserkit.Text(entry.Get("source")), "-"))
		}
	}
	for _, grant := range parserkit.EntriesOfKind(policy.Get("mcpTools"), name+".mcpTools", jsonvalue.Object, line,
		problems) {
		out = append(out, "mcpTools="+cmp.Or(parserkit.Text(grant.Get("server")), "-")+"/"+
			cmp.Or(parserkit.Text(grant.Get("tool")), "-")+" source="+
			cmp.Or(parserkit.Text(grant.Get("source")), "-"))
	}
	return out
}

// execDocument emits the permission rows of a decoded approvals file read
// from the row with the given rowid, and adds the problems of its members
// of the wrong kind. The socket's token is one row of its own, after the
// settings, so a file of many settings does not repeat it in each.
func (s *stateRows) execDocument(rowid int64, document jsonvalue.Value, timestamp string) {
	type scope struct {
		name   string
		policy jsonvalue.Value
	}
	const source = " (from state/openclaw.sqlite exec_approvals_config, updated_at_ms)"
	line := int(rowid)
	parserkit.ObjectMember(document, "raw_json", line, &s.problems)
	parserkit.ObjectMember(document.Get("socket"), "socket", line, &s.problems)
	// An agent's name stands in each of its settings' rows.
	carried := parserkit.NewCarried(s.artifact)
	defer carried.Report(line, &s.problems)
	scopes := []scope{{"defaults", document.Get("defaults")}}
	for _, member := range parserkit.ObjectMember(document.Get("agents"), "agents", line, &s.problems) {
		scopes = append(scopes, scope{"agents." + carried.Cut("agent name", member.Key), member.Value})
	}
	for _, each := range scopes {
		parserkit.ObjectMember(each.policy, each.name, line, &s.problems)
		for _, setting := range policySettings(each.policy, each.name, line, &s.problems) {
			s.emit(s.stateRow(s.artifact, rowid, timestamp, "permission setting: "+each.name+"."+setting+source, ""))
		}
	}
	if token := parserkit.Text(document.Get("socket").Get("token")); token != "" {
		s.emit(s.stateRow(s.artifact, rowid, timestamp, "permission setting: socket.token="+
			parserkit.SecretText(s.artifact, token)+source, ""))
	}
}

// execConfig emits one `permission setting:` row per policy setting of the
// migrated exec approvals in exec_approvals_config: each default and
// per-agent policy field, allowlist entry and MCP tool grant, then one row
// for the approvals socket's token through parserkit.SecretText when the file
// holds one; a raw_json that does not decode is reported with the reader's error
// through parserkit.ReaderErrorText, since the reader can quote that token,
// and a raw_json, socket, agents, policy, allowlist or mcpTools of the
// wrong kind adds a problem.
func (s *stateRows) execConfig(record sqlitefile.Row) {
	document, err := parserkit.DecodeJSON(s.artifact, parserkit.TextColumn(record.Get("raw_json")))
	if err != nil {
		s.problems.Add(int(record.Rowid), "exec_approvals_config raw_json unparseable ("+
			parserkit.ReaderErrorText(s.artifact, err)+")")
		return
	}
	s.execDocument(record.Rowid, document, parserkit.ColumnTime(record.Get("updated_at_ms")))
}
