package openclaw

import (
	"cmp"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/parser"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// stateRows emits the rows of the state database's tables and gathers the
// problems met.
type stateRows struct {
	*Parser
	artifact *input.Artifact
	emit     parser.Emit
	data     map[string][]sqlitefile.Row
	problems parser.Problems
}

// row emits a system row of a state table's row.
func (s *stateRows) row(record sqlitefile.Row, timestamp, text, sessionID string) {
	s.emit(s.stateRow(s.artifact, record.Rowid, timestamp, text, sessionID))
}

// emitAll emits the rows of every table in order.
func (s *stateRows) emitAll() {
	s.backups()
	for _, record := range s.data["operator_approvals"] {
		s.emit(s.approvalRow(record))
	}
	for _, record := range s.data["exec_approvals_config"] {
		s.execConfig(record)
	}
	for _, record := range s.data["operator_approval_standing_grants"] {
		s.row(record, parser.ColumnTime(record.Get("created_at_ms")), grantText(record), "")
	}
	for _, record := range s.data["channel_pairing_allow_entries"] {
		s.row(record, parser.ColumnTime(record.Get("updated_at")), "permission setting: channel "+
			orDash(columnStr(record.Get("channel_key")))+" account "+
			orDash(columnStr(record.Get("account_id")))+" allowFrom="+
			record.Get("entry").String()+
			" (from state/openclaw.sqlite channel_pairing_allow_entries, updated_at)", "")
	}
	for _, record := range s.data["channel_ingress_events"] {
		s.row(record, parser.ColumnTime(record.Get("received_at")), "channel ingress: "+
			orDash(columnStr(record.Get("channel_id")))+" account "+
			orDash(columnStr(record.Get("account_id")))+" status "+
			orDash(columnStr(record.Get("status")))+": "+record.Get("payload_json").String()+
			" (from state/openclaw.sqlite channel_ingress_events, received_at)", "")
	}
	for _, table := range githubPublicationTables() {
		for _, record := range s.data[table] {
			s.row(record, parser.ColumnTime(record.Get("created_at_ms")), publicationText(table, record),
				columnStr(record.Get("session_id")))
		}
	}
	s.projects()
}

// projects emits a `git remote:` row per project with an origin.
func (s *stateRows) projects() {
	for _, record := range s.data["projects"] {
		origin := ""
		if value := record.Get("origin_url"); !value.IsNull() {
			origin = timeline.Compact(value.String())
		}
		if origin == "" {
			continue
		}
		row := s.stateRow(s.artifact, record.Rowid, parser.ColumnTime(record.Get("created_at_ms")),
			timeline.GitRemotePrefix+origin, "")
		row.ProjectPath = columnStr(record.Get("repo_root"))
		s.emit(row)
	}
}

// backups emits a `backup uploaded:` row per successful upload or Git
// backup of backup_runs. A manifest_json that does not decode adds a
// problem.
func (s *stateRows) backups() {
	for _, run := range s.data["backup_runs"] {
		raw := columnText(run.Get("manifest_json"))
		manifest, err := jsonvalue.DecodeString(raw)
		if err != nil {
			s.problems.AddRaw(int(run.Rowid), "backup_runs manifest_json unparseable ("+err.Error()+")", raw)
			continue
		}
		if !manifest.IsObject() {
			manifest = jsonvalue.NewObject()
		}
		if text := backupText(run, manifest); text != "" {
			s.row(run, parser.ColumnTime(run.Get("created_at")), text, "")
		}
	}
}

// backupText returns the `backup uploaded:` text of a successful upload or
// Git backup, else "": a failed run, a local archive or a Git push that
// failed gives none.
func backupText(run sqlitefile.Row, manifest jsonvalue.Value) string {
	if columnText(run.Get("status")) != "ok" {
		return ""
	}
	location := manifest.Get("location")
	if !location.IsObject() {
		location = jsonvalue.NewObject()
	}
	target := manifest.Get("target")
	failed, _ := manifest.Get("pushFailed").Bool()
	pushed := isText(manifest.Get("kind"), "git") && !target.IsEmpty() && !failed
	if location.Len() == 0 && !pushed {
		return ""
	}
	return "backup uploaded: " + orDash(parser.Text(manifest.Get("kind"))) + " to " +
		orDash(cmp.Or(parser.Text(location.Get("name")), columnStr(run.Get("archive_path")))) +
		" key " + orDash(firstStr(location.Get("key"), target)) + " namespace " +
		orDash(firstStr(location.Get("namespace"), manifest.Get("namespace"))) +
		" (from state/openclaw.sqlite backup_runs, created_at)"
}

// approvalRow returns the `permission decision:` row of one
// operator_approvals row, at resolved_at_ms (created_at_ms while pending),
// joined to the call it governed on source_session_id and
// source_tool_call_id.
func (s *stateRows) approvalRow(record sqlitefile.Row) timeline.Row {
	field := "created_at_ms"
	if !record.Get("resolved_at_ms").IsNull() {
		field = "resolved_at_ms"
	}
	value := func(name string) string { return orDash(columnStr(record.Get(name))) }
	decision := orDash(cmp.Or(columnStr(record.Get("decision")), columnStr(record.Get("status"))))
	row := s.stateRow(s.artifact, record.Rowid, parser.ColumnTime(record.Get(field)), "permission decision: "+
		decision+" status="+value("status")+" kind="+value("kind")+" resolver="+
		value("resolver_kind")+" reason="+value("terminal_reason")+" approval "+
		value("approval_id")+" (from state/openclaw.sqlite operator_approvals, "+field+")",
		columnStr(record.Get("source_session_id")))
	row.ToolName = columnStr(record.Get("source_tool_name"))
	row.ToolUseID = columnStr(record.Get("source_tool_call_id"))
	return row
}

// grantText returns the `permission setting:` text of an
// operator_approval_standing_grants row: the grant an allow-always decision
// on a cron job mints.
func grantText(record sqlitefile.Row) string {
	value := func(name string) string { return orDash(columnStr(record.Get(name))) }
	uses := "-"
	if count := record.Get("use_count"); !count.IsNull() {
		uses = count.String()
	}
	return "permission setting: standing grant " + value("grant_id") +
		" allow-always cron job " + value("cron_job_id") + " agent " + value("agent_id") +
		" binding " + value("operation_binding") + " approval " + value("minted_by_approval_id") +
		" expires=" + orDash(parser.ColumnTime(record.Get("expires_at_ms"))) + " revoked=" +
		orDash(parser.ColumnTime(record.Get("revoked_at_ms"))) + " uses=" + uses +
		" (from state/openclaw.sqlite operator_approval_standing_grants, created_at_ms)"
}

// publicationText returns the `source host:` text of a GitHub publication
// request: the account the Gateway pushed or opened the pull request as,
// and what it published.
func publicationText(table string, record sqlitefile.Row) string {
	value := func(name string) string { return orDash(columnStr(record.Get(name))) }
	account := "-"
	if identifier := record.Get("identity_account_id"); !identifier.IsNull() {
		account = identifier.String()
	}
	return "source host: github account " + value("identity_login") + " id " + account +
		" identity " + value("identity_source") + "; publication " + value("status") +
		" repository " + value("repository") + " branch " + value("branch") + " pull request " +
		value("pull_request_url") + " (from state/openclaw.sqlite " + table + ", created_at_ms)"
}
