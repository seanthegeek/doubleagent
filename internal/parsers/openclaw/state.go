package openclaw

import (
	"errors"

	"github.com/seanthegeek/doubleagent/internal/input"
	"github.com/seanthegeek/doubleagent/internal/parserkit"
	"github.com/seanthegeek/doubleagent/internal/sqlitefile"
	"github.com/seanthegeek/doubleagent/internal/timeline"
)

// githubPublicationTables are the GitHub publication requests of
// state/openclaw.sqlite, each naming the GitHub account that pushed or
// opened the pull request (src/state/openclaw-state-schema.sql).
func githubPublicationTables() []string {
	return []string{"github_publication_requests", "github_personal_publication_requests",
		"github_repository_publication_requests"}
}

// stateQuery is the read of one state table: its columns and order.
type stateQuery struct {
	table   string
	columns []string
	order   []string
}

// stateQueries are the state/openclaw.sqlite tables read, in the order
// their rows are given.
func stateQueries() []stateQuery {
	queries := []stateQuery{
		{"backup_runs", []string{"id", "created_at", "archive_path", "status", "manifest_json"},
			[]string{"created_at", "id"}},
		{"operator_approvals", []string{"approval_id", "kind", "status", "decision",
			"terminal_reason", "resolver_kind", "source_session_id", "source_tool_call_id",
			"source_tool_name", "created_at_ms", "resolved_at_ms"}, []string{"created_at_ms", "rowid"}},
		{"exec_approvals_config", []string{"raw_json", "updated_at_ms"}, []string{"rowid"}},
		{"operator_approval_standing_grants", []string{"grant_id", "minted_by_approval_id",
			"agent_id", "cron_job_id", "operation_binding", "created_at_ms", "expires_at_ms",
			"revoked_at_ms", "use_count"}, []string{"created_at_ms", "rowid"}},
		{"channel_pairing_allow_entries", []string{"channel_key", "account_id", "entry", "updated_at"},
			[]string{"channel_key", "account_id", "sort_order"}},
		{"channel_ingress_events", []string{"channel_id", "account_id", "status", "payload_json",
			"received_at"}, []string{"received_at", "rowid"}},
	}
	for _, table := range githubPublicationTables() {
		queries = append(queries, stateQuery{table, []string{"session_id", "identity_source",
			"identity_account_id", "identity_login", "status", "repository", "branch",
			"pull_request_url", "created_at_ms"}, []string{"created_at_ms", "rowid"}})
	}
	return append(queries, stateQuery{"projects", []string{"repo_root", "origin_url", "created_at_ms"},
		[]string{"created_at_ms", "rowid"}})
}

// readState returns the rows of every state table the open database has,
// or the message of a PostgreSQL anchor that replaces them.
func readState(database *sqlitefile.Database) (map[string][]sqlitefile.Row, string, error) {
	names, err := parserkit.TablesOf(database)
	if err != nil {
		return nil, "", err
	}
	if !names.Has("backup_runs") && names.Has("openclaw_engine_anchor") {
		return nil, anchorMessage(database, "state"), nil
	}
	data := map[string][]sqlitefile.Row{}
	for _, query := range stateQueries() {
		if !names.Has(query.table) {
			continue
		}
		table, err := database.Table(query.table)
		if err != nil {
			return nil, "", err
		}
		if data[query.table], err = table.Select(query.columns...).OrderBy(query.order...).All(); err != nil {
			return nil, "", err
		}
	}
	return data, "", nil
}

// parseStateDatabase emits the rows of state/openclaw.sqlite: `backup
// uploaded:` from backup_runs, the permission rows of operator_approvals,
// exec_approvals_config, operator_approval_standing_grants and
// channel_pairing_allow_entries, `channel ingress:` from
// channel_ingress_events, `source host:` from the GitHub publication tables
// and `git remote:` from projects.
func (p *Parser) parseStateDatabase(artifact *input.Artifact, emit parserkit.Emit) {
	database, err := parserkit.OpenDatabase(artifact, artifact.DiskPath)
	var openErr *parserkit.OpenError
	if err != nil && !errors.As(err, &openErr) {
		message := parserkit.DatabaseError(err)
		if errors.Is(err, sqlitefile.ErrNotDatabase) {
			message = "state/openclaw.sqlite is not a SQLite database"
		}
		emit(p.ProblemRow(artifact, message))
		return
	}
	var data map[string][]sqlitefile.Row
	anchor := ""
	if err == nil {
		data, anchor, err = readState(database)
		database.Close()
	}
	switch {
	case err != nil:
		emit(p.ProblemRow(artifact, "state/openclaw.sqlite unreadable ("+err.Error()+")"))
		return
	case anchor != "":
		emit(p.ProblemRow(artifact, anchor))
		return
	}
	rows := &stateRows{Parser: p, artifact: artifact, emit: emit, data: data}
	rows.emitAll()
	parserkit.EmitAll(emit, p.ErrorRows(artifact, rows.problems, "", ""))
}

// stateRow returns a system row of the state database with the text
// compacted.
func (p *Parser) stateRow(artifact *input.Artifact, line int64, timestamp, text,
	sessionID string) timeline.Row {
	row := p.SystemRow(artifact, timeline.Compact(text))
	row.SourceLine = int(line)
	row.TimestampUTC = timestamp
	row.SessionID = sessionID
	return row
}
