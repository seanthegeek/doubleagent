package timeline

import (
	"slices"
	"strings"
)

// SessionSummary is one line of sessions.jsonl, built from a session's rows
// by Summarise.
type SessionSummary struct {
	Host              string
	User              string
	Agent             string
	SessionID         string
	ProjectPath       string
	FirstTimestampUTC string
	LastTimestampUTC  string
	// Models are the distinct models of the rows, in first-seen order.
	Models         []string
	UserTurns      int
	AssistantTurns int
	ToolCalls      int
	// SourceFile is the file that gave the most rows, the first of them
	// on a tie, so a transcript wins over a one-line-per-prompt history.
	SourceFile string
	// Synced, Relayed and Origin are "yes" ("remote" for Origin) when a
	// row carries the mark, "no" ("local") when the agent's parser records
	// the mark and none does, "" otherwise.
	Synced  string
	Relayed string
	Origin  string
	// Recovered is "yes" when every event row is recovered, "partial"
	// when some are, "no" when none are and the agent's parser reads a
	// secondary source, "" otherwise. System rows are not event rows.
	Recovered string
}

// SessionColumns returns the field names of sessions.jsonl in order.
func SessionColumns() []string {
	return []string{"host", "user", "agent", "session_id", "project_path",
		"first_timestamp_utc", "last_timestamp_utc", "models", "user_turns", "assistant_turns",
		"tool_calls", "source_file", "synced", "relayed", "origin", "recovered"}
}

// MarkAgents names, for each session mark, the agents whose parser records
// it; their sessions without the mark get "no" ("local" for Origin) where
// other agents' get "".
type MarkAgents struct {
	Synced    map[string]bool
	Relayed   map[string]bool
	Origin    map[string]bool
	Secondary map[string]bool
}

// sessionKey identifies a session: the host, user, agent and session id.
type sessionKey struct {
	host, user, agent, sessionID string
}

// sessionBuilder gathers one session's rows into its summary.
type sessionBuilder struct {
	summary       SessionSummary
	sources       map[string]int
	sourceOrder   []string
	models        map[string]bool
	eventRows     int
	recoveredRows int
	scratchPath   string
}

// Summarise returns one summary per host, user, agent and session id of
// rows, skipping rows without a session id, sorted stably by first
// timestamp, agent and session id (ties in the order the sessions first
// appear; an undated session sorts first).
func Summarise(rows []Row, marks MarkAgents) []SessionSummary {
	var builders []*sessionBuilder
	byKey := map[sessionKey]*sessionBuilder{}
	for i := range rows {
		row := &rows[i]
		if row.SessionID == "" {
			continue
		}
		key := sessionKey{row.Host, row.User, row.Agent, row.SessionID}
		builder := byKey[key]
		if builder == nil {
			builder = newSessionBuilder(row)
			byKey[key] = builder
			builders = append(builders, builder)
		}
		builder.add(row)
	}
	summaries := make([]SessionSummary, len(builders))
	for i, builder := range builders {
		summaries[i] = builder.finish(marks)
	}
	slices.SortStableFunc(summaries, compareSessions)
	return summaries
}

// compareSessions orders summaries by first timestamp, agent and session id.
func compareSessions(left, right SessionSummary) int {
	if order := strings.Compare(left.FirstTimestampUTC, right.FirstTimestampUTC); order != 0 {
		return order
	}
	if order := strings.Compare(left.Agent, right.Agent); order != 0 {
		return order
	}
	return strings.Compare(left.SessionID, right.SessionID)
}

// newSessionBuilder returns the builder of the session row belongs to.
func newSessionBuilder(row *Row) *sessionBuilder {
	return &sessionBuilder{
		summary: SessionSummary{Host: row.Host, User: row.User, Agent: row.Agent,
			SessionID: row.SessionID, Models: []string{}},
		sources: map[string]int{},
		models:  map[string]bool{},
	}
}

// add counts one row into the session.
func (b *sessionBuilder) add(row *Row) {
	summary := &b.summary
	system := row.TurnType == TurnSystem
	if !system {
		b.eventRows++
		if row.Recovered == "yes" {
			b.recoveredRows++
		}
	}
	if row.SourceFile != "" {
		if b.sources[row.SourceFile] == 0 {
			b.sourceOrder = append(b.sourceOrder, row.SourceFile)
		}
		b.sources[row.SourceFile]++
	}
	if row.TimestampUTC != "" {
		if summary.FirstTimestampUTC == "" || row.TimestampUTC < summary.FirstTimestampUTC {
			summary.FirstTimestampUTC = row.TimestampUTC
		}
		summary.LastTimestampUTC = max(summary.LastTimestampUTC, row.TimestampUTC)
	}
	if row.Model != "" && !b.models[row.Model] {
		b.models[row.Model] = true
		summary.Models = append(summary.Models, row.Model)
	}
	// A scratch directory row names the agent's directory, which is the
	// session's project only when no other row names one.
	if system && strings.HasPrefix(row.Text, ScratchPrefix) {
		if b.scratchPath == "" {
			b.scratchPath = row.ProjectPath
		}
	} else if summary.ProjectPath == "" {
		summary.ProjectPath = row.ProjectPath
	}
	b.countTurn(row)
}

// countTurn counts the row's turn type and the session marks it carries.
func (b *sessionBuilder) countTurn(row *Row) {
	summary := &b.summary
	switch {
	case row.TurnType == TurnUser:
		summary.UserTurns++
	case row.TurnType == TurnAssistant:
		summary.AssistantTurns++
	case row.TurnType == TurnToolUse:
		summary.ToolCalls++
	case row.TurnType != TurnSystem:
	case strings.HasPrefix(row.Text, SyncedPrefix):
		summary.Synced = "yes"
	case strings.HasPrefix(row.Text, RelayedPrefix):
		summary.Relayed = "yes"
	case strings.HasPrefix(row.Text, OriginPrefix):
		summary.Origin = "remote"
	}
}

// finish returns the summary with the source file chosen and the defaults
// of the marks applied.
func (b *sessionBuilder) finish(marks MarkAgents) SessionSummary {
	summary := b.summary
	most := 0
	for _, source := range b.sourceOrder {
		if count := b.sources[source]; count > most {
			summary.SourceFile, most = source, count
		}
	}
	if summary.ProjectPath == "" {
		summary.ProjectPath = b.scratchPath
	}
	agent := summary.Agent
	if summary.Synced == "" && marks.Synced[agent] {
		summary.Synced = "no"
	}
	if summary.Relayed == "" && marks.Relayed[agent] {
		summary.Relayed = "no"
	}
	if summary.Origin == "" && marks.Origin[agent] {
		summary.Origin = "local"
	}
	switch {
	case b.recoveredRows == b.eventRows && b.recoveredRows > 0:
		summary.Recovered = "yes"
	case b.recoveredRows > 0:
		summary.Recovered = "partial"
	case marks.Secondary[agent]:
		summary.Recovered = "no"
	}
	return summary
}
